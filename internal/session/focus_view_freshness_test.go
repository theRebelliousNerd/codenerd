package session

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/world"
)

type focusFreshnessClient struct {
	*MockLLMClient
	tool      string
	after     map[int]func()
	histories [][]types.Message
}

func (c *focusFreshnessClient) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	round := len(c.histories)
	c.histories = append(c.histories, append([]types.Message(nil), history...))
	if action := c.after[round]; action != nil {
		action() // An independent fixture writer changes context between requests.
	}
	if round == 3 {
		return &types.LLMToolResponse{Text: "observed all snapshots"}, nil
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: fmt.Sprintf("focus-call-%d", round), Name: c.tool,
		Input: map[string]any{"path": "target.go", "start_line": round + 1}}}}, nil
}

func TestFocusViewFreshness_NativeRequestsTrackRenderedDependencies(t *testing.T) {
	const toolName = "focus_view_freshness_read_probe"
	registerTestTool(t, &tools.Tool{Name: toolName, Category: tools.CategoryGeneral, Effect: tools.EffectRead,
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			return fmt.Sprintf("offline read observation %v", args["start_line"]), nil
		}})
	for _, tc := range []struct {
		name, before, after string
		changed             bool
	}{
		{"sibling_signature", "func Helper() (error)", "func Helper() (string, bool)", true},
		{"live_graph", "client.BeforeCaller", "client.AfterCaller", true},
		{"test_presence", "**Tests**: no", "**Tests**: yes", true},
		{"unchanged", "func Helper() (error)", "func Helper() (error)", false},
		{"unrendered_sibling_body", "func Helper() (error)", "func Helper() (error)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &focusFreshnessClient{MockLLMClient: &MockLLMClient{}, tool: toolName}
			e := newWorkingLoopExecutor(t, client)
			e.config.TokenBudget = 200000
			e.config.Working.LedgerCeilingBytes = 1 << 20
			root := e.config.WorkspaceRoot
			// Match the ordinary CLI's workspace CWD. The renderer's sibling
			// and test-file reads accept relative targets; outline reads also
			// know workDir. A package-test CWD would mix different directories.
			t.Chdir(root)
			write := func(name, content string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write("go.mod", "module example.com/focusfixture\n\ngo 1.27\n")
			focusSource := []byte("package target\n\nfunc Anchor() { Helper() }\n")
			write("target.go", string(focusSource))
			write("sibling.go", "package target\n\nfunc Helper() error { return nil }\n")
			write("hidden.go", "package target\n\nfunc hidden() int { return 1 }\n")
			focus := filepath.Join(root, "target.go")
			graph := &MockKernel{}
			setCaller := func(caller string) {
				graph.mu.Lock()
				graph.facts = []types.Fact{
					{Predicate: "code_defines", Args: []any{focus, "target.Anchor", types.MangleAtom("/function"), int64(3), int64(3)}},
					{Predicate: "code_calls", Args: []any{caller, "target.Anchor"}},
				}
				graph.mu.Unlock()
			}
			setCaller("client.BeforeCaller")
			provider := world.NewHolographicProvider(graph, root)
			e.SetFileContextProvider(provider)
			change := func(after bool) {
				switch tc.name {
				case "sibling_signature":
					if after {
						write("sibling.go", "package target\n\nfunc Helper() (string, bool) { return \"changed\", true }\n")
					} else {
						write("sibling.go", "package target\n\nfunc Helper() error { return nil }\n")
					}
				case "live_graph":
					if after {
						setCaller("client.AfterCaller")
					} else {
						setCaller("client.BeforeCaller")
					}
				case "test_presence":
					if after {
						write("target_test.go", "package target\n\nimport \"testing\"\nfunc TestAnchor(t *testing.T) {}\n")
					} else if err := os.Remove(filepath.Join(root, "target_test.go")); err != nil {
						t.Fatal(err)
					}
				case "unrendered_sibling_body":
					if after {
						write("hidden.go", "package target\n\nfunc hidden() int { return 200 }\n")
					} else {
						write("hidden.go", "package target\n\nfunc hidden() int { return 1 }\n")
					}
				}
			}
			// Independently verify that the real renderer sees the changed
			// dependency before asking whether session request reuse sees it.
			initial := provider.PromptSection(context.Background(), "target.go")
			change(true)
			modified := provider.PromptSection(context.Background(), "target.go")
			change(false)
			if !strings.Contains(initial, tc.before) || !strings.Contains(modified, tc.after) || (initial != modified) != tc.changed {
				t.Fatalf("renderer control for %s did not establish its premise:\nbefore=%s\nafter=%s", tc.name, truncateForFailure(initial), truncateForFailure(modified))
			}
			client.after = map[int]func(){0: func() { change(true) }, 2: func() { change(false) }}
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain", Target: "target.go"}}
			response, _, err := e.runToolLoop(context.Background(), "SYSTEM", "explain target.go",
				&jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
				&prompt.CompilationContext{ShardID: "focus-freshness", IntentTarget: "target.go"}, result)
			if err != nil || response == nil || response.Text != "observed all snapshots" || len(client.histories) != 4 {
				t.Fatalf("native request fixture: response=%+v error=%v requests=%d", response, err, len(client.histories))
			}
			data, err := os.ReadFile(focus)
			if err != nil || sha256.Sum256(data) != sha256.Sum256(focusSource) {
				t.Fatal("the focused file changed; this must isolate dependency freshness")
			}
			header := fmt.Sprintf(workingViewHeader, "target.go")
			for i, request := range client.histories {
				if i > 0 {
					requireAppendOnly(t, client.histories[i-1], request, fmt.Sprintf("native request %d", i+1))
				}
				wantCount, wantText := 1, tc.before
				if tc.changed && i > 0 {
					wantCount, wantText = 2, tc.after
					if i == 3 {
						// Dedup: same file at same revision never carries the same
						// focus view twice. The reverted render reuses its earlier
						// snapshot, so the request still holds 2 views, not 3.
						wantCount, wantText = 2, tc.before
					}
				}
				text := requestText(request)
				if got := strings.Count(text, header); got != wantCount {
					t.Errorf("request %d carries %d view snapshots, want %d", i+1, got, wantCount)
				}
				if i == 3 && tc.changed {
					// Dedup reuses the earlier snapshot for a reverted render:
					// the bytes are present once, not necessarily as latest.
					if !strings.Contains(text, wantText) {
						t.Errorf("request %d never delivered reverted dependency snapshot %q (deduped reuse)", i+1, wantText)
					}
				} else {
					last := strings.LastIndex(text, header)
					if last < 0 || !strings.Contains(text[last:], wantText) {
						t.Errorf("request %d never delivered dependency snapshot %q to the offline provider", i+1, wantText)
					}
				}
			}
		})
	}
}

func TestFocusViewFreshness_AvailabilityTransitionsDoNotReuseOldIdentity(t *testing.T) {
	for _, initiallyAvailable := range []bool{true, false} {
		t.Run(fmt.Sprint(initiallyAvailable), func(t *testing.T) {
			l := newLedgerLoop(t, 1<<20, 2)
			if !initiallyAvailable {
				l.files.section = ""
			}
			first := l.round("read_file", "first observation")
			l.files.section = ""
			second := l.round("read_file", "second observation")
			requireAppendOnly(t, first, second, "unavailable view")
			if initiallyAvailable && !strings.Contains(requestText(second), "unavailable") {
				t.Error("a disappeared view left its previous snapshot standing without an availability notice")
			}
			l.files.section = "RECOVERED-CURRENT-VIEW"
			third := l.round("read_file", "third observation")
			requireAppendOnly(t, second, third, "available view restored")
			if !strings.Contains(requestText(third), "RECOVERED-CURRENT-VIEW") {
				t.Error("an earlier empty render suppressed the later available view at the same focus digest")
			}
			fourth := l.round("read_file", "fourth observation")
			requireAppendOnly(t, third, fourth, "unchanged recovered view")
			if n := strings.Count(requestText(fourth), "RECOVERED-CURRENT-VIEW"); n != 1 {
				t.Errorf("unchanged recovered render appears %d times, want once", n)
			}
		})
	}
}
