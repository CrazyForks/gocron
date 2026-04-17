package leader

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// LockName 调度器锁的固定名称
	LockName = "scheduler_leader"
	// LeaseDuration 租约时长，领导者需要在此时间内续约
	LeaseDuration = 15 * time.Second
	// RenewInterval 续约间隔，必须小于 LeaseDuration
	RenewInterval = 5 * time.Second
	// RetryInterval 竞选失败后重试间隔
	RetryInterval = 5 * time.Second
)

// Election 基于数据库行锁的领导者选举
type Election struct {
	db         *gorm.DB
	instanceID string // 当前实例标识
	isLeader   atomic.Bool
	stopCh     chan struct{}
	stoppedCh  chan struct{}
	onElected  func() // 当选回调
	onEvicted  func() // 失去领导权回调
	mu         sync.Mutex
}

// New 创建选举实例
func New(db *gorm.DB, onElected, onEvicted func()) *Election {
	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s:%d", hostname, os.Getpid())

	return &Election{
		db:         db,
		instanceID: instanceID,
		stopCh:     make(chan struct{}),
		stoppedCh:  make(chan struct{}),
		onElected:  onElected,
		onEvicted:  onEvicted,
	}
}

// Start 开始参与选举（非阻塞）
func (e *Election) Start() {
	go e.run()
}

// Stop 停止选举并释放领导权
func (e *Election) Stop() {
	close(e.stopCh)
	<-e.stoppedCh
}

// IsLeader 当前实例是否是领导者
func (e *Election) IsLeader() bool {
	return e.isLeader.Load()
}

// InstanceID 返回当前实例标识
func (e *Election) InstanceID() string {
	return e.instanceID
}

func (e *Election) run() {
	defer close(e.stoppedCh)

	// 确保锁表和初始记录存在
	e.ensureLockRecord()

	for {
		select {
		case <-e.stopCh:
			e.releaseLock()
			return
		default:
		}

		if e.isLeader.Load() {
			// 已经是 leader，续约
			if !e.renewLock() {
				logger.Warn("Leader lease renewal failed, stepping down")
				e.isLeader.Store(false)
				if e.onEvicted != nil {
					e.onEvicted()
				}
			}
		} else {
			// 尝试竞选
			if e.tryAcquireLock() {
				logger.Infof("This node elected as leader: %s", e.instanceID)
				e.isLeader.Store(true)
				if e.onElected != nil {
					e.onElected()
				}
			}
		}

		// 等待下一次循环
		interval := RetryInterval
		if e.isLeader.Load() {
			interval = RenewInterval
		}

		select {
		case <-e.stopCh:
			e.releaseLock()
			return
		case <-time.After(interval):
		}
	}
}

// ensureLockRecord 确保锁记录存在
func (e *Election) ensureLockRecord() {
	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "",
		LockedAt: time.Time{},
		ExpireAt: time.Time{},
	}
	// 如果记录不存在则创建
	e.db.Where("lock_name = ?", LockName).FirstOrCreate(&lock)
}

// tryAcquireLock 尝试获取锁（FOR UPDATE + 检查过期）
func (e *Election) tryAcquireLock() bool {
	now := time.Now()
	result := e.db.Transaction(func(tx *gorm.DB) error {
		var lock models.SchedulerLock

		// SELECT ... FOR UPDATE 行锁
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("lock_name = ?", LockName).
			First(&lock).Error
		if err != nil {
			return err
		}

		// 锁被其他实例持有且未过期
		if lock.LockedBy != "" && lock.LockedBy != e.instanceID && lock.ExpireAt.After(now) {
			return fmt.Errorf("lock held by %s until %s", lock.LockedBy, lock.ExpireAt)
		}

		// 锁空闲或已过期，获取锁
		err = tx.Model(&lock).Updates(map[string]interface{}{
			"locked_by": e.instanceID,
			"locked_at": now,
			"expire_at": now.Add(LeaseDuration),
			"version":   lock.Version + 1,
		}).Error
		return err
	})

	return result == nil
}

// renewLock 续约（只有当前持有者才能续约）
func (e *Election) renewLock() bool {
	now := time.Now()
	result := e.db.Model(&models.SchedulerLock{}).
		Where("lock_name = ? AND locked_by = ?", LockName, e.instanceID).
		Updates(map[string]interface{}{
			"expire_at": now.Add(LeaseDuration),
			"locked_at": now,
		})

	if result.Error != nil {
		logger.Errorf("Failed to renew leader lease: %v", result.Error)
		return false
	}
	return result.RowsAffected > 0
}

// releaseLock 主动释放锁
func (e *Election) releaseLock() {
	if !e.isLeader.Load() {
		return
	}

	logger.Infof("Releasing leader lock: %s", e.instanceID)
	e.db.Model(&models.SchedulerLock{}).
		Where("lock_name = ? AND locked_by = ?", LockName, e.instanceID).
		Updates(map[string]interface{}{
			"locked_by": "",
			"expire_at": time.Time{},
		})

	e.isLeader.Store(false)
	if e.onEvicted != nil {
		e.onEvicted()
	}
}
