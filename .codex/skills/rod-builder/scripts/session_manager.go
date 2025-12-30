package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

// Session Manager
// Reusable session management with persistence and cleanup
// Copy this into your project and customize as needed

type SessionManager struct {
	browser      *rod.Browser
	sessions     map[string]*Session
	mu           sync.RWMutex
	storePath    string
	saveInterval time.Duration
}

type Session struct {
	ID         string                     `json:"id"`
	URL        string                     `json:"url"`
	Title      string                     `json:"title"`
	CreatedAt  time.Time                  `json:"created_at"`
	LastActive time.Time                  `json:"last_active"`
	Cookies    []*proto.NetworkCookie     `json:"-"`
	page       *rod.Page
}

func NewSessionManager(browser *rod.Browser, storePath string) *SessionManager {
	sm := &SessionManager{
		browser:      browser,
		sessions:     make(map[string]*Session),
		storePath:    storePath,
		saveInterval: 30 * time.Second,
	}

	// Load persisted sessions
	sm.load()

	// Start auto-save
	go sm.autoSave()

	return sm
}

func (sm *SessionManager) CreateSession(url string) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create incognito page
	page, err := sm.browser.Incognito().Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		page.Close()
		return nil, fmt.Errorf("wait load: %w", err)
	}

	info := page.MustInfo()

	session := &Session{
		ID:         uuid.New().String(),
		URL:        info.URL,
		Title:      info.Title,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		page:       page,
	}

	// Save cookies
	session.Cookies, _ = page.Cookies([]string{})

	sm.sessions[session.ID] = session

	return session, nil
}

func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	session.LastActive = time.Now()
	return session, nil
}

func (sm *SessionManager) CloseSession(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	if session.page != nil {
		session.page.Close()
	}

	delete(sm.sessions, id)
	return nil
}

func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

func (sm *SessionManager) ForkSession(id, url string) (*Session, error) {
	sm.mu.RLock()
	source, exists := sm.sessions[id]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("source session not found: %s", id)
	}

	// Create new incognito page
	targetURL := url
	if targetURL == "" {
		targetURL = source.URL
	}

	page, err := sm.browser.Incognito().Page(proto.TargetCreateTarget{URL: targetURL})
	if err != nil {
		return nil, fmt.Errorf("create page: %w", err)
	}

	// Copy cookies
	if source.Cookies != nil {
		if err := page.SetCookies(source.Cookies); err != nil {
			page.Close()
			return nil, fmt.Errorf("set cookies: %w", err)
		}
	}

	if err := page.WaitLoad(); err != nil {
		page.Close()
		return nil, fmt.Errorf("wait load: %w", err)
	}

	info := page.MustInfo()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &Session{
		ID:         uuid.New().String(),
		URL:        info.URL,
		Title:      info.Title,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		page:       page,
	}

	session.Cookies, _ = page.Cookies([]string{})
	sm.sessions[session.ID] = session

	return session, nil
}

func (sm *SessionManager) CleanupStale(maxAge time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, session := range sm.sessions {
		if session.LastActive.Before(cutoff) {
			if session.page != nil {
				session.page.Close()
			}
			delete(sm.sessions, id)
			removed++
		}
	}

	return removed
}

func (sm *SessionManager) save() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	file, err := os.Create(sm.storePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Only save metadata (not page references)
	metadata := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		metadata = append(metadata, &Session{
			ID:         session.ID,
			URL:        session.URL,
			Title:      session.Title,
			CreatedAt:  session.CreatedAt,
			LastActive: session.LastActive,
		})
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

func (sm *SessionManager) load() error {
	data, err := os.ReadFile(sm.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved sessions
		}
		return err
	}

	var metadata []*Session
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}

	// Restore session metadata (pages not recreated)
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, session := range metadata {
		sm.sessions[session.ID] = session
	}

	return nil
}

func (sm *SessionManager) autoSave() {
	ticker := time.NewTicker(sm.saveInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := sm.save(); err != nil {
			fmt.Printf("Failed to save sessions: %v\n", err)
		}
	}
}

func (sm *SessionManager) Shutdown() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close all pages
	for _, session := range sm.sessions {
		if session.page != nil {
			session.page.Close()
		}
	}

	// Save final state
	sm.save()
}

// Example usage
func main() {
	browser := rod.New().MustConnect()
	defer browser.MustClose()

	sm := NewSessionManager(browser, "sessions.json")
	defer sm.Shutdown()

	// Create session
	session, err := sm.CreateSession("https://example.com")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Created session: %s\n", session.ID)

	// List sessions
	sessions := sm.ListSessions()
	fmt.Printf("Active sessions: %d\n", len(sessions))

	// Cleanup stale sessions
	removed := sm.CleanupStale(1 * time.Hour)
	fmt.Printf("Removed %d stale sessions\n", removed)
}
