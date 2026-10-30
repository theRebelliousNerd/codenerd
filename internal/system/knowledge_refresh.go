package system

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/experts"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// Shard-agent knowledge is never static. The background refresh measures
// every roster agent's KB (what each source last acquired and when, what
// consultations retrieved, which questions they found nothing for), the
// subjects each agent should know (its registry topics and its domains'
// pinned packages) and which sources can run, asserts it, and does what
// policy/knowledge_refresh.mg derives: the budget's worth of subjects to
// acquire again, most urgent and most used first, and the acquired rows
// nobody used to prune. It runs as Cortex-owned background work on the
// schedule the policy sets (knowledge_refresh_schedule): a first run after a
// delay, so a short-lived process spends nothing, then again on an interval
// for as long as the session lives.

// knowledgeRefreshPredicates are the facts one run measures; each run
// replaces the last run's.
var knowledgeRefreshPredicates = []string{
	"knowledge_wanted", "knowledge_outcome", "knowledge_source_available",
	"knowledge_atom_held", "knowledge_atom_retrieved", "knowledge_miss",
	"knowledge_acquired_version", "dependency_pin",
}

// knowledgeRefresher is one Cortex's refresh: what it reads and the sources
// it acquires from.
type knowledgeRefresher struct {
	workspace string
	kernel    types.Kernel
	engine    embedding.EmbeddingEngine
	acquirer  knowledge.Acquirer
	now       func() time.Time
	// mu serializes runs (a scheduled run and an explicit one).
	mu sync.Mutex
}

// knowledgeRefreshReport is what one run did.
type knowledgeRefreshReport struct {
	// Refreshed names each subject acquired again, as agent:subject (reason).
	Refreshed []string
	Outcomes  []knowledge.Outcome
	Added     int
	Pruned    int64
}

// rosterKB is one roster agent's identity and KB.
type rosterKB struct {
	key    string
	atom   types.MangleAtom
	path   string
	topics []string
}

// run measures, asserts, and does what the policy derives.
func (r *knowledgeRefresher) run(ctx context.Context) (knowledgeRefreshReport, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var report knowledgeRefreshReport
	if r.kernel == nil || strings.TrimSpace(r.workspace) == "" {
		return report, nil
	}
	now := time.Now
	if r.now != nil {
		now = r.now
	}
	roster, err := r.roster()
	if err != nil {
		return report, err
	}
	if len(roster) == 0 {
		return report, nil
	}

	techs, err := r.domains()
	if err != nil {
		return report, err
	}
	var facts []types.Fact
	subjects := map[string]map[string]knowledge.Subject{}
	var failures []string
	// What the manifests pin now: a subject acquired at another version
	// is researched again (knowledge_refresh /version_changed).
	pins, err := nerdinit.ManifestPins(r.workspace)
	if err != nil {
		failures = append(failures, err.Error())
	}
	seenPin := map[[2]string]bool{}
	for _, pin := range pins {
		id := [2]string{pin.Module, pin.Version}
		if pin.Version == "" || seenPin[id] {
			continue
		}
		seenPin[id] = true
		facts = append(facts, types.Fact{Predicate: "dependency_pin", Args: []any{pin.Module, pin.Version}})
	}
	for _, agent := range roster {
		wanted := nerdinit.KnowledgeSubjects(r.workspace, agent.key, techs[agent.key], agent.topics, pins)
		subjects[agent.key] = map[string]knowledge.Subject{}
		for _, s := range wanted {
			subjects[agent.key][s.Key()] = s
			facts = append(facts, types.Fact{Predicate: "knowledge_wanted", Args: []any{agent.atom, s.Key()}})
		}
		measured, err := measureAgentKB(ctx, agent, now())
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", agent.key, err))
			continue
		}
		facts = append(facts, measured...)
	}
	for _, source := range r.acquirer.Available() {
		facts = append(facts, types.Fact{Predicate: "knowledge_source_available", Args: []any{source}})
	}
	for _, pred := range knowledgeRefreshPredicates {
		if err := r.kernel.Retract(pred); err != nil {
			return report, fmt.Errorf("retract %s: %w", pred, err)
		}
	}
	if len(facts) > 0 {
		if err := r.kernel.AssertBatch(facts); err != nil {
			return report, fmt.Errorf("assert knowledge measurements: %w", err)
		}
	}

	plan, err := r.plan(subjects)
	if err != nil {
		return report, err
	}
	byAgent := map[string]rosterKB{}
	for _, agent := range roster {
		byAgent[agent.key] = agent
	}
	for _, key := range sortedKeys(plan.refresh) {
		agent := byAgent[key]
		db, err := store.NewLocalStore(agent.path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: open KB: %v", key, err))
			continue
		}
		if r.engine != nil {
			db.SetEmbeddingEngine(r.engine)
		}
		added, outcomes, err := knowledge.Refresh(ctx, db, r.acquirer, plan.refresh[key])
		_ = db.Close()
		report.Added += added
		report.Outcomes = append(report.Outcomes, outcomes...)
		for _, s := range plan.refresh[key] {
			report.Refreshed = append(report.Refreshed, fmt.Sprintf("%s:%s (%s)", key, s.Key(), plan.reasons[key+"\x00"+s.Key()]))
		}
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: refresh: %v", key, err))
		}
	}
	for _, key := range sortedKeys(plan.prune) {
		agent := byAgent[key]
		db, err := store.NewLocalStore(agent.path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: open KB: %v", key, err))
			continue
		}
		n, err := db.DeleteKnowledgeAtomsByConcept(plan.prune[key]...)
		_ = db.Close()
		report.Pruned += n
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: prune: %v", key, err))
		}
	}
	if len(failures) > 0 {
		return report, errors.New(strings.Join(failures, "; "))
	}
	return report, nil
}

// roster is the ready agents of .nerd/agents.json with their KB paths.
func (r *knowledgeRefresher) roster() ([]rosterKB, error) {
	reg, _, err := loadAgentRegistry(filepath.Join(r.workspace, ".nerd", "agents.json"))
	if err != nil {
		return nil, fmt.Errorf("read agents.json: %w", err)
	}
	var out []rosterKB
	for _, agent := range reg.Agents {
		if !rosterAgent(agent) {
			continue
		}
		key := experts.AgentKey(agent.Name)
		path := agent.KnowledgePath
		if strings.TrimSpace(path) == "" {
			path = types.AgentKnowledgeDBPath(filepath.Join(r.workspace, ".nerd"), key)
		}
		out = append(out, rosterKB{key: key, atom: experts.AgentAtom(agent.Name), path: path, topics: agent.Topics})
	}
	return out, nil
}

// domains is each agent's technologies as the kernel holds them
// (shard_agent_domain, from the registry, the KB's tags and the catalog).
func (r *knowledgeRefresher) domains() (map[string][]string, error) {
	rows, err := r.kernel.Query("shard_agent_domain")
	if err != nil {
		return nil, fmt.Errorf("query shard_agent_domain: %w", err)
	}
	out := map[string][]string{}
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		key := experts.AgentKey(types.ExtractString(row.Args[0]))
		tech := strings.TrimPrefix(types.ExtractString(row.Args[1]), "/")
		if !hasString(out[key], tech) {
			out[key] = append(out[key], tech)
		}
	}
	return out, nil
}

func measureAgentKB(ctx context.Context, agent rosterKB, now time.Time) ([]types.Fact, error) {
	db, err := store.NewLocalStore(agent.path)
	if err != nil {
		return nil, fmt.Errorf("open KB: %w", err)
	}
	defer db.Close()
	return knowledge.Measure(ctx, db.GetDB(), agent.atom, now)
}

// knowledgePlan is what the policy derived for one run.
type knowledgePlan struct {
	refresh map[string][]knowledge.Subject
	reasons map[string]string
	prune   map[string][]string
}

// plan reads the derived refresh priorities within the budget, and the
// prunes.
func (r *knowledgeRefresher) plan(subjects map[string]map[string]knowledge.Subject) (knowledgePlan, error) {
	plan := knowledgePlan{refresh: map[string][]knowledge.Subject{}, reasons: map[string]string{}, prune: map[string][]string{}}
	budget, err := queryInt(r.kernel, "knowledge_refresh_budget")
	if err != nil {
		return plan, err
	}
	reasonRows, err := r.kernel.Query("knowledge_refresh")
	if err != nil {
		return plan, fmt.Errorf("query knowledge_refresh: %w", err)
	}
	gaps := map[string]bool{}
	for _, row := range reasonRows {
		if len(row.Args) < 3 {
			continue
		}
		id := experts.AgentKey(types.ExtractString(row.Args[0])) + "\x00" + types.ExtractString(row.Args[1])
		reason := strings.TrimPrefix(types.ExtractString(row.Args[2]), "/")
		if prior := plan.reasons[id]; prior != "" {
			reason = prior + "," + reason
		}
		plan.reasons[id] = reason
		if strings.Contains(reason, "gap") {
			gaps[id] = true
		}
	}
	rows, err := r.kernel.Query("knowledge_refresh_priority")
	if err != nil {
		return plan, fmt.Errorf("query knowledge_refresh_priority: %w", err)
	}
	type pick struct {
		agent, subject string
		weight, uses   int64
	}
	var picks []pick
	for _, row := range rows {
		if len(row.Args) < 4 {
			continue
		}
		picks = append(picks, pick{
			agent:   experts.AgentKey(types.ExtractString(row.Args[0])),
			subject: types.ExtractString(row.Args[1]),
			weight:  factInt64(row.Args[2]),
			uses:    factInt64(row.Args[3]),
		})
	}
	sort.Slice(picks, func(i, j int) bool {
		a, b := picks[i], picks[j]
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		if a.uses != b.uses {
			return a.uses > b.uses
		}
		if a.agent != b.agent {
			return a.agent < b.agent
		}
		return a.subject < b.subject
	})
	taken := int64(0)
	for _, p := range picks {
		if taken >= budget {
			break
		}
		s, ok := subjects[p.agent][p.subject]
		if !ok {
			if !gaps[p.agent+"\x00"+p.subject] {
				// A subject no longer wanted (its topic or domain left
				// the registry) is not researched again.
				continue
			}
			s = knowledge.Subject{Agent: p.agent, Topic: p.subject, Gap: true}
		}
		plan.refresh[p.agent] = append(plan.refresh[p.agent], s)
		taken++
	}
	pruneRows, err := r.kernel.Query("knowledge_prune")
	if err != nil {
		return plan, fmt.Errorf("query knowledge_prune: %w", err)
	}
	for _, row := range pruneRows {
		if len(row.Args) < 2 {
			continue
		}
		key := experts.AgentKey(types.ExtractString(row.Args[0]))
		plan.prune[key] = append(plan.prune[key], types.ExtractString(row.Args[1]))
	}
	return plan, nil
}

// knowledgeRefreshSchedule is when the refresh runs: Delay after boot, then
// every Interval.
type knowledgeRefreshSchedule struct {
	Delay, Interval time.Duration
}

// refreshSchedule reads knowledge_refresh_schedule(DelaySeconds,
// IntervalSeconds) from the policy.
func refreshSchedule(kernel types.Kernel) (knowledgeRefreshSchedule, error) {
	rows, err := kernel.Query("knowledge_refresh_schedule")
	if err != nil {
		return knowledgeRefreshSchedule{}, fmt.Errorf("query knowledge_refresh_schedule: %w", err)
	}
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		delay, interval := factInt64(row.Args[0]), factInt64(row.Args[1])
		if delay < 0 || interval <= 0 {
			return knowledgeRefreshSchedule{}, fmt.Errorf("knowledge_refresh_schedule(%d, %d) is not a schedule", delay, interval)
		}
		return knowledgeRefreshSchedule{Delay: time.Duration(delay) * time.Second, Interval: time.Duration(interval) * time.Second}, nil
	}
	return knowledgeRefreshSchedule{}, fmt.Errorf("the policy declares no knowledge_refresh_schedule")
}

// serve runs the refresh on its schedule until lifetime ends.
func (r *knowledgeRefresher) serve(lifetime context.Context, schedule knowledgeRefreshSchedule) {
	timer := time.NewTimer(schedule.Delay)
	defer timer.Stop()
	for {
		select {
		case <-lifetime.Done():
			return
		case <-timer.C:
		}
		started := time.Now()
		report, err := r.run(lifetime)
		log := logging.Get(logging.CategoryContext)
		if err != nil {
			log.Warn("knowledge refresh: %v", err)
		}
		if len(report.Refreshed) > 0 || report.Pruned > 0 {
			log.Info("knowledge refresh in %s: %d subject(s) acquired again (%s), %d atom(s) added, %d pruned; sources: %s",
				time.Since(started).Round(time.Millisecond), len(report.Refreshed), strings.Join(report.Refreshed, "; "),
				report.Added, report.Pruned, knowledge.SummarizeOutcomes(report.Outcomes))
		}
		timer.Reset(schedule.Interval)
	}
}

func queryInt(kernel types.Kernel, predicate string) (int64, error) {
	rows, err := kernel.Query(predicate)
	if err != nil {
		return 0, fmt.Errorf("query %s: %w", predicate, err)
	}
	for _, row := range rows {
		if len(row.Args) > 0 {
			return factInt64(row.Args[0]), nil
		}
	}
	return 0, fmt.Errorf("the policy declares no %s", predicate)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
