package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileTaskTypeWithWriteSet(t *testing.T) {
	tests := []struct {
		name       string
		taskType   TaskType
		writeSet   func(workspace string) []string
		wantType   TaskType
		wantChange bool
		wantReason string
	}{
		{
			name:     "file_modify with one non-existent exact path becomes file_create",
			taskType: TaskTypeFileModify,
			writeSet: func(workspace string) []string {
				return []string{filepath.Join(workspace, "does-not-exist.mg")}
			},
			wantType:   TaskTypeFileCreate,
			wantChange: true,
			wantReason: "no write-set path exists",
		},
		{
			name:     "file_create with one existing path becomes file_modify",
			taskType: TaskTypeFileCreate,
			writeSet: func(workspace string) []string {
				p := filepath.Join(workspace, "exists.mg")
				if err := os.WriteFile(p, []byte("Decl test/1.\n"), 0o644); err != nil {
					t.Fatalf("setup: write %s: %v", p, err)
				}
				return []string{p}
			},
			wantType:   TaskTypeFileModify,
			wantChange: true,
			wantReason: "every write-set path already exists",
		},
		{
			name:     "file_modify with one existing path stays file_modify",
			taskType: TaskTypeFileModify,
			writeSet: func(workspace string) []string {
				p := filepath.Join(workspace, "exists.mg")
				if err := os.WriteFile(p, []byte("Decl test/1.\n"), 0o644); err != nil {
					t.Fatalf("setup: write %s: %v", p, err)
				}
				return []string{p}
			},
			wantType:   TaskTypeFileModify,
			wantChange: false,
			wantReason: "",
		},
		{
			name:     "file_modify with glob entry is left alone",
			taskType: TaskTypeFileModify,
			writeSet: func(workspace string) []string {
				return []string{filepath.Join(workspace, "*.mg")}
			},
			wantType:   TaskTypeFileModify,
			wantChange: false,
			wantReason: "",
		},
		{
			name:       "empty write set is left alone",
			taskType:   TaskTypeFileModify,
			writeSet:   func(workspace string) []string { return nil },
			wantType:   TaskTypeFileModify,
			wantChange: false,
			wantReason: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			task := &Task{ID: "/task_retype", Type: tc.taskType, WriteSet: tc.writeSet(workspace)}
			changed, reason := reconcileTaskTypeWithWriteSet(workspace, task)
			if changed != tc.wantChange {
				t.Fatalf("changed = %v, want %v (type now %s, reason %q)", changed, tc.wantChange, task.Type, reason)
			}
			if task.Type != tc.wantType {
				t.Fatalf("type = %s, want %s", task.Type, tc.wantType)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tc.wantReason)
			}
		})
	}
}
