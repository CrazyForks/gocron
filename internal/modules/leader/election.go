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

// epochSentinel is a DATETIME-safe "never held" marker.
// MySQL DATETIME range is 1000-01-01 .. 9999-12-31; Go's time.Time{}
// zero value (0001-01-01) is rejected by MySQL strict mode
// (NO_ZERO_DATE + STRICT_TRANS_TABLES, default in MySQL 5.7+).
var epochSentinel = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

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

	// Ensure the lock row exists. If the DB rejects the initial insert
	// (e.g. MySQL strict mode + transient driver issue), retry with
	// backoff rather than exiting — this matches the rest of the loop's
	// self-healing semantics.
	for {
		if err := e.ensureLockRecord(); err != nil {
			logger.Warnf("Failed to ensure scheduler_lock row, retrying in %s: %v", RetryInterval, err)
			select {
			case <-e.stopCh:
				return
			case <-time.After(RetryInterval):
				continue
			}
		}
		break
	}

	for {
		select {
		case <-e.stopCh:
			e.releaseLock()
			return
		default:
		}

		if e.isLeader.Load() {
			if !e.renewLock() {
				logger.Warn("Leader lease renewal failed, stepping down")
				e.isLeader.Store(false)
				if e.onEvicted != nil {
					e.onEvicted()
				}
			}
		} else {
			acquired, err := e.tryAcquireLock()
			if acquired {
				logger.Infof("This node elected as leader: %s", e.instanceID)
				e.isLeader.Store(true)
				if e.onElected != nil {
					e.onElected()
				}
			} else if err != nil {
				logger.Warnf("Leader election attempt failed: %v", err)
			}
		}

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

// ensureLockRecord ensures the lock row exists. Safe to call repeatedly.
// Returns an error if the row cannot be created or read; the caller is
// expected to retry rather than treat this as fatal, because the DB may
// be transiently unreachable at startup.
func (e *Election) ensureLockRecord() error {
	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "",
		LockedAt: epochSentinel,
		ExpireAt: epochSentinel,
	}
	if err := e.db.Where("lock_name = ?", LockName).
		Attrs(models.SchedulerLock{
			LockedBy: "",
			LockedAt: epochSentinel,
			ExpireAt: epochSentinel,
		}).
		FirstOrCreate(&lock).Error; err != nil {
		return fmt.Errorf("ensure scheduler_lock row: %w", err)
	}
	return nil
}

// tryAcquireLock attempts to grab the lock via SELECT ... FOR UPDATE.
// Returns (true, nil) on success, (false, nil) when the lock is legitimately
// held by another live instance, and (false, err) when an unexpected error
// occurred (DB connectivity, missing row, etc.). Callers should log/observe
// the error case; the held-by-other-instance case is normal and quiet.
func (e *Election) tryAcquireLock() (bool, error) {
	now := time.Now()
	var heldByOther bool

	err := e.db.Transaction(func(tx *gorm.DB) error {
		var lock models.SchedulerLock

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("lock_name = ?", LockName).
			First(&lock).Error; err != nil {
			return err
		}

		// Order of conjuncts matters: LockedBy=="" must short-circuit before
		// ExpireAt is evaluated, so a freshly-inserted row with the sentinel
		// epoch never reaches the time comparison.
		if lock.LockedBy != "" && lock.LockedBy != e.instanceID && lock.ExpireAt.After(now) {
			heldByOther = true
			return fmt.Errorf("lock held by %s until %s", lock.LockedBy, lock.ExpireAt)
		}

		return tx.Model(&lock).Updates(map[string]interface{}{
			"locked_by": e.instanceID,
			"locked_at": now,
			"expire_at": now.Add(LeaseDuration),
			"version":   lock.Version + 1,
		}).Error
	})

	if err != nil {
		if heldByOther {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
	// Use epochSentinel rather than time.Time{} — MySQL strict mode rejects
	// 0001-01-01 and would silently fail this UPDATE, leaving the lock held
	// by the stopped instance for the full LeaseDuration (slowing failover).
	if err := e.db.Model(&models.SchedulerLock{}).
		Where("lock_name = ? AND locked_by = ?", LockName, e.instanceID).
		Updates(map[string]interface{}{
			"locked_by": "",
			"expire_at": epochSentinel,
		}).Error; err != nil {
		logger.Errorf("Failed to release leader lock: %v", err)
	}

	e.isLeader.Store(false)
	if e.onEvicted != nil {
		e.onEvicted()
	}
}
