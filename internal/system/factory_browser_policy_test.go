package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

func TestBootPublishesConfiguredBrowserPolicyBeforeManagerUse(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o700); err != nil {
		t.Fatal(err)
	}
	userCfg := config.DefaultUserConfig()
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	userCfg.Browser.Policy.SlowAPIMs = browserPolicyInt(100)
	userCfg.Browser.Policy.PriorityButton = browserPolicyInt(99)

	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace: workspace,
		DisableSystemShards: []string{
			"constitution_gate",
			"perception_firewall",
			"executive_policy",
			"session_planner",
			"tactile_router",
			"campaign_runner",
			"mangle_repair",
			"legislator",
		},
		UserConfigOverride: userCfg,
		LLMClientOverride:  &MockLLMClient{},
	})
	if err != nil {
		t.Fatalf("normal boot with configured browser policy: %v", err)
	}
	t.Cleanup(func() {
		if err := cortex.Close(); err != nil {
			t.Error(err)
		}
	})
	if cortex.BrowserManager == nil {
		t.Fatal("normal boot did not bind the browser manager")
	}

	params := userCfg.GetBrowserConfig().Policy.Params()
	if len(params) != 17 {
		t.Fatalf("browser policy params=%d, want all 17 required values", len(params))
	}
	rows, err := cortex.Kernel.Query(config.ConfigParamPredicate)
	if err != nil {
		t.Fatalf("query live config_param rows: %v", err)
	}
	got := make(map[string]int64)
	for _, row := range rows {
		if len(row.Args) != 2 {
			continue
		}
		key := types.ExtractString(row.Args[0])
		if strings.HasPrefix(key, "/browser_") {
			value, ok := row.Args[1].(int64)
			if !ok {
				t.Fatalf("config_param %s has non-integer value %T", key, row.Args[1])
			}
			got[key] = value
		}
	}
	if len(got) != 17 {
		t.Fatalf("live browser config_param rows=%d, want 17: %v", len(got), got)
	}
	for _, param := range params {
		if got[param.Key] != param.Value {
			t.Errorf("%s=%d, want configured value %d", param.Key, got[param.Key], param.Value)
		}
	}

	for _, fact := range []types.Fact{
		{Predicate: "interactive", Args: []any{"session", "button", "button", "Continue", "click"}},
		{Predicate: "element_enabled", Args: []any{"session", "button", "true"}},
		{Predicate: "net_request", Args: []any{"session", "request", "GET", "https://example.test/api", "XHR", int64(10)}},
		{Predicate: "net_response", Args: []any{"session", "request", int64(200), int64(0), int64(150)}},
	} {
		if err := cortex.Kernel.Assert(fact); err != nil {
			t.Fatalf("assert browser policy acceptance fact %s: %v", fact.Predicate, err)
		}
	}
	assertBrowserPolicyRow(t, cortex.Kernel, "action_candidate", []any{"session", "button", "Continue", "click", int64(99), "enabled_button"})
	assertBrowserPolicyRow(t, cortex.Kernel, "slow_api", []any{"session", "request", "https://example.test/api", int64(150)})
}

func TestInitAutopoiesisAndBrowserRejectsInvalidPolicyBeforeBinding(t *testing.T) {
	userCfg := config.DefaultUserConfig()
	userCfg.Browser.Policy.PriorityButton = browserPolicyInt(0)
	bctx := &bootContext{
		workspace: t.TempDir(),
		appCfg:    userCfg,
	}

	err := initAutopoiesisAndBrowser(bctx)
	if err == nil || !strings.Contains(err.Error(), "browser.policy.priority_button") {
		t.Fatalf("invalid browser policy error=%v, want priority_button validation", err)
	}
	if bctx.browserMgr != nil || bctx.poiesis != nil {
		t.Fatalf("invalid browser policy partially bound runtime: browser=%v poiesis=%v", bctx.browserMgr, bctx.poiesis)
	}
}

func TestInitAutopoiesisAndBrowserPropagatesPolicyPublicationFailure(t *testing.T) {
	assertErr := errors.New("kernel rejected config_param")
	kernel := &rejectingBrowserParamKernel{MockSystemKernel: &MockSystemKernel{}, err: assertErr}
	bctx := &bootContext{
		workspace: t.TempDir(),
		appCfg:    config.DefaultUserConfig(),
		kernel:    kernel,
	}

	err := initAutopoiesisAndBrowser(bctx)
	if err == nil || !strings.Contains(err.Error(), "publish browser policy parameters") || !strings.Contains(err.Error(), "config_param /browser_slow_api_ms") {
		t.Fatalf("policy publication error=%v, want visible config_param failure", err)
	}
	if bctx.browserMgr != nil || bctx.poiesis != nil {
		t.Fatalf("failed browser policy publication partially bound runtime: browser=%v poiesis=%v", bctx.browserMgr, bctx.poiesis)
	}
}

type rejectingBrowserParamKernel struct {
	*MockSystemKernel
	err error
}

func (k *rejectingBrowserParamKernel) Assert(fact core.Fact) error {
	if fact.Predicate == config.ConfigParamPredicate {
		return k.err
	}
	return k.MockSystemKernel.Assert(fact)
}

func assertBrowserPolicyRow(t *testing.T, kernel core.Kernel, predicate string, want []any) {
	t.Helper()
	rows, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	for _, row := range rows {
		if len(row.Args) != len(want) {
			continue
		}
		matched := true
		for i := range want {
			if row.Args[i] != want[i] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("%s rows=%v, want row %v", predicate, rows, want)
}

func browserPolicyInt(value int) *int { return &value }
