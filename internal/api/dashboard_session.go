package api

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	dashboardSessionCookie = "orivis_dashboard_session"
	dashboardSessionTTL    = 12 * time.Hour
)

type dashboardSessionStore struct {
	mu       sync.RWMutex
	ttl      time.Duration
	sessions map[string]dashboardSession
}

type dashboardSession struct {
	Username  string
	ExpiresAt time.Time
}

func newDashboardSessionStore(ttl time.Duration) *dashboardSessionStore {
	return &dashboardSessionStore{
		ttl:      ttl,
		sessions: make(map[string]dashboardSession),
	}
}

func (s *dashboardSessionStore) Create(username string) (string, time.Time, error) {
	if s == nil {
		return "", time.Time{}, errors.New("dashboard session store is nil")
	}
	token, err := newDashboardSessionToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := time.Now().UTC().Add(s.ttl)
	s.mu.Lock()
	s.sessions[token] = dashboardSession{
		Username:  strings.TrimSpace(username),
		ExpiresAt: expiresAt,
	}
	s.mu.Unlock()
	return token, expiresAt, nil
}

func (s *dashboardSessionStore) Authenticate(token string) bool {
	token = strings.TrimSpace(token)
	if s == nil || token == "" {
		return false
	}

	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		s.Delete(token)
		return false
	}
	return true
}

func (s *dashboardSessionStore) Delete(token string) {
	token = strings.TrimSpace(token)
	if s == nil || token == "" {
		return
	}
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func newDashboardSessionToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate dashboard session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
