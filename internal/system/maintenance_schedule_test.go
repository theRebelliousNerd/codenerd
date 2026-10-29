package system

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/store"
)

func TestStartMaintenanceSchedule_NoImmediateRunAndFastCancel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "maint.db")
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	// Interval long enough that it must not fire during the test.
	oldInterval := maintenanceInterval
	maintenanceInterval = 24 * time.Hour
	t.Cleanup(func() { maintenanceInterval = oldInterval })

	var runs atomic.Int32
	oldHook := maintenanceTestHook
	maintenanceTestHook = func() { runs.Add(1) }
	t.Cleanup(func() { maintenanceTestHook = oldHook })

	c := &Cortex{LocalDB: ls}
	start := time.Now()
	_ = c.StartMaintenanceSchedule(context.Background())

	// Immediate-run bug would invoke MaintenanceCleanup (and our hook) right away.
	time.Sleep(80 * time.Millisecond)
	if got := runs.Load(); got != 0 {
		t.Fatalf("expected 0 maintenance runs before first interval, got %d", got)
	}

	// Close path: cancel + wait must be fast with no in-flight work.
	c.stopMaintenanceSchedule(maintenanceStopWait)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("stopMaintenanceSchedule too slow for idle loop: %v", elapsed)
	}
	if c.maintenanceCancel != nil || c.maintenanceDone != nil {
		t.Fatalf("expected maintenance fields cleared after stop")
	}
	if got := runs.Load(); got != 0 {
		t.Fatalf("expected still 0 runs after cancel, got %d", got)
	}

	// LocalDB.Close must not contend with a ghost maintenance cycle.
	closeStart := time.Now()
	if err := ls.Close(); err != nil {
		t.Fatalf("LocalDB.Close: %v", err)
	}
	if elapsed := time.Since(closeStart); elapsed > time.Second {
		t.Fatalf("LocalDB.Close too slow after maintenance stop: %v", elapsed)
	}
}

func TestCortexClose_StopsMaintenanceBeforeLocalDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "close_maint.db")
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	oldInterval := maintenanceInterval
	maintenanceInterval = 24 * time.Hour
	t.Cleanup(func() { maintenanceInterval = oldInterval })

	c := &Cortex{LocalDB: ls}
	_ = c.StartMaintenanceSchedule(context.Background())

	start := time.Now()
	if err := c.Close(); err != nil {
		// Timeouts from other nil subsystems should not appear; only LocalDB was set.
		t.Fatalf("Close: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Close too slow for minimal cortex: %v", elapsed)
	}
	if c.LocalDB != nil {
		t.Fatalf("expected LocalDB nil after Close")
	}
	if c.maintenanceCancel != nil || c.maintenanceDone != nil {
		t.Fatalf("expected maintenance stopped after Close")
	}
}

// TestCortexClose_CancelsMaintenance asserts Close cancels an idle maintenance
// loop without requiring a full BootCortex path.
func TestCortexClose_CancelsMaintenance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "close_cancel.db")
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}

	oldInterval := maintenanceInterval
	maintenanceInterval = 24 * time.Hour
	t.Cleanup(func() { maintenanceInterval = oldInterval })

	var runs atomic.Int32
	oldHook := maintenanceTestHook
	maintenanceTestHook = func() { runs.Add(1) }
	t.Cleanup(func() { maintenanceTestHook = oldHook })

	c := &Cortex{LocalDB: ls}
	_ = c.StartMaintenanceSchedule(context.Background())
	if c.maintenanceCancel == nil || c.maintenanceDone == nil {
		t.Fatal("expected maintenance schedule fields set after Start")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.maintenanceCancel != nil {
		t.Fatal("maintenanceCancel still set after Close")
	}
	if c.maintenanceDone != nil {
		t.Fatal("maintenanceDone still set after Close")
	}
	if got := runs.Load(); got != 0 {
		t.Fatalf("maintenance ran during Close, runs=%d", got)
	}
	// Second Close on already-torn-down cortex must be safe.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
