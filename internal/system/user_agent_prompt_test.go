package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// userAgentPromptsYAML mirrors the layout `nerd init` and `nerd define-agent`
// write to .nerd/agents/<name>/prompts.yaml. The marker is what the assertions
// look for in the compiled prompt.
const userAgentPromptsYAML = `- id: "TestExpert/identity"
  category: "identity"
  subcategory: "TestExpert"
  priority: 100
  is_mandatory: true
  description: "Identity for TestExpert"
  content_concise: |
    UNIQUE_AGENT_IDENTITY_MARKER concise
  content_min: |
    UNIQUE_AGENT_IDENTITY_MARKER min
  content: |
    UNIQUE_AGENT_IDENTITY_MARKER
    You are TestExpert, an authority on widget calibration.
`

// writeAndSyncUserAgent lays down .nerd/agents/<name>/prompts.yaml and runs the
// same AgentSynchronizer boot uses (initExecutionLayer), returning the
// knowledge DB path it produced.
func writeAndSyncUserAgent(t *testing.T, workspace, name, yaml string) string {
	t.Helper()

	agentDir := filepath.Join(workspace, ".nerd", "agents", name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write prompts.yaml: %v", err)
	}

	synchronizer := prsync.NewAgentSynchronizer(workspace, prompt.NewAtomLoader(nil))
	if err := synchronizer.SyncAll(context.Background()); err != nil {
		t.Fatalf("AgentSynchronizer.SyncAll: %v", err)
	}
	discovered := synchronizer.GetDiscoveredAgents()
	if len(discovered) != 1 {
		t.Fatalf("expected 1 discovered agent, got %d (%+v)", len(discovered), discovered)
	}
	return discovered[0].DBPath
}

// TestUserAgentPromptsReachCompiledPrompt is the end-to-end guard for the
// defect that made every user-defined agent generic.
//
// Full chain under test: prompts.yaml -> AgentSynchronizer ->
// .nerd/shards/<lower(name)>_knowledge.db -> RegisterAgentDBWithJIT ->
// CompilationContext.ShardID (derived from the intent verb by
// session.UserAgentFromIntentVerb) -> compiled system prompt.
//
// Regression guarded: the agent's atoms were parsed, stored, and registered —
// and then never selected, because nothing set ShardID to the agent's name.
// The `nerd spawn <Name>` path additionally lower-cases the verb, so a
// case-sensitive shard-DB map turned the registration into a silent miss even
// when ShardID was right. This test fails if either the ShardID derivation
// (internal/session/executor.go buildCompilationContext) or the case-insensitive
// key (internal/prompt/compiler_db.go shardDBKey) is dropped.
func TestUserAgentPromptsReachCompiledPrompt(t *testing.T) {
	workspace := t.TempDir()
	dbPath := writeAndSyncUserAgent(t, workspace, "TestExpert", userAgentPromptsYAML)

	_, adapter := newPromptScopeTestKernel(t)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	// Boot registers under the on-disk directory name ("TestExpert").
	if err := prompt.RegisterAgentDBWithJIT(compiler, "TestExpert", dbPath); err != nil {
		t.Fatalf("RegisterAgentDBWithJIT: %v", err)
	}

	// Both verb shapes reach the same agent. "/consult/TestExpert" is chat
	// delegation; "/testexpert" is what `nerd spawn TestExpert` becomes after
	// normalizeTaskIntentVerb lower-cases it.
	for _, verb := range []string{"/consult/TestExpert", "/testexpert"} {
		t.Run(verb, func(t *testing.T) {
			agent := session.UserAgentFromIntentVerb(verb)
			if agent == "" {
				t.Fatalf("UserAgentFromIntentVerb(%q) returned empty; executor would compile with no shard context", verb)
			}

			result, err := compiler.Compile(context.Background(), &prompt.CompilationContext{
				IntentVerb:      verb,
				OperationalMode: "/active",
				TokenBudget:     32000,
				ShardID:         agent,
				ShardType:       "/" + agent,
			})
			if err != nil {
				t.Fatalf("Compile(%s): %v", verb, err)
			}
			if !strings.Contains(result.Prompt, "UNIQUE_AGENT_IDENTITY_MARKER") {
				t.Fatalf("user agent identity missing from compiled prompt for %s.\n"+
					"Its prompts.yaml atoms were synced and registered but never selected.\n"+
					"prompt (%d bytes): %.500s",
					verb, len(result.Prompt), result.Prompt)
			}
		})
	}
}

// TestUserAgentShardDBLookupIsCaseInsensitive isolates the registration/lookup
// casing contract.
//
// Regression guarded: agents are registered under their on-disk directory name
// (any casing the user chose, e.g. "RustExpert"), but every verb that reaches
// the compiler has been lower-cased on the way in. A case-sensitive map made
// that a silent miss — no error, no warning, just a generic prompt.
func TestUserAgentShardDBLookupIsCaseInsensitive(t *testing.T) {
	workspace := t.TempDir()
	dbPath := writeAndSyncUserAgent(t, workspace, "TestExpert", userAgentPromptsYAML)

	compiler, err := prompt.NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	if err := prompt.RegisterAgentDBWithJIT(compiler, "TestExpert", dbPath); err != nil {
		t.Fatalf("RegisterAgentDBWithJIT: %v", err)
	}

	for _, lookup := range []string{"TestExpert", "testexpert", "TESTEXPERT", "/testexpert"} {
		if _, ok := compiler.LookupShardDB(lookup); !ok {
			t.Errorf("LookupShardDB(%q) missed; agent atoms would be silently dropped", lookup)
		}
	}
	if _, ok := compiler.LookupShardDB("someotheragent"); ok {
		t.Error("LookupShardDB matched an unregistered agent")
	}
}

// TestRegisterUserAgentConfigAtomsHonorsDeclaredTools guards the tool-grant
// wiring for user-defined agents.
//
// Regression guarded: .nerd/agents.json has carried a per-agent `tools` array
// since agent creation (internal/init/agents.go GetToolsForAgentType). After
// the catalog moved into turn_tool_allowed, that array was dropped at runtime
// and this test still passed because both sides of the atom comparison were
// empty. The grant has to reach DeriveTurnTools: the declared names, and the
// /general floor a specialist would otherwise lose.
func TestRegisterUserAgentConfigAtomsHonorsDeclaredTools(t *testing.T) {
	kernel, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	// go_build and go_test are not persona-envelope tools. The host records
	// them as registered before an agent may declare them. read_file is
	// already in the /general envelope, so it needs no tool_registered fact.
	for _, tool := range []string{"go_build", "go_test"} {
		if err := kernel.Assert(types.Fact{
			Predicate: "tool_registered",
			Args:      []any{tool, int64(1)},
		}); err != nil {
			t.Fatalf("assert tool_registered(%s): %v", tool, err)
		}
	}

	workspace := t.TempDir()
	writeAgentsJSON(t, workspace, []map[string]any{
		{
			"name":   "GoExpert",
			"type":   "user",
			"status": "ready",
			"tools":  []string{"go_build", "go_test", "read_file"},
		},
		{
			// No declared tools: still an atom. The turn catalog is the
			// /general floor, via the verb_has_persona fallback, because
			// nothing was asserted for this verb.
			"name":   "PlainExpert",
			"type":   "user",
			"status": "ready",
		},
	})

	provider := prompt.NewDefaultConfigAtomProvider()
	general, ok := provider.GetAtom("/general")
	if !ok {
		t.Fatal("/general atom missing from the default provider")
	}
	if err := registerUserAgentConfigAtoms(provider, kernel, workspace); err != nil {
		t.Fatalf("register: %v", err)
	}

	for _, verb := range []string{"/goexpert", "/consult/goexpert"} {
		atom, found := provider.GetAtom(verb)
		if !found {
			t.Fatalf("no config atom registered for %q", verb)
		}
		for _, want := range []string{"go_build", "go_test", "read_file"} {
			if !containsString(atom.Tools, want) {
				t.Errorf("%s: declared tool %q missing from atom grant %v", verb, want, atom.Tools)
			}
		}
		for _, want := range general.Tools {
			if !containsString(atom.Tools, want) {
				t.Errorf("%s: core tool %q dropped from atom grant %v", verb, want, atom.Tools)
			}
		}
		derived, err := prompt.DeriveTurnTools(kernel, verb)
		if err != nil {
			t.Fatalf("DeriveTurnTools(%s): %v", verb, err)
		}
		for _, want := range []string{"go_build", "go_test", "read_file"} {
			if !containsString(derived, want) {
				t.Errorf("DeriveTurnTools(%s) = %v, missing %q", verb, derived, want)
			}
		}
	}

	plain, found := provider.GetAtom("/plainexpert")
	if !found {
		t.Fatal("agent with no declared tools got no config atom")
	}
	if len(plain.Tools) != len(general.Tools) {
		t.Errorf("agent with no declared tools should get exactly the core atom set, got %v", plain.Tools)
	}
	plainDerived, err := prompt.DeriveTurnTools(kernel, "/plainexpert")
	if err != nil {
		t.Fatalf("DeriveTurnTools(/plainexpert): %v", err)
	}
	if !containsString(plainDerived, "read_file") {
		t.Errorf("agent with no declared tools lost the /general floor: %v", plainDerived)
	}
	if containsString(plainDerived, "go_build") {
		t.Errorf("agent with no declared tools gained go_build: %v", plainDerived)
	}

	// A name that is not a registered agent must not gain an atom. Its
	// catalog is the unknown-verb floor, not this agent's tools.
	if _, found := provider.GetAtom("/consult/notanagent"); found {
		t.Error("unregistered agent name gained a config atom")
	}
	unknown, err := prompt.DeriveTurnTools(kernel, "/consult/notanagent")
	if err != nil {
		t.Fatalf("DeriveTurnTools(/consult/notanagent): %v", err)
	}
	if containsString(unknown, "go_build") {
		t.Errorf("unregistered verb gained go_build: %v", unknown)
	}

	// A declared tool the host has not seen registered is refused by name,
	// and nothing is registered for that file.
	refused := t.TempDir()
	writeAgentsJSON(t, refused, []map[string]any{
		{
			"name":   "GoExpert",
			"type":   "user",
			"status": "ready",
			"tools":  []string{"not_a_registered_tool"},
		},
	})
	refusedProvider := prompt.NewDefaultConfigAtomProvider()
	err = registerUserAgentConfigAtoms(refusedProvider, kernel, refused)
	if err == nil || !strings.Contains(err.Error(), "not_a_registered_tool") {
		t.Fatalf("unregistered tool error = %v, want it to name not_a_registered_tool", err)
	}
	if _, found := refusedProvider.GetAtom("/goexpert"); found {
		t.Error("atom registered after a refused tool")
	}

	// bash is a real executable name and is intentionally outside every
	// persona envelope. Declaring it must fail the same way a typo does.
	bashWS := t.TempDir()
	writeAgentsJSON(t, bashWS, []map[string]any{
		{
			"name":   "ShellExpert",
			"type":   "user",
			"status": "ready",
			"tools":  []string{"bash"},
		},
	})
	bashProvider := prompt.NewDefaultConfigAtomProvider()
	err = registerUserAgentConfigAtoms(bashProvider, kernel, bashWS)
	if err == nil || !strings.Contains(err.Error(), "bash") {
		t.Fatalf("bash error = %v, want it to name bash", err)
	}
	if _, found := bashProvider.GetAtom("/shellexpert"); found {
		t.Error("atom registered after bash was refused")
	}
}

func writeAgentsJSON(t *testing.T, workspace string, agents []map[string]any) {
	t.Helper()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(map[string]any{
		"version": "1.5.0",
		"agents":  agents,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "agents.json"), data, 0o644); err != nil {
		t.Fatalf("write agents.json: %v", err)
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
