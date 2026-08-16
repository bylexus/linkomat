package main

import (
	"sync"
	"time"
)

const (
	loginRatePerMinute = 5
	loginRateWindow    = time.Minute
)

type loginLimiter struct {
	mu     sync.Mutex
	bucket map[string]*tokenBucket
}

type tokenBucket struct {
	tokens   float64
	lastFill time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{bucket: make(map[string]*tokenBucket)}
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, ok := l.bucket[key]
	if !ok {
		l.bucket[key] = &tokenBucket{tokens: loginRatePerMinute - 1, lastFill: now}
		return true
	}
	elapsed := now.Sub(b.lastFill).Seconds()
	refill := elapsed * float64(loginRatePerMinute) / loginRateWindow.Seconds()
	b.tokens = minF(loginRatePerMinute, b.tokens+refill)
	b.lastFill = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *loginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for k, b := range l.bucket {
		if now.Sub(b.lastFill) > 10*loginRateWindow {
			delete(l.bucket, k)
		}
	}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}