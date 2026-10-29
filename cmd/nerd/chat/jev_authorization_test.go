package chat

import (
	"context"
	"sync"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// "Fix X, then commit it" is a request that names the commit. Jev reads the
// user's message (asks_to_commit), the kernel derives the authorization
// (jev_authorization, policy/jev_evidence.mg), and the chat carries it on the
// turn's context, so the task delegated from the turn holds it. These run the
// real processInput with a fake Jev server and a task executor that records
// what its context carries.

type authProbeExecutor struct {
	*processControlTaskExecutor
	mu   sync.Mutex
	seen []perception.Authorization
	rest []perception.Restriction
}

func (e *authProbeExecutor) note(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seen = append(e.seen, perception.AuthorizationFromContext(ctx))
	e.rest = append(e.rest, perception.RestrictionFromContext(ctx))
}

func (e *authProbeExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	e.note(ctx)
	return e.processControlTaskExecutor.Execute(ctx, req)
}

func (e *authProbeExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, sc *types.SessionContext, p types.SpawnPriority) (string, error) {
	e.note(ctx)
	return e.processControlTaskExecutor.ExecuteWithContext(ctx, req, sc, p)
}

func (e *authProbeExecutor) ExecuteObserved(ctx context.Context, req session.TaskRequest) (observation.Return, error) {
	e.note(ctx)
	return e.processControlTaskExecutor.ExecuteObserved(ctx, req)
}

func (e *authProbeExecutor) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, sc *types.SessionContext, p types.SpawnPriority) (observation.Return, error) {
	e.note(ctx)
	return e.processControlTaskExecutor.ExecuteObservedWithContext(ctx, req, sc, p)
}

func (e *authProbeExecutor) lastAuthorization(t *testing.T) perception.Authorization {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.seen) == 0 {
		t.Fatal("no task was delegated")
	}
	return e.seen[len(e.seen)-1]
}

// delegatedTurn is a turn the kernel delegates without a verification
// obligation, so the rig's executor (which has no task verifier behind it)
// sees the context the turn carries. A mutation reaches the same executor
// through the verifier with the same context (process.go passes ctx to
// VerifyWithRetry); the authorization is derived from Jev's reading of the
// message, not from the verb.
func delegatedTurn() perception.Intent {
	return perception.Intent{Category: "/instruction", Verb: "/fix", Target: "README.md", Confidence: 0.95, Response: "Fixing it."}
}

func authJourney(t *testing.T, mode config.WorkMode, jev map[string]float64) (Model, *authProbeExecutor, *fakeJevServer) {
	t.Helper()
	m, base := journeyModel(t, mode, delegatedTurn())
	probe := &authProbeExecutor{processControlTaskExecutor: base}
	m.taskExecutor = probe
	server := &fakeJevServer{probs: jev}
	return withJev(t, m, server), probe, server
}

var fixAndCommit = map[string]float64{
	"changes_files": 0.95, "names_target": 0.95, "hypothetical": 0.02, "rules_out_changes": 0.02, "asks_to_commit": 0.98,
}

func with(base map[string]float64, key string, p float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range base {
		out[k] = v
	}
	out[key] = p
	return out
}

func TestJevAuthorization_ThenCommitItReachesTheDelegatedTask(t *testing.T) {
	m, probe, _ := authJourney(t, config.WorkModeYolo, fixAndCommit)
	say(t, m, "fix the typo in README.md, then commit it")
	if got := probe.lastAuthorization(t); !got.Has(perception.AuthorizeCommit) || len(got.Kinds()) != 1 {
		t.Fatalf("the delegated task holds %v, want exactly commit", got.Kinds())
	}
}

func TestJevAuthorization_NoCommitWordGrantsNothing(t *testing.T) {
	for name, jev := range map[string]map[string]float64{
		"jev says no":           with(fixAndCommit, "asks_to_commit", 0.03),
		"jev unsettled":         with(fixAndCommit, "asks_to_commit", 0.5),
		"jev says nothing":      {"changes_files": 0.95, "names_target": 0.95},
		"commit but read only":  with(fixAndCommit, "rules_out_changes", 0.95),
		"commit but a what-if":  with(fixAndCommit, "hypothetical", 0.95),
		"yes just below thresh": with(fixAndCommit, "asks_to_commit", 0.69),
	} {
		t.Run(name, func(t *testing.T) {
			m, probe, _ := authJourney(t, config.WorkModeYolo, jev)
			m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "fix the typo in README.md", nil))
			if got := m.jevAuthorization(); !got.IsZero() {
				t.Fatalf("authorization = %v, want none", got.Kinds())
			}
			say(t, m, "fix the typo in README.md")
			probe.mu.Lock()
			defer probe.mu.Unlock()
			// A what-if takes the dream lane and delegates nothing; every other
			// case delegates, and its task is what the control inspects.
			if len(probe.seen) == 0 && name != "commit but a what-if" {
				t.Fatal("no task was delegated, so the control proves nothing")
			}
			for _, a := range probe.seen {
				if !a.IsZero() {
					t.Fatalf("a delegated task held %v", a.Kinds())
				}
			}
		})
	}
}

// Jev off or failing is no evidence: the turn runs as before, holding nothing.
func TestJevAuthorization_JevFailureGrantsNothing(t *testing.T) {
	m, probe, server := authJourney(t, config.WorkModeYolo, fixAndCommit)
	server.fail = true
	say(t, m, "fix the typo in README.md, then commit it")
	if got := probe.lastAuthorization(t); !got.IsZero() {
		t.Fatalf("a failed Jev call produced an authorization: %v", got.Kinds())
	}
}

// Last turn's judgment does not leak into the next turn's authorization.
func TestJevAuthorization_DoesNotOutliveItsTurn(t *testing.T) {
	m, probe, server := authJourney(t, config.WorkModeYolo, fixAndCommit)
	m, _, _ = say(t, m, "fix the typo in README.md, then commit it")
	if !probe.lastAuthorization(t).Has(perception.AuthorizeCommit) {
		t.Fatal("the first turn did not carry the authorization")
	}
	server.set(with(fixAndCommit, "asks_to_commit", 0.02))
	m.transducer = &scriptedTransducer{intents: []perception.Intent{delegatedTurn()}}
	say(t, m, "fix another typo in README.md")
	if got := probe.lastAuthorization(t); !got.IsZero() {
		t.Fatalf("the next turn inherited %v", got.Kinds())
	}
}

// Work held for a go-ahead keeps the authorization its request earned: the
// "go" that approves it asks no judgments, and the held intent is the request.
func TestJevAuthorization_HeldWorkKeepsItsRequestsAuthorizationThroughGo(t *testing.T) {
	m, probe, server := authJourney(t, config.WorkModePropose, fixAndCommit)
	m, msg, _ := say(t, m, "fix the typo in README.md, then commit it")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("propose answered %T, want a hold", msg)
	}
	if m.pendingWork == nil || !m.pendingWork.Intent.Authorization.Has(perception.AuthorizeCommit) {
		t.Fatalf("the held work lost the request's authorization: %+v", m.pendingWork)
	}
	// The go-ahead is not a commit request; Jev would say no to the bare word.
	server.set(with(fixAndCommit, "asks_to_commit", 0.02))
	m, cmd := m.approvePendingWork()
	_ = m
	awaitProcessStream(t, cmd())
	if got := probe.lastAuthorization(t); !got.Has(perception.AuthorizeCommit) {
		t.Fatalf("the approved work's task holds %v, want commit", got.Kinds())
	}
}

// A held request that earned no authorization does not gain one from "go".
func TestJevAuthorization_GoCannotGrantWhatTheRequestDidNot(t *testing.T) {
	m, probe, server := authJourney(t, config.WorkModePropose, with(fixAndCommit, "asks_to_commit", 0.02))
	m, _, _ = say(t, m, "fix the typo in README.md")
	server.set(with(fixAndCommit, "asks_to_commit", 0.99)) // the word "go" judged as a commit would be a bug
	m, cmd := m.approvePendingWork()
	_ = m
	awaitProcessStream(t, cmd())
	if got := probe.lastAuthorization(t); !got.IsZero() {
		t.Fatalf("approving held work granted %v", got.Kinds())
	}
}
