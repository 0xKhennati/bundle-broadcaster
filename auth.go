package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	sessionCookieName = "metrics_session"
	sessionLifetime   = 24 * time.Hour
)

type authGuard struct {
	passwordHash   string
	maxAttempts    int
	lockoutMinutes int
	sessions       map[string]time.Time
	attempts       map[string]*attemptRecord
	mu             sync.RWMutex
}

type attemptRecord struct {
	count       int
	lockedUntil time.Time
}

func newAuthGuard(passwordHash string, maxAttempts, lockoutMinutes int) *authGuard {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if lockoutMinutes <= 0 {
		lockoutMinutes = 15
	}
	return &authGuard{
		passwordHash:   passwordHash,
		maxAttempts:    maxAttempts,
		lockoutMinutes: lockoutMinutes,
		sessions:       make(map[string]time.Time),
		attempts:       make(map[string]*attemptRecord),
	}
}

func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func (a *authGuard) verifyPassword(password string) bool {
	return md5Hex(password) == a.passwordHash
}

func (a *authGuard) isLocked(ip string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	rec, ok := a.attempts[ip]
	if !ok {
		return false
	}
	return time.Now().Before(rec.lockedUntil)
}

func (a *authGuard) recordFailedAttempt(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rec := a.attempts[ip]
	if rec == nil {
		rec = &attemptRecord{}
		a.attempts[ip] = rec
	}
	if time.Now().After(rec.lockedUntil) {
		rec.count = 0
		rec.lockedUntil = time.Time{}
	}
	rec.count++
	if rec.count >= a.maxAttempts {
		rec.lockedUntil = time.Now().Add(time.Duration(a.lockoutMinutes) * time.Minute)
	}
}

func (a *authGuard) clearAttempts(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.attempts, ip)
}

func (a *authGuard) createSession() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *authGuard) setSession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[sessionID] = time.Now().Add(sessionLifetime)
}

func (a *authGuard) isValidSession(sessionID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	expiry, ok := a.sessions[sessionID]
	if !ok || time.Now().After(expiry) {
		if ok {
			delete(a.sessions, sessionID)
		}
		return false
	}
	return true
}

func (a *authGuard) clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
