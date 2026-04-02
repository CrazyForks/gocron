package leader

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gocronx-team/gocron/internal/models"
	"github.com/gocronx-team/gocron/internal/modules/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	logger.InitLogger()
	os.Exit(m.Run())
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// SQLite in-memory DB is per-connection; force single connection
	// so all goroutines share the same schema and data
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&models.SchedulerLock{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestElection_SingleNode_BecomesLeader(t *testing.T) {
	db := setupTestDB(t)

	elected := make(chan struct{}, 1)
	e := New(db, func() { elected <- struct{}{} }, nil)
	e.Start()
	defer e.Stop()

	select {
	case <-elected:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting to become leader")
	}

	if !e.IsLeader() {
		t.Error("expected IsLeader() to be true")
	}
}

func TestElection_Stop_ReleasesLock(t *testing.T) {
	db := setupTestDB(t)

	elected := make(chan struct{}, 1)
	evicted := make(chan struct{}, 1)
	e := New(db, func() { elected <- struct{}{} }, func() { evicted <- struct{}{} })
	e.Start()

	<-elected
	e.Stop()

	if e.IsLeader() {
		t.Error("expected IsLeader() to be false after Stop")
	}

	// Verify lock is released in DB
	var lock models.SchedulerLock
	db.Where("lock_name = ?", LockName).First(&lock)
	if lock.LockedBy != "" {
		t.Errorf("expected empty locked_by after stop, got %q", lock.LockedBy)
	}
}

func TestElection_TwoNodes_OnlyOneLeader(t *testing.T) {
	db := setupTestDB(t)

	var mu sync.Mutex
	leaderCount := 0

	makeElection := func() *Election {
		return New(db,
			func() {
				mu.Lock()
				leaderCount++
				mu.Unlock()
			},
			func() {
				mu.Lock()
				leaderCount--
				mu.Unlock()
			},
		)
	}

	e1 := makeElection()
	e2 := makeElection()
	// Give them different instance IDs
	e1.instanceID = "node1:1000"
	e2.instanceID = "node2:2000"

	e1.Start()
	time.Sleep(2 * time.Second)
	e2.Start()
	time.Sleep(2 * time.Second)

	// Exactly one should be leader
	if e1.IsLeader() == e2.IsLeader() {
		t.Errorf("expected exactly one leader: e1=%v e2=%v", e1.IsLeader(), e2.IsLeader())
	}

	mu.Lock()
	count := leaderCount
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected leaderCount=1, got %d", count)
	}

	e1.Stop()
	e2.Stop()
}

func TestElection_Failover(t *testing.T) {
	db := setupTestDB(t)

	elected1 := make(chan struct{}, 1)
	e1 := New(db, func() { elected1 <- struct{}{} }, nil)
	e1.instanceID = "node1:1000"
	e1.Start()

	select {
	case <-elected1:
	case <-time.After(3 * time.Second):
		t.Fatal("e1 timed out becoming leader")
	}

	elected2 := make(chan struct{}, 1)
	e2 := New(db, func() { elected2 <- struct{}{} }, nil)
	e2.instanceID = "node2:2000"
	e2.Start()

	// e1 stops — e2 should take over
	e1.Stop()

	select {
	case <-elected2:
		// ok, e2 became leader
	case <-time.After(10 * time.Second):
		t.Fatal("e2 timed out becoming leader after e1 stopped")
	}

	if !e2.IsLeader() {
		t.Error("expected e2 to be leader after e1 stopped")
	}

	e2.Stop()
}

func TestElection_InstanceID(t *testing.T) {
	db := setupTestDB(t)
	e := New(db, nil, nil)
	if e.InstanceID() == "" {
		t.Error("expected non-empty InstanceID")
	}
}

func TestElection_EnsureLockRecord_CreatesRow(t *testing.T) {
	db := setupTestDB(t)
	e := New(db, nil, nil)

	// No rows initially
	var count int64
	db.Model(&models.SchedulerLock{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 rows, got %d", count)
	}

	e.ensureLockRecord()

	db.Model(&models.SchedulerLock{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 row after ensureLockRecord, got %d", count)
	}

	// Calling again should not create duplicate
	e.ensureLockRecord()
	db.Model(&models.SchedulerLock{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected still 1 row after second call, got %d", count)
	}
}

func TestElection_TryAcquireLock_ExpiredLock(t *testing.T) {
	db := setupTestDB(t)

	// Insert an expired lock held by another node
	expired := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "old-node:999",
		LockedAt: time.Now().Add(-1 * time.Hour),
		ExpireAt: time.Now().Add(-30 * time.Minute), // expired
	}
	db.Create(&expired)

	e := New(db, nil, nil)
	e.instanceID = "new-node:1000"

	// Should succeed because lock is expired
	if !e.tryAcquireLock() {
		t.Error("expected to acquire expired lock")
	}

	// Verify DB updated
	var lock models.SchedulerLock
	db.Where("lock_name = ?", LockName).First(&lock)
	if lock.LockedBy != "new-node:1000" {
		t.Errorf("expected locked_by=%q, got %q", "new-node:1000", lock.LockedBy)
	}
}

func TestElection_TryAcquireLock_ActiveLockBlocks(t *testing.T) {
	db := setupTestDB(t)

	// Insert an active lock held by another node
	active := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "other-node:999",
		LockedAt: time.Now(),
		ExpireAt: time.Now().Add(1 * time.Hour), // not expired
	}
	db.Create(&active)

	e := New(db, nil, nil)
	e.instanceID = "my-node:1000"

	// Should fail because lock is active
	if e.tryAcquireLock() {
		t.Error("expected to fail acquiring active lock")
	}
}

func TestElection_RenewLock_Success(t *testing.T) {
	db := setupTestDB(t)

	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "my-node:1000",
		LockedAt: time.Now(),
		ExpireAt: time.Now().Add(10 * time.Second),
	}
	db.Create(&lock)

	e := New(db, nil, nil)
	e.instanceID = "my-node:1000"

	if !e.renewLock() {
		t.Error("expected renewLock to succeed")
	}

	var updated models.SchedulerLock
	db.Where("lock_name = ?", LockName).First(&updated)
	if updated.ExpireAt.Before(lock.ExpireAt) {
		t.Error("expected expire_at to be extended")
	}
}

func TestElection_RenewLock_FailsWhenNotOwner(t *testing.T) {
	db := setupTestDB(t)

	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "other-node:999",
		LockedAt: time.Now(),
		ExpireAt: time.Now().Add(10 * time.Second),
	}
	db.Create(&lock)

	e := New(db, nil, nil)
	e.instanceID = "my-node:1000"

	if e.renewLock() {
		t.Error("expected renewLock to fail when not owner")
	}
}

func TestElection_ReleaseLock_OnlyWhenLeader(t *testing.T) {
	db := setupTestDB(t)

	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "other-node:999",
		LockedAt: time.Now(),
		ExpireAt: time.Now().Add(1 * time.Hour),
	}
	db.Create(&lock)

	e := New(db, nil, nil)
	e.instanceID = "my-node:1000"
	// isLeader is false by default

	e.releaseLock() // should be a no-op

	var result models.SchedulerLock
	db.Where("lock_name = ?", LockName).First(&result)
	if result.LockedBy != "other-node:999" {
		t.Errorf("expected lock still held by other-node, got %q", result.LockedBy)
	}
}

func TestElection_NilCallbacks(t *testing.T) {
	db := setupTestDB(t)

	// Should not panic with nil onElected/onEvicted
	e := New(db, nil, nil)
	e.Start()

	time.Sleep(1 * time.Second)
	if !e.IsLeader() {
		t.Error("expected to become leader")
	}

	e.Stop()
	if e.IsLeader() {
		t.Error("expected not to be leader after stop")
	}
}

func TestElection_ReacquireOwnLock(t *testing.T) {
	db := setupTestDB(t)

	// Lock held by the same instance (e.g. after restart with same hostname:pid)
	lock := models.SchedulerLock{
		LockName: LockName,
		LockedBy: "my-node:1000",
		LockedAt: time.Now(),
		ExpireAt: time.Now().Add(1 * time.Hour),
	}
	db.Create(&lock)

	e := New(db, nil, nil)
	e.instanceID = "my-node:1000"

	// Should succeed — same instance can reacquire
	if !e.tryAcquireLock() {
		t.Error("expected to reacquire own lock")
	}
}
