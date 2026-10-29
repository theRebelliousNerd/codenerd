package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/world"
)

// The Go scanner names a function's symbol func:ComputeTotal; the user names
// it ComputeTotal. resolveTarget compared the two strings directly, so a
// unique, current function stayed unresolved (confidence 30, no file), and
// the focus-driven context rules lost the file seed (audit
// symbol-focus-identity-continuity). The facts here come from the real
// tree-sitter parser, not hand-written rows.
func symbolFocusShard(t *testing.T, files map[string]string) *PerceptionFirewallShard {
	t.Helper()
	root := t.TempDir()
	kern, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	parser := world.NewASTParser()
	defer parser.Close()
	var facts []core.Fact
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseAs(path, rel)
		if err != nil {
			t.Fatal(err)
		}
		facts = append(facts, parsed...)
	}
	if err := kern.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	shard := NewPerceptionFirewallShard()
	shard.Kernel = kern
	return shard
}

func TestResolveTarget_BareFunctionNameResolvesToItsScannerSymbol(t *testing.T) {
	shard := symbolFocusShard(t, map[string]string{
		"billing/charge.go": "package billing\n\nfunc ComputeTotal(items []int) int { return len(items) }\n",
		"billing/other.go":  "package billing\n\nfunc unrelated() {}\n",
	})
	got := shard.resolveTarget(context.Background(), "ComputeTotal")
	if got.ResolvedPath != "billing/charge.go" || got.ConfidencePercent != 85 {
		t.Fatalf("ComputeTotal resolved to %+v, want billing/charge.go at 85", got)
	}
	if got.SymbolName != "func:ComputeTotal" {
		t.Fatalf("symbol = %q, want the indexed id func:ComputeTotal", got.SymbolName)
	}
	if got.RawReference != "ComputeTotal" {
		t.Fatalf("the raw reference changed: %q", got.RawReference)
	}

	// The indexed id itself still resolves.
	if byID := shard.resolveTarget(context.Background(), "func:ComputeTotal"); byID.ResolvedPath != "billing/charge.go" {
		t.Fatalf("func:ComputeTotal resolved to %+v", byID)
	}
}

// A method resolves by its bare name too: method:Ledger.Close.
func TestResolveTarget_BareMethodNameResolves(t *testing.T) {
	shard := symbolFocusShard(t, map[string]string{
		"ledger/ledger.go": "package ledger\n\ntype Ledger struct{}\n\nfunc (l *Ledger) Settle() {}\n",
	})
	got := shard.resolveTarget(context.Background(), "Settle")
	if got.ResolvedPath != "ledger/ledger.go" || got.ConfidencePercent != 85 {
		t.Fatalf("Settle resolved to %+v, want ledger/ledger.go at 85", got)
	}
}

// One name in two files is ambiguous: no file is chosen for the user, not
// even one whose path happens to contain the name.
func TestResolveTarget_NameInTwoFilesStaysUnresolved(t *testing.T) {
	shard := symbolFocusShard(t, map[string]string{
		"a/a.go":          "package a\n\nfunc Render() {}\n",
		"b/b.go":          "package b\n\nfunc Render() {}\n",
		"render/notes.go": "package render\n\nfunc unrelated() {}\n",
	})
	got := shard.resolveTarget(context.Background(), "Render")
	if got.ResolvedPath != "" || got.ConfidencePercent >= 85 {
		t.Fatalf("an ambiguous name resolved to %+v", got)
	}
}

// Go identifiers are case-sensitive: an exact-case match wins over a
// different identifier that differs only by case.
func TestResolveTarget_ExactCaseBeatsCaseFold(t *testing.T) {
	shard := symbolFocusShard(t, map[string]string{
		"p/upper.go": "package p\n\nfunc Parse() {}\n",
		"p/lower.go": "package p\n\nfunc parse() {}\n",
	})
	got := shard.resolveTarget(context.Background(), "parse")
	if got.ResolvedPath != "p/lower.go" {
		t.Fatalf("parse resolved to %+v, want p/lower.go", got)
	}
}

// On a sharded boot the firewall's own kernel is the catch-all shard and the
// world facts live on the world shard, as the production manifests route
// them. SetParentKernel kept only the catch-all, so resolveTarget read an
// empty symbol_graph there and nothing ever resolved.
func TestResolveTarget_ReadsWorldFactsThroughTheShardedParent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "billing", "charge.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package billing\n\nfunc ComputeTotal() int { return 0 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parser := world.NewASTParser()
	defer parser.Close()
	facts, err := parser.ParseAs(path, "billing/charge.go")
	if err != nil {
		t.Fatal(err)
	}

	cortex := core.NewCortexKernel("cortex")
	for _, cfg := range []core.KernelShardConfig{
		{Domain: "world", WorkspaceRoot: root, OwnedPredicates: []string{"symbol_graph", "file_topology"}},
		{Domain: "cortex", WorkspaceRoot: root},
	} {
		shard, err := core.NewKernelShard(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := cortex.RegisterShard(shard); err != nil {
			t.Fatal(err)
		}
	}
	if err := cortex.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	if rows, err := cortex.GetPrimaryRealKernel().Query("symbol_graph"); err != nil || len(rows) != 0 {
		t.Fatalf("precondition: the catch-all shard must hold no symbol_graph rows, got %d (%v)", len(rows), err)
	}

	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(cortex)
	got := shard.resolveTarget(context.Background(), "ComputeTotal")
	if got.ResolvedPath != "billing/charge.go" || got.SymbolName != "func:ComputeTotal" {
		t.Fatalf("ComputeTotal on the sharded kernel resolved to %+v, want billing/charge.go", got)
	}
}
