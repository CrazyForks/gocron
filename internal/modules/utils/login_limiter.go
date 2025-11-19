package utils

import (
	"sync"
	"time"
)

const (
	// MaxLoginAttempts 最大登录失败次数
	MaxLoginAttempts = 5
	// LockDuration 账户锁定时长
	LockDuration = 10 * time.Minute
	// CleanupInterval 清理过期记录的间隔
	CleanupInterval = 30 * time.Minute
)

// LoginAttempt 登录尝试记录
type LoginAttempt struct {
	Count      int       // 失败次数
	LockedUtil time.Time // 锁定到期时间
}

// LoginLimiter 登录限制器
type LoginLimiter struct {
	attempts map[string]*LoginAttempt
	mu       sync.RWMutex
}

var limiter *LoginLimiter

func init() {
	limiter = &LoginLimiter{
		attempts: make(map[string]*LoginAttempt),
	}
	// 启动定期清理
	go limiter.cleanup()
}

// GetLoginLimiter 获取登录限制器实例
func GetLoginLimiter() *LoginLimiter {
	return limiter
}

// IsLocked 检查账户是否被锁定
func (l *LoginLimiter) IsLocked(username string) (bool, time.Time) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[username]
	if !exists {
		return false, time.Time{}
	}

	// 检查是否达到最大失败次数且在锁定期内
	if attempt.Count >= MaxLoginAttempts && time.Now().Before(attempt.LockedUtil) {
		return true, attempt.LockedUtil
	}

	return false, time.Time{}
}

// RecordFailure 记录登录失败
func (l *LoginLimiter) RecordFailure(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	attempt, exists := l.attempts[username]
	if !exists {
		attempt = &LoginAttempt{Count: 0}
		l.attempts[username] = attempt
	}

	// 如果已过锁定期，重置计数
	if !attempt.LockedUtil.IsZero() && time.Now().After(attempt.LockedUtil) {
		attempt.Count = 0
		attempt.LockedUtil = time.Time{}
	}

	attempt.Count++

	// 达到最大失败次数，锁定账户
	if attempt.Count >= MaxLoginAttempts {
		attempt.LockedUtil = time.Now().Add(LockDuration)
	}
}

// RecordSuccess 记录登录成功，清除失败记录
func (l *LoginLimiter) RecordSuccess(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, username)
}

// GetRemainingAttempts 获取剩余尝试次数
func (l *LoginLimiter) GetRemainingAttempts(username string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	attempt, exists := l.attempts[username]
	if !exists {
		return MaxLoginAttempts
	}

	// 如果已过锁定期，返回最大次数
	if !attempt.LockedUtil.IsZero() && time.Now().After(attempt.LockedUtil) {
		return MaxLoginAttempts
	}

	// 如果已经被锁定，返回0
	if attempt.Count >= MaxLoginAttempts {
		return 0
	}

	remaining := MaxLoginAttempts - attempt.Count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// cleanup 定期清理过期的记录
func (l *LoginLimiter) cleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for username, attempt := range l.attempts {
			// 清理已过期的锁定记录
			if !attempt.LockedUtil.IsZero() && now.After(attempt.LockedUtil.Add(CleanupInterval)) {
				delete(l.attempts, username)
			}
		}
		l.mu.Unlock()
	}
}
