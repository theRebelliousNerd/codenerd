package autopoiesis

import (
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestSafetyChecker(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"))
	cfg := OuroborosConfig{
		AllowFileSystem: false,
		AllowNetworking: false,
		AllowExec:       false,
	}
	checker := NewSafetyChecker(cfg)

	tests := []struct {
		name        string
		code        string
		shouldPass  bool
		violation   ViolationType
		descContain string
	}{
		{
			name: "Safe Simple Calculation",
			code: `package main
import "fmt"
func main() { fmt.Println("Hello") }`,
			shouldPass: true,
		},
		{
			name: "Forbidden Import",
			code: `package main
import "os/exec"
func main() { exec.Command("whoami") }`,
			shouldPass:  false,
			violation:   ViolationForbiddenImport,
			descContain: "os/exec",
		},
		{
			name: "Goroutine Leak (No Context)",
			code: `package main
func main() {
	go func() { 
		// forever
	}()
}`,
			shouldPass:  false,
			violation:   ViolationGoroutineLeak,
			descContain: "spawn",
		},
		{
			name: "Goroutine Safe (With Context)",
			code: `package main
import "context"
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(ctx context.Context) {
		<-ctx.Done()
	}(ctx)
}`,
			shouldPass: true,
		},
		{
			name: "Panic Usage",
			code: `package main
func main() {
	panic("crash")
}`,
			shouldPass:  false,
			violation:   ViolationPanic,
			descContain: "panic",
		},
		{
			name: "Empty Function",
			code: `package main
func ok() {}`,
			shouldPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := checker.Check(tt.code)
			if tt.shouldPass && !report.Safe {
				t.Errorf("Expected safe, got unsafe. Violations: %+v", report.Violations)
			}
			if !tt.shouldPass {
				if report.Safe {
					t.Errorf("Expected unsafe, got safe.")
				} else {
					found := false
					for _, v := range report.Violations {
						if v.Type == tt.violation {
							if tt.descContain != "" {
								if strings.Contains(v.Description, tt.descContain) || strings.Contains(v.Location, tt.descContain) {
									found = true
								}
							} else {
								found = true
							}
						}
					}
					if !found {
						t.Errorf("Expected violation type %v with desc %q, got %+v", tt.violation, tt.descContain, report.Violations)
					}
				}
			}
		})
	}
}
