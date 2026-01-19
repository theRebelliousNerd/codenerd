package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// Account represents a stored Google account for Antigravity
type Account struct {
	Email            string    `json:"email"`
	RefreshToken     string    `json:"refresh_token"`
	AccessToken      string    `json:"access_token,omitempty"`
	AccessExpiry     time.Time `json:"access_expiry,omitempty"`
	ProjectID        string    `json:"project_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	HealthScore      int       `json:"health_score"`
	LastUsed         time.Time `json:"last_used,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	ConsecutiveFails int       `json:"consecutive_fails,omitempty"`
}

// IsAccessTokenExpired checks if access token is expired (with 60s buffer)
func (a *Account) IsAccessTokenExpired() bool {
	if a.AccessToken == "" {
		return true
	}
	return time.Now().Add(60 * time.Second).After(a.AccessExpiry)
}

// HealthScoreConfig configures the health score system
type HealthScoreConfig struct {
	Initial             int     `json:"initial"`
	SuccessReward       int     `json:"success_reward"`
	RateLimitPenalty    int     `json:"rate_limit_penalty"`
	FailurePenalty      int     `json:"failure_penalty"`
	RecoveryRatePerHour float64 `json:"recovery_rate_per_hour"`
	MinUsable           int     `json:"min_usable"`
	MaxScore            int     `json:"max_score"`
}

// DefaultHealthScoreConfig returns sensible defaults
func DefaultHealthScoreConfig() HealthScoreConfig {
	return HealthScoreConfig{
		Initial:             70,
		SuccessReward:       1,
		RateLimitPenalty:    15, // Harsher penalty for 429s
		FailurePenalty:      25,
		RecoveryRatePerHour: 5, // Recover faster
		MinUsable:           30,
		MaxScore:            100,
	}
}

// AccountStore manages multiple Google accounts
type AccountStore struct {
	accountsFile string
	accounts     map[string]*Account
	config       HealthScoreConfig
	mu           sync.RWMutex
}

// NewAccountStore creates a new account store
func NewAccountStore() (*AccountStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	store := &AccountStore{
		accountsFile: filepath.Join(home, ".nerd", "antigravity_accounts.json"),
		accounts:     make(map[string]*Account),
		config:       DefaultHealthScoreConfig(),
	}

	// Load existing accounts
	_ = store.Load()

	// Migrate from old single-token file if exists and no accounts
	if len(store.accounts) == 0 {
		store.migrateFromLegacy()
	}

	return store, nil
}

// migrateFromLegacy migrates from the old single-token file
func (s *AccountStore) migrateFromLegacy() {
	home, _ := os.UserHomeDir()
	legacyFile := filepath.Join(home, ".nerd", "antigravity_tokens.json")

	data, err := os.ReadFile(legacyFile)
	if err != nil {
		return // No legacy file
	}

	var oldToken struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
		Email        string    `json:"email"`
		ProjectID    string    `json:"project_id"`
	}

	if err := json.Unmarshal(data, &oldToken); err != nil {
		return
	}

	if oldToken.Email != "" && oldToken.RefreshToken != "" {
		now := time.Now()
		account := &Account{
			Email:        oldToken.Email,
			RefreshToken: oldToken.RefreshToken,
			AccessToken:  oldToken.AccessToken,
			AccessExpiry: oldToken.Expiry,
			ProjectID:    oldToken.ProjectID,
			CreatedAt:    now,
			UpdatedAt:    now,
			HealthScore:  s.config.Initial,
		}

		s.accounts[account.Email] = account
		s.Save()

		logging.PerceptionDebug("[Antigravity] Migrated legacy token for %s", account.Email)
	}
}

// Load loads accounts from disk
func (s *AccountStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.accountsFile)
	if err != nil {
		return err
	}

	var accounts []*Account
	if err := json.Unmarshal(data, &accounts); err != nil {
		return err
	}

	s.accounts = make(map[string]*Account)
	for _, acc := range accounts {
		s.accounts[acc.Email] = acc
	}

	return nil
}

// Save saves accounts to disk
func (s *AccountStore) Save() error {
	s.mu.RLock()
	accounts := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.accountsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.accountsFile, data, 0600)
}

// AddAccount adds or updates an account
func (s *AccountStore) AddAccount(account *Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if account.Email == "" {
		return fmt.Errorf("account email is required")
	}

	now := time.Now()
	if existing, ok := s.accounts[account.Email]; ok {
		// Update existing
		existing.RefreshToken = account.RefreshToken
		existing.AccessToken = account.AccessToken
		existing.AccessExpiry = account.AccessExpiry
		existing.UpdatedAt = now
		if account.ProjectID != "" {
			existing.ProjectID = account.ProjectID
		}
	} else {
		// New account
		account.CreatedAt = now
		account.UpdatedAt = now
		if account.HealthScore == 0 {
			account.HealthScore = s.config.Initial
		}
		s.accounts[account.Email] = account
	}

	return s.saveUnlocked()
}

// GetAccount retrieves an account by email
func (s *AccountStore) GetAccount(email string) (*Account, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acc, ok := s.accounts[email]
	return acc, ok
}

// ListAccounts returns all accounts
func (s *AccountStore) ListAccounts() []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}

	// Sort by email for consistent ordering
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Email < accounts[j].Email
	})

	return accounts
}

// DeleteAccount removes an account
func (s *AccountStore) DeleteAccount(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.accounts, email)
	return s.saveUnlocked()
}

// GetEffectiveScore calculates score with time-based recovery
func (s *AccountStore) GetEffectiveScore(account *Account) int {
	hoursSinceUpdate := time.Since(account.UpdatedAt).Hours()
	recoveredPoints := int(hoursSinceUpdate * s.config.RecoveryRatePerHour)

	score := account.HealthScore + recoveredPoints
	if score > s.config.MaxScore {
		score = s.config.MaxScore
	}

	return score
}

// IsUsable checks if an account is healthy enough to use
func (s *AccountStore) IsUsable(account *Account) bool {
	return s.GetEffectiveScore(account) >= s.config.MinUsable
}

// RecordSuccess records a successful request
func (s *AccountStore) RecordSuccess(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[email]
	if !ok {
		return
	}

	score := s.GetEffectiveScore(acc) + s.config.SuccessReward
	if score > s.config.MaxScore {
		score = s.config.MaxScore
	}

	acc.HealthScore = score
	acc.LastUsed = time.Now()
	acc.UpdatedAt = time.Now()
	acc.ConsecutiveFails = 0
	acc.LastError = ""

	s.saveUnlocked()
}

// RecordRateLimit records a rate limit (429) hit
func (s *AccountStore) RecordRateLimit(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[email]
	if !ok {
		return
	}

	score := s.GetEffectiveScore(acc) - s.config.RateLimitPenalty
	if score < 0 {
		score = 0
	}

	acc.HealthScore = score
	acc.UpdatedAt = time.Now()
	acc.ConsecutiveFails++
	acc.LastError = "rate_limited"

	s.saveUnlocked()

	logging.PerceptionWarn("[Antigravity] Account %s rate limited, health: %d", email, score)
}

// RecordFailure records a general failure
func (s *AccountStore) RecordFailure(email string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[email]
	if !ok {
		return
	}

	score := s.GetEffectiveScore(acc) - s.config.FailurePenalty
	if score < 0 {
		score = 0
	}

	acc.HealthScore = score
	acc.UpdatedAt = time.Now()
	acc.ConsecutiveFails++
	acc.LastError = errMsg

	s.saveUnlocked()

	logging.PerceptionWarn("[Antigravity] Account %s failed: %s, health: %d", email, errMsg, score)
}

// UpdateToken updates the access token for an account
func (s *AccountStore) UpdateToken(email, accessToken string, expiry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[email]
	if !ok {
		return
	}

	acc.AccessToken = accessToken
	acc.AccessExpiry = expiry
	acc.UpdatedAt = time.Now()

	s.saveUnlocked()
}

// UpdateProjectID updates the project ID for an account
func (s *AccountStore) UpdateProjectID(email, projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	acc, ok := s.accounts[email]
	if !ok {
		return
	}

	acc.ProjectID = projectID
	acc.UpdatedAt = time.Now()

	s.saveUnlocked()
}

// saveUnlocked saves without acquiring lock (caller must hold lock)
func (s *AccountStore) saveUnlocked() error {
	accounts := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}

	data, err := json.MarshalIndent(accounts, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.accountsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.accountsFile, data, 0600)
}

// AccountSelector selects the best account for a request
type AccountSelector struct {
	store *AccountStore
}

// NewAccountSelector creates a new account selector
func NewAccountSelector(store *AccountStore) *AccountSelector {
	return &AccountSelector{store: store}
}

// SelectBest selects the best account based on health score and LRU
func (s *AccountSelector) SelectBest() (*Account, error) {
	accounts := s.store.ListAccounts()

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured - run 'codenerd auth antigravity' to add an account")
	}

	// Filter usable accounts and score them
	type scored struct {
		account *Account
		score   float64
	}

	var candidates []scored
	var unusable []*Account

	for _, acc := range accounts {
		if !s.store.IsUsable(acc) {
			unusable = append(unusable, acc)
			continue
		}

		healthScore := s.store.GetEffectiveScore(acc)
		secondsSinceUsed := time.Since(acc.LastUsed).Seconds()

		// Priority: health (weight 3) + freshness (weight 0.01, capped at 1hr)
		// Accounts not used recently get slight preference (LRU)
		freshnessBonus := min(secondsSinceUsed, 3600) * 0.01
		priorityScore := float64(healthScore)*3 + freshnessBonus

		// Penalize accounts with consecutive failures
		if acc.ConsecutiveFails > 0 {
			priorityScore -= float64(acc.ConsecutiveFails) * 5
		}

		candidates = append(candidates, scored{
			account: acc,
			score:   priorityScore,
		})
	}

	if len(candidates) == 0 {
		// All accounts exhausted - return the one with highest potential recovery
		if len(unusable) > 0 {
			sort.Slice(unusable, func(i, j int) bool {
				return s.store.GetEffectiveScore(unusable[i]) > s.store.GetEffectiveScore(unusable[j])
			})
			logging.PerceptionWarn("[Antigravity] All accounts exhausted, using least-damaged: %s", unusable[0].Email)
			return unusable[0], nil
		}
		return nil, fmt.Errorf("no usable accounts")
	}

	// Sort by priority score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	selected := candidates[0].account
	logging.PerceptionDebug("[Antigravity] Selected account %s (health: %d, score: %.1f)",
		selected.Email, s.store.GetEffectiveScore(selected), candidates[0].score)

	return selected, nil
}

// SelectNext selects the next best account, excluding the given email
func (s *AccountSelector) SelectNext(excludeEmail string) (*Account, error) {
	accounts := s.store.ListAccounts()

	var candidates []*Account
	for _, acc := range accounts {
		if acc.Email != excludeEmail && s.store.IsUsable(acc) {
			candidates = append(candidates, acc)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no alternative accounts available")
	}

	// Sort by health score
	sort.Slice(candidates, func(i, j int) bool {
		return s.store.GetEffectiveScore(candidates[i]) > s.store.GetEffectiveScore(candidates[j])
	})

	return candidates[0], nil
}

// GetStats returns statistics about accounts
func (s *AccountSelector) GetStats() map[string]interface{} {
	accounts := s.store.ListAccounts()

	var healthy, exhausted, total int
	for _, acc := range accounts {
		total++
		if s.store.IsUsable(acc) {
			healthy++
		} else {
			exhausted++
		}
	}

	return map[string]interface{}{
		"total":     total,
		"healthy":   healthy,
		"exhausted": exhausted,
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
