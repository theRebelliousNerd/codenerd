package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
)

func TestReflectionBacklog_ProgressWithFailurePriority(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		successes, failures     int
		watermark, wantBatch    int
		disabled, missingEngine bool
		drain, newerFailures    bool
	}{
		{name: "301_successes", successes: 301, watermark: 300, wantBatch: 16, drain: true},
		{name: "mixed_older_failures", successes: 301, failures: 3, watermark: 300, wantBatch: 16},
		{name: "mixed_newer_failures", successes: 301, failures: 3, watermark: 300, wantBatch: 16, newerFailures: true},
		{name: "failure_batch_full", successes: 301, failures: 20, watermark: 300, wantBatch: 16},
		{name: "at_watermark", successes: 300, watermark: 300, wantBatch: 32},
		{name: "watermark_off", successes: 301, watermark: 0, wantBatch: 32},
		{name: "disabled", successes: 301, watermark: 300, disabled: true},
		{name: "missing_engine", successes: 301, watermark: 300, missingEngine: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "backlog.db")
			s, err := NewLocalStore(path)
			if err != nil {
				t.Fatal(err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					_ = s.Close()
				}
			})
			var embedded []string
			task := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
			engine := &MockEmbeddingEngine{
				NameFunc: func() string { return "backlog-fixture" },
				EmbedWithTaskFunc: func(ctx context.Context, text, gotTask string) ([]float32, error) {
					if gotTask != task {
						t.Errorf("task = %q, want %q", gotTask, task)
					}
					embedded = append(embedded, text)
					return []float32{1, 0, 0, 0}, nil
				},
			}
			cfg := config.DefaultReflectionConfig()
			if cfg.BacklogWatermark != 300 {
				t.Fatal("fixture must exercise the default 300 watermark")
			}
			cfg.BacklogWatermark = tc.watermark
			cfg.Enabled = !tc.disabled
			// Attach the fixture under the store lock without starting the
			// ticker/backfill goroutines. Invoke the actual worker cycle.
			s.mu.Lock()
			s.reflectionCfg = &cfg
			if !tc.missingEngine {
				s.embeddingEngine = engine
			}
			s.mu.Unlock()
			for _, group := range []struct {
				prefix string
				n      int
				ok     bool
			}{{"failureprobe", tc.failures, false}, {"successprobe", tc.successes, true}} {
				for i := range group.n {
					id := fmt.Sprintf("%s%03d", group.prefix, i)
					if err := s.traceStore.StoreReasoningTrace(&ReasoningTrace{ID: id, ShardType: "coder", TaskContext: id, Success: group.ok}); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Older failures prove priority over newest-first selection; newer
			// ones also appear in the fill query and must be deduplicated.
			failureYear := 2000
			if tc.newerFailures {
				failureYear = 2100
			}
			if _, err := s.db.Exec("UPDATE reasoning_traces SET created_at = ? WHERE success = 0", time.Date(failureYear, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
				t.Fatal(err)
			}
			backlog := func() int {
				t.Helper()
				n, err := s.traceStore.CountTraceEmbeddingBacklog(engine.Name(), engine.Dimensions(), task)
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
			initial := backlog()
			s.processReflectionCycle()
			if got := len(embedded); got != tc.wantBatch {
				t.Fatalf("backlog=%d, watermark=%d: embedded %d, want %d; remaining=%d", initial, tc.watermark, got, tc.wantBatch, backlog())
			}
			if remaining := backlog(); remaining != initial-tc.wantBatch {
				t.Fatalf("backlog %d -> %d, want %d", initial, remaining, initial-tc.wantBatch)
			}
			for i, text := range embedded {
				if tc.watermark > 0 && initial > tc.watermark {
					wantFailure := i < min(tc.failures, tc.wantBatch)
					if strings.Contains(text, "Outcome: failure") != wantFailure {
						t.Errorf("embedding %d violates failure priority: %s", i, text)
					}
				}
			}
			if !tc.drain {
				return
			}
			for cycle := 0; backlog() > 0 && cycle < 20; cycle++ {
				before, calls := backlog(), len(embedded)
				s.processReflectionCycle()
				after := backlog()
				if after >= before || len(embedded)-calls > 32 {
					t.Fatalf("cycle stalled or exceeded batch: %d -> %d, calls=%d", before, after, len(embedded)-calls)
				}
			}
			if backlog() != 0 || len(embedded) != tc.successes {
				t.Fatalf("idle backlog did not drain exactly once: remaining=%d calls=%d", backlog(), len(embedded))
			}
			seen := map[string]bool{}
			for _, text := range embedded {
				if seen[text] {
					t.Fatal("the same trace was embedded twice")
				}
				seen[text] = true
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			closed = true
			reopened, err := NewLocalStore(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			hits, err := reopened.RecallTracesLexical("successprobe000", 5)
			if err != nil || len(hits) != 1 || hits[0].TraceID != "successprobe000" || hits[0].EmbeddingModelID != engine.Name() || hits[0].EmbeddingTask != task || hits[0].EmbeddingDim != 4 {
				t.Fatalf("refreshed trace not recallable with its identity after reopening: %+v / %v", hits, err)
			}
			negative, err := reopened.RecallTracesLexical("absentprobe", 5)
			if err != nil || len(negative) != 0 {
				t.Fatalf("negative recall = %+v / %v", negative, err)
			}
			semantic, err := reopened.RecallTracesByEmbedding([]float32{1, 0, 0, 0}, 301)
			if err != nil || len(semantic) != tc.successes {
				t.Fatalf("refreshed traces not in reopened ANN index: hits=%d / %v", len(semantic), err)
			}
		})
	}
}
