package models

import (
	"errors"

	"github.com/gocronx-team/gocron/internal/modules/logger"
	"gorm.io/gorm"
)

type Migration struct{}

// 首次安装, 创建数据库表
func (migration *Migration) Install(dbName string) error {
	setting := new(Setting)
	tables := []interface{}{
		&User{}, &Task{}, &TaskLog{}, &Host{}, setting, &LoginLog{}, &TaskHost{},
	}

	for _, table := range tables {
		if Db.Migrator().HasTable(table) {
			return errors.New("数据表已存在")
		}
		err := Db.AutoMigrate(table)
		if err != nil {
			return err
		}
	}
	setting.InitBasicField()

	return nil
}

// 迭代升级数据库, 新建表、新增字段等
func (migration *Migration) Upgrade(oldVersionId int) {
	// v1.2版本不支持升级
	if oldVersionId == 120 {
		return
	}

	versionIds := []int{110, 122, 130, 140, 150}
	upgradeFuncs := []func(*gorm.DB) error{
		migration.upgradeFor110,
		migration.upgradeFor122,
		migration.upgradeFor130,
		migration.upgradeFor140,
		migration.upgradeFor150,
	}

	startIndex := -1
	// 从当前版本的下一版本开始升级
	for i, value := range versionIds {
		if value > oldVersionId {
			startIndex = i
			break
		}
	}

	if startIndex == -1 {
		return
	}

	length := len(versionIds)
	if startIndex >= length {
		return
	}

	err := Db.Transaction(func(tx *gorm.DB) error {
		for startIndex < length {
			err := upgradeFuncs[startIndex](tx)
			if err != nil {
				return err
			}
			startIndex++
		}
		return nil
	})

	if err != nil {
		logger.Fatal("数据库升级失败", err)
	}
}

// 升级到v1.1版本
func (migration *Migration) upgradeFor110(tx *gorm.DB) error {
	logger.Info("开始升级到v1.1")

	// 创建表task_host
	err := tx.AutoMigrate(&TaskHost{})
	if err != nil {
		return err
	}

	// 把task对应的host_id写入task_host表
	type OldTask struct {
		Id     int
		HostId int16
	}
	var results []OldTask
	err = tx.Table(TablePrefix+"task").Select("id", "host_id").Where("host_id > ?", 0).Find(&results).Error
	if err != nil {
		return err
	}

	for _, value := range results {
		taskHostModel := &TaskHost{
			TaskId: value.Id,
			HostId: value.HostId,
		}
		err = tx.Create(taskHostModel).Error
		if err != nil {
			return err
		}
	}

	// 删除task表host_id字段
	err = tx.Migrator().DropColumn(&Task{}, "host_id")

	logger.Info("已升级到v1.1\n")

	return err
}

// 升级到1.2.2版本
func (migration *Migration) upgradeFor122(tx *gorm.DB) error {
	logger.Info("开始升级到v1.2.2")

	// task表增加tag字段
	if !tx.Migrator().HasColumn(&Task{}, "tag") {
		err := tx.Migrator().AddColumn(&Task{}, "tag")
		if err != nil {
			return err
		}
	}

	logger.Info("已升级到v1.2.2\n")

	return nil
}

// 升级到v1.3版本
func (migration *Migration) upgradeFor130(tx *gorm.DB) error {
	logger.Info("开始升级到v1.3")

	// 删除user表deleted字段（如果存在）
	if tx.Migrator().HasColumn(&User{}, "deleted") {
		err := tx.Migrator().DropColumn(&User{}, "deleted")
		if err != nil {
			return err
		}
	}

	logger.Info("已升级到v1.3\n")

	return nil
}

// 升级到v1.4版本
func (migration *Migration) upgradeFor140(tx *gorm.DB) error {
	logger.Info("开始升级到v1.4")

	// task表增加字段
	// retry_interval 重试间隔时间(秒)
	// http_method    http请求方法
	if !tx.Migrator().HasColumn(&Task{}, "retry_interval") {
		err := tx.Migrator().AddColumn(&Task{}, "retry_interval")
		if err != nil {
			return err
		}
	}

	if !tx.Migrator().HasColumn(&Task{}, "http_method") {
		err := tx.Migrator().AddColumn(&Task{}, "http_method")
		if err != nil {
			return err
		}
	}

	logger.Info("已升级到v1.4\n")

	return nil
}

func (m *Migration) upgradeFor150(tx *gorm.DB) error {
	logger.Info("开始升级到v1.5")

	// task表增加字段 notify_keyword
	if !tx.Migrator().HasColumn(&Task{}, "notify_keyword") {
		err := tx.Migrator().AddColumn(&Task{}, "notify_keyword")
		if err != nil {
			return err
		}
	}

	settingModel := new(Setting)
	settingModel.Code = MailCode
	settingModel.Key = MailTemplateKey
	settingModel.Value = emailTemplate
	err := tx.Create(settingModel).Error
	if err != nil {
		return err
	}

	settingModel.Id = 0
	settingModel.Code = SlackCode
	settingModel.Key = SlackTemplateKey
	settingModel.Value = slackTemplate
	err = tx.Create(settingModel).Error
	if err != nil {
		return err
	}

	settingModel.Id = 0
	settingModel.Code = WebhookCode
	settingModel.Key = WebhookUrlKey
	settingModel.Value = ""
	err = tx.Create(settingModel).Error
	if err != nil {
		return err
	}

	settingModel.Id = 0
	settingModel.Code = WebhookCode
	settingModel.Key = WebhookTemplateKey
	settingModel.Value = webhookTemplate
	err = tx.Create(settingModel).Error
	if err != nil {
		return err
	}

	logger.Info("已升级到v1.5\n")

	return nil
}
