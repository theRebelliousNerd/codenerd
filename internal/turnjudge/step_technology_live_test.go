//go:build jevlive

package turnjudge

// Live probe of the planned-step technology reading against the configured
// Jev:
//
//	go test -tags jevlive ./internal/turnjudge/ -run LiveStepTechnologies -v
//
// Each case is a planned step on a file that imports the technology, with the
// answer a person would give: does carrying out the change write that
// technology's code? Reports each probability against the kernel's
// thresholds (yes >= 0.70, no <= 0.30) and asserts only that the call worked.
// The key travels in the client's Authorization header and is never printed.

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

var stepTechnologyCases = []struct {
	name, file, change, tech string
	imports                  []string
	writes                   bool
}{
	{"go-early-return", "internal/core/kernel_query.go", "return early from Query when the predicate name is empty, with an error naming the caller", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, false},
	{"go-rename-field", "internal/core/kernel_query.go", "rename the queryCache field to resultCache and update its two uses", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, false},
	{"go-mutex", "internal/core/kernel_query.go", "guard the evaluation counter with the kernel's mutex so concurrent queries do not race", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, false},
	{"mangle-ast-atom", "internal/core/kernel_query.go", "build the bound pattern query as an ast.Atom with ast.Constant arguments instead of formatting a string", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, true},
	{"mangle-analysis", "internal/core/kernel_query.go", "run analysis.AnalyzeOneUnit on the parsed query clause and return its error before evaluating", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, true},
	{"mangle-rule-text", "internal/core/policy_loader.go", "add the rule `stale(F) :- file_hash(F, H), !current_hash(F, H).` to the embedded program the loader parses", "mangle", []string{"codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/analysis", "codeberg.org/TauCeti/mangle-go/engine"}, true},
	{"tea-log-line", "cmd/nerd/chat/view.go", "add a debug log line when the viewport height is recomputed", "bubbletea", []string{"github.com/charmbracelet/bubbletea"}, false},
	{"tea-key-msg", "cmd/nerd/chat/view.go", "handle tea.KeyMsg for ctrl+l by returning a tea.ClearScreen command from Update", "bubbletea", []string{"github.com/charmbracelet/bubbletea"}, true},
}

func TestLiveStepTechnologies(t *testing.T) {
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
	right, unsettled, wrong := 0, 0, 0
	for _, tc := range stepTechnologyCases {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		got, err := JudgeStepTechnologies(ctx, c, tc.file, tc.change, map[string][]string{tc.tech: tc.imports})
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		p := got[tc.tech]
		verdict := "ok"
		switch {
		case p > 30 && p < 70:
			verdict = "~ unsettled"
			unsettled++
		case tc.writes && p < 70, !tc.writes && p > 30:
			verdict = "x WRONG"
			wrong++
		default:
			right++
		}
		t.Logf("%-18s tech=%-9s writes=%-5v p=%3d%%  %s", tc.name, tc.tech, tc.writes, p, verdict)
	}
	t.Logf("step technologies on %s: %d right, %d unsettled, %d wrong of %d", r.Model, right, unsettled, wrong, len(stepTechnologyCases))
}
