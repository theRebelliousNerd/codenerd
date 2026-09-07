package logging

import (
	"sync"
	"testing"
)

func TestAuditConcurrentInitializationAndClose(t *testing.T) {
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)
	ApplyConfig(Config{DebugMode: true})
	if err := Initialize(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 30; j++ {
				Audit().SafetyCheck("concurrent", true, "witness")
			}
		})
	}
	wg.Go(func() {
		for i := 0; i < 30; i++ {
			CloseAudit()
			if err := InitAudit(); err != nil {
				t.Error(err)
			}
		}
	})
	wg.Wait()
	if got := readLog(t, BoundWorkspace(), "audit"); len(got) == 0 {
		t.Fatal("audit records missing")
	}
}
