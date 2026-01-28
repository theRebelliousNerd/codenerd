package antigravity

import (
	"os"
	"testing"
	"time"
)

func TestAccountManager_AddAndGet(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "nerd_auth_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Override home dir for test
	originalHome, _ := os.UserHomeDir()
	os.Setenv("HOME", tempDir) // Linux/Mac
	os.Setenv("USERPROFILE", tempDir) // Windows
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalHome)
	}()

	manager, err := NewAccountManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	acc := &Account{
		Email:        "test@example.com",
		RefreshToken: "refresh_token",
		AccessToken:  "access_token",
		ProjectID:    "test-project",
	}

	if err := manager.AddAccount(acc); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	retrieved := manager.GetAccount("test@example.com")
	if retrieved == nil {
		t.Fatal("Account not found")
	}

	if retrieved.Email != acc.Email {
		t.Errorf("Expected email %s, got %s", acc.Email, retrieved.Email)
	}
	if retrieved.Index != 0 {
		t.Errorf("Expected index 0, got %d", retrieved.Index)
	}
}

func TestAccountManager_Rotation(t *testing.T) {
	t.Log("Starting Rotation Test")
	tempDir, _ := os.MkdirTemp("", "nerd_auth_test_rot")
	defer os.RemoveAll(tempDir)
	
	originalHome, _ := os.UserHomeDir()
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalHome)
	}()

	manager, _ := NewAccountManager()

	// Add 3 accounts
	t.Log("Adding accounts")
	manager.AddAccount(&Account{Email: "acc1@example.com"})
	manager.AddAccount(&Account{Email: "acc2@example.com"})
	manager.AddAccount(&Account{Email: "acc3@example.com"})

	// Test Round Robin / Sticky default behavior
	// First get
	t.Log("Selecting first account")
	acc1, err := manager.GetCurrentOrNextForFamily("gemini", "", "sticky")
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}
	if acc1.Email != "acc1@example.com" {
		t.Errorf("Expected acc1, got %s", acc1.Email)
	}

	// Should stay sticky
	t.Log("Verifying stickiness")
	acc1_again, _ := manager.GetCurrentOrNextForFamily("gemini", "", "sticky")
	if acc1_again.Email != "acc1@example.com" {
		t.Errorf("Expected acc1 again, got %s", acc1_again.Email)
	}

	// Mark Rate Limited
	t.Log("Marking acc1 as rate limited")
	manager.MarkRateLimited(0, "gemini-antigravity", 1*time.Minute)

	// Should rotate to acc2
	t.Log("Selecting next account after rotation")
	acc2, err := manager.GetCurrentOrNextForFamily("gemini", "", "sticky")
	if err != nil {
		t.Fatalf("Failed to rotate: %v", err)
	}
	if acc2.Email != "acc2@example.com" {
		t.Errorf("Expected acc2 after limit, got %s", acc2.Email)
	}
	t.Log("Rotation test complete")
}

func TestAccountManager_Delete(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "nerd_auth_test_del")
	defer os.RemoveAll(tempDir)
	
	originalHome, _ := os.UserHomeDir()
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalHome)
	}()

	manager, _ := NewAccountManager()
	manager.AddAccount(&Account{Email: "acc1@example.com"})
	manager.AddAccount(&Account{Email: "acc2@example.com"})

	if err := manager.DeleteAccount("acc1@example.com"); err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	if manager.GetAccount("acc1@example.com") != nil {
		t.Error("Account should be gone")
	}

	acc2 := manager.GetAccount("acc2@example.com")
	if acc2 == nil {
		t.Error("Account 2 should exist")
	}
	// Check re-indexing
	if acc2.Index != 0 {
		t.Errorf("Account 2 should be re-indexed to 0, got %d", acc2.Index)
	}
}

// TODO: TEST_GAP: Verify behavior when AddAccount is called with a nil Account.

// TODO: TEST_GAP: Verify behavior when AddAccount is called with an Account having an empty Email.

// TODO: TEST_GAP: Verify DeleteAccount returns error when attempting to delete a non-existent email.

// TODO: TEST_GAP: Verify DeleteAccount correctly updates ActiveIndex when the active account is deleted.

// TODO: TEST_GAP: Verify MarkRateLimited handles out-of-bounds index (negative or too large) gracefully.

// TODO: TEST_GAP: Verify GetCurrentOrNextForFamily returns error when all accounts are rate-limited.

// TODO: TEST_GAP: Verify GetCurrentOrNextForFamily returns error when no accounts are configured.

// TODO: TEST_GAP: Verify Load handles corrupted JSON file gracefully (should return error or start empty).

// TODO: TEST_GAP: Verify behavior when file system is read-only during Save (simulate write failure).
