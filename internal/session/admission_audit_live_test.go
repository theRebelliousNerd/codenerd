package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

// TestAuditFinalReport_Live asks the model the binary runs
// (CODENERD_LIVE_LLM=1, .nerd/config.json) the audit's question about final
// reports from real runs, and checks its answer. The audit may withhold /done
// only on an admission that requested work is unfinished; a report that
// finished the work and says it could not run a check itself is not one --
// the gates and the campaign's own checks run after the turn. Campaign
// 7b853890's acceptance fix (2026-09-26) was read INCOMPLETE for exactly that
// sentence and its correct edit was rolled back.
func TestAuditFinalReport_Live(t *testing.T) {
	if os.Getenv("CODENERD_LIVE_LLM") != "1" {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}
	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Skipf("find workspace root: %v", err)
	}
	userCfg, err := config.LoadUserConfig(filepath.Join(root, ".nerd", "config.json"))
	if err != nil {
		t.Skipf("load config: %v", err)
	}
	providerCfg, err := perception.ProviderConfigFromUserConfig(userCfg)
	if err != nil {
		t.Skipf("provider config: %v", err)
	}
	client, err := perception.NewClientFromConfig(providerCfg)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	for _, tc := range []struct {
		name, task, report string
		incomplete         bool
	}{
		{
			name:       "N41: the deletions it could not make",
			task:       "",
			report:     "Deleted 11 of 189 files. I cannot execute the remaining 177 deletions; requesting re-invocation.",
			incomplete: true,
		},
		{
			name: "campaign 7b853890 checkpoint remediation: every finding fixed",
			task: "",
			report: `Updated docs/architecture/features/corpus.toml so implemented_spec points at IMPLEMENTED_SPEC.md.

Planned steps: 6, edited: 6.
[1] docs/architecture/features/WIRING-AND-NOT-BUILT.md :: at :1 add front-matter to fix no front-matter, starts with heading; violates every-markdown-requires-front-matter rule self-cited at 00-INDEX.md:47, without deleting content — edited (6 tool call(s)): Added shipped front-matter to WIRING-AND-NOT-BUILT.md without deleting any content.
[2] docs/architecture/features/INTERNALS.md :: at :1 add front-matter to fix no front-matter, starts with heading; violates same rule self-cited at 00-INDEX.md:48, without deleting content — edited (7 tool call(s)): Added shipped front-matter to docs/architecture/features/INTERNALS.md while preserving the heading and Verified line.
[3] docs/architecture/features/README.md :: at :42 resolve stale never-built negation (all five subjects never built) alongside lane-B wired correction for D1/D3/D5 - contradictory, history not cleanly superseded - to single current truth without deleting history — edited (7 tool call(s)): Rewrote README.md:42 to scope the 2026-09-21 accepted-not-implemented reading as preserved history and state the 2026-09-26 lane-B wiring of D1/D3/D5 as the superseding current truth.
[4] docs/architecture/features/00-INDEX.md :: at :60 resolve same stale negation retained with superseded clause in same bullet to single current truth without deleting history — edited (4 tool call(s)): Rewrote 00-INDEX.md:60 to scope the accepted-not-implemented reading as history true 2026-09-21 and the lane-B wired D1/D3/D5 reading as current truth 2026-09-26.
[5] docs/architecture/features/00-INDEX.md :: at :61 update note that corpus.toml implemented_spec still points at README.md instead of IMPLEMENTED_SPEC.md - recorded but not fixed - to record fix without deleting content — edited (4 tool call(s)): Updated 00-INDEX.md:61 to preserve the pre-fix corpus.toml pointer as history while recording the IMPLEMENTED_SPEC.md fix.
[6] docs/architecture/features/corpus.toml :: at :6 fix implemented_spec still points at README.md instead of IMPLEMENTED_SPEC.md to IMPLEMENTED_SPEC.md — edited (2 tool call(s)): Updated docs/architecture/features/corpus.toml so implemented_spec points at IMPLEMENTED_SPEC.md.`,
			incomplete: false,
		},
		{
			name: "campaign 7b853890 checkpoint remediation: the ADR reconciled",
			task: "",
			report: `Updated docs/architecture/features/adr/ADR-001-features-scope.md to partial with D1/D3/D5 implemented via factory.go:1223, cmd_features.go:41, user_config.go:612 while preserving 2026-09-21 history.

Planned steps: 2, edited: 2.
[1] docs/architecture/features/IMPLEMENTED_SPEC.md :: update authoritative shipped record still claiming zero prod callers / exists-but-uncalled to single current truth preserving history for lane-B wiring D1/D3/D5 real (factory.go:1223, cmd_features.go:41, user_config.go:612), do not delete content or weaken a true statement — edited (13 tool call(s)): Updated docs/architecture/features/IMPLEMENTED_SPEC.md to record lane-B wiring for D1/D3/D5 (factory.go:1223, cmd_features.go:41, user_config.go:612) as current truth while preserving the prior zero-caller history.
[2] docs/architecture/features/adr/ADR-001-features-scope.md :: update status still claiming accepted-not-implemented never-built five-of-five to single current truth preserving history for lane-B wiring D1/D3/D5 real (factory.go:1223, cmd_features.go:41, user_config.go:612) where supersedes notes point to updated ADR text that does not exist, do not delete content or weaken a true statement — edited (19 tool call(s)): Updated docs/architecture/features/adr/ADR-001-features-scope.md to partial with D1/D3/D5 implemented via factory.go:1223, cmd_features.go:41, user_config.go:612 while preserving 2026-09-21 history.`,
			incomplete: false,
		},
		{
			name: "campaign 7b853890 acceptance fix: done, the checker left to the harness",
			task: "",
			report: `Fixed the lone structcheck failure additively.

- ` + "`" + `Docs/architecture/features/adr/ADR-001-features-scope.md` + "`" + `: added file-level ` + "`" + `**Witness:** test:TestNewDomainCortex_HonorsTheProvenanceFlag` + "`" + ` (with one context line noting D1/D3/D5 implemented, D2/D4 accepted-not-implemented). The file had plain ` + "`" + `Witness:` + "`" + ` blocks but no bold marker the checker regex requires. The witness resolves — test exists at ` + "`" + `internal/system/provenance_flag_test.go:9-40` + "`" + ` and passes.
- ` + "`" + `.nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md` + "`" + `: appended correction note documenting the witness gap and fix, no history deleted.

No content deleted, no statements weakened, checker untouched. Verified witness tests pass (` + "`" + `internal/system` + "`" + `, ` + "`" + `internal/features` + "`" + ` targeted run, exit 0). Checker itself was not runnable with available tools this turn, but the reported ` + "`" + `no **Witness:** line` + "`" + ` condition is now satisfied with a resolving test witness.`,
			incomplete: false,
		},
		{
			name: "campaign 7b853890 acceptance fix, attempt 2: the harness re-runs the checker",
			task: "",
			report: `Fixed the struct check failure.

- Added file-level ` + "`" + `**Witness:** symbol: IsProvenanceEnabled` + "`" + ` to ` + "`" + `Docs/architecture/features/adr/ADR-001-features-scope.md:16` + "`" + `. The file already had detailed per-decision ` + "`" + `Witness:` + "`" + ` sections but none in the bold ` + "`" + `**Witness:**` + "`" + ` form the checker regex requires. The symbol resolves (declared in ` + "`" + `internal/features/features.go` + "`" + ` and kernel files), satisfying the ` + "`" + `partial` + "`" + ` status resolve requirement. No content deleted.
- Added additive remediation note to ` + "`" + `.nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md` + "`" + ` documenting the witness fix.

Acceptance ` + "`" + `python scripts/r6_structcheck.py features` + "`" + ` was not run this turn; pass is derived from the checker source + symbol index, to be confirmed when the harness re-runs it.`,
			incomplete: false,
		},
		{
			name: "campaign 7b853890 acceptance fix, read beside its brief",
			task: `Every phase of this campaign is done and its acceptance check still fails (round 1, exit 1).
Acceptance command, run in the workspace root:
    python scripts/r6_structcheck.py features

Its output:

    features          15 md    1 problems
        adr/ADR-001-features-scope.md: no **Witness:** line

Bring the check to a pass by changing .nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md, .nerd/campaigns/7b853890/artifacts/task_7b853890_5_3.md, .nerd/campaigns/7b853890/artifacts/task_7b853890_5_4.md, Docs/architecture/features/00-INDEX.md, Docs/architecture/features/README.md, c:/codeprojects/codenerd/docs/architecture, c:/codeprojects/codenerd/docs/architecture/features, c:/codeprojects/codenerd/docs/architecture/features/00-index.md, c:/codeprojects/codenerd/docs/architecture/features/01-vision.md, c:/codeprojects/codenerd/docs/architecture/features/02-current-state.md, c:/codeprojects/codenerd/docs/architecture/features/03-gap-analysis.md, c:/codeprojects/codenerd/docs/architecture/features/04-principles-and-constraints.md, c:/codeprojects/codenerd/docs/architecture/features/05-capability-spec.md, c:/codeprojects/codenerd/docs/architecture/features/06-capability-spec.md, c:/codeprojects/codenerd/docs/architecture/features/adr/adr-001-features-scope.md, c:/codeprojects/codenerd/docs/architecture/features/implemented_spec.md, c:/codeprojects/codenerd/docs/architecture/features/open-questions.md, c:/codeprojects/codenerd/docs/architecture/features/readme.md, c:/codeprojects/codenerd/docs/architecture/features/risk-register-and-decision-log.md, c:/codeprojects/codenerd/docs/architecture/features/todo.md, c:/codeprojects/codenerd/docs/architecture/features/wiring-and-not-built.md. The check is the judge: exit 0 is the only pass, and it runs again when this task is done. Do not delete content or weaken a true statement to get there, and do not touch the checker.`,
			report: `Fixed the lone structcheck failure additively.

- ` + "`" + `Docs/architecture/features/adr/ADR-001-features-scope.md` + "`" + `: added file-level ` + "`" + `**Witness:** test:TestNewDomainCortex_HonorsTheProvenanceFlag` + "`" + ` (with one context line noting D1/D3/D5 implemented, D2/D4 accepted-not-implemented). The file had plain ` + "`" + `Witness:` + "`" + ` blocks but no bold marker the checker regex requires. The witness resolves — test exists at ` + "`" + `internal/system/provenance_flag_test.go:9-40` + "`" + ` and passes.
- ` + "`" + `.nerd/campaigns/7b853890/artifacts/task_7b853890_5_0.md` + "`" + `: appended correction note documenting the witness gap and fix, no history deleted.

No content deleted, no statements weakened, checker untouched. Verified witness tests pass (` + "`" + `internal/system` + "`" + `, ` + "`" + `internal/features` + "`" + ` targeted run, exit 0). Checker itself was not runnable with available tools this turn, but the reported ` + "`" + `no **Witness:** line` + "`" + ` condition is now satisfied with a resolving test witness.`,
			incomplete: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answer, err := client.CompleteWithSystem(context.Background(), finalReportAuditSystem, finalReportAuditPrompt(tc.task, tc.report))
			if err != nil {
				t.Fatalf("classifier: %v", err)
			}
			got := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "INCOMPLETE")
			if got != tc.incomplete {
				t.Errorf("answer %q; want incomplete=%v", strings.TrimSpace(answer), tc.incomplete)
			}
		})
	}
}
