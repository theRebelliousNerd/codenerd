package config

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
)

// Severity ranks what Check found.
type Severity string

const (
	// SeverityError is a contradiction: two settings that cannot both be meant,
	// or a value outside its vocabulary. LoadUserConfig refuses the file.
	SeverityError Severity = "error"
	// SeverityWarning is an incomplete setting: something a run will need and
	// the file does not say. The component that needs it refuses when reached.
	SeverityWarning Severity = "warning"
	// SeverityImplicit is a configurable field the file does not mention, so a
	// default the user never saw is in force.
	SeverityImplicit Severity = "implicit"
)

// Problem is one thing wrong with a config file, addressed by JSON path.
type Problem struct {
	Severity Severity
	Path     string // e.g. shard_profiles.coder.model
	Message  string
	Fix      string
}

func (p Problem) String() string {
	s := fmt.Sprintf("[%s] %s: %s", p.Severity, p.Path, p.Message)
	if p.Fix != "" {
		s += " -- " + p.Fix
	}
	return s
}

// ConfigError is what LoadUserConfig returns for a file with contradictions.
// It carries every one of them: a user fixing a config should not have to
// discover its errors one boot at a time.
type ConfigError struct {
	Path     string
	Problems []Problem
}

func (e *ConfigError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s has %d error(s) and was not loaded:", e.Path, len(e.Problems))
	for _, p := range e.Problems {
		b.WriteString("\n  " + p.String())
	}
	b.WriteString("\n  (`nerd config check` lists these with every warning and every defaulted field)")
	return b.String()
}

// modelFamilies maps the family a model name opens with to the provider that
// serves it. Families, not models: no entry names a version, so nothing here
// can become a default. A provider that routes to other vendors' models
// (routerProviders) or runs local tags is exempt, as is any provider reached
// through an explicit base_url or endpoint, which may be a proxy for anything.
var modelFamilies = map[string]string{
	"gemini":   "gemini",
	"claude":   "anthropic",
	"gpt":      "openai",
	"grok":     "xai",
	"glm":      "zai",
	"muse":     "meta",
	"kimi":     "moonshot",
	"moonshot": "moonshot",
	"qwen":     "dashscope",
}

// routerProviders accept model ids that belong to other vendors.
var routerProviders = map[string]bool{"openrouter": true, "ollama": true}

var knownProviders = map[string]bool{
	"anthropic": true, "openai": true, "gemini": true, "xai": true, "zai": true,
	"openrouter": true, "dashscope": true, "meta": true, "moonshot": true, "ollama": true,
}

// modelBelongsElsewhere reports why model cannot be served by provider, or "".
func modelBelongsElsewhere(provider, model string) string {
	provider, model = strings.ToLower(strings.TrimSpace(provider)), strings.ToLower(strings.TrimSpace(model))
	if provider == "" || model == "" || routerProviders[provider] {
		return ""
	}
	if strings.Contains(model, "/") {
		return fmt.Sprintf("%q is a router id (vendor/model), which only a routing provider such as openrouter serves; provider is %q", model, provider)
	}
	for family, owner := range modelFamilies {
		if strings.HasPrefix(model, family) && owner != provider {
			return fmt.Sprintf("%q is a %s model; provider is %q", model, owner, provider)
		}
	}
	return ""
}

// Check reads a config for what strict decoding cannot see: settings that
// contradict each other, settings a run will need and the file omits, and
// every configurable field the file leaves to a default. raw is the file's
// bytes (for the implicit-field pass); it may be nil.
//
// Steve, 2026-09-21, on finding every shard profile naming an OpenRouter model
// under provider "meta" with nothing having said a word: "there needs to be a
// system that checks and raises errors for fuckups in the config.json".
func (c *UserConfig) Check(raw []byte) []Problem {
	var out []Problem
	add := func(sev Severity, path, msg, fix string) {
		out = append(out, Problem{Severity: sev, Path: path, Message: msg, Fix: fix})
	}
	proxied := strings.TrimSpace(c.BaseURL) != ""

	// --- vocabulary ---
	if p := strings.TrimSpace(c.Provider); p != "" && !knownProviders[strings.ToLower(p)] {
		add(SeverityError, "provider", fmt.Sprintf("%q is not a provider codeNERD has", p), "one of "+strings.Join(sortedKeys(knownProviders), ", "))
	}
	switch strings.TrimSpace(c.Engine) {
	case "", "api", "claude-cli", "codex-cli", "xai-oauth":
	default:
		add(SeverityError, "engine", fmt.Sprintf("%q is not an engine", c.Engine), "api, claude-cli, codex-cli or xai-oauth")
	}
	switch strings.TrimSpace(c.Theme) {
	case "", "light", "dark":
	default:
		add(SeverityError, "theme", fmt.Sprintf("%q is not a theme", c.Theme), "light or dark")
	}

	// --- the main slot ---
	apiEngine := c.Engine == "" || c.Engine == "api"
	if !proxied {
		if why := modelBelongsElsewhere(c.Provider, c.Model); why != "" {
			add(SeverityError, "model", why, "name a model of that provider, or change provider")
		}
		if why := modelBelongsElsewhere(c.Provider, c.ClassificationModel); why != "" {
			add(SeverityError, "classification_model", why, "name a model of that provider")
		}
	}
	if apiEngine {
		if strings.TrimSpace(c.Provider) == "" {
			add(SeverityWarning, "provider", "engine is api and no provider is named", "set provider")
		}
		if strings.TrimSpace(c.Model) == "" {
			add(SeverityWarning, "model", "engine is api and no model is named; the client factory refuses to build without one", "set model")
		}
		if p := strings.ToLower(strings.TrimSpace(c.Provider)); p != "" && p != "ollama" && c.keyFor(p) == "" {
			add(SeverityWarning, p+"_api_key", fmt.Sprintf("provider is %q and it has no key in this file", p), "set "+p+"_api_key (or its environment variable)")
		}
	}

	// --- worker and planner slots ---
	shardProvider := c.Provider
	for _, slot := range []struct {
		name string
		cfg  *WorkerLLMConfig
	}{{"worker", c.Worker}, {"planner", (*WorkerLLMConfig)(c.Planner)}} {
		if slot.cfg == nil {
			continue
		}
		provider := strings.TrimSpace(slot.cfg.Provider)
		if provider != "" && !knownProviders[strings.ToLower(provider)] {
			add(SeverityError, slot.name+".provider", fmt.Sprintf("%q is not a provider codeNERD has", provider), "")
		}
		effective := provider
		if effective == "" {
			effective = c.Provider
		}
		if strings.TrimSpace(slot.cfg.Endpoint) == "" && !(provider == "" && proxied) {
			if why := modelBelongsElsewhere(effective, slot.cfg.Model); why != "" {
				add(SeverityError, slot.name+".model", why, "name a model of that provider, or set "+slot.name+".provider")
			}
		}
		if provider != "" && strings.TrimSpace(slot.cfg.Model) == "" {
			add(SeverityWarning, slot.name+".model", "the slot names a provider and no model", "set "+slot.name+".model")
		}
		if p := strings.ToLower(provider); p != "" && p != "ollama" && c.keyFor(p) == "" {
			add(SeverityWarning, p+"_api_key", fmt.Sprintf("%s.provider is %q and it has no key in this file", slot.name, p), "set "+p+"_api_key")
		}
		if slot.name == "worker" && provider != "" {
			shardProvider = provider
		}
	}

	// --- shard profiles: served by the worker slot, or the main one ---
	profiles := map[string]ShardProfile{}
	for name, p := range c.ShardProfiles {
		profiles["shard_profiles."+name] = p
	}
	if c.DefaultShard != nil {
		profiles["default_shard"] = *c.DefaultShard
	}
	for _, path := range sortedKeys(profiles) {
		p := profiles[path]
		own := strings.ToLower(strings.TrimSpace(p.Provider))
		switch {
		case own != "" && !knownProviders[own]:
			add(SeverityError, path+".provider", fmt.Sprintf("%q is not a provider codeNERD has", p.Provider), "")
		case own != "":
			// The profile routes itself: its model is judged against its own provider.
			if strings.TrimSpace(p.Model) == "" {
				add(SeverityError, path+".model", fmt.Sprintf("the profile routes to %q and names no model for it to run", own), "set "+path+".model")
			} else if why := modelBelongsElsewhere(own, p.Model); why != "" {
				add(SeverityError, path+".model", why, "")
			}
			if own != "ollama" && c.keyFor(own) == "" {
				add(SeverityWarning, own+"_api_key", fmt.Sprintf("%s.provider is %q and it has no key in this file", path, own), "set "+own+"_api_key")
			}
		case !proxied:
			if why := modelBelongsElsewhere(shardProvider, p.Model); why != "" {
				add(SeverityError, path+".model",
					why+" (with no provider of its own a shard runs on the worker slot, or the main provider when there is none)",
					`set `+path+`.provider to the provider that serves this model, or set the model to "" to run the serving client's`)
			}
		}
		if p.Temperature < 0 || p.Temperature > 2 {
			add(SeverityError, path+".temperature", fmt.Sprintf("%v is outside 0-2", p.Temperature), "")
		}
		if p.TopP < 0 || p.TopP > 1 {
			add(SeverityError, path+".top_p", fmt.Sprintf("%v is outside 0-1", p.TopP), "")
		}
	}

	// --- embedding ---
	if c.Embedding != nil {
		switch c.Embedding.Provider {
		case "", "ollama", "genai":
		default:
			add(SeverityError, "embedding.provider", fmt.Sprintf("%q is not an embedding provider", c.Embedding.Provider), "ollama or genai")
		}
	}
	emb := c.GetEmbeddingConfig()
	if missing := emb.MissingModel(); missing != "" {
		add(SeverityWarning, "embedding", missing, "")
	}

	// --- execution ---
	// A secret pattern that path.Match cannot parse matches nothing, so the
	// file it was written to protect would be readable while the config says
	// it is not. That is a contradiction, and the file is refused.
	if c.Execution != nil {
		for i, pattern := range c.Execution.SecretPaths {
			if strings.TrimSpace(pattern) == "" {
				add(SeverityError, fmt.Sprintf("execution.secret_paths[%d]", i), "an empty pattern protects nothing", "remove it")
				continue
			}
			if _, err := path.Match(pattern, ""); err != nil {
				add(SeverityError, fmt.Sprintf("execution.secret_paths[%d]", i), fmt.Sprintf("%q is not a valid pattern: %v", pattern, err), "path.Match syntax: *, ?, [a-z]")
			}
		}
	}

	// --- campaign ---
	if c.Campaign != nil {
		out = append(out, c.Campaign.Check("campaign")...)
	}

	// --- session ---
	if c.Session != nil {
		out = append(out, c.Session.Check("session")...)
	}

	// --- routing ---
	if c.Routing != nil {
		out = append(out, c.Routing.Check("routing")...)
	}

	// --- ui ---
	if c.UI != nil {
		out = append(out, c.UI.Check("ui")...)
	}

	// --- retrieval ---
	if c.Retrieval != nil {
		out = append(out, c.Retrieval.Check("retrieval")...)
	}

	// --- delegation ---
	if c.Delegation != nil {
		out = append(out, c.Delegation.Check("delegation")...)
	}

	// --- working ---
	if c.Working != nil {
		out = append(out, c.Working.Check("working")...)
	}

	// --- everything the file leaves to a default ---
	if raw != nil {
		for _, path := range ImplicitFields(raw) {
			add(SeverityImplicit, path, "not in the file: a default is in force", "`nerd config full` prints the file with every field and the value in force")
		}
	}
	return out
}

// keyFor is the API key this file holds for provider ("" when none).
func (c *UserConfig) keyFor(provider string) string {
	keys := map[string]string{
		"anthropic": c.AnthropicAPIKey, "openai": c.OpenAIAPIKey, "gemini": c.GeminiAPIKey,
		"xai": c.XAIAPIKey, "zai": c.ZAIAPIKey, "openrouter": c.OpenRouterAPIKey,
		"dashscope": c.DashScopeAPIKey, "meta": c.MetaAPIKey, "moonshot": c.MoonshotAPIKey,
	}
	if k := strings.TrimSpace(keys[provider]); k != "" {
		return k
	}
	return strings.TrimSpace(c.APIKey)
}

// Errors filters problems to the ones that refuse the file.
func Errors(problems []Problem) []Problem {
	var out []Problem
	for _, p := range problems {
		if p.Severity == SeverityError {
			out = append(out, p)
		}
	}
	return out
}

// ImplicitFields lists every configurable leaf of UserConfig that raw does not
// mention, as JSON paths. It walks the struct's types, so a field added to the
// schema is reported the day it is added; nothing here is a list to maintain.
// Map-valued sections (shard_profiles, integrations) are reported as a whole
// when absent, and per entry against their element type when present.
func ImplicitFields(raw []byte) []string {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []string
	walkImplicit(reflect.TypeOf(UserConfig{}), doc, "", &out)
	sort.Strings(out)
	return out
}

func walkImplicit(t reflect.Type, doc map[string]any, prefix string, out *[]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" || !field.IsExported() {
			continue
		}
		path := prefix + name
		value, present := doc[name]
		ft := field.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.Struct:
			child, _ := value.(map[string]any)
			if !present || child == nil {
				child = map[string]any{}
			}
			walkImplicit(ft, child, path+".", out)
		case reflect.Map:
			entries, _ := value.(map[string]any)
			if !present || entries == nil {
				*out = append(*out, path)
				continue
			}
			et := ft.Elem()
			for et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() != reflect.Struct {
				continue
			}
			for key, entry := range entries {
				if child, ok := entry.(map[string]any); ok {
					walkImplicit(et, child, path+"."+key+".", out)
				}
			}
		default:
			if !present {
				*out = append(*out, path)
			}
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DecodeUserConfig decodes a config file's bytes the way LoadUserConfig does
// (removed keys named, unknown keys refused) and stops there: no Check, no
// side effects. It is for reading a file the loader would refuse.
func DecodeUserConfig(raw []byte) (*UserConfig, error) {
	if err := rejectRemovedKeys(raw); err != nil {
		return nil, err
	}
	cfg := &UserConfig{}
	if err := decodeStrictJSON(raw, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
