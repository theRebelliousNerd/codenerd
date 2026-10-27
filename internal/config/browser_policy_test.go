package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestBrowserPolicyParamsMatchRequiredPolicyInputs(t *testing.T) {
	params := BrowserPolicyConfig{}.Params()
	want := []Param{
		{Key: "/browser_slow_api_ms", Value: 1000},
		{Key: "/browser_causal_bucket_ms", Value: 2000},
		{Key: "/browser_unhydrated_age_ms", Value: 3000},
		{Key: "/browser_attended_navigation_window_ms", Value: 0},
		{Key: "/browser_priority_primary", Value: 100},
		{Key: "/browser_priority_button", Value: 80},
		{Key: "/browser_priority_input", Value: 78},
		{Key: "/browser_priority_select", Value: 72},
		{Key: "/browser_priority_checkbox", Value: 68},
		{Key: "/browser_priority_radio", Value: 66},
		{Key: "/browser_priority_link", Value: 60},
		{Key: "/browser_priority_tab", Value: 64},
		{Key: "/browser_priority_combobox_click", Value: 62},
		{Key: "/browser_priority_combobox_type", Value: 76},
		{Key: "/browser_priority_menuitem", Value: 60},
		{Key: "/browser_priority_option", Value: 58},
		{Key: "/browser_priority_clickable", Value: 56},
	}
	if !reflect.DeepEqual(params, want) {
		t.Fatalf("BrowserPolicyConfig.Params() = %#v, want %#v", params, want)
	}

	facts := ParamFacts(params)
	if len(facts) != len(want) {
		t.Fatalf("ParamFacts returned %d facts, want %d", len(facts), len(want))
	}
	for i, fact := range facts {
		if fact.Predicate != ConfigParamPredicate || len(fact.Args) != 2 {
			t.Fatalf("fact[%d] has wrong shape: %#v", i, fact)
		}
		key, ok := fact.Args[0].(types.MangleAtom)
		if !ok || string(key) != want[i].Key {
			t.Fatalf("fact[%d] key = %#v, want name constant %q", i, fact.Args[0], want[i].Key)
		}
		if value, ok := fact.Args[1].(int64); !ok || value != want[i].Value {
			t.Fatalf("fact[%d] value = %#v, want %d", i, fact.Args[1], want[i].Value)
		}
	}
}

func TestGetBrowserConfigPreservesAndPublishesPolicyValues(t *testing.T) {
	defaults := (&UserConfig{}).GetBrowserConfig()
	if problems := defaults.Policy.Check("browser.policy"); len(problems) != 0 {
		t.Fatalf("default browser policy has validation errors: %+v", problems)
	}
	if defaults.Policy.SlowAPIMs == nil || *defaults.Policy.SlowAPIMs != 1000 {
		t.Fatalf("default slow API threshold = %v, want 1000", defaults.Policy.SlowAPIMs)
	}

	// This is the same public path used by a loaded JSON config and then by the
	// system factory. Partial policy values retain their explicit values while
	// every omitted parameter receives its data default.
	input := BrowserPolicyConfig{
		SlowAPIMs:             browserIntPointer(1750),
		PriorityComboboxClick: browserIntPointer(91),
	}
	got := (&UserConfig{Browser: &BrowserAutomationConfig{Policy: input}}).GetBrowserConfig().Policy
	params := got.Params()
	values := make(map[string]int64, len(params))
	for _, p := range params {
		values[p.Key] = p.Value
	}
	if len(values) != 17 || values["/browser_slow_api_ms"] != 1750 || values["/browser_priority_combobox_click"] != 91 {
		t.Fatalf("custom browser params were not preserved: %#v", values)
	}
	if values["/browser_causal_bucket_ms"] != 2000 || values["/browser_priority_button"] != 80 {
		t.Fatalf("omitted browser params did not get defaults: %#v", values)
	}

	// Normalization must not expose pointer aliases back into caller-owned
	// config, since factory consumers keep the resolved snapshot.
	*input.SlowAPIMs = 9999
	if got.Params()[0].Value != 1750 {
		t.Fatal("resolved browser policy shares mutable pointers with its source")
	}
}

func TestBrowserPolicyCheckRejectsInvalidThresholdsAndPriorities(t *testing.T) {
	tests := []struct {
		name string
		set  func(*BrowserPolicyConfig)
	}{
		{"slow_api_ms", func(c *BrowserPolicyConfig) { c.SlowAPIMs = browserIntPointer(0) }},
		{"causal_bucket_ms", func(c *BrowserPolicyConfig) { c.CausalBucketMs = browserIntPointer(-1) }},
		{"unhydrated_age_ms", func(c *BrowserPolicyConfig) { c.UnhydratedAgeMs = browserIntPointer(0) }},
		{"priority_primary", func(c *BrowserPolicyConfig) { c.PriorityPrimary = browserIntPointer(0) }},
		{"priority_button", func(c *BrowserPolicyConfig) { c.PriorityButton = browserIntPointer(0) }},
		{"priority_input", func(c *BrowserPolicyConfig) { c.PriorityInput = browserIntPointer(0) }},
		{"priority_select", func(c *BrowserPolicyConfig) { c.PrioritySelect = browserIntPointer(0) }},
		{"priority_checkbox", func(c *BrowserPolicyConfig) { c.PriorityCheckbox = browserIntPointer(0) }},
		{"priority_radio", func(c *BrowserPolicyConfig) { c.PriorityRadio = browserIntPointer(0) }},
		{"priority_link", func(c *BrowserPolicyConfig) { c.PriorityLink = browserIntPointer(0) }},
		{"priority_tab", func(c *BrowserPolicyConfig) { c.PriorityTab = browserIntPointer(0) }},
		{"priority_combobox_click", func(c *BrowserPolicyConfig) { c.PriorityComboboxClick = browserIntPointer(0) }},
		{"priority_combobox_type", func(c *BrowserPolicyConfig) { c.PriorityComboboxType = browserIntPointer(0) }},
		{"priority_menuitem", func(c *BrowserPolicyConfig) { c.PriorityMenuitem = browserIntPointer(0) }},
		{"priority_option", func(c *BrowserPolicyConfig) { c.PriorityOption = browserIntPointer(0) }},
		{"priority_clickable", func(c *BrowserPolicyConfig) { c.PriorityClickable = browserIntPointer(0) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultBrowserPolicyConfig()
			tc.set(&policy)
			problems := policy.Check("browser.policy")
			if len(problems) != 1 {
				t.Fatalf("Check returned %+v, want one error", problems)
			}
			if problems[0].Severity != SeverityError || problems[0].Path != "browser.policy."+tc.name {
				t.Fatalf("Check returned %+v, want an error at browser.policy.%s", problems[0], tc.name)
			}
		})
	}

	zeroWindow := DefaultBrowserPolicyConfig()
	zeroWindow.AttendedNavigationWindowMs = browserIntPointer(0)
	if problems := zeroWindow.Check("browser.policy"); len(problems) != 0 {
		t.Fatalf("zero attended-navigation window should be valid: %+v", problems)
	}
	zeroWindow.AttendedNavigationWindowMs = browserIntPointer(-1)
	if problems := zeroWindow.Check("browser.policy"); len(problems) != 1 || problems[0].Path != "browser.policy.attended_navigation_window_ms" {
		t.Fatalf("negative attended-navigation window was not rejected: %+v", problems)
	}
}

func TestLoadUserConfigRejectsExplicitZeroBrowserPolicyValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"browser":{"policy":{"slow_api_ms":0}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "browser.policy.slow_api_ms") {
		t.Fatalf("LoadUserConfig error = %v, want visible browser policy path", err)
	}
}

func TestLoadUserConfigBrowserPolicyValuesReachParams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"browser":{"policy":{"slow_api_ms":1750,"priority_combobox_type":83}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	params := loaded.GetBrowserConfig().Policy.Params()
	values := make(map[string]int64, len(params))
	for _, p := range params {
		values[p.Key] = p.Value
	}
	if len(values) != 17 || values["/browser_slow_api_ms"] != 1750 || values["/browser_priority_combobox_type"] != 83 {
		t.Fatalf("loaded browser policy values did not reach Params: %#v", values)
	}
	if values["/browser_causal_bucket_ms"] != 2000 {
		t.Fatalf("omitted policy value did not receive its pinned default: %#v", values)
	}
}
