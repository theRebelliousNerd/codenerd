package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/world"
)

func writeSearchMatches(t *testing.T, root string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("MATCH-PATH-SENTINEL-%02d.txt", i)
		if err := os.WriteFile(filepath.Join(root, name), []byte("literal.[needle] literal.[needle] literal needle"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// The real slash dispatcher must distinguish acquisition coverage at the cap,
// count files rather than occurrences, and keep the outcome in slash history.
// Before the repair the same dispatch omitted coverage for every case.
func TestSlashSearch_CoverageBoundaries(t *testing.T) {
	for _, count := range []int{0, 19, 20, 21} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ws := t.TempDir()
			writeSearchMatches(t, ws, count)
			m := NewTestModel(WithWorkspace(ws))
			t.Cleanup(m.shutdownCancel)
			next, _ := m.handleCommand("/search literal.[needle]")
			reply := lastAssistant(next.(Model))
			for _, want := range []string{
				fmt.Sprintf("Found %d matching files (limit 20)", min(count, 20)),
				"domain: admitted workspace members", "file bodies are not supplied",
			} {
				if !strings.Contains(reply, want) {
					t.Fatalf("missing %q in slash response:\n%s", want, reply)
				}
			}
			if count <= 20 && !strings.Contains(reply, "Coverage: COMPLETE") {
				t.Fatalf("completed search was not identified:\n%s", reply)
			}
			if count > 20 && (!strings.Contains(reply, "Coverage: PARTIAL") || !strings.Contains(reply, "Further match count is unknown")) {
				t.Fatalf("capped search was not identified:\n%s", reply)
			}
			if strings.Contains(reply, "MATCH-PATH-SENTINEL-20.txt") {
				t.Fatal("slash exceeded its matching-file limit")
			}
			// Follow-up uses the actual retained slash history, not a hand-built
			// evidence observation. It preserves paths and coverage verbatim.
			client := NewMockLLMClient()
			intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "which file"}
			_, err := articulateWithConversation(context.Background(), client, intent,
				payloadForArticulation(intent, nil), nil, "sys",
				&ConversationContext{RecentTurns: next.(Model).history}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(client.GetLastPrompt(), reply) {
				t.Fatal("follow-up answering request lost the search outcome")
			}
		})
	}
}

func searchEvidenceIntent(confidence float64) perception.Intent {
	return perception.Intent{
		Category: "/query", Verb: "/search", Target: "literal needle",
		Confidence: confidence, IsQuestion: true, ToolsNeeded: []string{"grep"},
	}
}

// Normal processInput composition: real policy and residual acquisition, no
// shard manager/system executor to rescue it, and an actual answering request.
// The pre-repair request has a count warning but no discovered filename sentinel
// or search coverage; its no-file gather fallback also contradicts acquisition.
func TestProcessInput_SearchDeliversPathsAndCoverage(t *testing.T) {
	for _, count := range []int{0, 9, 10, 11} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ws := SetupLiveWorkspace(t)
			writeSearchMatches(t, ws, count)
			// Keep normal world/source context enabled. This nonmatching
			// source sorts before the matches so it does not turn the exact
			// matching-file cap into an unknown trailing domain.
			if err := os.WriteFile(filepath.Join(ws, "000-world.go"), []byte("package fixture\nfunc SearchFixture() string { return \"fixture\" }\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			kernel, err := core.NewRealKernelWithWorkspace(ws)
			if err != nil {
				t.Fatal(err)
			}
			client := NewMockLLMClient()
			client.SetDefaultResponse("Answered from search evidence.")
			m := NewTestModel(WithSize(120, 50), WithWorkspace(ws))
			t.Cleanup(m.shutdownCancel)
			m.kernel = kernel
			m.client = client
			intent := searchEvidenceIntent(0.3)
			m.transducer = &scriptedTransducer{intents: []perception.Intent{intent}}
			m.virtualStore = core.NewVirtualStore(nil)
			m.scanner = world.NewScanner()
			if m.shardMgr != nil {
				t.Fatal("fixture must have no independent system rescue")
			}
			msg := awaitProcessStream(t, m.processInput("find the literal needle text", 0)())
			if _, ok := msg.(assistantMsg); !ok {
				t.Fatalf("turn returned %T, want articulated answer", msg)
			}
			rows, err := kernel.Query("route_decision")
			if err != nil || len(rows) != 1 || !strings.Contains(fmt.Sprint(rows), "/gather_evidence") {
				t.Fatalf("expected gather lane, got %v, %v", rows, err)
			}
			if facts, err := kernel.Query("file_topology"); err != nil || len(facts) == 0 {
				t.Fatalf("normal world context was not loaded: %v, %v", facts, err)
			}
			prompt := client.GetLastPrompt()
			if !strings.Contains(prompt, fmt.Sprintf("Found %d matching files (limit 10)", min(count, 10))) {
				t.Fatalf("answer request lost file-count evidence:\n%s", prompt)
			}
			coverage := "Coverage: COMPLETE"
			if count > 10 {
				coverage = "Coverage: PARTIAL"
			}
			if !strings.Contains(prompt, coverage) || strings.Contains(prompt, "names no file") {
				t.Fatalf("search coverage missing or contradicted:\n%s", prompt)
			}
			if count > 0 && !strings.Contains(prompt, "MATCH-PATH-SENTINEL-00.txt") {
				t.Fatalf("discovered filename missing in answering request:\n%s", prompt)
			}
			if strings.Contains(prompt, "MATCH-PATH-SENTINEL-10.txt") {
				t.Fatal("answer request exceeds the residual matching-file cap")
			}
		})
	}
}

// The search composition does not widen the configured delegation gate.
func TestDecideRoute_SearchRetainsConfidentDelegation(t *testing.T) {
	m := newRoundtripModel(t)
	t.Cleanup(m.shutdownCancel)
	intent := searchEvidenceIntent(0.96)
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("find the literal needle text", intent, "researcher"); route.Kind != RouteDelegate {
		t.Fatalf("confident search route = %s, want delegate", route.Kind)
	}
}

func denySearchFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("host can read mode-000 files; admitted read-failure fixture requires permission enforcement")
	}
}

// Membership and traversal succeed; the admitted member fails at the read
// boundary. Both consumers must retain positive paths and reject false absence.
func TestSearchConsumers_AdmittedReadFailure(t *testing.T) {
	for _, positive := range []int{0, 1} {
		t.Run(fmt.Sprint(positive), func(t *testing.T) {
			ws := t.TempDir()
			writeSearchMatches(t, ws, positive)
			failed := filepath.Join(ws, "zzz-unreadable.txt")
			if err := os.WriteFile(failed, []byte("literal.[needle] literal needle"), 0o600); err != nil {
				t.Fatal(err)
			}
			denySearchFile(t, failed)
			m := NewTestModel(WithWorkspace(ws))
			t.Cleanup(m.shutdownCancel)
			next, _ := m.handleCommand("/search literal.[needle]")
			slash := lastAssistant(next.(Model))
			warnings := []string{}
			facts, _, evidence := m.executeInfoGatheringActions(context.Background(), "find it", searchEvidenceIntent(0.3),
				[]core.Fact{{Predicate: "next_action", Args: []any{"/search_files"}}}, &warnings)
			if len(facts) != 0 {
				t.Fatalf("search must not assert an undeclared substitute for evidence: %v", facts)
			}
			evidence = append(evidence, gatherWorkspaceEvidence(ws, searchEvidenceIntent(0.3), evidence)...)
			client := NewMockLLMClient()
			intent := searchEvidenceIntent(0.3)
			if _, err := articulateWithConversation(context.Background(), client, intent,
				payloadForArticulation(intent, nil), warnings, "sys", &ConversationContext{Evidence: evidence}, nil, nil); err != nil {
				t.Fatal(err)
			}
			for _, result := range []string{slash, client.GetLastPrompt()} {
				for _, want := range []string{"Coverage: PARTIAL", "Failed to acquire 1 member files", "zzz-unreadable.txt"} {
					if !strings.Contains(result, want) {
						t.Fatalf("missing %q:\n%s", want, result)
					}
				}
				if strings.Contains(result, "No matching files found") || strings.Contains(result, "names no file") {
					t.Fatalf("false absence in partial acquisition:\n%s", result)
				}
				if positive > 0 && !strings.Contains(result, "MATCH-PATH-SENTINEL-00.txt") {
					t.Fatal("positive path disappeared after a read failure")
				}
			}
		})
	}
}

func TestSlashSearch_PartialTraversalPreservesPositivePaths(t *testing.T) {
	ws := t.TempDir()
	writeSearchMatches(t, ws, 1)
	dir := filepath.Join(ws, "zzz-unreadable-dir")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, err := os.ReadDir(dir); err == nil {
		t.Skip("host can read mode-000 directories")
	}
	m := NewTestModel(WithWorkspace(ws))
	t.Cleanup(m.shutdownCancel)
	next, _ := m.handleCommand("/search literal.[needle]")
	reply := lastAssistant(next.(Model))
	for _, want := range []string{"MATCH-PATH-SENTINEL-00.txt", "Coverage: PARTIAL", "Search stopped:"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("missing %q in partial traversal:\n%s", want, reply)
		}
	}
}

func TestSearchInFiles_StreamedLiteralAndCancellation(t *testing.T) {
	ws := t.TempDir()
	// Literal straddles the acquisition buffer, appears twice, and contains
	// regexp punctuation. A long line must not require a whole-line buffer.
	body := strings.Repeat("x", 32*1024-4) + "literal.[needle]" + strings.Repeat("x", 100000) + "literal.[needle]"
	if err := os.WriteFile(filepath.Join(ws, "large.txt"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "nonmatching.txt"), []byte(strings.Repeat("x", 100000)), 0o600); err != nil {
		t.Fatal(err)
	}
	search := searchInFiles(context.Background(), ws, "literal.[needle]", 10)
	if !search.complete() || len(search.Paths) != 1 || search.FilesAttempted != 2 || search.BytesRead < 100000 {
		t.Fatalf("streamed search = %+v", search)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	search = searchInFiles(ctx, ws, "literal.[needle]", 10)
	if search.complete() || search.TerminalError != context.Canceled.Error() || search.BytesRead != 0 {
		t.Fatalf("canceled search = %+v", search)
	}
}

func TestSearchInFiles_DeniesOutsideSymlink(t *testing.T) {
	ws := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("literal.[needle]"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(ws, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	search := searchInFiles(context.Background(), ws, "literal.[needle]", 10)
	if search.complete() || len(search.Paths) != 0 || search.FailedFiles != 1 || search.BytesRead != 0 {
		t.Fatalf("outside source was acquired or disguised as absence: %+v", search)
	}
}

func TestSearchInFiles_AtLimitDoesNotInventOmittedMatches(t *testing.T) {
	ws := t.TempDir()
	writeSearchMatches(t, ws, 10)
	if err := os.WriteFile(filepath.Join(ws, "zzz-nonmatching.txt"), []byte("other text"), 0o600); err != nil {
		t.Fatal(err)
	}
	search := searchInFiles(context.Background(), ws, "literal needle", 10)
	if search.complete() || !search.AtLimit || search.FilesAttempted != 10 || len(search.Paths) != 10 {
		t.Fatalf("unsearched member must leave completeness unknown: %+v", search)
	}
	if !strings.Contains(renderSearchEvidence(search), "Further match count is unknown") {
		t.Fatal("the cap was presented as a known omitted match count")
	}
}

func TestSlashSearch_UnavailableDomainCannotEstablishAbsence(t *testing.T) {
	m := NewTestModel(WithWorkspace(filepath.Join(t.TempDir(), "missing-root")))
	t.Cleanup(m.shutdownCancel)
	next, _ := m.handleCommand("/search literal needle")
	reply := lastAssistant(next.(Model))
	if !strings.Contains(reply, "Coverage: PARTIAL") || !strings.Contains(reply, "Search stopped:") || strings.Contains(reply, "No matching files found") {
		t.Fatalf("unavailable domain looked like complete absence:\n%s", reply)
	}
}

func TestGatherWorkspaceEvidence_SearchQueryIsNotAnUnreadFile(t *testing.T) {
	ws := t.TempDir()
	for _, query := range []string{"plain term", "literal.[needle]", "missing.go"} {
		search := searchInFiles(context.Background(), ws, query, 10)
		have := []evidenceObservation{{Search: &search}}
		if got := gatherWorkspaceEvidence(ws, perception.Intent{Target: query}, have); len(got) != 0 {
			t.Fatalf("search literal %q was reinterpreted as a file: %+v", query, got)
		}
		// An explicit scope still asks for a body; a path match cannot stand
		// in for one, even when its string equals the search query.
		got := gatherWorkspaceEvidence(ws, perception.Intent{Target: query, ScopeFile: "scoped.go"}, have)
		if len(got) != 1 || got[0].Path != "scoped.go" || got[0].Coverage != coverageUnavailable {
			t.Fatalf("explicit file scope lost to search: %+v", got)
		}
	}
}
