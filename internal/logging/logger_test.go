package logging

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAllCategoriesLog tests that all categories create log files when debug_mode is true
func TestAllCategoriesLog(t *testing.T) {
	// Create temp directory for test logs
	tempDir, err := os.MkdirTemp("", "logging_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test config with debug_mode: true
	configDir := filepath.Join(tempDir, ".nerd")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `{
		"logging": {
			"level": "debug",
			"debug_mode": true,
			"categories": {
				"boot": true,
				"session": true,
				"kernel": true,
				"api": true,
				"perception": true,
				"articulation": true,
				"routing": true,
				"tools": true,
				"virtual_store": true,
				"shards": true,
				"coder": true,
				"tester": true,
				"reviewer": true,
				"researcher": true,
				"system_shards": true,
				"dream": true,
				"autopoiesis": true,
				"campaign": true,
				"context": true,
				"world": true,
				"embedding": true,
				"store": true,
				"browser": true
			}
		}
	}`

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset logging state
	CloseAll()
	CloseAudit()
	loggers = make(map[Category]*Logger)
	logsDir = ""
	workspace = ""
	configLoaded = false
	auditLogger = nil

	// Initialize logging with temp workspace
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize logging: %v", err)
	}

	// Verify debug mode is enabled
	if !IsDebugMode() {
		t.Error("Expected debug mode to be enabled")
	}

	// All categories to test
	categories := []Category{
		CategoryBoot,
		CategorySession,
		CategoryKernel,
		CategoryAPI,
		CategoryPerception,
		CategoryArticulation,
		CategoryRouting,
		CategoryTools,
		CategoryVirtualStore,
		CategoryShards,
		CategoryCoder,
		CategoryTester,
		CategoryReviewer,
		CategoryResearcher,
		CategorySystemShards,
		CategoryDream,
		CategoryAutopoiesis,
		CategoryCampaign,
		CategoryContext,
		CategoryWorld,
		CategoryEmbedding,
		CategoryStore,
		CategoryBrowser,
	}

	// Log to each category
	for _, cat := range categories {
		if !IsCategoryEnabled(cat) {
			t.Errorf("Category %s should be enabled", cat)
		}

		logger := Get(cat)
		logger.Info("Test info message for %s", cat)
		logger.Debug("Test debug message for %s", cat)
		logger.Warn("Test warn message for %s", cat)
		logger.Error("Test error message for %s", cat)
	}

	// Also test convenience functions
	Boot("Convenience boot log")
	Session("Convenience session log")
	Kernel("Convenience kernel log")
	API("Convenience api log")
	Perception("Convenience perception log")
	Articulation("Convenience articulation log")
	Routing("Convenience routing log")
	Tools("Convenience tools log")
	VirtualStore("Convenience virtual_store log")
	Shards("Convenience shards log")
	Coder("Convenience coder log")
	Tester("Convenience tester log")
	Reviewer("Convenience reviewer log")
	Researcher("Convenience researcher log")
	SystemShards("Convenience system_shards log")
	Dream("Convenience dream log")
	Autopoiesis("Convenience autopoiesis log")
	Campaign("Convenience campaign log")
	Context("Convenience context log")
	World("Convenience world log")
	Embedding("Convenience embedding log")
	Store("Convenience store log")

	// Close all loggers to flush
	CloseAll()
	CloseAudit()

	// Verify log files were created
	logsPath := filepath.Join(tempDir, ".nerd", "logs")
	entries, err := os.ReadDir(logsPath)
	if err != nil {
		t.Fatalf("Failed to read logs dir: %v", err)
	}

	t.Logf("Created %d log files in %s", len(entries), logsPath)

	// Check each category has a log file with content
	for _, cat := range categories {
		found := false
		for _, entry := range entries {
			if strings.Contains(entry.Name(), string(cat)+".log") {
				found = true
				// Read and verify content
				content, err := os.ReadFile(filepath.Join(logsPath, entry.Name()))
				if err != nil {
					t.Errorf("Failed to read log file for %s: %v", cat, err)
					continue
				}
				if len(content) == 0 {
					t.Errorf("Log file for %s is empty", cat)
				} else {
					t.Logf("✓ %s: %d bytes", cat, len(content))
				}
				break
			}
		}
		if !found {
			t.Errorf("No log file found for category: %s", cat)
		}
	}
}

// TestDebugModeDisabled tests that no logs are created when debug_mode is false
func TestDebugModeDisabled(t *testing.T) {
	// Create temp directory for test logs
	tempDir, err := os.MkdirTemp("", "logging_test_disabled")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test config with debug_mode: false (PRODUCTION MODE)
	configDir := filepath.Join(tempDir, ".nerd")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `{
		"logging": {
			"level": "debug",
			"debug_mode": false,
			"categories": {
				"boot": true,
				"kernel": true,
				"shards": true
			}
		}
	}`

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset logging state completely (including sync.Once guard)
	CloseAll()
	CloseAudit()
	loggers = make(map[Category]*Logger)
	logsDir = ""
	workspace = ""
	configLoaded = false
	config = loggingConfig{} // Reset config to avoid state leakage from previous tests
	initOnce = sync.Once{}   // Reset sync.Once to allow re-initialization
	initErr = nil
	initialized = false
	auditLogger = nil

	// Initialize logging with temp workspace
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize logging: %v", err)
	}

	// Verify debug mode is DISABLED
	if IsDebugMode() {
		t.Error("Expected debug mode to be DISABLED (production mode)")
	}

	// All categories should be disabled
	categories := []Category{
		CategoryBoot,
		CategoryKernel,
		CategoryShards,
		CategoryPerception,
	}

	for _, cat := range categories {
		if IsCategoryEnabled(cat) {
			t.Errorf("Category %s should be DISABLED when debug_mode=false", cat)
		}
	}

	// Try to log - should be no-ops
	Boot("This should NOT be logged")
	Kernel("This should NOT be logged")
	Shards("This should NOT be logged")

	logger := Get(CategoryBoot)
	logger.Info("This should NOT be logged")
	logger.Debug("This should NOT be logged")
	logger.Error("This should NOT be logged")

	// Close all loggers
	CloseAll()
	CloseAudit()

	// Verify NO log files were created (logs directory shouldn't even exist)
	logsPath := filepath.Join(tempDir, ".nerd", "logs")
	_, err = os.Stat(logsPath)
	if err == nil {
		// Directory exists - check if it has any files
		entries, _ := os.ReadDir(logsPath)
		if len(entries) > 0 {
			t.Errorf("Expected NO log files in production mode, but found %d files", len(entries))
			for _, e := range entries {
				t.Logf("  - %s", e.Name())
			}
		} else {
			t.Log("✓ Logs directory exists but is empty (correct)")
		}
	} else if os.IsNotExist(err) {
		t.Log("✓ Logs directory was not created (correct for production mode)")
	}
}

// TestCategoryToggle tests individual category enable/disable
func TestCategoryToggle(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "logging_test_category")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config with some categories enabled, some disabled
	configDir := filepath.Join(tempDir, ".nerd")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `{
		"logging": {
			"level": "debug",
			"debug_mode": true,
			"categories": {
				"boot": true,
				"kernel": true,
				"shards": false,
				"perception": false
			}
		}
	}`

	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset logging state completely (including sync.Once guard)
	CloseAll()
	CloseAudit()
	loggers = make(map[Category]*Logger)
	logsDir = ""
	workspace = ""
	configLoaded = false
	config = loggingConfig{} // Reset config to avoid state leakage from previous tests
	initOnce = sync.Once{}   // Reset sync.Once to allow re-initialization
	initErr = nil
	initialized = false
	auditLogger = nil

	// Initialize
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Check enabled categories
	if !IsCategoryEnabled(CategoryBoot) {
		t.Error("boot should be enabled")
	}
	if !IsCategoryEnabled(CategoryKernel) {
		t.Error("kernel should be enabled")
	}

	// Check disabled categories
	if IsCategoryEnabled(CategoryShards) {
		t.Error("shards should be DISABLED")
	}
	if IsCategoryEnabled(CategoryPerception) {
		t.Error("perception should be DISABLED")
	}

	// Check category not in config (should default to enabled when debug_mode=true)
	if !IsCategoryEnabled(CategoryCoder) {
		t.Error("coder (not in config) should default to enabled")
	}

	// Log to all
	Boot("This SHOULD be logged")
	Kernel("This SHOULD be logged")
	Shards("This should NOT be logged")
	Perception("This should NOT be logged")
	Coder("This SHOULD be logged (default enabled)")

	CloseAll()
	CloseAudit()

	// Verify correct files created
	logsPath := filepath.Join(tempDir, ".nerd", "logs")
	entries, _ := os.ReadDir(logsPath)

	hasBootLog := false
	hasKernelLog := false
	hasShardsLog := false
	hasPerceptionLog := false

	for _, e := range entries {
		name := e.Name()
		if strings.Contains(name, "boot") {
			hasBootLog = true
		}
		if strings.Contains(name, "kernel") {
			hasKernelLog = true
		}
		if strings.Contains(name, "shards") {
			hasShardsLog = true
		}
		if strings.Contains(name, "perception") {
			hasPerceptionLog = true
		}
	}

	if !hasBootLog {
		t.Error("Expected boot log file")
	}
	if !hasKernelLog {
		t.Error("Expected kernel log file")
	}
	if hasShardsLog {
		t.Error("Should NOT have shards log file (disabled)")
	}
	if hasPerceptionLog {
		t.Error("Should NOT have perception log file (disabled)")
	}

	t.Logf("✓ Category toggle test passed - %d files created", len(entries))
}

// TestTimerLogging tests the timing helper
func TestTimerLogging(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "logging_test_timer")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create config with debug_mode: true
	configDir := filepath.Join(tempDir, ".nerd")
	os.MkdirAll(configDir, 0755)

	configContent := `{"logging": {"level": "debug", "debug_mode": true}}`
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configContent), 0644)

	// Reset and initialize
	CloseAll()
	CloseAudit()
	loggers = make(map[Category]*Logger)
	logsDir = ""
	workspace = ""
	configLoaded = false
	auditLogger = nil
	Initialize(tempDir)

	// Test timer
	timer := StartTimer(CategoryKernel, "TestOperation")
	// Simulate some work with a small sleep to ensure measurable duration
	time.Sleep(time.Millisecond)
	elapsed := timer.Stop()

	if elapsed <= 0 {
		t.Error("Timer should have recorded non-zero duration")
	}

	t.Logf("✓ Timer recorded: %v", elapsed)

	CloseAll()
	CloseAudit()
}
