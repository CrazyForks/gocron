package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

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

// resetUserPassword 重置指定用户名的密码（按 name 查找）。
// 该函数不含任何 I/O，仅操作传入的 db，便于独立测试。
// disable2FA 为真时一并清空 two_factor_key / two_factor_on。
// 返回重置前查到的用户记录，供调用方展示信息。
func resetUserPassword(db *gorm.DB, username, newPassword string, disable2FA bool) (models.User, error) {
	var user models.User
	if err := db.Where("name = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, fmt.Errorf("找不到用户 [%s]", username)
		}
		return user, fmt.Errorf("查询用户失败: %w", err)
	}

	hashed, err := utils.HashPassword(newPassword)
	if err != nil {
		return user, fmt.Errorf("密码哈希失败: %w", err)
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
		return user, fmt.Errorf("更新密码失败: %w", err)
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

	// 仅引导配置 + DB，不启动 web / 调度器 / 选举
	app.InitEnv(AppVersion)
	if !app.Installed {
		return errors.New("gocron 尚未安装，无法重置密码")
	}
	config, err := setting.Read(app.AppConfig)
	if err != nil {
		return fmt.Errorf("读取配置失败 (%s): %w", app.AppConfig, err)
	}
	app.Setting = config
	models.Db = models.CreateDb()

	reader := bufio.NewReader(os.Stdin)

	// 二次确认
	fmt.Printf("此操作将重置用户 [%s] 的密码", username)
	if disable2FA {
		fmt.Print("，并关闭其两步验证(2FA)")
	}
	fmt.Print("，是否继续? (y/N): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		fmt.Println("操作已取消。")
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
		fmt.Println("提示: 请确认连接的是正确的数据库 (conf/app.ini)。")
		return err
	}

	fmt.Println("--------------------------------------------------")
	fmt.Printf("用户 [%s] 密码已重置成功。\n", user.Name)
	if generated {
		fmt.Printf("新密码: %s\n", newPassword)
		fmt.Println("请妥善保管该随机密码，登录后请及时修改。")
	} else {
		fmt.Println("请使用你刚才输入的新密码登录。")
	}
	if disable2FA {
		fmt.Println("该用户的两步验证(2FA)已关闭。")
	}
	fmt.Println("--------------------------------------------------")
	return nil
}

// readNewPassword 隐藏地读取新密码；留空则自动生成随机密码。
// 返回 (密码, 是否为随机生成, 错误)。手动输入时要求二次确认。
func readNewPassword() (string, bool, error) {
	fmt.Print("请输入新密码 (留空则自动生成随机密码): ")
	first, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", false, fmt.Errorf("读取密码失败: %w", err)
	}
	password := strings.TrimSpace(string(first))

	if password == "" {
		return generateRandomPassword(), true, nil
	}

	fmt.Print("请再次输入新密码以确认: ")
	second, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", false, fmt.Errorf("读取密码失败: %w", err)
	}
	if password != strings.TrimSpace(string(second)) {
		return "", false, errors.New("两次输入的密码不一致，操作已取消")
	}
	return password, false, nil
}
