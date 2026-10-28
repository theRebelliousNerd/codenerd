package system

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/tools/research"
)

// chromeProcessesUnder lists live processes whose command line mentions dir.
// Chrome launches with --user-data-dir under os.TempDir(), so pointing TMPDIR
// at a private directory makes this the process-exit witness for exactly the
// browsers this test started.
func chromeProcessesUnder(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("cannot inspect processes: %v", err)
	}
	var found []string
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.ReplaceAll(string(raw), "\x00", " ")
		if strings.Contains(cmdline, dir) {
			found = append(found, entry.Name()+": "+cmdline)
		}
	}
	return found
}

func waitNoChrome(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		left := chromeProcessesUnder(t, dir)
		if len(left) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("research Chrome outlived its runtime: %v", left)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func privateTempDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("process inspection uses /proc")
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", dir)
	return dir
}

func pageServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body><h1>owned by cortex</h1></body></html>`)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func disabledShards() []string {
	return []string{
		"constitution_gate", "perception_firewall", "executive_policy", "world_model_ingestor",
		"session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator",
	}
}

// The default research read launches its own headless Chrome. Closing the
// Cortex that booted the runtime must end that process: the interactive
// manager's shutdown never reached it.
func TestCortexClose_EndsResearchHeadlessChrome(t *testing.T) {
	tmp := privateTempDir(t)
	userCfg := config.DefaultUserConfig()
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}

	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace:           t.TempDir(),
		DisableSystemShards: disabledShards(),
		UserConfigOverride:  userCfg,
		LLMClientOverride:   &MockLLMClient{},
	})
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = cortex.Close()
		}
	})
	if cortex.researchBrowser == nil {
		t.Fatal("boot did not hand the research runtime to the Cortex")
	}

	ts := pageServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := research.WebFetchTool().Execute(ctx, map[string]any{"url": ts.URL})
	if err != nil {
		if strings.Contains(err.Error(), "failed to launch Chrome") {
			t.Skipf("Chrome cannot be launched: %v", err)
		}
		t.Fatalf("web_fetch through the booted Cortex: %v", err)
	}
	if !strings.Contains(out, "owned by cortex") {
		t.Fatalf("web_fetch = %q", out)
	}
	if len(chromeProcessesUnder(t, tmp)) == 0 {
		t.Fatal("the research read started no Chrome under the private temp dir; the witness is blind")
	}

	if err := cortex.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	closed = true
	waitNoChrome(t, tmp)
}

// Boot rollback goes through the same ownership path as Close, so a failure
// after the research runtime was bound cannot leave its Chrome behind.
func TestBootRollback_EndsResearchHeadlessChrome(t *testing.T) {
	tmp := privateTempDir(t)
	userCfg := config.DefaultUserConfig()
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	forced := errors.New("forced failure after research runtime binding")
	ts := pageServer(t)

	steps := append(defaultBootSteps(), bootStep{
		name: "research rollback probe",
		run: func(bctx *bootContext) error {
			if bctx.researchBrowser == nil {
				t.Error("boot context holds no research runtime")
				return forced
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			// The read may fail on the stub kernel; the launch is what matters.
			_, _ = research.WebFetchTool().Execute(ctx, map[string]any{"url": ts.URL})
			if len(chromeProcessesUnder(t, tmp)) == 0 {
				t.Skip("Chrome cannot be launched here")
			}
			return forced
		},
	})
	cortex, err := bootCortexWithSteps(context.Background(), BootConfig{
		Workspace:           t.TempDir(),
		APIKey:              "test-key",
		DisableSystemShards: disabledShards(),
		UserConfigOverride:  userCfg,
		LLMClientOverride:   &MockLLMClient{},
		KernelOverride:      &MockSystemKernel{},
	}, steps)
	if cortex != nil {
		t.Fatal("late boot failure returned a partial Cortex")
	}
	if !errors.Is(err, forced) {
		t.Fatalf("boot error = %v, want the forced failure", err)
	}
	waitNoChrome(t, tmp)
}
