package models

import (
	"time"
)

// 用户登录日志
type LoginLog struct {
	Id        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Username  string    `json:"username" gorm:"type:varchar(32);not null"`
	Ip        string    `json:"ip" gorm:"type:varchar(15);not null"`
	CreatedAt time.Time `json:"created" gorm:"autoCreateTime"`
	BaseModel `json:"-" gorm:"-"`
}

func (log *LoginLog) Create() (insertId int, err error) {
	result := Db.Create(log)
	if result.Error == nil {
		insertId = log.Id
	}

	return insertId, result.Error
}

func (log *LoginLog) List(params CommonMap) ([]LoginLog, error) {
	log.parsePageAndPageSize(params)
	list := make([]LoginLog, 0)
	err := Db.Order("id DESC").Limit(log.PageSize).Offset(log.pageLimitOffset()).Find(&list).Error

	return list, err
}

func (log *LoginLog) Total() (int64, error) {
	var count int64
	err := Db.Model(&LoginLog{}).Count(&count).Error
	return count, err
}
