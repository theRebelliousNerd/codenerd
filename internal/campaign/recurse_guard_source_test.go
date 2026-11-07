package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
	"gopkg.in/yaml.v3"
)

var guardSourcePaths = []string{"internal/usage/attempt_scope.go", "internal/perception/usage_track.go", "internal/broker/broker.go",
	"internal/system/knowledge_refresh_scope.go", "internal/system/knowledge_refresh_boot.go"}

func TestMissionGuardSourcesAreProtectedWithinAdmittedScope(t *testing.T) {
	m, ctx := missionPolicyFixture(t)
	k := m.Deliberator.Kernel
	for _, f := range []types.Fact{
		{Predicate: "mission_design", Args: []any{m.State.ID, types.MangleAtom("/ready")}},
		{Predicate: "mission_source", Args: []any{m.State.ID, "source", "source", "vision", "vision"}},
	} {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	o := &Orchestrator{workspace: m.Deliberator.Workspace, kernel: k,
		campaign: &Campaign{Mission: &MissionReceipt{MissionID: m.State.ID, Scope: []string{"."},
			ProtectedPaths: config.DefaultMissionConfig().ProtectedPaths, VerificationInputs: map[string]string{}}}}
	guard := o.writeGuard(&Task{ID: "/guard-control"})
	for _, path := range append(append([]string(nil), guardSourcePaths...), "internal/campaign/recurse_mission.go") {
		if err := guard(ctx, []string{path}); err == nil || !strings.Contains(err.Error(), "no write admission") {
			t.Errorf("guard source admitted within node: %s: %v", path, err)
		}
	}
	for _, path := range []string{"internal/perception/ordinary.go", "internal/broker/ordinary.go", "internal/system/ordinary.go"} {
		if err := guard(ctx, []string{path}); err != nil {
			t.Errorf("ordinary in-scope source refused: %s: %v", path, err)
		}
	}
}

// Use the shipped forbid data but retain this fixture's own commands and gates.
// A missing guard rule must fail the rollback control, not be restated in it.
func guardSourceFixtureDoc(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "nerd.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := projectdoc.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	data, err = yaml.Marshal(projectdoc.Spec{Schema: projectdoc.SchemaVersion, Forbid: doc.Spec.Forbid})
	if err != nil {
		t.Fatal(err)
	}
	return "---\n" + string(data) + "---\n"
}

func TestRecurseGuardSourceBypassIsRefusedAndRolledBack(t *testing.T) {
	t.Parallel()
	paths := append(append([]string(nil), guardSourcePaths...), "internal/perception/ordinary.go")
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			t.Parallel() // each path gets its own git fixture and kernel
			const source = "package guard\n"
			root := recurseFixture(t, map[string]string{"nerd.md": guardSourceFixtureDoc(t), path: source})
			calls := 0
			res, err := runRecurse(t, context.Background(), root, 1, onlyFixes(func(_ context.Context, _ RecurseAttempt) error {
				calls++
				write(t, root, "store/store.go", fixedStore)
				write(t, root, path, source+"\n// changed by the attempt\n")
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("attempts=%d result=%+v", calls, res)
			}
			if path == "internal/perception/ordinary.go" {
				if res.Kept != 1 || res.Refused != 0 || mustRead(t, root, "store/store.go") != fixedStore || mustRead(t, root, path) == source {
					t.Fatalf("ordinary repair was not kept: %+v", res)
				}
				return
			}
			if res.Refused != 1 || res.Kept != 0 || mustRead(t, root, path) != source || mustRead(t, root, "store/store.go") != recurseModule["store/store.go"] {
				t.Fatalf("guard bypass was not refused and compensated: %+v", res)
			}
			r := ratchets(journalOf(t, root))
			if len(r) != 1 || r[0].Outcome != outcomeRefused || !strings.Contains(r[0].Detail, path) {
				t.Fatalf("missing durable refusal: %+v", r)
			}
		})
	}
}
