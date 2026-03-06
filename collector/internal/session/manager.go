package session

import (
	"sync"
	"time"
)

type Manager struct {
	mu            sync.Mutex
	lastSeen      map[string]time.Time
	sessionWindow time.Duration
}

func NewManager(sessionWindow time.Duration) *Manager {
	return &Manager{lastSeen: make(map[string]time.Time), sessionWindow: sessionWindow}
}

// Touch tracks activity and returns true when the session should be considered a continuation.
func (m *Manager) Touch(sessionID string, now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	last, ok := m.lastSeen[sessionID]
	m.lastSeen[sessionID] = now
	if !ok {
		return false
	}
	return now.Sub(last) <= m.sessionWindow
}
