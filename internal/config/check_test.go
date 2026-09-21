package config

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func problemAt(problems []Problem, sev Severity, path string) *Problem {
	for i := range problems {
		if problems[i].Severity == sev && problems[i].Path == path {
			return &problems[i]
		}
	}
	return nil
}

// The file this checker was written for, 2026-09-21: provider "meta", and all
// four shard profiles plus default_shard naming an OpenRouter router id left
// over from an earlier provider. Nothing said a word; the Meta client rewrote
// the model on every call. It is refused, with every profile named at once.
func TestCheck_AShardModelItsProviderCannotServeIsRefused(t *testing.T) {
	stale := ShardProfile{Model: "stealth/union-alpha", Temperature: 0.7, TopP: 0.9}
	cfg := &UserConfig{
		Provider: "meta", Model: "muse-spark-1-contributor", MetaAPIKey: "k",
		ShardProfiles: map[string]ShardProfile{"coder": stale, "tester": stale, "reviewer": stale, "researcher": stale},
		DefaultShard:  &stale,
	}
	errs := Errors(cfg.Check(nil))
	if len(errs) != 5 {
		t.Fatalf("errors = %d, want one per profile and default_shard:\n%v", len(errs), errs)
	}
	p := problemAt(errs, SeverityError, "shard_profiles.coder.model")
	if p == nil || !strings.Contains(p.Message, "router id") || !strings.Contains(p.Fix, "shard_profiles.coder.provider") {
		t.Errorf("the coder profile's problem does not name the cause and the fix: %+v", p)
	}
}

// The same intent, said in full: the profile routes itself to the provider
// that serves its model. That is legal, and needs that provider's key.
func TestCheck_AProfileThatNamesItsProviderIsLegal(t *testing.T) {
	cfg := &UserConfig{
		Provider: "meta", Model: "muse-spark-1-contributor", MetaAPIKey: "k",
		ShardProfiles: map[string]ShardProfile{"coder": {Provider: "openrouter", Model: "stealth/union-alpha"}},
	}
	problems := cfg.Check(nil)
	if errs := Errors(problems); len(errs) != 0 {
		t.Fatalf("a self-routing profile was refused: %v", errs)
	}
	if problemAt(problems, SeverityWarning, "openrouter_api_key") == nil {
		t.Error("routing a shard to openrouter with no openrouter key was not reported")
	}

	cfg.ShardProfiles["coder"] = ShardProfile{Provider: "openrouter"}
	if problemAt(cfg.Check(nil), SeverityError, "shard_profiles.coder.model") == nil {
		t.Error("a profile naming a provider and no model was accepted: there is nothing for it to run")
	}
}

func TestCheck_ContradictionsAcrossSlots(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  UserConfig
		path string
	}{
		"main model of another vendor":   {UserConfig{Provider: "gemini", Model: "claude-x-1"}, "model"},
		"classification model elsewhere": {UserConfig{Provider: "meta", Model: "muse-1", ClassificationModel: "gemini-9-flash"}, "classification_model"},
		"worker model elsewhere":         {UserConfig{Provider: "meta", Worker: &WorkerLLMConfig{Provider: "xai", Model: "gpt-9"}}, "worker.model"},
		"planner inherits main provider": {UserConfig{Provider: "meta", Planner: &PlannerLLMConfig{Model: "grok-9"}}, "planner.model"},
		"unknown provider":               {UserConfig{Provider: "metta"}, "provider"},
		"unknown engine":                 {UserConfig{Engine: "gemini"}, "engine"},
		"unknown theme":                  {UserConfig{Theme: "solarized"}, "theme"},
		"unknown embedding provider":     {UserConfig{Embedding: &EmbeddingConfig{Provider: "openai"}}, "embedding.provider"},
		"top_p out of range":             {UserConfig{ShardProfiles: map[string]ShardProfile{"coder": {TopP: 1.5}}}, "shard_profiles.coder.top_p"},
	} {
		if problemAt(tc.cfg.Check(nil), SeverityError, tc.path) == nil {
			t.Errorf("%s: no error at %s; got %v", name, tc.path, tc.cfg.Check(nil))
		}
	}
}

// A router, a local runtime and an explicit endpoint may serve anything; a
// shard on a worker slot is judged against the worker's provider.
func TestCheck_WhatIsNotAContradiction(t *testing.T) {
	for name, cfg := range map[string]UserConfig{
		"openrouter serves router ids": {Provider: "openrouter", Model: "stealth/union-alpha"},
		"ollama runs any tag":          {Provider: "ollama", Model: "qwen9:4b"},
		"base_url may be a proxy":      {Provider: "openai", BaseURL: "https://proxy.example/v1", Model: "claude-x-1"},
		"worker endpoint may be proxy": {Provider: "meta", Worker: &WorkerLLMConfig{Provider: "openai", Endpoint: "https://p.example", Model: "glm-9"}},
		"shard follows the worker": {Provider: "meta", Worker: &WorkerLLMConfig{Provider: "openrouter", Model: "a/b"},
			ShardProfiles: map[string]ShardProfile{"coder": {Model: "stealth/union-alpha"}}},
		"empty shard model": {Provider: "meta", ShardProfiles: map[string]ShardProfile{"coder": {}}},
	} {
		if errs := Errors(cfg.Check(nil)); len(errs) != 0 {
			t.Errorf("%s: refused: %v", name, errs)
		}
	}
}

// LoadUserConfig is where the file is refused, and the error carries every
// contradiction: a user fixing a config does not find them one boot at a time.
func TestLoadUserConfig_RefusesAContradictoryFileNamingEveryError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"provider":"meta","model":"muse-1-contributor","theme":"solarized",
		"shard_profiles":{"coder":{"model":"stealth/union-alpha","temperature":0.7,"top_p":0.9,"max_retries":3,"enable_learning":true}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	var cerr *ConfigError
	if !errors.As(err, &cerr) {
		t.Fatalf("LoadUserConfig error = %v, want a *ConfigError", err)
	}
	if len(cerr.Problems) != 2 || !strings.Contains(err.Error(), "shard_profiles.coder.model") || !strings.Contains(err.Error(), "theme") {
		t.Errorf("the error does not carry both contradictions:\n%v", err)
	}
}

// Nothing is implicit without being listed: every configurable leaf the file
// does not mention is reported, from the schema's types, so a field added
// tomorrow is reported tomorrow.
func TestImplicitFields_ListsWhatTheFileLeavesToDefaults(t *testing.T) {
	raw := []byte(`{"provider":"meta","jit":{"token_budget":1000},"shard_profiles":{"coder":{"model":""}}}`)
	implicit := strings.Join(ImplicitFields(raw), "\n") + "\n"
	for _, want := range []string{"model\n", "engine\n", "jit.reserved_tokens\n", "embedding.ollama_model\n", "shard_profiles.coder.temperature\n", "\nintegrations."} {
		if !strings.Contains(implicit, want) {
			t.Errorf("implicit fields do not list %q", strings.TrimSpace(want))
		}
	}
	for _, not := range []string{"\nprovider\n", "jit.token_budget\n", "shard_profiles.coder.model\n"} {
		if strings.Contains("\n"+implicit, not) {
			t.Errorf("%q is in the file and was reported implicit", strings.TrimSpace(not))
		}
	}
}

// The full document has every top-level key of the schema, writes zero values
// down, keeps what the user set, and is itself a loadable config: it is meant
// to be merged into config.json by hand.
func TestFullJSON_EveryFieldPresentAndLoadable(t *testing.T) {
	raw := []byte(`{"provider":"meta","model":"muse-1-contributor","jit":{"token_budget":1234},
		"shard_profiles":{"reviewer":{"model":"","temperature":0.3,"top_p":0.9,"max_retries":2,"enable_learning":false}}}`)
	full, err := FullJSON(raw)
	if err != nil {
		t.Fatalf("FullJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(full, &doc); err != nil {
		t.Fatalf("FullJSON is not JSON: %v", err)
	}
	ut := reflect.TypeOf(UserConfig{})
	for i := 0; i < ut.NumField(); i++ {
		name := strings.Split(ut.Field(i).Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		if _, ok := doc[name]; !ok {
			t.Errorf("top-level key %q is missing from the full document", name)
		}
	}
	if doc["jit"].(map[string]any)["token_budget"].(float64) != 1234 {
		t.Error("the user's jit.token_budget was not kept")
	}
	reviewer := doc["shard_profiles"].(map[string]any)["reviewer"].(map[string]any)
	if v, ok := reviewer["enable_learning"]; !ok || v != false {
		t.Errorf("a false value was not written down: %v", reviewer)
	}
	if implicit := ImplicitFields(full); len(implicit) != 0 {
		t.Errorf("the full document still leaves %d field(s) implicit: %v", len(implicit), implicit)
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, full, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatalf("the full document does not load as a config: %v", err)
	}
}

// A profile becomes behaviour in one place. Model, provider and sampling all
// ride the context; an unset field attaches nothing.
func TestShardProfileContext_CarriesModelProviderAndSampling(t *testing.T) {
	ctx := ShardProfileContext(context.Background(), ShardProfile{Provider: " OpenRouter ", Model: "stealth/union-alpha", Temperature: 0.3})
	model, _ := types.ModelNameFromContext(ctx)
	provider, _ := types.ProviderFromContext(ctx)
	if model != "stealth/union-alpha" || provider != "openrouter" {
		t.Errorf("model=%q provider=%q", model, provider)
	}
	empty := ShardProfileContext(context.Background(), ShardProfile{})
	if _, set := types.ModelNameFromContext(empty); set {
		t.Error("an empty profile named a model")
	}
	if _, set := types.ProviderFromContext(empty); set {
		t.Error("an empty profile named a provider")
	}
}
