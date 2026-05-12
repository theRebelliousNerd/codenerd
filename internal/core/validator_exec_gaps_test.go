package core

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// TEST_GAP: Null/Undefined: empty output, missing target
func TestExecutionValidator_EmptyOutput(t *testing.T) {
	v := NewExecutionValidator()
	req := ActionRequest{Type: ActionExecCmd, Target: ""}
	res := ActionResult{Success: false, Output: ""}
	vr := v.Validate(context.Background(), req, res)
	if !vr.Verified || vr.Confidence != 0.9 {
		t.Errorf("Expected valid default on empty, got %v", vr)
	}
}

// TEST_GAP: User Extremes: ANSI codes, 100MB massive outputs, multi-byte UTF-8, etc.
func TestExecutionValidator_ANSI_Strip(t *testing.T) {
	v := NewExecutionValidator()
	req := ActionRequest{Type: ActionExecCmd, Target: "test"}
	// \x1b[31mfatal:\x1b[0m
	res := ActionResult{Success: false, Output: "\x1b[31mfatal:\x1b[0m failed"}
	vr := v.Validate(context.Background(), req, res)
	if vr.Verified {
		t.Errorf("Expected ANSI stripped failure, got verified")
	}
}

func TestExecutionValidator_MassiveOutput(t *testing.T) {
	v := NewExecutionValidator()
	req := ActionRequest{Type: ActionExecCmd, Target: "test"}
	
	hugeOutput := strings.Repeat("A", 10*1024*1024) + "fatal: failed" // 10MB
	res := ActionResult{Success: false, Output: hugeOutput}
	vr := v.Validate(context.Background(), req, res)
	if vr.Verified {
		t.Errorf("Expected truncation to catch tail failure, got verified")
	}
}

func TestExecutionValidator_MultiByteRune(t *testing.T) {
	v := NewExecutionValidator()
	req := ActionRequest{Type: ActionExecCmd, Target: "test"}
	res := ActionResult{Success: false, Output: "日本語日本語 fatal: failed"}
	vr := v.Validate(context.Background(), req, res)
	if vr.Verified {
		t.Errorf("Expected failure to be found with multibyte context, got verified")
	}
}

// TEST_GAP: State Conflicts: Concurrency
func TestExecutionValidator_ConcurrentAccess(t *testing.T) {
	v := NewExecutionValidator()
	req := ActionRequest{Type: ActionExecCmd, Target: "test"}
	res := ActionResult{Success: false, Output: "fatal: failure"}
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Validate(context.Background(), req, res)
		}()
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Use simple string to avoid format dependency
			v.AddFailurePattern("testpattern_concurrent") 
		}(i)
	}
	wg.Wait()
}
