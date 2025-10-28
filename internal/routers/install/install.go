package install

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/app"
	"github.com/gocronx-team/gocron/internal/modules/setting"
	"github.com/gocronx-team/gocron/internal/modules/utils"
	"github.com/gocronx-team/gocron/internal/service"
)

// 系统安装

type InstallForm struct {
	DbType               string `form:"db_type" binding:"required,oneof=mysql postgres"`
	DbHost               string `form:"db_host" binding:"required,max=50"`
	DbPort               int    `form:"db_port" binding:"required,min=1,max=65535"`
	DbUsername           string `form:"db_username" binding:"required,max=50"`
	DbPassword           string `form:"db_password" binding:"required,max=30"`
	DbName               string `form:"db_name" binding:"required,max=50"`
	DbTablePrefix        string `form:"db_table_prefix" binding:"max=20"`
	AdminUsername        string `form:"admin_username" binding:"required,min=3"`
	AdminPassword        string `form:"admin_password" binding:"required,min=6"`
	ConfirmAdminPassword string `form:"confirm_admin_password" binding:"required,min=6"`
	AdminEmail           string `form:"admin_email" binding:"required,email,max=50"`
}



// 安装
func Store(c *gin.Context) {
	var form InstallForm
	if err := c.ShouldBind(&form); err != nil {
		json := utils.JsonResponse{}
		result := json.CommonFailure("表单验证失败, 请检测输入")
		c.String(http.StatusOK, result)
		return
	}
	
	json := utils.JsonResponse{}
	var result string
	if app.Installed {
		result = json.CommonFailure("系统已安装!")
		c.String(http.StatusOK, result)
		return
	}
	if form.AdminPassword != form.ConfirmAdminPassword {
		result = json.CommonFailure("两次输入密码不匹配")
		c.String(http.StatusOK, result)
		return
	}
	err := testDbConnection(form)
	if err != nil {
		result = json.CommonFailure(err.Error())
		c.String(http.StatusOK, result)
		return
	}
	// 写入数据库配置
	err = writeConfig(form)
	if err != nil {
		result = json.CommonFailure("数据库配置写入文件失败", err)
		c.String(http.StatusOK, result)
		return
	}

	appConfig, err := setting.Read(app.AppConfig)
	if err != nil {
		result = json.CommonFailure("读取应用配置失败", err)
		c.String(http.StatusOK, result)
		return
	}
	app.Setting = appConfig

	models.Db = models.CreateDb()
	// 创建数据库表
	migration := new(models.Migration)
	err = migration.Install(form.DbName)
	if err != nil {
		result = json.CommonFailure(fmt.Sprintf("创建数据库表失败-%s", err.Error()), err)
		c.String(http.StatusOK, result)
		return
	}

	// 创建管理员账号
	err = createAdminUser(form)
	if err != nil {
		result = json.CommonFailure("创建管理员账号失败", err)
		c.String(http.StatusOK, result)
		return
	}

	// 创建安装锁
	err = app.CreateInstallLock()
	if err != nil {
		result = json.CommonFailure("创建文件安装锁失败", err)
		c.String(http.StatusOK, result)
		return
	}

	// 更新版本号文件
	app.UpdateVersionFile()

	// 标记为已安装
	app.Installed = true
	// 初始化定时任务
	service.ServiceTask.Initialize()

	result = json.Success("安装成功", nil)
	c.String(http.StatusOK, result)
}

// 配置写入文件
func writeConfig(form InstallForm) error {
	dbConfig := []string{
		"db.engine", form.DbType,
		"db.host", form.DbHost,
		"db.port", strconv.Itoa(form.DbPort),
		"db.user", form.DbUsername,
		"db.password", form.DbPassword,
		"db.database", form.DbName,
		"db.prefix", form.DbTablePrefix,
		"db.charset", "utf8",
		"db.max.idle.conns", "5",
		"db.max.open.conns", "100",
		"allow_ips", "",
		"app.name", "定时任务管理系统", // 应用名称
		"api.key", "",
		"api.secret", "",
		"enable_tls", "false",
		"concurrency.queue", "500",
		"auth_secret", utils.RandAuthToken(),
		"ca_file", "",
		"cert_file", "",
		"key_file", "",
	}

	return setting.Write(dbConfig, app.AppConfig)
}

// 创建管理员账号
func createAdminUser(form InstallForm) error {
	user := new(models.User)
	user.Name = form.AdminUsername
	user.Password = form.AdminPassword
	user.Email = form.AdminEmail
	user.IsAdmin = 1
	_, err := user.Create()

	return err
}

// 测试数据库连接
func testDbConnection(form InstallForm) error {
	var s setting.Setting
	s.Db.Engine = form.DbType
	s.Db.Host = form.DbHost
	s.Db.Port = form.DbPort
	s.Db.User = form.DbUsername
	s.Db.Password = form.DbPassword
	s.Db.Database = form.DbName
	s.Db.Charset = "utf8"
	db, err := models.CreateTmpDb(&s)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	err = sqlDB.Ping()
	if s.Db.Engine == "postgres" && err != nil {
		pgError, ok := err.(*pq.Error)
		if ok && pgError.Code == "3D000" {
			err = errors.New("数据库不存在")
		}
		return err
	}

	if s.Db.Engine == "mysql" && err != nil {
		mysqlError, ok := err.(*mysql.MySQLError)
		if ok && mysqlError.Number == 1049 {
			err = errors.New("数据库不存在")
		}
		return err
	}

	return err

}
