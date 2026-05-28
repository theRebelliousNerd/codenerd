//go:build ignore
// +build ignore

// scratch_cleanup.go is a one-off diagnostic script that exercises the
// tool-execution store's AutoCleanup path with a tiny synthetic dataset.
// Excluded from normal builds via the ignore tag; run with:
//
//	go run scratch_cleanup.go
package main

import (
	"fmt"
	"path/filepath"
	"os"

	"codenerd/internal/store"
)

func main() {
	dbPath := filepath.Join(os.TempDir(), "tools_test.db")
	os.Remove(dbPath)
	s, err := store.NewToolStore(dbPath)
	if err != nil {
		fmt.Printf("Error creating store: %v\n", err)
		return
	}
	defer s.Close()

	exec := store.ToolExecution{
		CallID:           "c1",
		SessionID:        "s1",
		ResultSize:       60,
		SessionRuntimeMs: 1000,
	}
	if err := s.Store(exec); err != nil {
		fmt.Printf("Store failed: %v\n", err)
		return
	}

	cfgSize := store.CleanupConfig{
		MaxSizeBytes:         50,
		AutoCleanupThreshold: 0.5,
		CleanupMode:          "size",
	}

	stats, err := s.AutoCleanup(cfgSize)
	if err != nil {
		fmt.Printf("AutoCleanup err: %v\n", err)
	}
	fmt.Printf("ExecutionsDeleted: %d\n", stats.ExecutionsDeleted)
}
