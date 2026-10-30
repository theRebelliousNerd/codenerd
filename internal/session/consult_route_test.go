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
