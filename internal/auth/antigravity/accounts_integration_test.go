//go:build integration
package antigravity_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/auth/antigravity"

	"github.com/stretchr/testify/suite"
)

type AccountStoreSuite struct {
	suite.Suite
	originalHome        string
	originalUserProfile string
	tempHome            string
	store               *antigravity.AccountStore
}

func (s *AccountStoreSuite) SetupTest() {
	// Create temp home directory
	var err error
	s.tempHome, err = os.MkdirTemp("", "nerd_integration_test")
	s.Require().NoError(err)

	// Save original env vars
	s.originalHome = os.Getenv("HOME")
	s.originalUserProfile = os.Getenv("USERPROFILE")

	// Set env vars to temp dir
	os.Setenv("HOME", s.tempHome)
	os.Setenv("USERPROFILE", s.tempHome)

	// Initialize store
	s.store, err = antigravity.NewAccountStore()
	s.Require().NoError(err)
}

func (s *AccountStoreSuite) TearDownTest() {
	// Restore env vars
	os.Setenv("HOME", s.originalHome)
	os.Setenv("USERPROFILE", s.originalUserProfile)

	// Cleanup temp dir
	os.RemoveAll(s.tempHome)
}

func (s *AccountStoreSuite) TestNewAccountStore_CreatesFile() {
	// Initially file might not exist until we save something
	// But let's check if we can trigger a save
	acc := &antigravity.Account{
		Email: "init@example.com",
	}
	err := s.store.AddAccount(acc)
	s.Require().NoError(err)

	// Check file existence
	expectedPath := filepath.Join(s.tempHome, ".nerd", "antigravity_accounts.json")
	_, err = os.Stat(expectedPath)
	s.Require().NoError(err, "Account file should exist after adding account")
}

func (s *AccountStoreSuite) TestLifecycle_AddGetListDelete() {
	acc := &antigravity.Account{
		Email:        "test@example.com",
		RefreshToken: "refresh-123",
		AccessToken:  "access-123",
		ProjectID:    "test-project",
		HealthScore:  80,
	}

	// Add
	err := s.store.AddAccount(acc)
	s.Require().NoError(err)

	// Get
	fetched, found := s.store.GetAccount("test@example.com")
	s.Require().True(found)
	s.Equal("test@example.com", fetched.Email)
	s.Equal("refresh-123", fetched.RefreshToken)
	s.Equal(80, fetched.HealthScore)

	// List
	list := s.store.ListAccounts()
	s.Require().Len(list, 1)
	s.Equal("test@example.com", list[0].Email)

	// Delete
	err = s.store.DeleteAccount("test@example.com")
	s.Require().NoError(err)

	// Verify Delete
	_, found = s.store.GetAccount("test@example.com")
	s.Require().False(found)
	s.Require().Empty(s.store.ListAccounts())
}

func (s *AccountStoreSuite) TestPersistence_AcrossInstances() {
	acc := &antigravity.Account{
		Email:        "persistent@example.com",
		RefreshToken: "persist-token",
	}
	s.Require().NoError(s.store.AddAccount(acc))

	// Create a NEW store instance simulating app restart
	// It should read from the same temp file
	store2, err := antigravity.NewAccountStore()
	s.Require().NoError(err)

	fetched, found := store2.GetAccount("persistent@example.com")
	s.Require().True(found, "Should find account in new store instance")
	s.Equal("persistent@example.com", fetched.Email)
}

func (s *AccountStoreSuite) TestHealthScore_Integration() {
	email := "health@example.com"
	acc := &antigravity.Account{
		Email:       email,
		HealthScore: 50,
	}
	s.Require().NoError(s.store.AddAccount(acc))

	// Record Success
	s.store.RecordSuccess(email)
	updated, _ := s.store.GetAccount(email)
	s.Greater(updated.HealthScore, 50, "Health score should increase on success")

	// Record Failure
	currentScore := updated.HealthScore
	s.store.RecordFailure(email, "some error")
	updated, _ = s.store.GetAccount(email)
	s.Less(updated.HealthScore, currentScore, "Health score should decrease on failure")

	// Record Rate Limit
	currentScore = updated.HealthScore
	s.store.RecordRateLimit(email)
	updated, _ = s.store.GetAccount(email)
	s.Less(updated.HealthScore, currentScore, "Health score should decrease significantly on rate limit")
}

func (s *AccountStoreSuite) TestSelector_SelectBest() {
	// Add multiple accounts
	goodAcc := &antigravity.Account{Email: "good@example.com", HealthScore: 90, LastUsed: time.Now().Add(-2 * time.Hour)}
	badAcc := &antigravity.Account{Email: "bad@example.com", HealthScore: 20, LastUsed: time.Now()} // Below default min usable (30)

	s.Require().NoError(s.store.AddAccount(goodAcc))
	s.Require().NoError(s.store.AddAccount(badAcc))

	selector := antigravity.NewAccountSelector(s.store)
	best, err := selector.SelectBest()
	s.Require().NoError(err)
	s.Equal("good@example.com", best.Email)

	// Verify stats
	stats := selector.GetStats()
	s.Equal(2, stats["total"])
	s.Equal(1, stats["healthy"])
	s.Equal(1, stats["exhausted"])
}

func (s *AccountStoreSuite) TestConcurrency_Safety() {
	// This test ensures that concurrent writes to the file don't cause crashes or corruption
	// The store uses a mutex, so this verifies the mutex is working effectively.

	email := "concurrent@example.com"
	s.Require().NoError(s.store.AddAccount(&antigravity.Account{Email: email}))

	var wg sync.WaitGroup
	concurrency := 10
	iterations := 20

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Mix of reads and writes
				if j%2 == 0 {
					s.store.RecordSuccess(email)
				} else {
					_, _ = s.store.GetAccount(email)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify file is still valid json and account exists
	fetched, found := s.store.GetAccount(email)
	s.Require().True(found)
	s.Greater(fetched.HealthScore, 0)
}

func TestAccountStoreSuite(t *testing.T) {
	suite.Run(t, new(AccountStoreSuite))
}
