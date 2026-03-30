package models

import "time"

// SchedulerLock 调度器分布式锁表
// 参考 XXL-JOB 的数据库行锁方案，用 SELECT ... FOR UPDATE 实现选主
type SchedulerLock struct {
	Id        int       `gorm:"primaryKey;autoIncrement"`
	LockName  string    `gorm:"type:varchar(64);uniqueIndex;not null"` // 锁名称
	LockedBy  string    `gorm:"type:varchar(255);not null"`            // 持有者标识 (hostname:pid)
	LockedAt  time.Time `gorm:"not null"`                              // 获取锁时间
	ExpireAt  time.Time `gorm:"not null"`                              // 过期时间
	Version   int       `gorm:"not null;default:0"`                    // 乐观锁版本号
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (SchedulerLock) TableName() string {
	return TablePrefix + "scheduler_lock"
}
