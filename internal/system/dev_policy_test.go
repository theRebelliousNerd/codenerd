package system

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// sourceTreeWorkspace is a workspace that is codeNERD's source tree: go.mod
// declaring module codenerd and the policy corpus under internal/core,
// copied from this checkout.
func sourceTreeWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module codenerd\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs(filepath.Join("..", "core", "defaults"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(ws, "internal", "core", "defaults")
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		if !strings.HasSuffix(p, ".mg") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func awaitReload(t *testing.T, events <-chan PolicyReloadEvent) PolicyReloadEvent {
	t.Helper()
	select {
	case ev := <-events:
		return ev
	case <-time.After(60 * time.Second):
		t.Fatal("no policy reload outcome was published")
		return PolicyReloadEvent{}
	}
}

// Dev mode: a boot in codeNERD's source tree with dev.policy_hot_reload on
// reads the policy from the tree, and an edit to a policy file is swapped into
// the running domain cortex once the whole program compiles; an edit that
// does not compile is reported and the running program stays.
func TestBootDevMode_HotReloadsThePolicyFromTheSourceTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := sourceTreeWorkspace(t)
	writeCortexIdentityConfig(t, ws, `{"dev":{"policy_hot_reload":true}}`)
	t.Cleanup(core.UseEmbeddedConstitution)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	want := filepath.Join(ws, "internal", "core")
	if cortex.DevPolicyDir() != want || core.DevConstitutionDir() != want {
		t.Fatalf("dev policy dir = %q (core %q), want %q", cortex.DevPolicyDir(), core.DevConstitutionDir(), want)
	}
	if err := cortex.Kernel.Assert(types.Fact{Predicate: "user_intent", Args: []any{
		types.MangleAtom("/current_intent"), types.MangleAtom("/mutation"), types.MangleAtom("/fix"), "internal/retry/policy.go", ""}}); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(want, "defaults", "policy", "dev_reload_probe.mg")
	const rule = "Decl dev_reload_probe(Target) bound [/string].\ndev_reload_probe(Target) :- user_intent(/current_intent, _, _, Target, _).\n"
	if err := os.WriteFile(probe, []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	accepted := awaitReload(t, cortex.PolicyReloads())
	if ev := accepted; ev.Err != nil || len(ev.Files) != 1 || ev.Reload.Disposition != "adopted" || ev.Reload.Active == "" || ev.Reload.Active != ev.Reload.Candidate {
		t.Fatalf("the valid edit did not reload: %+v", ev)
	}
	rows, err := cortex.Kernel.Query("dev_reload_probe")
	if err != nil || len(rows) != 1 || types.ExtractString(rows[0].Args[0]) != "internal/retry/policy.go" {
		t.Fatalf("after the reload dev_reload_probe = %v (%v); want the kept intent's target", rows, err)
	}

	if err := os.WriteFile(probe, []byte(rule+"this is not mangle(\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := awaitReload(t, cortex.PolicyReloads()); ev.Err == nil || ev.Reload.Disposition != "rejected" || ev.Reload.Active != accepted.Reload.Active {
		t.Fatalf("the invalid edit reloaded: %+v", ev)
	}
	if rows, err := cortex.Kernel.Query("dev_reload_probe"); err != nil || len(rows) != 1 {
		t.Fatalf("the invalid edit disturbed the running program: %v (%v)", rows, err)
	}

	// A compile-valid candidate may still fail evaluation. Exercise the
	// watcher -> adapter -> multi-shard reload -> event path, then recover.
	ck, ok := cortex.Kernel.(*core.CortexKernel)
	if !ok {
		t.Fatalf("expected domain Cortex, got %T", cortex.Kernel)
	}
	primary := ck.GetPrimaryRealKernel()
	limit := primary.GetDerivedFactLimit()
	primary.SetDerivedFactLimit(1)
	defer primary.SetDerivedFactLimit(limit)
	if err := os.WriteFile(probe, []byte(rule+"\n# evaluation rejection witness\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := awaitReload(t, cortex.PolicyReloads()); ev.Err == nil || ev.Reload.Stage != "evaluate" || ev.Reload.Active != accepted.Reload.Active || ev.Reload.Disposition != "rejected" {
		t.Fatalf("evaluation failure did not preserve the accepted generation: %+v", ev)
	}
	if rows, err := cortex.Kernel.Query("dev_reload_probe"); err != nil || len(rows) != 1 {
		t.Fatalf("evaluation rejection poisoned the accepted store: %v (%v)", rows, err)
	}
	primary.SetDerivedFactLimit(limit)
	if err := os.WriteFile(probe, []byte(rule+"\n# ready recovery witness\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ev := awaitReload(t, cortex.PolicyReloads()); ev.Err != nil || ev.Reload.Disposition != "adopted" || ev.Reload.Active == accepted.Reload.Active || ev.Reload.Active != ev.Reload.Candidate {
		t.Fatalf("reload did not recover after rejection: %+v", ev)
	}
}

// Opting in outside codeNERD's tree changes nothing: the binary's policy.
func TestDevPolicyTree_RefusesAnotherModule(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := devPolicyTree(ws); err == nil {
		t.Fatal("another module's workspace was taken for codeNERD's tree")
	}
}
