package prompt

import (
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/tools/mcpctl"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

// inheritedRequiredTools returns the tools an atom requires, directly or
// through its depends_on chain. The resolver prunes dependents of a blocked
// atom transitively, so a requirement on an ancestor gates the descendant too.
func inheritedRequiredTools(a *PromptAtom, byID map[string]*PromptAtom, seen map[string]bool) map[string]bool {
	req := make(map[string]bool)
	for _, t := range a.RequiresTools {
		req[t] = true
	}
	for _, dep := range a.DependsOn {
		if seen[dep] {
			continue
		}
		seen[dep] = true
		if parent, ok := byID[dep]; ok {
			for t := range inheritedRequiredTools(parent, byID, seen) {
				req[t] = true
			}
		}
	}
	return req
}

// catalogHoldsAll reports whether catalog offers every tool in req.
func catalogHoldsAll(catalog, req map[string]bool) bool {
	for t := range req {
		if !catalog[t] {
			return false
		}
	}
	return true
}

// knownToolNames returns every executable tool name a turn's catalog can draw
// from. It unions three sources because each alone is incomplete:
//
//   - the five modular families the session executor hydrates into
//     tools.Global() (VirtualStore.HydrateModularTools): the only source that
//     knows withheld tools (run_command, bash) the catalogs never grant;
//   - every name any persona catalog grants (DeriveTurnTools, the kernel
//     envelope the executor offers): covers conditionally-registered tools
//     the bare registry lacks (grounded_web_search without a searcher,
//     browser_*);
//   - every name any atom gates on (requires_tools): covers tools known to
//     the corpus but neither registered nor granted.
//
// A backticked name outside this union is prose (`path`, `do`, `let`), not a
// tool teaching. Slash-prefixed names (`/run_command`) are Mangle action
// predicates, not tool calls, and the mention regex excludes them.
func knownToolNames(t *testing.T, corpus []*PromptAtom) map[string]bool {
	t.Helper()
	reg := tools.NewRegistry()
	families := map[string]func(*tools.Registry) error{
		"core":     toolscore.RegisterAll,
		"shell":    shell.RegisterAll,
		"codedom":  codedom.RegisterAll,
		"research": research.RegisterAll,
		"mcpctl":   mcpctl.RegisterAll,
	}
	names := make([]string, 0, len(families))
	for name := range families {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := families[name](reg); err != nil {
			t.Fatalf("%s.RegisterAll: %v", name, err)
		}
	}
	known := make(map[string]bool)
	for _, name := range reg.Names() {
		known[name] = true
	}
	_, catalogs := catalogsByVerb(t)
	for _, catalog := range catalogs {
		for tool := range catalog {
			known[tool] = true
		}
	}
	for _, a := range corpus {
		for _, tool := range a.RequiresTools {
			known[tool] = true
		}
	}
	// The vocabulary is derived from live sources; if it collapses the test
	// would pass by checking nothing. run_command is registry-only (withheld
	// from every catalog) and read_file is universal (core): losing either
	// means a source went silent.
	if !known["run_command"] {
		t.Fatalf("tool vocabulary lost run_command; the shell family did not register")
	}
	if !known["read_file"] {
		t.Fatalf("tool vocabulary lost read_file; the core family did not register")
	}
	if len(known) < 40 {
		t.Fatalf("only %d known tool names; expected the five modular families plus catalog grants", len(known))
	}
	return known
}

var (
	turnKernelOnce sync.Once
	turnKernel     types.Kernel
	turnKernelErr  error
)

// testTurnKernel is the booted kernel the prompt tests ask for a verb's
// envelope. One boot per process: NewRealKernel loads the embedded policy.
func testTurnKernel(t *testing.T) types.Kernel {
	t.Helper()
	turnKernelOnce.Do(func() {
		turnKernel, turnKernelErr = core.NewRealKernel()
	})
	if turnKernelErr != nil {
		t.Fatalf("NewRealKernel: %v", turnKernelErr)
	}
	return turnKernel
}

// catalogsByVerb resolves the effective executable tool catalog for every
// registered intent verb through DeriveTurnTools -- the same projection the
// session executor applies (resolveAvailableTools) before compiling the
// turn's prompt, so the test and the turn agree on the envelope.
func catalogsByVerb(t *testing.T) (verbs []string, catalogs map[string]map[string]bool) {
	t.Helper()
	provider := NewDefaultConfigAtomProvider()
	verbs = provider.RegisteredIntents()
	if len(verbs) == 0 {
		t.Fatalf("RegisteredIntents returned no verbs")
	}
	k := testTurnKernel(t)
	catalogs = make(map[string]map[string]bool, len(verbs))
	for _, verb := range verbs {
		granted, err := DeriveTurnTools(k, verb)
		if err != nil {
			t.Fatalf("DeriveTurnTools(%s): %v", verb, err)
		}
		set := make(map[string]bool, len(granted))
		for _, tool := range granted {
			set[tool] = true
		}
		catalogs[verb] = set
	}
	return verbs, catalogs
}

// selectableVerbs returns the intent verbs whose catalogs can serve this atom.
// An atom with intent_verbs matches only those verbs; an atom without matches
// any verb, so every registered catalog must offer the mention. ShardTypes is
// deliberately ignored: the catalog is verb-derived (executor
// resolveAvailableTools), and a shard gate only narrows where the atom lands,
// so requiring the mention across the verb set is fail-closed -- it may ask
// for an explicit requires_tools on a shard-narrowed atom, never lets a
// teaching slip onto a catalog that withholds the tool.
func selectableVerbs(a *PromptAtom, allVerbs []string) []string {
	if len(a.IntentVerbs) == 0 {
		return allVerbs
	}
	verbs := make([]string, 0, len(a.IntentVerbs))
	for _, v := range a.IntentVerbs {
		verbs = append(verbs, "/"+strings.TrimPrefix(strings.TrimSpace(v), "/"))
	}
	return verbs
}

// TestAtomCorpus_OptionalToolsAreNamedOnlyByAtomsThatRequireThem pins the
// capability-gating contract from the atom side: a prompt atom must never
// teach a tool the turn's catalog does not offer.
//
// Every backticked tool name an atom mentions must either be a tool the atom's
// requires_tools gates (directly or via depends_on, which the resolver prunes
// transitively), or be offered to every persona/verb the atom can be selected
// for. Catalogs come from the same source the executor uses
// (DeriveTurnTools over turn_tool_allowed), not a hand-copied list, so a
// catalog change re-derives the expectation.
//
// Measured 2026-09-28: the mandatory identity/coder/tool_usage taught
// "`run_command` / `run_build` for build and test commands" with no
// requires_tools, while the coder catalog grants run_build but has never
// granted run_command. On campaign 7b853890 (2026-09-26) acceptance-fix models
// reported the checker "was not runnable with available tools". The previous
// version of this test policed only tools appearing in some atom's
// requires_tools list, so a mention of a tool no atom gates slipped through.
func TestAtomCorpus_OptionalToolsAreNamedOnlyByAtomsThatRequireThem(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atoms := corpus.All()
	byID := make(map[string]*PromptAtom, len(atoms))
	for _, a := range atoms {
		byID[a.ID] = a
	}
	known := knownToolNames(t, atoms)
	allVerbs, catalogs := catalogsByVerb(t)

	// A mention is a backticked span whose first token is a tool name: bare
	// (`run_command`) or an invocation sketch (`repoint from=...`,
	// `get_element ref=...`). Slash-prefixed spans (`/run_command`) are
	// Mangle action predicates, not tool calls, and never match.
	mention := regexp.MustCompile("`([^`\n]+)`")
	leadingTool := regexp.MustCompile(`^([a-z][a-z0-9_]*)`)

	checked := 0
	for _, a := range atoms {
		text := a.Content + "\n" + a.ContentConcise + "\n" + a.ContentMin
		req := inheritedRequiredTools(a, byID, map[string]bool{a.ID: true})
		verbs := selectableVerbs(a, allVerbs)
		seen := make(map[string]bool)
		for _, m := range mention.FindAllStringSubmatch(text, -1) {
			lead := leadingTool.FindStringSubmatch(m[1])
			if lead == nil {
				continue
			}
			tool := lead[1]
			if !known[tool] || seen[tool] {
				continue
			}
			seen[tool] = true
			checked++
			if req[tool] {
				continue
			}
			var missing []string
			for _, verb := range verbs {
				catalog, ok := catalogs[verb]
				if !ok {
					// An intent verb no catalog knows falls back to
					// the /general floor in DeriveTurnTools; resolve it
					// the same way rather than treating it as empty.
					fallback, ferr := DeriveTurnTools(testTurnKernel(t), verb)
					if ferr != nil {
						missing = append(missing, verb)
						continue
					}
					catalog = make(map[string]bool, len(fallback))
					for _, name := range fallback {
						catalog[name] = true
					}
					catalogs[verb] = catalog
				}
				// requires_tools is an inclusion gate (atomToolSatisfied): a
				// catalog missing any required tool never receives the atom,
				// so a mention cannot mislead a model there.
				if !catalogHoldsAll(catalog, req) {
					continue
				}
				if !catalog[tool] {
					missing = append(missing, verb)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				t.Errorf("%s names `%s` without requiring it (requires_tools, directly or via depends_on): missing from the %d catalog(s) %s of the %d verb(s) it can be selected for",
					a.ID, tool, len(missing), strings.Join(missing, ", "), len(verbs))
			}
		}
	}
	if checked < 30 {
		t.Fatalf("matched only %d tool mentions across the corpus; the mention detection no longer sees the tool guidance", checked)
	}
}

// TestAtomCorpus_TeachesNoPhantomTransactionTools pins the removal of the
// explicit-transaction-control list capability/codedom_transaction used to
// teach: begin_transaction, add_edit, prepare, commit and abort match no
// registered tool (codedom/register.go lists the CodeDOM tools; grep finds
// no begin_transaction or add_edit registration under internal/tools), so
// the model must never be told to call them. Only the two multi-word names
// are pinned: prepare, commit and abort are ordinary prose elsewhere, and a
// bare backticked name is syntactically identical to a Mangle predicate
// taught bare (deny_edit, edit_warning) or a schema field (intent_verbs),
// so a general unknown-name rule cannot separate them without false
// positives.
func TestAtomCorpus_TeachesNoPhantomTransactionTools(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	phantoms := []string{"`begin_transaction`", "`add_edit`"}
	for _, a := range corpus.All() {
		text := a.Content + "\n" + a.ContentConcise + "\n" + a.ContentMin
		for _, phantom := range phantoms {
			if strings.Contains(text, phantom) {
				t.Errorf("%s teaches phantom tool %s, which matches no registered tool", a.ID, phantom)
			}
		}
	}
}
