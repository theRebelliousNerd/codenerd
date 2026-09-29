package orient

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func writeRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sourceByID(sources []Source, id string) (Source, bool) {
	for _, src := range sources {
		if src.ID == id {
			return src, true
		}
	}
	return Source{}, false
}

func mustSource(t *testing.T, sources []Source, id string) Source {
	t.Helper()
	src, ok := sourceByID(sources, id)
	if !ok {
		ids := make([]string, 0, len(sources))
		for _, src := range sources {
			ids = append(ids, src.ID)
		}
		t.Fatalf("missing %s in %v", id, ids)
	}
	return src
}

func TestDiscover_Formats(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".claude/skills/lint/SKILL.md", "---\nname: lint\ndescription: parent long description\ntags: [shared-tag]\nextra_key: keep me\n---\nparent body\n")
	writeRepoFile(t, root, ".claude/skills/lint/references/note.md", "---\ndescription: sibling note\ntags: [only-sibling]\n---\nnote body\n")
	writeRepoFile(t, root, ".claude/skills/lint/extra/SKILL.md", "---\nname: extra-skill\ndescription: nested skill\n---\nnested body\n")
	writeRepoFile(t, root, ".claude/skills/lint/extra/notes.md", "owned by the nested skill\n")
	writeRepoFile(t, root, ".claude/agents/reviewer.md", "---\nname: reviewer\ndescription: Reviews changes\ntools: [Read, Bash]\nmodel: opus\n---\nREVIEWER_BODY\n")
	writeRepoFile(t, root, ".claude/agent-memory/reviewer/today.md", "remember this\n")
	writeRepoFile(t, root, ".claude/commands/ship.md", "ship it\n")
	writeRepoFile(t, root, ".codex/skills/lint/SKILL.md", "---\nname: lint\n---\ncodex lint\n")
	writeRepoFile(t, root, ".codex/agents/worker.toml", "name = \"worker\"\ndescription = \"does work\"\ndeveloper_instructions = \"\"\"\nBe careful.\n\"\"\"\ntools = [\"Read\", \"Bash\"]\nunknown_key = \"kept\"\n")
	writeRepoFile(t, root, ".agents/skills/shared/SKILL.md", "---\nname: shared\n---\nshared skill\n")
	writeRepoFile(t, root, ".agents/rules/packaging.md", "---\nname: packaging-notes\ndescription: how releases are cut\n---\nRULE_BODY\n")
	writeRepoFile(t, root, ".gemini/skills/foo/SKILL.md", "---\nname: foo\n---\nfoo skill\n")
	writeRepoFile(t, root, ".gemini/guide.md", "loose gemini guidance\n")
	writeRepoFile(t, root, ".gemini/settings.json", "{\"description\":\"gemini cli\",\"model\":\"x\"}\n")
	writeRepoFile(t, root, ".roomodes", "{\"customModes\":[{\"slug\":\"plan\",\"name\":\"Plan\",\"roleDefinition\":\"plans the work\",\"customInstructions\":\"do the plan\",\"groups\":[\"read\",\"edit\"]},{\"slug\":\"act\",\"name\":\"Act\",\"roleDefinition\":\"acts\",\"groups\":[\"command\"]}],\"extra\":\"kept\"}\n")
	writeRepoFile(t, root, "pkg/.roomodes", "customModes:\n  - slug: yamlmode\n    name: Yaml\n    roleDefinition: from yaml\n    groups: [read]\n")
	writeRepoFile(t, root, "bad/.roomodes", "not: [\n")
	writeRepoFile(t, root, ".jules/CLAUDE.md", "journal entry\n")
	writeRepoFile(t, root, ".cursor/rules/go.mdc", "---\ndescription: go files\nglobs: internal/**/*.go, cmd/**/*.go\n---\nuse gofmt\n")
	writeRepoFile(t, root, ".cursor/rules/anywhere.md", "---\nglobs: \"**/*.md\"\n---\nall markdown\n")
	writeRepoFile(t, root, ".grok/skills/x/SKILL.md", "---\nname: grok-skill\n---\ngrok skill\n")
	writeRepoFile(t, root, ".grok/agents/a.md", "grok agent\n")
	writeRepoFile(t, root, ".grok/rules/r.md", "grok rule\n")
	writeRepoFile(t, root, ".grok/commands/c.md", "grok command\n")
	writeRepoFile(t, root, ".grok/memory/m.md", "grok memory\n")
	writeRepoFile(t, root, ".grok/README.md", "grok instructions\n")
	writeRepoFile(t, root, ".github/copilot-instructions.md", "copilot root\n")
	writeRepoFile(t, root, "docs/.github/copilot-instructions.md", "copilot nested\n")
	writeRepoFile(t, root, "CLAUDE.md", "root claude\n")
	writeRepoFile(t, root, "sub/CLAUDE.md", "nested claude\n")
	writeRepoFile(t, root, "AGENTS.md", "agents instructions\n")
	writeRepoFile(t, root, "GEMINI.md", "gemini instructions\n")
	writeRepoFile(t, root, "README.md", "not an agent corpus\n")
	writeRepoFile(t, root, ".nerd/skills/hidden/SKILL.md", "hidden\n")
	writeRepoFile(t, root, ".nerd/config.json", "{\"marker\":\"DO_NOT_READ\"}\n")

	sources, err := discover(root, filepath.Join(t.TempDir(), "no-home"))
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range sources {
		if strings.Contains(src.Body, "DO_NOT_READ") || strings.Contains(src.Path, ".nerd/") {
			t.Fatalf("discovered .nerd state: %+v", src.Path)
		}
		if src.Path == "README.md" {
			t.Fatal("README.md is not an instruction filename")
		}
	}

	lint := mustSource(t, sources, ".claude/skills/lint/SKILL.md")
	if lint.Tool != "claude" || lint.Kind != "skill" || lint.Name != "lint" {
		t.Fatalf("lint skill: %+v", lint)
	}
	if lint.Frontmatter["extra_key"] != "keep me" {
		t.Fatalf("unknown frontmatter dropped: %#v", lint.Frontmatter)
	}
	if !strings.Contains(lint.Body, "parent body") || strings.Contains(lint.Body, "extra_key") {
		t.Fatalf("body = %q", lint.Body)
	}

	note := mustSource(t, sources, ".claude/skills/lint/references/note.md")
	if note.Name != "lint" || note.Kind != "skill" || note.Tool != "claude" {
		t.Fatalf("sibling: %+v", note)
	}
	if !containsString(note.Tags, "shared-tag") || containsString(note.Tags, "only-sibling") {
		t.Fatalf("sibling tags = %v", note.Tags)
	}
	if !containsString(note.Topics, "sibling note") || containsString(note.Topics, "parent long description") {
		t.Fatalf("sibling topics = %v", note.Topics)
	}

	nestedNote := mustSource(t, sources, ".claude/skills/lint/extra/notes.md")
	if nestedNote.Name != "extra-skill" {
		t.Fatalf("nested notes name = %s", nestedNote.Name)
	}

	reviewer := mustSource(t, sources, ".claude/agents/reviewer.md")
	if reviewer.Kind != "subagent" || reviewer.Prompt != "REVIEWER_BODY\n" {
		t.Fatalf("reviewer prompt = %q kind %s", reviewer.Prompt, reviewer.Kind)
	}
	if !containsString(reviewer.DeclaredTools, "Read") || !containsString(reviewer.DeclaredTools, "Bash") {
		t.Fatalf("tools = %v", reviewer.DeclaredTools)
	}
	if reviewer.Frontmatter["model"] != "opus" {
		t.Fatalf("model dropped: %#v", reviewer.Frontmatter)
	}

	mem := mustSource(t, sources, ".claude/agent-memory/reviewer/today.md")
	if mem.Kind != "memory" || mem.Name != "reviewer" || mem.Tool != "claude" {
		t.Fatalf("memory: %+v", mem)
	}

	cmd := mustSource(t, sources, ".claude/commands/ship.md")
	if cmd.Kind != "command" || cmd.ScopeDir != ".claude/commands" {
		t.Fatalf("command: %+v", cmd)
	}

	if got := mustSource(t, sources, ".codex/skills/lint/SKILL.md"); got.Tool != "codex" || got.Kind != "skill" {
		t.Fatalf("codex skill: %+v", got)
	}
	worker := mustSource(t, sources, ".codex/agents/worker.toml")
	if worker.Tool != "codex" || worker.Kind != "subagent" || worker.Name != "worker" {
		t.Fatalf("worker: %+v", worker)
	}
	if worker.Prompt != "Be careful.\n" {
		t.Fatalf("toml prompt = %q", worker.Prompt)
	}
	if worker.Frontmatter["unknown_key"] != "kept" {
		t.Fatalf("toml unknown key: %#v", worker.Frontmatter)
	}
	if !containsString(worker.DeclaredTools, "Read") {
		t.Fatalf("toml tools = %v", worker.DeclaredTools)
	}
	if !strings.Contains(worker.Body, "developer_instructions") {
		t.Fatalf("toml body is not the whole file: %q", worker.Body)
	}
	if worker.Digest != digest(worker.Body) {
		t.Fatal("toml digest is not the full file")
	}

	if got := mustSource(t, sources, ".agents/skills/shared/SKILL.md"); got.Tool != "agents" || got.Kind != "skill" {
		t.Fatalf("agents skill: %+v", got)
	}
	rule := mustSource(t, sources, ".agents/rules/packaging.md")
	if rule.Tool != "agents" || rule.Kind != "rule" || rule.ScopeDir != ".agents/rules" {
		t.Fatalf("rule: %+v", rule)
	}

	if got := mustSource(t, sources, ".gemini/guide.md"); got.Tool != "gemini" || got.Kind != "skill" || got.Name != "guide" {
		t.Fatalf("loose gemini file: %+v", got)
	}
	settings := mustSource(t, sources, ".gemini/settings.json")
	if settings.Kind != "instructions" || settings.Name != "settings" || settings.Description != "gemini cli" {
		t.Fatalf("settings: %+v", settings)
	}
	if settings.Frontmatter["model"] != "x" || !strings.Contains(settings.Body, "gemini cli") {
		t.Fatalf("settings body/frontmatter: %#v %q", settings.Frontmatter, settings.Body)
	}

	plan := mustSource(t, sources, ".roomodes#plan")
	act := mustSource(t, sources, ".roomodes#act")
	if plan.Tool != "roo" || plan.Kind != "mode" || plan.Prompt != "do the plan" {
		t.Fatalf("plan mode: %+v", plan)
	}
	if act.Prompt != "acts" {
		t.Fatalf("act prompt = %q", act.Prompt)
	}
	if plan.Digest != act.Digest || plan.Body != act.Body {
		t.Fatal("modes of one file do not share the file body")
	}
	if plan.Description != "" || containsString(plan.Topics, "plans the work") {
		t.Fatalf("roleDefinition became a topic: %q %v", plan.Description, plan.Topics)
	}
	if !containsString(plan.Tags, "read") || plan.Frontmatter["extra"] != "kept" && plan.Frontmatter["file:extra"] != "kept" {
		t.Fatalf("plan frontmatter: %#v tags %v", plan.Frontmatter, plan.Tags)
	}
	if got := mustSource(t, sources, "pkg/.roomodes#yamlmode"); got.Name != "Yaml" || got.Prompt != "from yaml" {
		t.Fatalf("yaml roomodes: %+v", got)
	}
	bad := mustSource(t, sources, "bad/.roomodes")
	if bad.Name != "roomodes" || bad.Frontmatter["unparsed_frontmatter"] == nil || bad.Body == "" {
		t.Fatalf("bad roomodes: %+v", bad)
	}

	journal := mustSource(t, sources, ".jules/CLAUDE.md")
	if journal.Tool != "jules" || journal.Kind != "journal" {
		t.Fatalf(".jules/CLAUDE.md classified as %+v", journal)
	}

	goRule := mustSource(t, sources, ".cursor/rules/go.mdc")
	if goRule.Tool != "cursor" || goRule.Kind != "rule" {
		t.Fatalf("cursor rule: %+v", goRule)
	}
	if !containsString(goRule.Scopes, "internal") || !containsString(goRule.Scopes, "cmd") {
		t.Fatalf("cursor scopes = %v", goRule.Scopes)
	}
	anywhere := mustSource(t, sources, ".cursor/rules/anywhere.md")
	if anywhere.ScopeDir != ".cursor/rules" {
		t.Fatalf("glob ** fell back to %q", anywhere.ScopeDir)
	}

	checks := []struct{ id, tool, kind string }{
		{".grok/skills/x/SKILL.md", "grok", "skill"},
		{".grok/agents/a.md", "grok", "subagent"},
		{".grok/rules/r.md", "grok", "rule"},
		{".grok/commands/c.md", "grok", "command"},
		{".grok/memory/m.md", "grok", "memory"},
		{".grok/README.md", "grok", "instructions"},
		{".github/copilot-instructions.md", "copilot", "instructions"},
		{"docs/.github/copilot-instructions.md", "copilot", "instructions"},
		{"CLAUDE.md", "claude", "instructions"},
		{"sub/CLAUDE.md", "claude", "instructions"},
		{"AGENTS.md", "agents", "instructions"},
		{"GEMINI.md", "gemini", "instructions"},
	}
	for _, check := range checks {
		got := mustSource(t, sources, check.id)
		if got.Tool != check.tool || got.Kind != check.kind {
			t.Fatalf("%s: tool %s kind %s", check.id, got.Tool, got.Kind)
		}
	}
	if mustSource(t, sources, "CLAUDE.md").ScopeDir != "." {
		t.Fatal("root CLAUDE.md scope")
	}
	if mustSource(t, sources, "sub/CLAUDE.md").ScopeDir != "sub" {
		t.Fatal("nested CLAUDE.md scope")
	}
}

func TestDiscover_FrontmatterEdges(t *testing.T) {
	root := t.TempDir()
	long := strings.Repeat("A", 80000)
	writeRepoFile(t, root, "CLAUDE.md", "---\r\nname: edge\r\n---\r\n"+long+"\r\n")
	writeRepoFile(t, root, "sub/AGENTS.md", "---\nname: never\nthis fence is not closed\n\nstill the body\n")

	sources, err := discover(root, "")
	if err != nil {
		t.Fatal(err)
	}
	edge := mustSource(t, sources, "CLAUDE.md")
	if strings.Contains(edge.Body, "\r") {
		t.Fatal("CR survived normalization")
	}
	if !strings.Contains(edge.Body, long) || len(edge.Body) < 80000 {
		t.Fatalf("long line was cut: len %d", len(edge.Body))
	}
	open := mustSource(t, sources, "sub/AGENTS.md")
	if !strings.HasPrefix(open.Body, "---\n") || !strings.Contains(open.Body, "still the body") {
		t.Fatalf("unclosed frontmatter dropped the file: %q", open.Body)
	}
	if open.Name == "never" {
		t.Fatal("unclosed frontmatter was parsed as a name")
	}
}

func TestDiscover_NerdRootIsWalked(t *testing.T) {
	// The walk starts at root even when that directory is named .nerd.
	// A nested .nerd is still skipped; the skill has to sit on a known tool root.
	root := filepath.Join(t.TempDir(), ".nerd")
	writeRepoFile(t, root, ".claude/skills/shown/SKILL.md", "---\nname: shown\n---\nshown\n")
	writeRepoFile(t, root, ".nerd/skills/hidden/SKILL.md", "hidden\n")
	sources, err := discover(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sourceByID(sources, ".claude/skills/shown/SKILL.md"); !ok {
		t.Fatalf("a root named .nerd hid its own skill: %v", sources)
	}
	for _, src := range sources {
		if strings.Contains(src.Path, "/.nerd/") || strings.HasPrefix(src.Path, ".nerd/") {
			t.Fatalf("nested .nerd was walked: %s", src.Path)
		}
	}
}

func TestDiscover_ClaudeMemory(t *testing.T) {
	if got := claudeProjectSlug(`C:\CodeProjects\foo`); got != "C--CodeProjects-foo" {
		t.Fatalf("slug = %s", got)
	}
	root := t.TempDir()
	home := t.TempDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	mem := filepath.Join(home, ".claude", "projects", claudeProjectSlug(abs), "memory")
	writeRepoFile(t, mem, "top.md", "project memory\n")
	writeRepoFile(t, mem, "nested/skip.md", "not one level\n")

	sources, err := discover(root, home)
	if err != nil {
		t.Fatal(err)
	}
	var found Source
	for _, src := range sources {
		if strings.HasSuffix(src.Path, "/top.md") || strings.HasSuffix(src.Path, "top.md") {
			found = src
		}
		if strings.Contains(src.Path, "skip.md") {
			t.Fatal("nested memory directory was read")
		}
	}
	if found.ID == "" {
		t.Fatal("home memory was not read")
	}
	if found.Tool != "claude" || found.Kind != "memory" || found.Tracked {
		t.Fatalf("memory source: %+v", found)
	}
	if found.ID != filepath.ToSlash(filepath.Join(mem, "top.md")) {
		t.Fatalf("memory id = %s", found.ID)
	}
	if _, err := discover(root, filepath.Join(home, "missing")); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_GitTracked(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".claude/agents/tracked.md", "tracked\n")
	writeRepoFile(t, root, ".claude/agents/local.md", "local only\n")
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init")
	git("add", ".claude/agents/tracked.md")
	git("-c", "user.email=dev@example.com", "-c", "user.name=dev", "-c", "commit.gpgsign=false", "commit", "-m", "track")

	sources, err := discover(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !mustSource(t, sources, ".claude/agents/tracked.md").Tracked {
		t.Fatal("committed file was not tracked")
	}
	if mustSource(t, sources, ".claude/agents/local.md").Tracked {
		t.Fatal("untracked file was marked tracked")
	}
}

func TestDiscover_UnreadableSiblingStillReturnsTheRest(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, ".claude/agents/ok.md", "ok\n")
	gone := filepath.Join(root, ".claude", "agents", "gone.md")
	if err := os.Symlink(filepath.Join(root, "missing-target"), gone); err != nil {
		t.Skip(err)
	}
	sources, err := discover(root, "")
	if err == nil {
		t.Fatal("broken symlink was not reported")
	}
	if _, ok := sourceByID(sources, ".claude/agents/ok.md"); !ok {
		t.Fatal("a readable source was dropped with the unreadable one")
	}
}

func TestFacts_ShapesAndReferences(t *testing.T) {
	sources := []Source{
		{ID: "docs/a.md", Tool: "claude", Kind: "skill", Name: "Go Expert", Path: "docs/a.md", Tracked: true, Body: "see docs/b.md and then Linter", Digest: digest("see docs/b.md and then Linter"), Topics: []string{"ideomatic go"}, Tags: []string{"go"}, Description: "ideomatic go"},
		{ID: "docs/b.md", Tool: "codex", Kind: "skill", Name: "Lint", Path: "docs/b.md", Tracked: false, Body: "other", Digest: digest("other")},
		{ID: "c.md", Tool: "grok", Kind: "rule", Name: "Ab", Path: "c.md", Tracked: true, Body: "Ab is short", Digest: digest("Ab is short")},
	}
	facts := Facts(sources)
	body := "see docs/b.md and then Linter"
	var gotSource, gotDigest, gotNorm, gotBytes, gotTopic bool
	var refers []string
	for _, fact := range facts {
		switch fact.Predicate {
		case "agent_source":
			if types.ExtractString(fact.Args[0]) != "docs/a.md" {
				continue
			}
			gotSource = true
			if types.ExtractString(fact.Args[1]) != "/claude" || types.ExtractString(fact.Args[2]) != "/skill" || types.ExtractString(fact.Args[5]) != "/yes" {
				t.Fatalf("agent_source atoms: %#v", fact.Args)
			}
			if _, ok := fact.Args[1].(types.MangleAtom); !ok {
				t.Fatalf("tool arg is %T", fact.Args[1])
			}
		case "agent_source_digest":
			if types.ExtractString(fact.Args[0]) == "docs/a.md" {
				gotDigest = len(types.ExtractString(fact.Args[1])) == 64
			}
		case "agent_source_norm":
			if types.ExtractString(fact.Args[0]) == "docs/a.md" && types.ExtractString(fact.Args[1]) == "goexpert" {
				gotNorm = true
			}
		case "agent_source_bytes":
			if types.ExtractString(fact.Args[0]) == "docs/a.md" && types.ExtractString(fact.Args[1]) == strconv.Itoa(len(body)) {
				gotBytes = true
			}
		case "agent_source_topic":
			if types.ExtractString(fact.Args[0]) == "docs/a.md" && types.ExtractString(fact.Args[1]) == "ideomatic go" {
				gotTopic = true
			}
		case "agent_source_refers":
			refers = append(refers, types.ExtractString(fact.Args[0])+"->"+types.ExtractString(fact.Args[1]))
		}
	}
	if !gotSource || !gotDigest || !gotNorm || !gotBytes || !gotTopic {
		t.Fatalf("shapes source=%v digest=%v norm=%v bytes=%v topic=%v", gotSource, gotDigest, gotNorm, gotBytes, gotTopic)
	}
	if !containsString(refers, "docs/a.md->docs/b.md") {
		t.Fatalf("path reference missing: %v", refers)
	}
	if containsString(refers, "docs/a.md->c.md") {
		t.Fatalf("short name matched: %v", refers)
	}

	tokenSources := []Source{
		{ID: "p", Tool: "claude", Kind: "skill", Name: "Alpha", Path: "p.md", Body: "Linter only"},
		{ID: "q", Tool: "codex", Kind: "skill", Name: "Lint", Path: "q.md", Body: "quiet"},
		{ID: "r", Tool: "grok", Kind: "skill", Name: "Lint", Path: "r.md", Body: "use Lint now"},
	}
	var tokenRefs []string
	for _, fact := range referenceFacts(tokenSources) {
		tokenRefs = append(tokenRefs, types.ExtractString(fact.Args[0])+"->"+types.ExtractString(fact.Args[1]))
	}
	if containsString(tokenRefs, "p->q") {
		t.Fatalf("Linter counted as Lint: %v", tokenRefs)
	}
	if !containsString(tokenRefs, "r->q") {
		t.Fatalf("whole-token Lint was not a reference: %v", tokenRefs)
	}
}

func TestParseTOML_Escapes(t *testing.T) {
	fm, err := parseTOMLMap("name = \"w\"\nmsg = \"a\\nb\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if fm["msg"] != "a\nb" {
		t.Fatalf("escape = %#v", fm["msg"])
	}
	if _, err := parseTOMLMap("name = \"\\q\"\n"); err == nil {
		t.Fatal("unknown escape was accepted")
	}
	src := parseTOMLSource("a.toml", "codex", "subagent", "name = \"\\q\"\n", false)
	if src.Body != "name = \"\\q\"\n" || src.Frontmatter["unparsed_frontmatter"] == nil {
		t.Fatalf("failed toml dropped the file: %#v", src.Frontmatter)
	}
}
