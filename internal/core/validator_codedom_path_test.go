package core

import (
	"path/filepath"
	"testing"
)

func TestCodeDOMValidator_ExtractFilePath(t *testing.T) {
	absPath := filepath.Join(t.TempDir(), "w.go")

	tests := []struct {
		name   string
		req    ActionRequest
		result ActionResult
		want   string
	}{
		{
			name:   "absolute path returned unchanged",
			req:    ActionRequest{Target: absPath},
			result: ActionResult{},
			want:   absPath,
		},
		{
			name:   "relative file path",
			req:    ActionRequest{Target: "internal/foo.go"},
			result: ActionResult{},
			want:   "internal/foo.go",
		},
		{
			name:   "ref format",
			req:    ActionRequest{Target: "go:internal/foo.go:FuncName"},
			result: ActionResult{},
			want:   "internal/foo.go",
		},
		{
			name:   "payload path",
			req:    ActionRequest{Target: "", Payload: map[string]any{"path": "p.go"}},
			result: ActionResult{},
			want:   "p.go",
		},
		{
			name:   "metadata file wins over target",
			req:    ActionRequest{Target: "internal/foo.go"},
			result: ActionResult{Metadata: map[string]any{"file": "m.go"}},
			want:   "m.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewCodeDOMValidator()
			if got := v.extractFilePath(tt.req, tt.result); got != tt.want {
				t.Errorf("extractFilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}
