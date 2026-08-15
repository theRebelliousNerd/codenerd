package tools_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/tools/core"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
)

// routingFile is the Mangle source of truth for which modular tools an intent
// may reach.
const routingFile = "../mangle/intent_routing.mg"

// modularToolAllowedRe extracts the tool name from a modular_tool_allowed
// head. Names are Mangle /name constants, so the leading slash is stripped to
// compare against Go registry names.
var modularToolAllowedRe = regexp.MustCompile(`modular_tool_allowed\(/([A-Za-z0-9_]+)`)

// intentionalCatalogExceptions are tools deliberately absent from
// modular_tool_allowed, with the reason. Anything not listed here and not in
// the Mangle catalog is drift: the Go registry grew a tool and nobody told the
// executive about it, so the agent can see it in the piggyback catalog and the
// policy layer has no opinion on whether it may run.
//
// Add to this map only with a reason. An empty-string reason fails the test.
var intentionalCatalogExceptions = map[string]string{}

func fullyHydratedRegistry(t *testing.T) *tools.Registry {
	t.Helper()
	reg := tools.NewRegistry()
	for name, register := range map[string]func(*tools.Registry) error{
		"core":     core.RegisterAll,
		"shell":    shell.RegisterAll,
		"codedom":  codedom.RegisterAll,
		"research": research.RegisterAll,
	} {
		if err := register(reg); err != nil {
			t.Fatalf("%s.RegisterAll: %v", name, err)
		}
	}
	return reg
}

func mangleAllowedTools(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(routingFile))
	if err != nil {
		t.Fatalf("read %s: %v", routingFile, err)
	}
	allowed := make(map[string]bool)
	for _, m := range modularToolAllowedRe.FindAllStringSubmatch(string(data), -1) {
		allowed[m[1]] = true
	}
	if len(allowed) == 0 {
		t.Fatalf("%s parsed to zero modular_tool_allowed heads — the regex or the file layout changed", routingFile)
	}
	return allowed
}

// TestCatalog_WhenToolRegistered_ShouldAppearInMangleRouting is the golden
// test for catalog drift in the Go -> Mangle direction.
func TestCatalog_WhenToolRegistered_ShouldAppearInMangleRouting(t *testing.T) {
	t.Parallel()
	reg := fullyHydratedRegistry(t)
	allowed := mangleAllowedTools(t)

	var missing []string
	for _, name := range reg.Names() {
		if allowed[name] {
			continue
		}
		if reason, ok := intentionalCatalogExceptions[name]; ok {
			if reason == "" {
				t.Errorf("%s is listed as an intentional exception with no reason", name)
			}
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("tools registered in Go but absent from modular_tool_allowed in %s: %v\n"+
			"Either add a modular_tool_allowed rule for each, or record it in "+
			"intentionalCatalogExceptions with the reason it must not be agent-callable.",
			routingFile, missing)
	}
}

// TestCatalog_WhenMangleRoutesTool_ShouldBeRegistered catches the other
// direction: a routing rule for a tool no registry provides means the JIT
// config hands the model a tool name that resolves to nothing, and the call
// fails at execution time with "tool not found".
func TestCatalog_WhenMangleRoutesTool_ShouldBeRegistered(t *testing.T) {
	t.Parallel()
	reg := fullyHydratedRegistry(t)
	allowed := mangleAllowedTools(t)

	// grounded_web_search registers only when a provider-backed searcher is
	// supplied (research.RegisterGroundedWebSearchIfSupported), so its absence
	// from a bare RegisterAll is expected.
	conditional := map[string]bool{"grounded_web_search": true}

	var unregistered []string
	for name := range allowed {
		if conditional[name] || reg.Has(name) {
			continue
		}
		unregistered = append(unregistered, name)
	}
	sort.Strings(unregistered)

	if len(unregistered) > 0 {
		t.Errorf("modular_tool_allowed routes tools no registry provides: %v", unregistered)
	}
}

// TestCatalog_WhenIntentMapsToCategory_ShouldHaveTools pins the fix for
// /review and /attack. intentToCategory mapped those verbs onto categories no
// registered tool declared, so FilterByIntent returned an empty slice and a
// reviewer agent was handed no tools at all.
func TestCatalog_WhenIntentMapsToCategory_ShouldHaveTools(t *testing.T) {
	t.Parallel()
	reg := fullyHydratedRegistry(t)

	// One representative verb per branch of intentToCategory.
	for _, intent := range []string{
		"/research", "/explore", "/learn", "/document",
		"/fix", "/implement", "/refactor", "/create", "/edit",
		"/test", "/cover", "/verify",
		"/review", "/audit", "/check",
		"/attack", "/break", "/nemesis",
		"/general",
	} {
		if got := reg.FilterByIntent(intent); len(got) == 0 {
			t.Errorf("intent %s resolves to zero tools — it maps to a category no tool declares", intent)
		}
	}
}

// TestCatalog_WhenHydratedTwice_ShouldProduceIdenticalRegistries bounds the
// dual-map drift risk.
//
// VirtualStore.HydrateModularTools registers every family into TWO registries
// (its own and tools.Global()), so the two can in principle disagree — a tool
// added to one call site and not the other is invisible to whichever consumer
// reads the other map. Collapsing that to a single registration belongs in
// internal/core. What is enforceable from here is the property the duplication
// depends on: RegisterAll must be deterministic and idempotent, so two
// registries fed the same calls hold exactly the same catalog.
func TestCatalog_WhenHydratedTwice_ShouldProduceIdenticalRegistries(t *testing.T) {
	t.Parallel()
	a := fullyHydratedRegistry(t)
	b := fullyHydratedRegistry(t)

	an, bn := a.Names(), b.Names()
	if len(an) != len(bn) {
		t.Fatalf("registry sizes differ: %d vs %d", len(an), len(bn))
	}
	for i := range an {
		if an[i] != bn[i] {
			t.Fatalf("catalogs diverge at %d: %q vs %q", i, an[i], bn[i])
		}
	}

	// Idempotent: hydrating the same registry again must be a no-op, which is
	// what makes the second RegisterAll call in HydrateModularTools safe.
	before := a.Count()
	if err := core.RegisterAll(a); err != nil {
		t.Fatalf("second core.RegisterAll: %v", err)
	}
	if err := shell.RegisterAll(a); err != nil {
		t.Fatalf("second shell.RegisterAll: %v", err)
	}
	if err := codedom.RegisterAll(a); err != nil {
		t.Fatalf("second codedom.RegisterAll: %v", err)
	}
	if err := research.RegisterAll(a); err != nil {
		t.Fatalf("second research.RegisterAll: %v", err)
	}
	if a.Count() != before {
		t.Fatalf("re-hydration changed the catalog: %d -> %d", before, a.Count())
	}
}

// TestCatalog_WhenDocCommentListsTools_ShouldMatchRegisterAll keeps each
// package's doc.go honest. codedom/doc.go and shell/doc.go had both drifted
// from their RegisterAll lists, which is how a reader learns the package
// registers tools it does not, and misses ones it does.
func TestCatalog_WhenDocCommentListsTools_ShouldMatchRegisterAll(t *testing.T) {
	t.Parallel()

	packages := []struct {
		doc      string
		register func(*tools.Registry) error
	}{
		{"core/doc.go", core.RegisterAll},
		{"shell/doc.go", shell.RegisterAll},
		{"codedom/doc.go", codedom.RegisterAll},
		{"research/doc.go", research.RegisterAll},
	}

	// research/doc.go describes tool families rather than individual tools
	// (there are 21 of them), so it is checked only for the families.
	skipExact := map[string]bool{"research/doc.go": true}

	docListRe := regexp.MustCompile(`(?m)^//\s+-\s+([a-z0-9_]+):`)

	for _, pkg := range packages {
		reg := tools.NewRegistry()
		if err := pkg.register(reg); err != nil {
			t.Fatalf("%s RegisterAll: %v", pkg.doc, err)
		}
		data, err := os.ReadFile(filepath.Clean(pkg.doc))
		if err != nil {
			t.Fatalf("read %s: %v", pkg.doc, err)
		}
		documented := make(map[string]bool)
		for _, m := range docListRe.FindAllStringSubmatch(string(data), -1) {
			documented[m[1]] = true
		}

		if skipExact[pkg.doc] {
			continue
		}

		var undocumented []string
		for _, name := range reg.Names() {
			if !documented[name] {
				undocumented = append(undocumented, name)
			}
		}
		if len(undocumented) > 0 {
			t.Errorf("%s does not document registered tools: %v", pkg.doc, undocumented)
		}

		var phantom []string
		for name := range documented {
			if !reg.Has(name) {
				phantom = append(phantom, name)
			}
		}
		sort.Strings(phantom)
		if len(phantom) > 0 {
			t.Errorf("%s documents tools that are not registered: %v", pkg.doc, phantom)
		}
	}
}
