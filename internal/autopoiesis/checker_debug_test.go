package autopoiesis

import (
	"fmt"
	"testing"
)

func TestDebugEmptyFunc(t *testing.T) {
	cfg := DefaultOuroborosConfig("/tmp/workspace")
	checker := NewSafetyChecker(cfg)

	safe := `package tools
func ok() {}`
	report := checker.Check(safe)
	if report.Score != 1.0 {
		fmt.Printf("DEBUG: Score %f\n", report.Score)
		for _, v := range report.Violations {
			fmt.Printf("DEBUG: Violation: %v\n", v)
		}
		t.Fatalf("expected safe score 1.0, got %f", report.Score)
	}
}
