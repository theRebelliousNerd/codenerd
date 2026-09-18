package tactile

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// Every backend in this package caps captured output at MaxOutputBytes and
// records Truncated/TruncatedBytes. Until those flags reached the text, that
// was where the knowledge stopped: ActionResult carries a string, the tool
// result carries a string, and the model received the first 10 MiB of a build
// log framed as the whole build log. A model given a truncated `go test` run
// reports the packages it can see as the run's result, which is the specific
// failure the "never silently truncate" rule exists to prevent.
func TestToolResult_TruncationVisibleToModel(t *testing.T) {
	tests := []struct {
		name       string
		result     ExecutionResult
		wantMarker bool
		mustKeep   []string
	}{
		{
			name:     "an untruncated result is returned verbatim",
			result:   ExecutionResult{Combined: "ok\tcodenerd/internal/types\t3.7s\n"},
			mustKeep: []string{"ok\tcodenerd/internal/types"},
		},
		{
			name: "a truncated combined capture carries the marker and the byte count",
			result: ExecutionResult{
				Combined:       "HEAD of the build log\n",
				Truncated:      true,
				TruncatedBytes: 41_943_040,
			},
			wantMarker: true,
			mustKeep:   []string{"HEAD of the build log", "41943040"},
		},
		{
			name: "a truncated stdout+stderr capture carries it too",
			result: ExecutionResult{
				Stdout:         "compiled 41 packages\n",
				Stderr:         "vet: too many errors\n",
				Truncated:      true,
				TruncatedBytes: 2048,
			},
			wantMarker: true,
			mustKeep:   []string{"compiled 41 packages", "vet: too many errors", "2048"},
		},
		{
			name: "truncated with no byte count still announces the cut",
			result: ExecutionResult{
				Combined:  "partial\n",
				Truncated: true,
			},
			wantMarker: true,
			mustKeep:   []string{"partial", "not reported"},
		},
		{
			name: "truncated with nothing captured is the marker alone",
			result: ExecutionResult{
				Truncated:      true,
				TruncatedBytes: 900,
			},
			wantMarker: true,
			mustKeep:   []string{"900"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Output()
			if marked := types.IsClamped(got); marked != tt.wantMarker {
				t.Errorf("marker present = %v, want %v; output was:\n%s", marked, tt.wantMarker, got)
			}
			for _, want := range tt.mustKeep {
				if !strings.Contains(got, want) {
					t.Errorf("output lost %q:\n%s", want, got)
				}
			}
		})
	}
}

// Stdout and stderr also travel separately (VirtualStore.Exec hands them back
// as a pair), so the truncation has to survive that projection or it vanishes
// between the executor and whatever renders it.
func TestMarkTruncated_CarriesTheCutOnASeparateProjection(t *testing.T) {
	r := ExecutionResult{
		Stdout:         "out",
		Stderr:         "err",
		Truncated:      true,
		TruncatedBytes: 77,
	}
	if got := r.MarkTruncated(r.Stderr); !types.IsClamped(got) || !strings.Contains(got, "77") {
		t.Errorf("MarkTruncated dropped the announcement: %q", got)
	}

	clean := ExecutionResult{Stdout: "out", Stderr: "err"}
	if got := clean.MarkTruncated(clean.Stderr); got != "err" {
		t.Errorf("MarkTruncated altered an untruncated projection: %q", got)
	}
}
