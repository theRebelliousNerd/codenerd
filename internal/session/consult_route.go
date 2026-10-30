package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The consultation routing ladder. A consultation the kernel derived
// (expert_spawn) is not automatically a model call: the policy decides, per
// agent, whether the turn relies on the expert's knowledge rows the
// compile already delivered, asks the configured stronger profile, asks as
// usual, or leaves the question open (consult_route,
// policy/shard_experts.mg). Completed answers have no evidence receipts and
// are never reused across results. Go measures -- previous attempt outcomes,
// file observations before and after each call and now, the knowledge rows the
// compile carried, whether a stronger profile is configured -- and obeys.

// Routes, the value set of consult_route/3's Route.
const (
	routeKnowledge  = "/knowledge"
	routeEscalate   = "/escalate"
	routeConsult    = "/consult"
	routeUnresolved = "/unresolved"
)

// consultHistory holds attempt outcomes, never answer bodies. Task clones
// share it so policy can escalate failures without reusing completed advice.
type consultHistory struct {
	mu      sync.Mutex
	entries map[string]consultAttempt
}

// consultAttempt records the result and complete filesystem observations of
// one call. These samples can qualify failure history, not certify advice.
type consultAttempt struct {
	disposition   string
	before, after map[string]consultFileObservation
	escalated     bool
}

func newConsultHistory() *consultHistory {
	return &consultHistory{entries: map[string]consultAttempt{}}
}

func (c *consultHistory) get(key string) (consultAttempt, bool) {
	if c == nil {
		return consultAttempt{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	return e, ok
}

func (c *consultHistory) put(key string, e consultAttempt) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = e
}

// consultQuestion is a consultation's identity: the agent, its mode, the
// verb, the files it was selected for and the request it was asked about.
func consultQuestion(need experts.Need, verb, input string) string {
	files := slices.Clone(need.Files)
	slices.Sort(files)
	sum := sha256.Sum256([]byte(strings.TrimSpace(input)))
	return strings.Join([]string{experts.AgentKey(need.Agent), need.Spawn, verb, strings.Join(files, ","), hex.EncodeToString(sum[:])}, "\x00")
}

// consultFileObservation preserves a read failure as an outcome for the
// expected file, rather than silently shrinking the dependency set.
type consultFileObservation struct {
	digest    string
	readError string
}

func observeConsultFiles(workspace string, files []string) map[string]consultFileObservation {
	out := make(map[string]consultFileObservation, len(files))
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(f)))
		if err != nil {
			out[f] = consultFileObservation{readError: err.Error()}
			continue
		}
		sum := sha256.Sum256(data)
		out[f] = consultFileObservation{digest: hex.EncodeToString(sum[:])}
	}
	return out
}

// consultObservationFacts ferries measurements; coverage and stability are
// judgments in policy/shard_experts.mg. Errors carry no pretend digest.
func consultObservationFacts(ask string, agent types.MangleAtom, phase string, observed map[string]consultFileObservation) []types.Fact {
	facts := make([]types.Fact, 0, len(observed))
	for file, observation := range observed {
		status := types.MangleAtom("/read")
		if observation.readError != "" {
			status = types.MangleAtom("/unreadable")
		}
		facts = append(facts, types.Fact{Predicate: "consult_file_observation", Args: []any{
			ask, agent, file, types.MangleAtom(phase), status, observation.digest,
		}})
	}
	return facts
}

// knowledgeDelivered counts, per agent key, the knowledge rows of that
// expert a compile's prompt carries.
func knowledgeDelivered(compiled *prompt.CompilationResult) map[string]int {
	out := map[string]int{}
	if compiled == nil {
		return out
	}
	for _, atom := range compiled.IncludedAtoms {
		if atom != nil && atom.KnowledgeExpert != "" {
			out[experts.AgentKey(atom.KnowledgeExpert)]++
		}
	}
	return out
}

var consultAskSeq atomic.Int64

// consultPlan retains pre-call measurements for one routed consultation.
type consultPlan struct {
	need   experts.Need
	route  string
	key    string
	before map[string]consultFileObservation
	rows   int
}

// routeConsultations asks the kernel the route of each consultation. A
// kernel that cannot be asked routes every one to an ordinary consultation,
// which is what the turn did before the ladder existed.
func (e *Executor) routeConsultations(spawns []experts.Need, verb, input, complexity string, delivered map[string]int) []consultPlan {
	workspace := e.workspaceForVerification()
	ask := fmt.Sprintf("consult_ask_%d", consultAskSeq.Add(1))
	plans := make([]consultPlan, len(spawns))
	var facts []types.Fact
	if complexity == "high" {
		facts = append(facts, types.Fact{Predicate: "consult_ask_complex", Args: []any{ask}})
	}
	if e.escalationAvailable() {
		facts = append(facts, types.Fact{Predicate: "consult_escalation_available", Args: []any{ask}})
	}
	for i, need := range spawns {
		agent := experts.AgentAtom(need.Agent)
		key := consultQuestion(need, verb, input)
		plans[i] = consultPlan{need: need, route: routeConsult, key: key,
			before: observeConsultFiles(workspace, need.Files), rows: delivered[experts.AgentKey(need.Agent)]}
		facts = append(facts, types.Fact{Predicate: "consult_ask", Args: []any{ask, agent, types.MangleAtom("/" + need.Spawn)}})
		if plans[i].rows > 0 {
			facts = append(facts, types.Fact{Predicate: "consult_knowledge_delivered", Args: []any{ask, agent, int64(plans[i].rows)}})
		}
		for _, file := range need.Files {
			facts = append(facts, types.Fact{Predicate: "consult_ask_file", Args: []any{ask, agent, file}})
		}
		facts = append(facts, consultObservationFacts(ask, agent, "/current", plans[i].before)...)
		if cached, ok := e.consultAttempts.get(key); ok {
			facts = append(facts, types.Fact{Predicate: "consult_card_cached", Args: []any{ask, agent, types.MangleAtom(cached.disposition)}})
			if cached.escalated {
				facts = append(facts, types.Fact{Predicate: "consult_card_escalated", Args: []any{ask, agent}})
			}
			facts = append(facts, consultObservationFacts(ask, agent, "/before", cached.before)...)
			facts = append(facts, consultObservationFacts(ask, agent, "/after", cached.after)...)
		}
	}
	if e.kernel == nil {
		return plans
	}
	if err := e.kernel.AssertBatch(facts); err != nil {
		logging.Get(logging.CategorySession).Warn("consultation routing could not be asked (%v); every consultation runs", err)
		return plans
	}
	defer func() {
		if err := e.kernel.RetractExactFactsBatch(facts); err != nil {
			logging.Get(logging.CategorySession).Warn("consultation routing facts not retracted: %v", err)
		}
	}()
	rows, err := e.kernel.Query("consult_route")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("consultation routing could not be read (%v); every consultation runs", err)
		return plans
	}
	routes := map[string]string{}
	for _, row := range rows {
		id, okA := nthArg(row.Args, 0)
		agent, okG := nthArg(row.Args, 1)
		route, okR := nthArg(row.Args, 2)
		if okA && okG && okR && types.ExtractString(id) == ask {
			routes[experts.AgentKey(types.ExtractString(agent))] = types.ExtractString(route)
		}
	}
	for i := range plans {
		if r, ok := routes[experts.AgentKey(plans[i].need.Agent)]; ok {
			plans[i].route = r
		}
	}
	return plans
}

// SetConsultEscalation installs the stronger consultation profile: apply
// puts it on a consultation's context. Boot installs it only when the user's
// configuration names one (shard_profiles.consultation_escalation); nil
// means there is no stronger route.
func (e *Executor) SetConsultEscalation(apply func(context.Context) context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.consultEscalation = apply
}

// HasConsultEscalation reports whether boot installed a stronger
// consultation profile, so boot can be checked for it.
func (e *Executor) HasConsultEscalation() bool { return e.escalationAvailable() }

// escalationAvailable reports whether a stronger consultation profile is
// installed.
func (e *Executor) escalationAvailable() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.consultEscalation != nil
}

// escalatedContext is the context a consultation the policy escalated runs
// under: the stronger profile, over whatever the turn's profile set.
func (e *Executor) escalatedContext(ctx context.Context) context.Context {
	e.mu.RLock()
	apply := e.consultEscalation
	e.mu.RUnlock()
	if apply == nil {
		return ctx
	}
	return apply(ctx)
}
