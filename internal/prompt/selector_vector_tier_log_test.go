package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/logging"
)

// The "Vector tier:" line is what the live validation of the selector reads:
// after af736619 the check is "run reembed, run a fix, read Vector tier: N
// scored, M eligible, floor=..., kept K". When the search returns no scores --
// every stored vector unstamped, the state before reembed -- the tier returned
// early and logged nothing, so the absence of the line could not be told from
// the tier never running. Measured 2026-09-17: vector_ms:25 in every compile
// record, and no Vector tier line in the session's jit.log.
func TestVectorTier_LogsWhenItHasNothingToRank(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := `{"logging":{"debug_mode":true,"level":"debug"}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{DebugMode: false})
		logging.ClearInjectedConfig()
	})

	var s *AtomSelector
	if kept := s.topKEligibleVectorScores(map[string]float64{}, []*PromptAtom{{ID: "a"}}, nil, 10); kept != nil {
		t.Fatalf("no scores must keep nothing, got %v", kept)
	}

	logging.CloseAll()
	matches, err := filepath.Glob(filepath.Join(ws, ".nerd", "logs", "*_jit.log"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("the vector tier returned early without writing a jit log line")
	}
	var logged strings.Builder
	for _, m := range matches {
		b, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read %s: %v", m, err)
		}
		logged.Write(b)
	}
	if !strings.Contains(logged.String(), "Vector tier: nothing to rank (0 scored, 1 flesh atoms)") {
		t.Errorf("jit.log does not say the vector channel contributed nothing; got: %s", logged.String())
	}
}
