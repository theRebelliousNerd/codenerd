package broker

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the package directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate go.mod from the test working directory")
	return ""
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestEveryClientConstructionPathIsBrokered is the wiring audit that keeps the
// single accounting boundary from rotting.
//
// The whole design rests on one claim: there is no way to obtain an LLM client
// in this codebase that is not metered. That claim is a property of three
// functions in one file, and it is exactly the kind of property a future change
// breaks silently — someone adds a provider case, or a new exported
// constructor, and inference quietly starts happening off the books again.
func TestEveryClientConstructionPathIsBrokered(t *testing.T) {
	const rel = "internal/perception/client_factory.go"
	path := filepath.Join(repoRoot(t), rel)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	// Constructors that intentionally return a raw, un-metered client. They are
	// unexported precisely so nothing outside the factory can reach one.
	rawAllowed := map[string]bool{
		"newRawClientFromConfig":               true,
		"newRawClassificationClientFromConfig": true,
		"newSuperGrokClientOrAPIFallback":      true,
	}

	var checked int
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}
		if !fn.Name.IsExported() || !returnsLLMClient(fn) {
			continue
		}
		if rawAllowed[fn.Name.Name] {
			continue
		}

		checked++
		if !callsAny(fn, "InstallBroker", "NewClientFromConfig", "NewClassificationClientFromConfig",
			// Delegating to this in-file helper counts, because the helper is
			// itself asserted to route through the factory just below.
			"newSecondarySlotClient") {
			t.Errorf("%s returns an LLMClient without routing through metering.\n"+
				"Every exported constructor must call InstallBroker (directly, or via "+
				"NewClientFromConfig / NewClassificationClientFromConfig). Un-metered inference "+
				"is the exact condition internal/broker exists to end.", fn.Name.Name)
		}
	}

	if checked < 3 {
		t.Errorf("the audit only inspected %d exported constructors; it has drifted from the source "+
			"and is no longer proving anything", checked)
	}

	// Delegation is only safe while the delegate is metered. Without this the
	// allowance above would be a hole large enough to drive the worker and
	// planner tiers through.
	var sawHelper bool
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "newSecondarySlotClient" || fn.Body == nil {
			continue
		}
		sawHelper = true
		if !callsAny(fn, "InstallBroker", "NewClientFromConfig") {
			t.Error("newSecondarySlotClient no longer routes through metering, so every exported " +
				"constructor that delegates to it (worker, planner) now spends off the books")
		}
	}
	if !sawHelper {
		t.Error("newSecondarySlotClient has been removed or renamed; the delegation allowance above " +
			"is now unverified and must be updated")
	}
}

func returnsLLMClient(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil {
		return false
	}
	for _, result := range fn.Type.Results.List {
		if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "LLMClient" {
			return true
		}
		if sel, ok := result.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "LLMClient" {
			return true
		}
	}
	return false
}

func callsAny(fn *ast.FuncDecl, names ...string) bool {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch target := call.Fun.(type) {
		case *ast.Ident:
			if want[target.Name] {
				found = true
			}
		case *ast.SelectorExpr:
			if want[target.Sel.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// TestNoCompetingTokenCounters proves the duplicate ruler is gone rather than
// merely unused.
//
// internal/context carried charsPerToken = 4.0 behind an estimator seam nothing
// ever filled. Leaving it in place "for compatibility" is how a codebase ends up
// with two answers to one question and no way to tell which is authoritative.
func TestNoCompetingTokenCounters(t *testing.T) {
	root := repoRoot(t)

	banned := []struct {
		needle string
		why    string
	}{
		{"charsPerToken", "the heuristic chars-per-token field was replaced by the broker's calibrated ratio"},
		{"CharsPerTokenEstimator", "the heuristic estimator type was deleted; use broker.TextCounter"},
		{"NewTokenCounterWithEstimator", "the unused estimator seam was replaced by broker-backed counting"},
	}

	// internal/broker documents the removed names in prose; that is the record
	// of why they are gone and must not be mistaken for their return.
	skipDirs := map[string]bool{
		filepath.Join(root, "internal", "broker"): true,
		filepath.Join(root, "Docs"):               true,
		filepath.Join(root, ".git"):               true,
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			for skip := range skipDirs {
				if path == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		content, readErr := sourceWithoutComments(path)
		if readErr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)

		for _, b := range banned {
			if strings.Contains(content, b.needle) {
				t.Errorf("%s reintroduces %q: %s", rel, b.needle, b.why)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestOrphanBudgetConstantsAreGone pins the specific magic numbers that used to
// be independent budget authorities, so a future edit cannot quietly restore one.
func TestOrphanBudgetConstantsAreGone(t *testing.T) {
	cases := []struct {
		file   string
		needle string
		why    string
	}{
		{
			"internal/session/semantic_compressor.go",
			"const maxTokens = 64000",
			"a constant that named tokens, measured characters, and was known to no other component; " +
				"it now derives from the ledger via summarizationCharAllowance",
		},
		{
			"internal/session/executor.go",
			"const DefaultTokenBudget = 65536",
			"a flat prompt budget that ignored the configured window; DefaultTokenBudget() now derives from the ledger",
		},
		{
			"internal/init/jit_integration.go",
			"cc.TokenBudget = 120000",
			"a literal larger than some configured windows and a fraction of others; it now takes a share of the enforced window",
		},
	}

	for _, tc := range cases {
		src, err := sourceWithoutComments(filepath.Join(repoRoot(t), tc.file))
		if err != nil {
			t.Fatalf("scan %s: %v", tc.file, err)
		}
		if strings.Contains(src, tc.needle) {
			t.Errorf("%s reintroduces %q: %s", tc.file, tc.needle, tc.why)
		}
	}
}

// sourceWithoutComments returns a file's Go source with all comments removed.
//
// The audits below must check code, not prose. Several of the removed
// identifiers are deliberately named in comments explaining why they are gone —
// that record is worth keeping, and a textual scan that cannot tell the
// difference would force the explanation out of the codebase.
func sourceWithoutComments(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	fset := token.NewFileSet()
	f := fset.AddFile(path, fset.Base(), len(data))

	var sc scanner.Scanner
	sc.Init(f, data, nil, 0) // no scanner.ScanComments: comments are skipped

	var b strings.Builder
	for {
		_, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		if lit != "" {
			b.WriteString(lit)
		} else {
			b.WriteString(tok.String())
		}
		b.WriteByte(' ')
	}
	return b.String(), nil
}

// TestUsageObserverHookIsWired proves the mechanism the broker depends on to
// learn provider actuals is still present in the usage plumbing.
//
// Without this hook the broker records nothing on the plain Complete paths,
// which return no usage of their own — and it would fail silently, reporting
// zero spend rather than an error.
func TestUsageObserverHookIsWired(t *testing.T) {
	content := readRepoFile(t, "internal/usage/usage_tracker.go")

	if !strings.Contains(content, "ObserverFromContext(ctx)") {
		t.Error("TrackFromContext no longer notifies the in-flight observer; " +
			"the broker would silently record zero spend on every non-tool call")
	}
	if !strings.Contains(content, "obs.Observed(") {
		t.Error("TrackFromContext no longer calls Observed on the observer")
	}
}

// TestBrokerIsInstalledAtBoot proves the meter is pointed at the configured
// window before any client is constructed.
func TestBrokerIsInstalledAtBoot(t *testing.T) {
	content := readRepoFile(t, "internal/system/factory.go")

	if !strings.Contains(content, "configureBrokerMeter(bctx.appCfg)") {
		t.Error("boot no longer configures the broker meter; the ledger would enforce no window")
	}

	meterFile := readRepoFile(t, "internal/system/broker_meter.go")
	for _, needle := range []string{"GetContextWindowConfig", "broker.Configure", "ctxCfg.MaxTokens"} {
		if !strings.Contains(meterFile, needle) {
			t.Errorf("boot meter configuration no longer references %s", needle)
		}
	}
}

// TestLegacyClientPathIsMetered covers the one client construction that does not
// go through the factory.
func TestLegacyClientPathIsMetered(t *testing.T) {
	content := readRepoFile(t, "internal/system/factory.go")

	if !strings.Contains(content, "perception.InstallBrokerForProvider(") {
		t.Error("the legacy raw-apiKey client is no longer metered; it would spend off the books")
	}
	if strings.Contains(content, "baseLLMClient = perception.NewZAIClient(") {
		t.Error("the legacy raw-apiKey path constructs a client directly again, bypassing metering")
	}
}

// TestConcreteTypeAssertionsReachThroughTheDecorator guards the one behavioural
// break a decorator introduces: an assertion on a concrete client type stops
// matching once the client is wrapped.
func TestConcreteTypeAssertionsReachThroughTheDecorator(t *testing.T) {
	content := readRepoFile(t, "cmd/nerd/chat/model_session_context.go")

	if strings.Contains(content, "m.client.(*perception.CodexCLIClient)") {
		t.Error("the Codex CLI engine check asserts on the wrapped client and will never match; " +
			"it must go through broker.Base first, or the codex_cli framework tag silently disappears " +
			"from JIT atom selection")
	}
	if !strings.Contains(content, "broker.Base(m.client).(*perception.CodexCLIClient)") {
		t.Error("the Codex CLI engine check no longer reaches through the decorator via broker.Base")
	}
}
