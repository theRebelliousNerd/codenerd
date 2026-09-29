package config

import (
	"strings"
	"testing"
)

// The JIT injection bounds are tunables owned by .nerd/config.json. These
// pin their defaults and the GetJITConfig overlay so a zero value keeps
// meaning "unset", never "show nothing".
func TestDefaultJITConfig_InjectionLimits(t *testing.T) {
	got := DefaultJITConfig()
	want := map[string]int{
		"KernelContextRows":         60,
		"KernelContextRowChars":     1024,
		"KernelInjectedAtomChars":   16 * 1024,
		"SpecialistKnowledgeBlocks": 12,
		"SpecialistTopicChars":      200,
		"SpecialistBlockChars":      4 * 1024,
		"PredicateLimit":            100,
		"PredicateVecLimit":         200,
		"FallbackIdentityMaxBytes":  1024 * 1024,
	}
	seen := map[string]int{
		"KernelContextRows":         got.KernelContextRows,
		"KernelContextRowChars":     got.KernelContextRowChars,
		"KernelInjectedAtomChars":   got.KernelInjectedAtomChars,
		"SpecialistKnowledgeBlocks": got.SpecialistKnowledgeBlocks,
		"SpecialistTopicChars":      got.SpecialistTopicChars,
		"SpecialistBlockChars":      got.SpecialistBlockChars,
		"PredicateLimit":            got.PredicateLimit,
		"PredicateVecLimit":         got.PredicateVecLimit,
		"FallbackIdentityMaxBytes":  got.FallbackIdentityMaxBytes,
	}
	for name, w := range want {
		if seen[name] != w {
			t.Errorf("DefaultJITConfig().%s = %d, want %d", name, seen[name], w)
		}
	}
}

// A negative predicate bound is a contradiction that refuses the file with
// the key named; zero still means "unset" and takes the default.
func TestJITConfig_CheckPredicateBounds(t *testing.T) {
	if p := DefaultJITConfig().Check("jit"); len(p) != 0 {
		t.Errorf("defaults fail their own check: %+v", p)
	}
	if p := (JITConfig{}).Check("jit"); len(p) != 0 {
		t.Errorf("absent bounds fail their own check: %+v", p)
	}
	bad := JITConfig{PredicateLimit: -1, PredicateVecLimit: -2}
	problems := bad.Check("jit")
	if len(problems) != 2 {
		t.Fatalf("problems = %+v, want two errors", problems)
	}
	for _, p := range problems {
		if p.Severity != SeverityError {
			t.Errorf("problem %+v is not an error", p)
		}
	}
	paths := problems[0].Path + " " + problems[1].Path
	if !strings.Contains(paths, "jit.predicate_limit") || !strings.Contains(paths, "jit.predicate_vec_limit") {
		t.Errorf("problems name %q, want both predicate keys", paths)
	}
	wired := (&UserConfig{JIT: &JITConfig{PredicateLimit: -1}}).Check(nil)
	found := false
	for _, p := range wired {
		if p.Severity == SeverityError && p.Path == "jit.predicate_limit" {
			found = true
		}
	}
	if !found {
		t.Errorf("UserConfig.Check did not surface jit.predicate_limit: %+v", wired)
	}
}

func TestGetJITConfig_InjectionLimitsOverlay(t *testing.T) {
	tests := []struct {
		name string
		set  func(*JITConfig, int)
		get  func(JITConfig) int
		dflt int
	}{
		{"KernelContextRows", func(c *JITConfig, v int) { c.KernelContextRows = v }, func(c JITConfig) int { return c.KernelContextRows }, 60},
		{"KernelContextRowChars", func(c *JITConfig, v int) { c.KernelContextRowChars = v }, func(c JITConfig) int { return c.KernelContextRowChars }, 1024},
		{"KernelInjectedAtomChars", func(c *JITConfig, v int) { c.KernelInjectedAtomChars = v }, func(c JITConfig) int { return c.KernelInjectedAtomChars }, 16 * 1024},
		{"SpecialistKnowledgeBlocks", func(c *JITConfig, v int) { c.SpecialistKnowledgeBlocks = v }, func(c JITConfig) int { return c.SpecialistKnowledgeBlocks }, 12},
		{"SpecialistTopicChars", func(c *JITConfig, v int) { c.SpecialistTopicChars = v }, func(c JITConfig) int { return c.SpecialistTopicChars }, 200},
		{"SpecialistBlockChars", func(c *JITConfig, v int) { c.SpecialistBlockChars = v }, func(c JITConfig) int { return c.SpecialistBlockChars }, 4 * 1024},
		{"PredicateLimit", func(c *JITConfig, v int) { c.PredicateLimit = v }, func(c JITConfig) int { return c.PredicateLimit }, 100},
		{"PredicateVecLimit", func(c *JITConfig, v int) { c.PredicateVecLimit = v }, func(c JITConfig) int { return c.PredicateVecLimit }, 200},
		{"FallbackIdentityMaxBytes", func(c *JITConfig, v int) { c.FallbackIdentityMaxBytes = v }, func(c JITConfig) int { return c.FallbackIdentityMaxBytes }, 1024 * 1024},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absent := (&UserConfig{}).GetJITConfig()
			if tt.get(absent) != tt.dflt {
				t.Errorf("absent %s = %d, want default %d", tt.name, tt.get(absent), tt.dflt)
			}
			cfg := &UserConfig{JIT: &JITConfig{}}
			tt.set(cfg.JIT, 7)
			if got := tt.get(cfg.GetJITConfig()); got != 7 {
				t.Errorf("set %s = %d, want 7", tt.name, got)
			}
		})
	}
}
