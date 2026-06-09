package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/app"
	"github.com/gocronx-team/gocron/internal/modules/setting"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	"gorm.io/gorm"
)

const (
	// defaultResetUser 不传用户名时默认重置的账号
	defaultResetUser = "admin"
	// randomPasswordLength 自动生成密码的长度
	randomPasswordLength = 12
)

// resetPasswordCommand 构建 resetpwd 子命令
func resetPasswordCommand() *cli.Command {
	return &cli.Command{
		Name:      "resetpwd",
		Usage:     "reset a user's password (default: admin)",
		ArgsUsage: "[username]",
		Action:    runResetPassword,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "disable-2fa",
				Usage: "also turn off two-factor authentication for the user",
			},
		},
	}
}

// generateRandomPassword 生成随机密码
func generateRandomPassword() string {
	return utils.RandString(randomPasswordLength)
}

// resolveSQLitePath 把配置里的相对 SQLite 路径锚定到 baseDir。
// resetpwd 可能从任意目录运行，而 SQLite 相对路径默认相对当前工作目录解析，
// 会连到错误（甚至新建的空）库；锚定到 gocron 的基准目录可确保连到服务端用的同一个库。
// 非 SQLite、绝对路径或空值原样返回。
func resolveSQLitePath(engine, database, baseDir string) string {
	if !strings.EqualFold(engine, "sqlite") || database == "" || filepath.IsAbs(database) {
		return database
	}
	return filepath.Join(baseDir, database)
}

// resetUserPassword 重置指定用户名的密码（按 name 查找）。
// 该函数不含任何 I/O，仅操作传入的 db，便于独立测试。
// disable2FA 为真时一并清空 two_factor_key / two_factor_on。
// 返回重置前查到的用户记录，供调用方展示信息。
// listUsernames 返回所有用户名（按 id 排序），用于在用户不存在时给出提示。
func listUsernames(db *gorm.DB) ([]string, error) {
	var names []string
	err := db.Model(&models.User{}).Order("id").Pluck("name", &names).Error
	return names, err
}

// findUserByName 按用户名查找用户，找不到时返回明确错误。
func findUserByName(db *gorm.DB, username string) (models.User, error) {
	var user models.User
	if err := db.Where("name = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, fmt.Errorf("user [%s] not found", username)
		}
		return user, fmt.Errorf("failed to query user: %w", err)
	}
	return user, nil
}

func resetUserPassword(db *gorm.DB, username, newPassword string, disable2FA bool) (models.User, error) {
	user, err := findUserByName(db, username)
	if err != nil {
		return user, err
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return user, fmt.Errorf("failed to hash password: %w", err)
	}

	updates := map[string]interface{}{
		"password": hashed,
		"salt":     "", // bcrypt 不需要单独的 salt
	}
	if disable2FA {
		updates["two_factor_key"] = ""
		updates["two_factor_on"] = 0
	}

	if err := db.Model(&models.User{}).Where("id = ?", user.Id).UpdateColumns(updates).Error; err != nil {
		return user, fmt.Errorf("failed to update password: %w", err)
	}
	return user, nil
}

// runResetPassword 是 resetpwd 子命令的交互式入口
func runResetPassword(ctx *cli.Context) error {
	username := defaultResetUser
	if ctx.Args().Len() >= 1 {
		username = ctx.Args().Get(0)
	}
	disable2FA := ctx.Bool("disable-2fa")

	// Release mode keeps GORM's logger silent so internal SQL (e.g. the
	// "record not found" probe) isn't dumped to the operator.
	gin.SetMode(gin.ReleaseMode)

	// 仅引导配置 + DB，不启动 web / 调度器 / 选举
	app.InitEnv(AppVersion)
	if !app.Installed {
		return errors.New("gocron is not installed; cannot reset password")
	}
	config, err := setting.Read(app.AppConfig)
	if err != nil {
		return fmt.Errorf("failed to read config (%s): %w", app.AppConfig, err)
	}
	// Anchor a relative SQLite path to gocron's base directory so the tool
	// connects to the same DB the server uses, regardless of the directory
	// resetpwd was launched from.
	config.Db.Database = resolveSQLitePath(config.Db.Engine, config.Db.Database, filepath.Dir(app.AppDir))
	app.Setting = config
	if strings.EqualFold(config.Db.Engine, "sqlite") {
		fmt.Printf("Using SQLite database: %s\n", config.Db.Database)
	}
	models.Db = models.CreateDb()

	// Verify the user exists before prompting, so the operator isn't asked to
	// type a password only to find out the account doesn't exist.
	if _, err := findUserByName(models.Db, username); err != nil {
		fmt.Println(err)
		if names, lerr := listUsernames(models.Db); lerr == nil {
			if len(names) > 0 {
				fmt.Printf("Available users: %s\n", strings.Join(names, ", "))
			} else {
				fmt.Println("No users exist in this database.")
			}
		}
		return cli.Exit("", 1)
	}

	reader := bufio.NewReader(os.Stdin)

	// Confirmation prompt
	fmt.Printf("This will reset the password for user [%s]", username)
	if disable2FA {
		fmt.Print(" and disable their two-factor authentication (2FA)")
	}
	fmt.Print(". Continue? (y/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("Aborted.")
		return nil
	}

	// 读取新密码（隐藏输入；留空则随机生成）
	newPassword, generated, err := readNewPassword()
	if err != nil {
		return err
	}

	user, err := resetUserPassword(models.Db, username, newPassword, disable2FA)
	if err != nil {
		fmt.Println(err)
		return cli.Exit("", 1)
	}

	fmt.Println("--------------------------------------------------")
	fmt.Printf("Password for user [%s] has been reset successfully.\n", user.Name)
	if generated {
		fmt.Printf("New password: %s\n", newPassword)
		fmt.Println("Please keep this random password safe and change it after logging in.")
	} else {
		fmt.Println("Please log in with the new password you just entered.")
	}
	if disable2FA {
		fmt.Println("Two-factor authentication (2FA) for this user has been disabled.")
	}
	fmt.Println("--------------------------------------------------")
	return nil
}

// readNewPassword 隐藏地读取新密码；留空则自动生成随机密码。
// 返回 (密码, 是否为随机生成, 错误)。手动输入时要求二次确认。
func readNewPassword() (string, bool, error) {
	fmt.Print("Enter new password (leave blank to auto-generate a random one): ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", false, fmt.Errorf("failed to read password: %w", err)
	}
	password := strings.TrimSpace(string(first))

	if password == "" {
		return generateRandomPassword(), true, nil
	}

	fmt.Print("Re-enter the new password to confirm: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", false, fmt.Errorf("failed to read password: %w", err)
	}
	if password != strings.TrimSpace(string(second)) {
		return "", false, errors.New("passwords do not match; aborted")
	}
	return password, false, nil
}
