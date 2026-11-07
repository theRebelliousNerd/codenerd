package tools

import (
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestModelFacing_SeparatesTheCallsFaultFromTheTools(t *testing.T) {
	_, notExist := os.ReadDir("/definitely/not/a/dir")
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"refusal", Refuse("refusing edit: %s", "unbalanced"), true},
		{"wrapped refusal", fmt.Errorf("modular tool execution failed: %w", Refuse("refusing to write %s", "a.go")), true},
		{"missing path", fmt.Errorf("failed to read directory: %w", notExist), true},
		{"missing argument", fmt.Errorf("path: %w", ErrMissingRequiredArg), true},
		{"outside workspace", fmt.Errorf("%w: ../x", ErrPathOutsideWorkspace), true},
		{"not allowed", ErrToolNotAllowed, true},
		{"io failure", errors.New("write a.go: input/output error"), false},
		{"tool limitation", fmt.Errorf("refusing x on a.go: the file has no element model"), false},
	} {
		if got := ModelFacing(tc.err); got != tc.want {
			t.Errorf("%s: ModelFacing(%v) = %v, want %v", tc.name, tc.err, got, tc.want)
		}
	}
	if Refuse("refusing %s", "x").Error() != "refusing x" {
		t.Fatal("a refusal must keep its message")
	}
}
