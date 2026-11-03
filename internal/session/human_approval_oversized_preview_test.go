package session

import (
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/approval"
)

// An oversized delete must not size its preview work by the file's stat size.
// human_approval.go approvalPreview materializes change.Before as
// make([]byte, r.size) for a nil-content, non-directory read, so previewing a
// 256MiB deletion allocates ~256MiB to show nothing: BuildPreview withholds an
// over-limit file unconditionally.
func TestApprovalPreview_OversizedDeleteDoesNotAllocateFileSize(t *testing.T) {
	tests := []struct {
		name string
		size int64
	}{
		{name: "sixteen mebibytes", size: 16 << 20},
		{name: "sixty-four mebibytes", size: 64 << 20},
		{name: "audited two-hundred-fifty-six mebibytes", size: 256 << 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := ToolCall{Name: "delete_file"}
			subject := approval.Subject{Tool: "delete_file", Payload: `{"path":"big.bin"}`}
			reads := []fileRevision{{path: "big.bin", exists: true, content: nil, size: tt.size}}

			// Warm up so the measurement below sees only this preview's work.
			if _, err := approvalPreview(call, subject, reads, 4096); err != nil {
				t.Fatalf("warm-up approvalPreview = %v", err)
			}

			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			prev, err := approvalPreview(call, subject, reads, 4096)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatalf("approvalPreview = %v", err)
			}
			if prev.Complete {
				t.Errorf("preview of a %d-byte oversized delete is Complete; want it withheld", tt.size)
			}
			if grew := after.TotalAlloc - before.TotalAlloc; grew >= uint64(tt.size)/2 {
				t.Errorf("approvalPreview allocated %d bytes for a %d-byte oversized delete; want under %d",
					grew, tt.size, uint64(tt.size)/2)
			}
		})
	}
}

// The oversized path must stay explicit: the withheld file is named so the
// person knows content was not shown, and no diff is computed for it.
func TestApprovalPreview_OversizedDeleteNamesWhatItWithheld(t *testing.T) {
	call := ToolCall{Name: "delete_file"}
	subject := approval.Subject{Tool: "delete_file", Payload: `{"path":"big.bin"}`}
	reads := []fileRevision{{path: "big.bin", exists: true, content: nil, size: 256 << 20}}

	prev, err := approvalPreview(call, subject, reads, 4096)
	if err != nil {
		t.Fatalf("approvalPreview = %v", err)
	}
	if prev.Complete {
		t.Fatal("preview of an oversized delete is Complete; want it withheld")
	}
	if prev.Diff != nil {
		t.Errorf("preview of an oversized delete carries a diff; want none")
	}
	namesPath := false
	for _, note := range prev.Incomplete {
		if strings.Contains(note, "big.bin") {
			namesPath = true
		}
	}
	if !namesPath {
		t.Errorf("Incomplete = %q; want it to name big.bin", prev.Incomplete)
	}
}
