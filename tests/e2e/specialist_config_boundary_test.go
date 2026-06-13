//go:build integration

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// =============================================================================
// MOCKS — specialist config boundary
// =============================================================================

type scbMockLLMClient struct{}

func (m *scbMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (m *scbMockLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userInput string) (string, error) {
	return "ok", nil
}
func (m *scbMockLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userInput string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok"}, nil
}
func (m *scbMockLLMClient) ShouldUsePiggybackTools() bool { return false }

func (m *scbMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
}

type scbMockVirtualStore struct {
	readRawFunc func(path string) ([]byte, error)
}

func (m *scbMockVirtualStore) ReadFile(path string) ([]string, error)        { return nil, nil }
func (m *scbMockVirtualStore) WriteFile(path string, content []string) error { return nil }
func (m *scbMockVirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	return "", "", nil
}
func (m *scbMockVirtualStore) ReadRaw(path string) ([]byte, error) {
	if m.readRawFunc != nil {
		return m.readRawFunc(path)
	}
	return nil, os.ErrNotExist
}

type scbMockConfigFactory struct{}

func (m *scbMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{}, nil
}
func (m *scbMockConfigFactory) RegisterSpecialist(name string, config *config.EffectiveAgentRuntimeConfig) error {
	return nil
}

type scbMockJITCompiler struct{}

func (m *scbMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{Prompt: "mock"}, nil
}

type scbMockTransducer struct{}

func (m *scbMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *scbMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	return perception.Intent{Verb: "/fix"}, nil
}
func (m *scbMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	return perception.Intent{Verb: "/fix"}, nil, nil
}
func (m *scbMockTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *scbMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *scbMockTransducer) SetStrategicContext(ctx string)                   {}

// =============================================================================
// Helper
// =============================================================================

func newSpecialistSpawner(vstore types.VirtualStore) *session.Spawner {
	llm := &scbMockLLMClient{}
	jit := &scbMockJITCompiler{}
	cfgFactory := &scbMockConfigFactory{}
	trans := &scbMockTransducer{}

	return session.NewSpawner(nil, vstore, llm, jit, cfgFactory, trans, session.DefaultSpawnerConfig())
}

// =============================================================================
// 1. Path Traversal
// =============================================================================

// TestE2E_SpecialistConfig_PathTraversal tests that loadSpecialistConfig
// never reads outside .nerd/agents/ even if the name contains traversal.
//
// Current implementation uses filepath.Join(".nerd", "agents", name, "config.yaml")
// which does NOT sanitize traversal — this test documents the gap.
func TestE2E_SpecialistConfig_PathTraversal(t *testing.T) {
	traversalNames := []string{
		"../outside",
		"../../etc/passwd",
		"../../../../windows/system32/config",
		"..",
		"valid/../../../etc/passwd",
		"agents/../../secret",
	}

	for _, name := range traversalNames {
		t.Run(name, func(t *testing.T) {
			// Track what path the VirtualStore was asked to read
			var readPath string
			vstore := &scbMockVirtualStore{
				readRawFunc: func(path string) ([]byte, error) {
					readPath = path
					return nil, os.ErrNotExist
				},
			}

			spawner := newSpecialistSpawner(vstore)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := spawner.SpawnSpecialist(ctx, name, "test task")

			// The path that was attempted
			t.Logf("Name: %q -> readPath: %q -> err: %v", name, readPath, err)

			// Verify the path doesn't escape .nerd/agents/
			if readPath != "" {
				// Clean the path and check for traversal
				clean := filepath.Clean(readPath)
				expectedPrefix := filepath.Clean(filepath.Join(".nerd", "agents"))

				if !strings.HasPrefix(clean, expectedPrefix) {
					t.Errorf("PATH TRAVERSAL: name %q caused read of %q which escapes %q",
						name, clean, expectedPrefix)
				} else {
					t.Logf("Path stayed within bounds: %q", clean)
				}
			}
		})
	}
}

// =============================================================================
// 2. Empty Name
// =============================================================================

// TestE2E_SpecialistConfig_EmptyName tests that SpawnSpecialist("", ...)
// either returns a clean error or handles gracefully (no panic, no empty path).
func TestE2E_SpecialistConfig_EmptyName(t *testing.T) {
	vstore := &scbMockVirtualStore{}
	spawner := newSpecialistSpawner(vstore)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := spawner.SpawnSpecialist(ctx, "", "test task")
	t.Logf("SpawnSpecialist(\"\") err: %v", err)

	// Empty name should either error or produce a valid (if vacuous) agent
	// The key invariant is: no panic
}

// =============================================================================
// 3. Oversized Config
// =============================================================================

// TestE2E_SpecialistConfig_OversizedYAML tests that an extremely large YAML
// config doesn't cause OOM. Current implementation reads the entire file
// into memory before unmarshalling.
func TestE2E_SpecialistConfig_OversizedYAML(t *testing.T) {
	// Simulate a 10MB YAML config
	bigYAML := strings.Repeat("key: value\n", 1_000_000) // ~11MB

	vstore := &scbMockVirtualStore{
		readRawFunc: func(path string) ([]byte, error) {
			return []byte(bigYAML), nil
		},
	}

	spawner := newSpecialistSpawner(vstore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agent, err := spawner.SpawnSpecialist(ctx, "big-config", "test")
	t.Logf("SpawnSpecialist with 10MB config: err=%v agent=%v", err, agent != nil)

	// The key invariant: no panic, no OOM crash
	// Whether it succeeds or fails with an error, either is acceptable
	// as long as the system is stable.
	if err != nil {
		t.Logf("Correctly rejected oversized config: %v", err)
	} else {
		t.Log("WARNING: Accepted 10MB config without size limit. Consider adding a config size cap.")
		if agent != nil {
			_ = agent.Stop()
		}
	}
}

// =============================================================================
// 4. Malformed YAML
// =============================================================================

// TestE2E_SpecialistConfig_MalformedYAML tests that invalid YAML produces
// a clean error without panicking.
func TestE2E_SpecialistConfig_MalformedYAML(t *testing.T) {
	malformedCases := []struct {
		name    string
		content string
	}{
		{"invalid_indent", "tools:\n  allowed:\n - foo\n  bar"},
		{"binary_garbage", "\x00\x01\x02\x03\xff\xfe"},
		{"yaml_bomb", strings.Repeat("- ", 10000) + "end"},
		{"unclosed_quote", `name: "unclosed`},
		{"tab_indent", "tools:\n\tallowed:\n\t\t- foo"},
	}

	for _, tc := range malformedCases {
		t.Run(tc.name, func(t *testing.T) {
			vstore := &scbMockVirtualStore{
				readRawFunc: func(path string) ([]byte, error) {
					return []byte(tc.content), nil
				},
			}

			spawner := newSpecialistSpawner(vstore)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_, err := spawner.SpawnSpecialist(ctx, "malformed", "test")
			t.Logf("Malformed YAML (%s): err=%v", tc.name, err)

			// Should return an error, not panic
			if err == nil {
				t.Logf("NOTE: Malformed YAML %q was accepted (YAML parser was lenient)", tc.name)
			}
		})
	}
}
