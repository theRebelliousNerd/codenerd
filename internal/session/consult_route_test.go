package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// scriptedConsultant answers from a queue of outcomes and keeps the model
// each call ran under.
type scriptedConsultant struct {
	mu       sync.Mutex
	outcomes []error
	models   []string
}

func (c *scriptedConsultant) SpawnConsultation(ctx context.Context, agent, _ string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	model, _ := types.ModelNameFromContext(ctx)
	c.models = append(c.models, model)
	var err error
	if len(c.outcomes) > 0 {
		err, c.outcomes = c.outcomes[0], c.outcomes[1:]
	}
	if err != nil {
		return "", err
	}
	return "ROUTE_ADVICE from " + agent, nil
}

func (c *scriptedConsultant) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.models)
}

func routeFixture(t *testing.T) (*Executor, string) {
	t.Helper()
	e := newObligationExec(t)
	ws := e.config.WorkspaceRoot
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return e, ws
}

func ask(e *Executor, sel experts.Selection, delivered map[string]int) string {
	return e.withExpertConsultations(context.Background(), "turn prompt", perception.Intent{Verb: "/fix", Target: "a.go"}, "fix a", sel, &ExecutionResult{}, delivered)
}

// Unchanged filesystem samples do not certify the evidence behind an answer.
// A new result asks again, including through a clone sharing attempt history.
func TestConsultRoute_AsksAgainWithoutEvidenceReceipts(t *testing.T) {
	e, ws := routeFixture(t)
	c := &scriptedConsultant{}
	e.SetExpertConsultant(c)
	sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"a.go"}}}}

	ask(e, sel, nil)
	second := ask(e.CloneForTask(), sel, nil)
	if c.calls() != 2 || !strings.Contains(second, "ROUTE_ADVICE from leadexpert") || strings.Contains(second, "reused") {
		t.Fatalf("a completed answer was reused without evidence receipts (%d call(s)):\n%s", c.calls(), second)
	}
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package a\n\nvar x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ask(e, sel, nil)
	if c.calls() != 3 {
		t.Fatalf("a card whose file changed was reused (%d call(s))", c.calls())
	}
}

// An adviser whose knowledge rows the compile already carried is not asked
// on a task that is not complex; a complex task still asks it.
func TestConsultRoute_AnAdviserWhoseKnowledgeIsInThePromptIsNotAsked(t *testing.T) {
	e, _ := routeFixture(t)
	c := &scriptedConsultant{}
	e.SetExpertConsultant(c)
	need := experts.Need{Agent: "advexpert", Spawn: experts.SpawnAdvise, Files: []string{"a.go"}}
	out := ask(e, experts.Selection{Needs: []experts.Need{need}}, map[string]int{"advexpert": 3})
	if c.calls() != 0 || !strings.Contains(out, "Not consulted: 3 row(s) of its knowledge base") {
		t.Fatalf("the adviser was asked though its knowledge was delivered (%d call(s)):\n%s", c.calls(), out)
	}
	ask(e, experts.Selection{Complexity: "high", Needs: []experts.Need{need}}, map[string]int{"advexpert": 3})
	if c.calls() != 1 {
		t.Fatalf("a complex task did not ask the adviser (%d call(s))", c.calls())
	}
}

// A question that went unanswered is asked under the stronger profile when
// one is configured, and one unanswered there too is left open.
func TestConsultRoute_AnUnansweredQuestionEscalatesThenStaysOpen(t *testing.T) {
	e, _ := routeFixture(t)
	e.SetConsultEscalation(func(ctx context.Context) context.Context {
		return types.WithModelName(ctx, "configured-stronger-model")
	})
	failed := errors.New("consultation timed out")
	c := &scriptedConsultant{outcomes: []error{failed, failed}}
	e.SetExpertConsultant(c)
	sel := experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"a.go"}}}}

	ask(e, sel, nil)
	ask(e, sel, nil)
	if c.calls() != 2 || c.models[1] != "configured-stronger-model" {
		t.Fatalf("the unanswered question was not escalated: models %q", c.models)
	}
	open := ask(e, sel, nil)
	if c.calls() != 2 || !strings.Contains(open, "Left open") {
		t.Fatalf("a question unanswered under the stronger profile was asked again (%d call(s)):\n%s", c.calls(), open)
	}
}

// A consultation that failed on a provider outage is not asked again on the
// turn's later steps, whatever files they are on: the step reads why instead.
// With a stronger profile configured, the outage escalates once, and a
// failure there too settles it. A failure that is not the provider's (a
// timeout of the consultation itself) still follows the per-question ladder.
func TestConsultRoute_AProviderOutageIsNotAskedAgainThisTurn(t *testing.T) {
	e, ws := routeFixture(t)
	for _, f := range []string{"b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(ws, f), []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outage := errors.New(`LLM generation failed: meta credential "muse1": provider request failed with status 503`)
	need := func(file string) experts.Selection {
		return experts.Selection{Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{file}}}}
	}
	step := func(e *Executor, turn *ExecutionResult, file string) string {
		return e.withExpertConsultations(context.Background(), "turn prompt", perception.Intent{Verb: "/fix", Target: file}, "fix "+file, need(file), turn, nil)
	}

	c := &scriptedConsultant{outcomes: []error{outage}}
	e.SetExpertConsultant(c)
	turn := &ExecutionResult{}
	step(e, turn, "a.go")
	second := step(e, turn, "b.go")
	if c.calls() != 1 || !strings.Contains(second, "provider error (transient)") {
		t.Fatalf("a later step asked the agent again through the same outage (%d call(s)):\n%s", c.calls(), second)
	}
	if fresh := step(e, &ExecutionResult{}, "b.go"); c.calls() != 2 || !strings.Contains(fresh, "ROUTE_ADVICE") {
		t.Fatalf("a new turn inherited the outage (%d call(s)):\n%s", c.calls(), fresh)
	}

	esc, _ := routeFixture(t)
	for _, f := range []string{"b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(esc.config.WorkspaceRoot, f), []byte("package a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	esc.SetConsultEscalation(func(ctx context.Context) context.Context {
		return types.WithModelName(ctx, "configured-stronger-model")
	})
	ec := &scriptedConsultant{outcomes: []error{outage, outage}}
	esc.SetExpertConsultant(ec)
	turn = &ExecutionResult{}
	step(esc, turn, "a.go")
	step(esc, turn, "b.go")
	if ec.calls() != 2 || ec.models[1] != "configured-stronger-model" {
		t.Fatalf("the outage did not escalate once: models %q", ec.models)
	}
	third := step(esc, turn, "c.go")
	if ec.calls() != 2 || !strings.Contains(third, "not asked again this turn") {
		t.Fatalf("an agent failed under both profiles was asked a third time (%d call(s)):\n%s", ec.calls(), third)
	}

	nc, _ := routeFixture(t)
	if err := os.WriteFile(filepath.Join(nc.config.WorkspaceRoot, "b.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cc := &scriptedConsultant{outcomes: []error{errors.New("consultation produced no plan")}}
	nc.SetExpertConsultant(cc)
	turn = &ExecutionResult{}
	step(nc, turn, "a.go")
	step(nc, turn, "b.go")
	if cc.calls() != 2 {
		t.Fatalf("a failure that is not the provider's silenced the agent for the turn (%d call(s))", cc.calls())
	}
}
