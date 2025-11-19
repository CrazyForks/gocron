package utils

import (
	"testing"
	"time"
)

func TestLoginLimiter_IsLocked(t *testing.T) {
	limiter := &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
	}

	username := "testuser"

	// 初始状态不应该被锁定
	locked, _ := limiter.IsLocked(username)
	if locked {
		t.Error("User should not be locked initially")
	}

	// 记录5次失败
	for i := 0; i < MaxLoginAttempts; i++ {
		limiter.RecordFailure(username)
	}

	// 应该被锁定
	locked, lockTime := limiter.IsLocked(username)
	if !locked {
		t.Error("User should be locked after max attempts")
	}
	if lockTime.IsZero() {
		t.Error("Lock time should be set")
	}
}

func TestLoginLimiter_RecordSuccess(t *testing.T) {
	limiter := &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
	}

	username := "testuser"

	// 记录几次失败
	limiter.RecordFailure(username)
	limiter.RecordFailure(username)

	// 记录成功，应该清除失败记录
	limiter.RecordSuccess(username)

	remaining := limiter.GetRemainingAttempts(username)
	if remaining != MaxLoginAttempts {
		t.Errorf("Expected %d remaining attempts, got %d", MaxLoginAttempts, remaining)
	}
}

func TestLoginLimiter_GetRemainingAttempts(t *testing.T) {
	limiter := &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
	}

	username := "testuser"

	// 初始应该有最大次数
	remaining := limiter.GetRemainingAttempts(username)
	if remaining != MaxLoginAttempts {
		t.Errorf("Expected %d remaining attempts, got %d", MaxLoginAttempts, remaining)
	}

	// 记录2次失败
	limiter.RecordFailure(username)
	limiter.RecordFailure(username)

	remaining = limiter.GetRemainingAttempts(username)
	expected := MaxLoginAttempts - 2
	if remaining != expected {
		t.Errorf("Expected %d remaining attempts, got %d", expected, remaining)
	}
}

func TestLoginLimiter_LockExpiration(t *testing.T) {
	limiter := &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
	}

	username := "testuser"

	// 手动设置一个已过期的锁定
	limiter.attempts[username] = &LoginAttempt{
		Count:      MaxLoginAttempts,
		LockedUtil: time.Now().Add(-1 * time.Minute), // 1分钟前过期
	}

	// 应该不再被锁定
	locked, _ := limiter.IsLocked(username)
	if locked {
		t.Error("User should not be locked after expiration")
	}

	// 剩余次数应该恢复
	remaining := limiter.GetRemainingAttempts(username)
	if remaining != MaxLoginAttempts {
		t.Errorf("Expected %d remaining attempts after expiration, got %d", MaxLoginAttempts, remaining)
	}
}
