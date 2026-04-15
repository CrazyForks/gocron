package models

import (
	"time"

	"gorm.io/gorm"
)

// AuditLog records who did what and when
type AuditLog struct {
	Id         int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Username   string    `json:"username" gorm:"type:varchar(32);not null;index"`
	Ip         string    `json:"ip" gorm:"type:varchar(45);not null"`
	Module     string    `json:"module" gorm:"type:varchar(32);not null;index"` // task, host, user, system
	Action     string    `json:"action" gorm:"type:varchar(32);not null"`       // create, update, delete, enable, disable, run
	TargetId   int       `json:"target_id" gorm:"default:0"`
	TargetName string    `json:"target_name" gorm:"type:varchar(128)"`
	Detail     string    `json:"detail" gorm:"type:text"`
	CreatedAt  time.Time `json:"created" gorm:"column:created;autoCreateTime;index"`
	BaseModel  `json:"-" gorm:"-"`
}

func (log *AuditLog) Create() (insertId int, err error) {
	result := Db.Create(log)
	if result.Error == nil {
		insertId = log.Id
	}

	return insertId, result.Error
}

func (log *AuditLog) List(params CommonMap) ([]AuditLog, error) {
	log.parsePageAndPageSize(params)
	list := make([]AuditLog, 0)
	err := log.buildQuery(params).Order("id DESC").Limit(log.PageSize).Offset(log.pageLimitOffset()).Find(&list).Error

	return list, err
}

func (log *AuditLog) Total(params CommonMap) (int64, error) {
	var count int64
	err := log.buildQuery(params).Model(&AuditLog{}).Count(&count).Error

	return count, err
}

func (log *AuditLog) buildQuery(params CommonMap) *gorm.DB {
	db := Db
	if module, ok := params["Module"]; ok && module != "" {
		db = db.Where("module = ?", module)
	}
	if action, ok := params["Action"]; ok && action != "" {
		db = db.Where("action = ?", action)
	}
	if username, ok := params["Username"]; ok && username != "" {
		db = db.Where("username LIKE ?", "%"+username.(string)+"%")
	}
	if startDate, ok := params["StartDate"]; ok && startDate != "" {
		db = db.Where("created >= ?", startDate)
	}
	if endDate, ok := params["EndDate"]; ok && endDate != "" {
		db = db.Where("created <= ?", endDate)
	}

	return db
}
