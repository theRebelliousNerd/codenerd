//go:build jevlive

package turnjudge

// Live probe of the pin effect reading:
//
//	go test -tags jevlive ./internal/turnjudge/ -run LivePinEffect -v

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

var pinEffectCases = []struct {
	name, label, with, without string
	observable                 bool
}{
	{"loop-rewrite", "sum.go: Sum",
		"for _, x := range xs {\n\ts += x\n}", "for i := 0; i < len(xs); i++ {\n\ts += xs[i]\n}", false},
	{"cache-added", "lookup.go: Find",
		"if v, ok := cache[k]; ok {\n\treturn v\n}\nv := slowFind(k)\ncache[k] = v\nreturn v", "return slowFind(k)", false},
	{"loadorstore-race", "registry.go: Register (if loaded held at false)",
		"if e, ok := m.Load(k); ok {\n\treturn e.(*Entry)\n}\nv := newEntry(k)\nactual, loaded := m.LoadOrStore(k, v)\nif loaded {\n\tv.Close()\n\treturn actual.(*Entry)\n}\nreturn v",
		"if e, ok := m.Load(k); ok {\n\treturn e.(*Entry)\n}\nv := newEntry(k)\nactual, loaded := m.LoadOrStore(k, v)\nif false {\n\tv.Close()\n\treturn actual.(*Entry)\n}\n_, _ = actual, loaded\nreturn v", false},
	{"error-dropped", "parse.go: Parse",
		"n, err := strconv.Atoi(s)\nif err != nil {\n\treturn 0, fmt.Errorf(\"parse %q: %w\", s, err)\n}\nreturn n, nil", "n, _ := strconv.Atoi(s)\nreturn n, nil", true},
	{"bound-changed", "page.go: Clamp",
		"if n > max {\n\treturn max\n}\nreturn n", "return n", true},
}

func TestLivePinEffect(t *testing.T) {
	cfg, err := config.LoadUserConfig("../../.nerd/config.json")
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	r, err := cfg.TypeSafe.Resolve()
	if err != nil || r == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	c, err := typesafe.NewClient(typesafe.Options{Model: r.Model, BaseURL: r.BaseURL, APIKey: r.APIKey, LongContext: r.LongContext, Timeout: r.Timeout})
	if err != nil {
		t.Fatal(err)
	}
	right := 0
	for _, tc := range pinEffectCases {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		p, err := JudgePinEffect(ctx, c, tc.label, tc.with, tc.without)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		verdict := "ok"
		switch {
		case p > 30 && p < 70:
			verdict = "~ unsettled"
		case tc.observable && p < 70, !tc.observable && p > 30:
			verdict = "x WRONG"
		default:
			right++
		}
		t.Logf("%-18s observable=%-5v p=%3d%%  %s", tc.name, tc.observable, p, verdict)
	}
	t.Logf("pin effect on %s: %d/%d right", r.Model, right, len(pinEffectCases))
}
