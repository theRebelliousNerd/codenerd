package autopoiesis

import "fmt"

// Test conveniences over live constructors; production calls the configured forms.

// NewThunderdome creates a new Thunderdome arena.
func NewThunderdome() *Thunderdome {
	return NewThunderdomeWithConfig(DefaultThunderdomeConfig())
}

// generateTestHarness creates Go test code that wraps the tool for attack execution.
// FIX: Now accepts entryPoint parameter to actually call the tool's function with attack input.
// This fixes the "Phantom Punch" bug where attack inputs were being discarded.
// NOTE: Verified "Phantom Punch" bug fix (see thunderdome_harness_test.go).
func (t *Thunderdome) generateTestHarness(_ *GeneratedTool, entryPoint string) string {
	return t.generateTestHarnessWithCall(nil, fmt.Sprintf("_, toolErr = %s(ctx, input)", entryPoint))
}

// NewYaegiExecutor creates a new Yaegi-based tool executor with the default
// standalone allowlist.
//
// Prefer NewYaegiExecutorForPolicy inside the Ouroboros loop: this list is a
// second, independent answer to "what may a generated tool import", and a tool
// that passed the SafetyChecker could still be refused here (the old list had
// no "context", which every tool the compiler accepts must import).
func NewYaegiExecutor() *YaegiExecutor {
	return NewYaegiExecutorForPolicy([]string{
		"bytes",
		"context",
		"encoding/base64",
		"encoding/json",
		"errors",
		"fmt",
		"math",
		"path",
		"path/filepath",
		"regexp",
		"sort",
		"strconv",
		"strings",
		"time",
		// EXPLICITLY BLOCKED (unsafe packages):
		// "os" - filesystem access
		// "os/exec" - command execution
		// "net" - network access
		// "net/http" - HTTP client
		// "syscall" - system calls
		// "unsafe" - unsafe operations
	})
}
