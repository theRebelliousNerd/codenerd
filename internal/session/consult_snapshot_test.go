package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

type snapshotConsultant func(context.Context, string, string) (string, error)

func (f snapshotConsultant) SpawnConsultation(ctx context.Context, agent, task string) (string, error) {
	return f(ctx, agent, task)
}

func writeConsultSource(t *testing.T, workspace, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(workspace, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// These drive the shipping session helper and the full production kernel.
// The controlled consultant actually reads the source before returning advice;
// it is not a live specialist/model evaluation.
func TestConsultSnapshot_CompletedAdviceIsNeverCertifiedByFilesystemSamples(t *testing.T) {
	for _, scenario := range []string{
		"unchanged", "changed_after_publication", "deleted_after_publication",
		"changed_during_call", "deleted_during_call", "missing_then_created",
		"unreadable_then_readable", "mixed_coverage", "fileless", "aba",
	} {
		t.Run(scenario, func(t *testing.T) {
			e, workspace := routeFixture(t)
			file := filepath.Join(workspace, "a.go")
			files := []string{"a.go"}
			if scenario == "missing_then_created" || scenario == "unreadable_then_readable" {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "unreadable_then_readable" {
				// A directory gives a portable ReadFile error without chmod,
				// which can still be readable when tests run as root.
				if err := os.Mkdir(file, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "mixed_coverage" {
				files = append(files, "missing.go")
			}
			if scenario == "fileless" {
				files = nil
			}
			if scenario == "aba" {
				writeConsultSource(t, workspace, "a.go", "revision A")
			}
			sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: files}}}
			calls := 0
			e.SetExpertConsultant(snapshotConsultant(func(_ context.Context, _, _ string) (string, error) {
				calls++
				if calls == 1 && scenario == "aba" {
					// The consultant sees B while both endpoint samples see A.
					if err := os.WriteFile(file, []byte("revision B"), 0o644); err != nil {
						return "", err
					}
				}
				body, readErr := os.ReadFile(file)
				answer := fmt.Sprintf("ADVICE_CALL_%d: %s (read error: %v)", calls, body, readErr)
				if calls == 1 {
					switch scenario {
					case "changed_during_call":
						return answer, os.WriteFile(file, []byte("revision B"), 0o644)
					case "deleted_during_call":
						return answer, os.Remove(file)
					case "aba":
						return answer, os.WriteFile(file, []byte("revision A"), 0o644)
					}
				}
				return answer, nil
			}))

			first := ask(e, sel, nil)
			if !strings.Contains(first, "ADVICE_CALL_1:") {
				t.Fatalf("first advice was not delivered: %s", first)
			}
			switch scenario {
			case "changed_after_publication", "deleted_during_call", "missing_then_created":
				writeConsultSource(t, workspace, "a.go", "revision B")
			case "deleted_after_publication":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			case "unreadable_then_readable":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				writeConsultSource(t, workspace, "a.go", "revision B")
			case "mixed_coverage":
				writeConsultSource(t, workspace, "missing.go", "package missing")
			}
			second := ask(e.CloneForTask(), sel, nil)
			if calls != 2 || !strings.Contains(second, "ADVICE_CALL_2:") ||
				strings.Contains(second, "ADVICE_CALL_1:") || strings.Contains(second, "reused") {
				t.Fatalf("completed advice escaped into a later result (%d calls): %s", calls, second)
			}
		})
	}
}

// This records the real input boundary while forwarding every operation to
// the production kernel, including derivation and exact retraction.
type consultBoundaryKernel struct {
	types.Kernel
	facts []types.Fact
}

func (k *consultBoundaryKernel) AssertBatch(facts []types.Fact) error {
	k.facts = append(k.facts, facts...)
	return k.Kernel.AssertBatch(facts)
}

func TestConsultSnapshot_RoutingPreservesUnreadableCoverage(t *testing.T) {
	e, workspace := routeFixture(t)
	if err := os.Mkdir(filepath.Join(workspace, "unreadable.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	k := &consultBoundaryKernel{Kernel: e.kernel}
	e.kernel = k
	e.SetExpertConsultant(&scriptedConsultant{})
	sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead,
		Files: []string{"a.go", "unreadable.go", "missing.go"}}}}
	ask(e, sel, nil)
	k.facts = nil
	ask(e, sel, nil)
	for _, file := range sel.Needs[0].Files {
		expected := false
		phases := map[string]bool{}
		for _, fact := range k.facts {
			if fact.Predicate == "consult_ask_file" && types.ExtractString(fact.Args[2]) == file {
				expected = true
			}
			if fact.Predicate != "consult_file_observation" || types.ExtractString(fact.Args[2]) != file {
				continue
			}
			phase := types.ExtractString(fact.Args[3])
			status := types.ExtractString(fact.Args[4])
			digest := types.ExtractString(fact.Args[5])
			if file == "a.go" && (status != "/read" || digest == "") {
				t.Fatalf("readable observation lost its digest: %v", fact)
			}
			if file != "a.go" && (status != "/unreadable" || digest != "") {
				t.Fatalf("unreadable observation was treated as readable: %v", fact)
			}
			phases[phase] = true
		}
		if !expected || !phases["/before"] || !phases["/after"] || !phases["/current"] {
			t.Fatalf("%s lost expected coverage or a read outcome: expected=%v phases=%v", file, expected, phases)
		}
	}
	for _, predicate := range []string{"consult_ask", "consult_ask_file", "consult_file_observation", "consult_card_cached", "consult_route"} {
		rows, err := k.Query(predicate)
		if err != nil || len(rows) != 0 {
			t.Fatalf("routing left %s facts behind: %v, %v", predicate, rows, err)
		}
	}
}

// A sibling changes a.go after the fast consultant returned and blocks batch
// publication until it has done so. This widens the original bug's window
// deterministically, without sleeps or a scheduler race.
func TestConsultSnapshot_SiblingWaitDoesNotRedateAnAnswer(t *testing.T) {
	e, workspace := routeFixture(t)
	writeConsultSource(t, workspace, "b.go", "package b")
	fastReturned := make(chan struct{})
	mutated := make(chan struct{})
	e.SetExpertConsultant(snapshotConsultant(func(_ context.Context, agent, _ string) (string, error) {
		if agent == "fast" {
			body, err := os.ReadFile(filepath.Join(workspace, "a.go"))
			defer close(fastReturned)
			return "OLD_FAST_ADVICE: " + string(body), err
		}
		<-fastReturned
		err := os.WriteFile(filepath.Join(workspace, "a.go"), []byte("revision B"), 0o644)
		close(mutated)
		return "SLOW_ADVICE", err
	}))
	fast := experts.Need{Agent: "fast", Spawn: experts.SpawnLead, Files: []string{"a.go"}}
	sel := experts.Selection{Needs: []experts.Need{fast, {Agent: "slow", Spawn: experts.SpawnLead, Files: []string{"b.go"}}}}
	first := ask(e, sel, nil)
	<-mutated
	if !strings.Contains(first, "OLD_FAST_ADVICE:") || !strings.Contains(first, "SLOW_ADVICE") {
		t.Fatalf("fixture did not deliver both answers: %s", first)
	}
	calls := 0
	e.SetExpertConsultant(snapshotConsultant(func(_ context.Context, _, _ string) (string, error) {
		calls++
		body, err := os.ReadFile(filepath.Join(workspace, "a.go"))
		return "NEW_FAST_ADVICE: " + string(body), err
	}))
	second := ask(e, experts.Selection{Needs: []experts.Need{fast}}, nil)
	if calls != 1 || !strings.Contains(second, "NEW_FAST_ADVICE: revision B") || strings.Contains(second, "OLD_FAST_ADVICE") {
		t.Fatalf("batch publication redated the old answer: %s", second)
	}
}

func TestConsultSnapshot_IncompleteFailureHistoryCannotSuppressConsultation(t *testing.T) {
	for _, scenario := range []string{"missing", "unreadable", "mixed", "fileless", "mutated_during_failure", "deleted_during_failure"} {
		t.Run(scenario, func(t *testing.T) {
			e, workspace := routeFixture(t)
			file := filepath.Join(workspace, "a.go")
			files := []string{"a.go"}
			switch scenario {
			case "missing", "unreadable":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
				if scenario == "unreadable" {
					if err := os.Mkdir(file, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			case "mixed":
				files = append(files, "missing.go")
			case "fileless":
				files = nil
			}
			e.SetConsultEscalation(func(ctx context.Context) context.Context { return ctx })
			calls := 0
			e.SetExpertConsultant(snapshotConsultant(func(_ context.Context, _, _ string) (string, error) {
				calls++
				if scenario == "mutated_during_failure" {
					if err := os.WriteFile(file, []byte(fmt.Sprintf("revision %d", calls)), 0o644); err != nil {
						return "", err
					}
				}
				if scenario == "deleted_during_failure" && calls == 1 {
					if err := os.Remove(file); err != nil {
						return "", err
					}
				}
				return "", errors.New("unanswered")
			}))
			sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: files}}}
			for range 3 {
				out := ask(e, sel, nil)
				if strings.Contains(out, "Left open") || strings.Contains(out, "stronger consultation profile") {
					t.Fatalf("incomplete or unstable failure history affected routing: %s", out)
				}
			}
			if calls != 3 {
				t.Fatalf("an unknown dependency set suppressed consultation: %d calls", calls)
			}
		})
	}
}

func TestConsultSnapshot_ChangedFilesRestartFailureEscalation(t *testing.T) {
	e, workspace := routeFixture(t)
	e.SetConsultEscalation(func(ctx context.Context) context.Context { return ctx })
	failed := errors.New("unanswered")
	c := &scriptedConsultant{outcomes: []error{failed, failed, failed, failed}}
	e.SetExpertConsultant(c)
	sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"a.go"}}}}
	ask(e, sel, nil)
	ask(e, sel, nil)
	if out := ask(e, sel, nil); c.calls() != 2 || !strings.Contains(out, "Left open") {
		t.Fatalf("fixture did not exhaust escalation: %s", out)
	}
	writeConsultSource(t, workspace, "a.go", "revision B")
	if out := ask(e, sel, nil); c.calls() != 3 || strings.Contains(out, "Left open") || strings.Contains(out, "stronger consultation profile") {
		t.Fatalf("a new revision did not restart ordinary consultation: %s", out)
	}
	if out := ask(e, sel, nil); c.calls() != 4 || !strings.Contains(out, "stronger consultation profile") {
		t.Fatalf("old escalation history suppressed escalation on the new revision: %s", out)
	}
}

func TestConsultSnapshot_SameResultStillSuppressesCoveredScope(t *testing.T) {
	e, _ := routeFixture(t)
	c := &scriptedConsultant{}
	e.SetExpertConsultant(c)
	sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"a.go"}}}}
	result := &ExecutionResult{}
	intent := perception.Intent{Verb: "/fix", Target: "a.go"}
	first := e.withExpertConsultations(t.Context(), "prompt", intent, "fix a", sel, result, nil)
	second := e.withExpertConsultations(t.Context(), first, intent, "fix a", sel, result, nil)
	if c.calls() != 1 || second != first || len(result.expertCards) != 1 {
		t.Fatalf("within-result continuity bought a duplicate consultation: %d calls, %d cards", c.calls(), len(result.expertCards))
	}
}
