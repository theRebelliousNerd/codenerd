package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// QuotaKey represents different quota pools
type QuotaKey string

const (
	QuotaClaude            QuotaKey = "claude"
	QuotaGeminiAntigravity QuotaKey = "gemini-antigravity"
	QuotaGeminiCLI         QuotaKey = "gemini-cli"
)

// Account represents a stored Google account for Antigravity
type Account struct {
	Index            int       `json:"index"`
	Email            string    `json:"email"`
	RefreshToken     string    `json:"refreshToken"`
	AccessToken      string    `json:"accessToken,omitempty"`
	AccessExpiry     time.Time `json:"-"`
	ProjectID        string    `json:"projectId,omitempty"`
	ManagedProjectID string    `json:"managedProjectId,omitempty"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	LastUsed  time.Time `json:"-"`

	// Rate Limiting & Cooldowns
	RateLimitResetTimes map[string]time.Time `json:"-"`
	CoolingDownUntil    time.Time            `json:"-"`
	CooldownReason      string               `json:"cooldownReason,omitempty"`
	ConsecutiveFailures int                  `json:"consecutiveFailures,omitempty"`
}

// Custom JSON marshalling to match TypeScript milliseconds timestamps
type accountJSON struct {
	Account
	AccessExpiry        int64              `json:"accessExpiry,omitempty"`
	AddedAt             int64              `json:"addedAt"`
	UpdatedAt           int64              `json:"updatedAt"`
	LastUsed            int64              `json:"lastUsed,omitempty"`
	RateLimitResetTimes map[string]float64 `json:"rateLimitResetTimes,omitempty"`
	CoolingDownUntil    int64              `json:"coolingDownUntil,omitempty"`
}

func (a *Account) MarshalJSON() ([]byte, error) {
	resetTimes := make(map[string]float64)
	for k, v := range a.RateLimitResetTimes {
		resetTimes[k] = float64(v.UnixMilli())
	}

	return json.Marshal(accountJSON{
		Account:             *a,
		AccessExpiry:        a.AccessExpiry.UnixMilli(),
		AddedAt:             a.CreatedAt.UnixMilli(),
		UpdatedAt:           a.UpdatedAt.UnixMilli(),
		LastUsed:            a.LastUsed.UnixMilli(),
		RateLimitResetTimes: resetTimes,
		CoolingDownUntil:    a.CoolingDownUntil.UnixMilli(),
	})
}

func (a *Account) UnmarshalJSON(data []byte) error {
	var aux accountJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*a = aux.Account
	if aux.AccessExpiry > 0 {
		a.AccessExpiry = time.UnixMilli(aux.AccessExpiry)
	}
	if aux.AddedAt > 0 {
		a.CreatedAt = time.UnixMilli(aux.AddedAt)
	}
	if aux.UpdatedAt > 0 {
		a.UpdatedAt = time.UnixMilli(aux.UpdatedAt)
	}
	if aux.LastUsed > 0 {
		a.LastUsed = time.UnixMilli(aux.LastUsed)
	}
	if aux.CoolingDownUntil > 0 {
		a.CoolingDownUntil = time.UnixMilli(aux.CoolingDownUntil)
	}

	a.RateLimitResetTimes = make(map[string]time.Time)
	for k, v := range aux.RateLimitResetTimes {
		a.RateLimitResetTimes[k] = time.UnixMilli(int64(v))
	}

	return nil
}

// IsAccessTokenExpired checks if access token is expired (with 60s buffer)
func (a *Account) IsAccessTokenExpired() bool {
	if a.AccessToken == "" {
		return true
	}
	return time.Now().Add(60 * time.Second).After(a.AccessExpiry)
}

// IsRateLimited checks if the account is rate limited for a specific quota
func (a *Account) IsRateLimited(quotaKey string) bool {
	if a.RateLimitResetTimes == nil {
		return false
	}
	resetTime, ok := a.RateLimitResetTimes[quotaKey]
	if !ok {
		return false
	}
	if time.Now().After(resetTime) {
		delete(a.RateLimitResetTimes, quotaKey)
		return false
	}
	return true
}

// AccountStorageV3 represents the disk format
type AccountStorageV3 struct {
	Version             int            `json:"version"`
	Accounts            []*Account     `json:"accounts"`
	ActiveIndex         int            `json:"activeIndex"`
	ActiveIndexByFamily map[string]int `json:"activeIndexByFamily"`
}

// AccountManager manages multiple Google accounts with rotation logic
type AccountManager struct {
	filePath            string
	accounts            []*Account
	activeIndex         int
	activeIndexByFamily map[string]int

	healthTracker *HealthTracker
	tokenTracker  *TokenTracker

	mu sync.RWMutex
}

// NewAccountManager creates a new account manager
func NewAccountManager() (*AccountManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	am := &AccountManager{
		filePath:            filepath.Join(home, ".nerd", "antigravity_accounts.json"),
		accounts:            make([]*Account, 0),
		activeIndexByFamily: make(map[string]int),
		// Initialize trackers with defaults
		healthTracker: NewHealthTracker(DefaultHealthScoreConfig()),
		tokenTracker:  NewTokenTracker(100, 10.0, 100), // Example: 100 tokens, 10/min regen
	}

	// Load existing accounts
	if err := am.Load(); err != nil {
		// Ignore load error if file doesn't exist, just start empty
		if !os.IsNotExist(err) {
			logging.PerceptionWarn("[Antigravity] Failed to load accounts: %v", err)
		}
	}

	return am, nil
}

// Load loads accounts from disk
func (am *AccountManager) Load() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(am.filePath)
	if err != nil {
		return err
	}

	// Try V3 format
	var storage AccountStorageV3
	if err := json.Unmarshal(data, &storage); err == nil && storage.Version == 3 {
		am.accounts = storage.Accounts
		am.activeIndex = storage.ActiveIndex
		am.activeIndexByFamily = storage.ActiveIndexByFamily
		if am.activeIndexByFamily == nil {
			am.activeIndexByFamily = make(map[string]int)
		}
		// Re-index accounts
		for i, acc := range am.accounts {
			acc.Index = i
			if acc.RateLimitResetTimes == nil {
				acc.RateLimitResetTimes = make(map[string]time.Time)
			}
		}
		return nil
	}

	// Fallback: Try legacy format (list of accounts)
	var legacyAccounts []*Account
	if err := json.Unmarshal(data, &legacyAccounts); err == nil {
		am.accounts = legacyAccounts
		am.activeIndex = 0
		am.activeIndexByFamily = make(map[string]int)
		for i, acc := range am.accounts {
			acc.Index = i
			acc.RateLimitResetTimes = make(map[string]time.Time)
		}
		return nil
	}

	return fmt.Errorf("unknown account file format")
}

// Save saves accounts to disk
func (am *AccountManager) Save() error {
	am.mu.RLock()
	storage := AccountStorageV3{
		Version:             3,
		Accounts:            am.accounts,
		ActiveIndex:         am.activeIndex,
		ActiveIndexByFamily: am.activeIndexByFamily,
	}
	am.mu.RUnlock()

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(am.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(am.filePath, data, 0600)
}

// AddAccount adds or updates an account
func (am *AccountManager) AddAccount(account *Account) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check if account exists by email
	for i, existing := range am.accounts {
		if existing.Email == account.Email {
			// Update existing
			existing.RefreshToken = account.RefreshToken
			existing.AccessToken = account.AccessToken
			existing.AccessExpiry = account.AccessExpiry
			existing.UpdatedAt = time.Now()
			if account.ProjectID != "" {
				existing.ProjectID = account.ProjectID
			}
			am.accounts[i] = existing
			return am.saveUnlocked()
		}
	}

	// Add new
	account.Index = len(am.accounts)
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()
	account.RateLimitResetTimes = make(map[string]time.Time)
	am.accounts = append(am.accounts, account)

	return am.saveUnlocked()
}

// DeleteAccount removes an account by email
func (am *AccountManager) DeleteAccount(email string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	idx := -1
	for i, acc := range am.accounts {
		if acc.Email == email {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("account not found")
	}

	// Remove from slice
	am.accounts = append(am.accounts[:idx], am.accounts[idx+1:]...)

	// Re-index
	for i, acc := range am.accounts {
		acc.Index = i
	}

	// Adjust active indices if needed
	// Simplification: just reset to 0 if out of bounds
	if am.activeIndex >= len(am.accounts) {
		am.activeIndex = 0
	}
	for k, v := range am.activeIndexByFamily {
		if v >= len(am.accounts) {
			am.activeIndexByFamily[k] = 0
		}
	}

	return am.saveUnlocked()
}

// GetAccount retrieves an account by email
func (am *AccountManager) GetAccount(email string) *Account {
	am.mu.RLock()
	defer am.mu.RUnlock()
	for _, acc := range am.accounts {
		if acc.Email == email {
			return acc
		}
	}
	return nil
}

// ListAccounts returns all accounts
func (am *AccountManager) ListAccounts() []*Account {
	am.mu.RLock()
	defer am.mu.RUnlock()
	result := make([]*Account, len(am.accounts))
	copy(result, am.accounts)
	return result
}

// GetCurrentOrNextForFamily selects an account for a given model family
func (am *AccountManager) GetCurrentOrNextForFamily(family string, model string, strategy string) (*Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if len(am.accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured")
	}

	// Identify quota key
	// Simple mapping for now
	var quotaKey string
	if family == "claude" {
		quotaKey = "claude"
	} else {
		// Default to Antigravity for Gemini models
		quotaKey = "gemini-antigravity"
	}

	if strategy == "hybrid" {
		// Hybrid strategy using HealthTracker & TokenTracker
		candidates := make([]AccountWithMetrics, len(am.accounts))
		for i, acc := range am.accounts {
			candidates[i] = AccountWithMetrics{
				Index:         acc.Index,
				LastUsed:      acc.LastUsed,
				HealthScore:   am.healthTracker.GetScore(acc.Index),
				IsRateLimited: acc.IsRateLimited(quotaKey),
				IsCoolingDown: time.Now().Before(acc.CoolingDownUntil),
			}
		}

		selectedIndex := SelectHybridAccount(candidates, am.tokenTracker)
		if selectedIndex >= 0 {
			selected := am.accounts[selectedIndex]
			selected.LastUsed = time.Now()
			am.activeIndexByFamily[family] = selectedIndex
			am.saveUnlocked() // Async save in real impl, sync here for safety
			return selected, nil
		}
	}

	// Fallback: Sticky / Round-Robin
	// Check current
	currentIndex := am.activeIndexByFamily[family]
	if currentIndex >= 0 && currentIndex < len(am.accounts) {
		current := am.accounts[currentIndex]
		if !current.IsRateLimited(quotaKey) && !time.Now().Before(current.CoolingDownUntil) {
			current.LastUsed = time.Now()
			return current, nil
		}
	}

	// Find next available
	for i := 0; i < len(am.accounts); i++ {
		idx := (currentIndex + 1 + i) % len(am.accounts)
		candidate := am.accounts[idx]
		if !candidate.IsRateLimited(quotaKey) && !time.Now().Before(candidate.CoolingDownUntil) {
			am.activeIndexByFamily[family] = idx
			candidate.LastUsed = time.Now()
			am.saveUnlocked()
			return candidate, nil
		}
	}

	return nil, fmt.Errorf("all accounts rate limited")
}

// MarkRateLimited marks an account as rate limited for a quota
func (am *AccountManager) MarkRateLimited(index int, quotaKey string, retryAfter time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if index < 0 || index >= len(am.accounts) {
		return
	}

	acc := am.accounts[index]
	if acc.RateLimitResetTimes == nil {
		acc.RateLimitResetTimes = make(map[string]time.Time)
	}
	acc.RateLimitResetTimes[quotaKey] = time.Now().Add(retryAfter)
	acc.ConsecutiveFailures++

	// Update health
	am.healthTracker.RecordRateLimit(index)

	am.saveUnlocked()
}

// MarkSuccess marks a request as successful
func (am *AccountManager) MarkSuccess(index int) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if index < 0 || index >= len(am.accounts) {
		return
	}

	acc := am.accounts[index]
	acc.ConsecutiveFailures = 0

	// Update health
	am.healthTracker.RecordSuccess(index)

	am.saveUnlocked()
}

func (am *AccountManager) saveUnlocked() error {
	storage := AccountStorageV3{
		Version:             3,
		Accounts:            am.accounts,
		ActiveIndex:         am.activeIndex,
		ActiveIndexByFamily: am.activeIndexByFamily,
	}

	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(am.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(am.filePath, data, 0600)
}

// GetHealthTracker returns the health tracker
func (am *AccountManager) GetHealthTracker() *HealthTracker {
	return am.healthTracker
}

// GetTokenTracker returns the token tracker
func (am *AccountManager) GetTokenTracker() *TokenTracker {
	return am.tokenTracker
}

// GetStats returns account statistics for debugging
func (am *AccountManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_accounts"] = len(am.accounts)
	stats["active_index"] = am.activeIndex

	accountStats := make([]map[string]interface{}, len(am.accounts))
	for i, acc := range am.accounts {
		accountStats[i] = map[string]interface{}{
			"email":        acc.Email,
			"health_score": am.healthTracker.GetScore(acc.Index),
			"is_expired":   acc.IsAccessTokenExpired(),
			"failures":     acc.ConsecutiveFailures,
		}
	}
	stats["accounts"] = accountStats

	return stats
}

// GetEffectiveScore returns the health score for an account.
// This is a convenience method for backward compatibility with config_wizard.
func (am *AccountManager) GetEffectiveScore(acc *Account) int {
	return am.healthTracker.GetScore(acc.Index)
}

// NewAccountStore is an alias for NewAccountManager for backward compatibility.
// Deprecated: Use NewAccountManager instead.
func NewAccountStore() (*AccountManager, error) {
	return NewAccountManager()
}
