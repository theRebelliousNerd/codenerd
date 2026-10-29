// Package experts asks the kernel which shard agents a task needs.
//
// A shard agent is a user-defined specialist (.nerd/agents/<name>/prompts.yaml,
// .nerd/shards/<name>_knowledge.db, listed in .nerd/agents.json). Which one a
// task needs is not this package's decision, nor the model's: Select measures
// the task (its verb and the workspace paths it targets), asserts it, and reads
// back what policy/shard_experts.mg derived -- expert_needed, expert_file,
// expert_confidence, and the routing judgments policy/shards.mg derives from
// them (specialist_should_execute, specialist_should_advise,
// strategic_advisor_required). The task's facts are retracted before Select
// returns, so a selection leaves nothing behind in the shared kernel.
//
// The roster side (shard_agent, shard_agent_domain, technology_import_prefix)
// is asserted once at boot by internal/system.
package experts

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"codenerd/internal/types"
)

// Task is one unit of work to select shard agents for.
type Task struct {
	// Verb is the intent verb the task runs as ("/fix", "/explain").
	Verb string
	// Targets are the workspace paths the task names: files or package
	// directories, absolute or workspace-relative.
	Targets []string
	// Workspace is the root that canonical paths are relative to.
	Workspace string
	// Speaker, when set, is the shard agent the task already runs as (its
	// key, "goexpert"): asserted as expert_task_speaker, so a consultation
	// never spawns another (policy/shard_experts.mg expert_spawn).
	Speaker string
}

// Spawn modes, the value set of expert_spawn/3's Mode.
const (
	SpawnLead   = "lead"   // an executor expert lays out the work
	SpawnAdvise = "advise" // an advisor says what must hold
)

// Need is one shard agent the kernel derived the task needs.
type Need struct {
	// Agent is the agent's identity: its lower-cased registry name, without
	// the leading slash ("goexpert"). It is the key its knowledge DB is
	// registered with the JIT compiler under.
	Agent string
	// Technologies are the reasons: each technology joining the task's files
	// to the agent's domain ("mangle", "bubbletea"), sorted.
	Technologies []string
	// Files are the canonical task files the agent's domain covers, sorted.
	Files []string
	// Confidence is the strongest evidence for the agent, 0-100.
	Confidence int64
	// Execute and Advise are the routing the kernel derived for the agent on
	// this task (specialist_should_execute / specialist_should_advise).
	Execute bool
	Advise  bool
	// Spawn is the consultation the kernel derived for the agent before the
	// task's turn (expert_spawn): SpawnLead, SpawnAdvise, or "" for none.
	Spawn string
	// Critique reports expert_critique: the agent also reviews the turn's
	// change before its verdict (the verb's mode is /advisory_with_critique).
	Critique bool
}

// Selection is the kernel's answer for one task.
type Selection struct {
	// Task is the id the task was asserted under.
	Task string
	// Needs are sorted by confidence, highest first, then by agent name.
	Needs []Need
	// StrategicAdvisorRequired reports strategic_advisor_required(Task).
	StrategicAdvisorRequired bool
	// Complexity is task_complexity(Task, Level) as the kernel derived it
	// from the task's verb, file count and file sizes ("high", "normal"), or
	// "" when the task asked nothing.
	Complexity string
}

// Agents returns the needed agents' identities, in Needs order.
func (s Selection) Agents() []string {
	out := make([]string, 0, len(s.Needs))
	for _, n := range s.Needs {
		out = append(out, n.Agent)
	}
	return out
}

var taskSeq atomic.Uint64

// Select asserts the task, reads the kernel's judgments for it, and retracts
// the task's facts. A task with no verb or no target needs no expert and asks
// nothing. An error means the kernel could not be asked; the selection is
// then empty, never guessed.
func Select(kernel types.Kernel, task Task) (Selection, error) {
	verb := strings.TrimSpace(task.Verb)
	if kernel == nil || !validName(verb) {
		return Selection{}, nil
	}
	id := fmt.Sprintf("expert_task_%d", taskSeq.Add(1))
	facts := taskFacts(id, verb, task)
	if len(facts) <= 1 {
		return Selection{Task: id}, nil
	}
	if err := kernel.AssertBatch(facts); err != nil {
		return Selection{Task: id}, fmt.Errorf("assert expert task: %w", err)
	}
	sel, readErr := read(kernel, id)
	if err := kernel.RetractExactFactsBatch(facts); err != nil && readErr == nil {
		readErr = fmt.Errorf("retract expert task: %w", err)
	}
	if readErr != nil {
		return Selection{Task: id}, readErr
	}
	return sel, nil
}

// taskFacts is the measured task: its verb, each canonical target, and the
// extension of each target that has one (detected_language reads it).
func taskFacts(id, verb string, task Task) []types.Fact {
	facts := []types.Fact{{Predicate: "expert_task", Args: []any{id, types.MangleAtom(verb)}}}
	seen := map[string]bool{}
	for _, target := range task.Targets {
		canonical := types.CanonicalPath(task.Workspace, strings.TrimSpace(target))
		if canonical == "" || canonical == "." || seen[canonical] {
			continue
		}
		seen[canonical] = true
		facts = append(facts, types.Fact{Predicate: "expert_task_target", Args: []any{id, canonical}})
		if ext := strings.ToLower(filepath.Ext(canonical)); ext != "" {
			facts = append(facts, types.Fact{Predicate: "file_extension", Args: []any{canonical, ext}})
		}
	}
	if speaker := AgentKey(task.Speaker); speaker != "" && validName("/"+speaker) {
		facts = append(facts, types.Fact{Predicate: "expert_task_speaker", Args: []any{id, AgentAtom(speaker)}})
	}
	return facts
}

// read collects every judgment the kernel derived for task id.
func read(kernel types.Kernel, id string) (Selection, error) {
	sel := Selection{Task: id}
	needs := map[string]*Need{}
	need := func(agent string) *Need {
		n := needs[agent]
		if n == nil {
			n = &Need{Agent: agent}
			needs[agent] = n
		}
		return n
	}

	rows, err := kernel.Query("expert_needed")
	if err != nil {
		return sel, fmt.Errorf("query expert_needed: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != id {
			continue
		}
		n := need(agentKey(f.Args[1]))
		n.Technologies = appendUnique(n.Technologies, strings.TrimPrefix(types.ExtractString(f.Args[2]), "/"))
	}
	// The task's complexity and the advice it calls for hold whether or not
	// any shard agent is needed.
	if rows, err = kernel.Query("task_complexity"); err != nil {
		return sel, fmt.Errorf("query task_complexity: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == id {
			sel.Complexity = strings.TrimPrefix(types.ExtractString(f.Args[1]), "/")
		}
	}

	if rows, err = kernel.Query("strategic_advisor_required"); err != nil {
		return sel, fmt.Errorf("query strategic_advisor_required: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) >= 1 && types.ExtractString(f.Args[0]) == id {
			sel.StrategicAdvisorRequired = true
		}
	}

	if len(needs) == 0 {
		return sel, nil
	}

	if rows, err = kernel.Query("expert_file"); err != nil {
		return sel, fmt.Errorf("query expert_file: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != id {
			continue
		}
		if n, ok := needs[agentKey(f.Args[1])]; ok {
			n.Files = appendUnique(n.Files, types.ExtractString(f.Args[2]))
		}
	}

	if rows, err = kernel.Query("expert_confidence"); err != nil {
		return sel, fmt.Errorf("query expert_confidence: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != id {
			continue
		}
		if n, ok := needs[agentKey(f.Args[1])]; ok {
			n.Confidence = number(f.Args[2])
		}
	}

	if rows, err = kernel.Query("specialist_should_execute"); err != nil {
		return sel, fmt.Errorf("query specialist_should_execute: %w", err)
	}
	markRouting(rows, id, needs, func(n *Need) { n.Execute = true })
	if rows, err = kernel.Query("specialist_should_advise"); err != nil {
		return sel, fmt.Errorf("query specialist_should_advise: %w", err)
	}
	markRouting(rows, id, needs, func(n *Need) { n.Advise = true })

	if rows, err = kernel.Query("expert_critique"); err != nil {
		return sel, fmt.Errorf("query expert_critique: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) < 2 || types.ExtractString(f.Args[0]) != id {
			continue
		}
		if n, ok := needs[agentKey(f.Args[1])]; ok {
			n.Critique = true
		}
	}

	if rows, err = kernel.Query("expert_spawn"); err != nil {
		return sel, fmt.Errorf("query expert_spawn: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != id {
			continue
		}
		if n, ok := needs[agentKey(f.Args[1])]; ok {
			n.Spawn = strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")
		}
	}

	for _, n := range needs {
		sort.Strings(n.Technologies)
		sort.Strings(n.Files)
		sel.Needs = append(sel.Needs, *n)
	}
	sort.Slice(sel.Needs, func(i, j int) bool {
		if sel.Needs[i].Confidence != sel.Needs[j].Confidence {
			return sel.Needs[i].Confidence > sel.Needs[j].Confidence
		}
		return sel.Needs[i].Agent < sel.Needs[j].Agent
	})
	return sel, nil
}

// markRouting applies set to each need a specialist_should_* row names for
// task id (Specialist, Task).
func markRouting(rows []types.Fact, id string, needs map[string]*Need, set func(*Need)) {
	for _, f := range rows {
		if len(f.Args) < 2 || types.ExtractString(f.Args[1]) != id {
			continue
		}
		if n, ok := needs[agentKey(f.Args[0])]; ok {
			set(n)
		}
	}
}

// AgentAtom is the name constant an agent carries in the kernel: its
// lower-cased registry name. shard_agent, shard_agent_domain and every
// specialist_* predicate key on it.
func AgentAtom(name string) types.MangleAtom {
	return types.MangleAtom("/" + AgentKey(name))
}

// AgentKey is an agent's identity without the slash: the lower-cased
// registry name, the key its knowledge DB is registered under.
func AgentKey(name string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "/"))
}

func agentKey(arg any) string { return AgentKey(types.ExtractString(arg)) }

func number(arg any) int64 {
	switch v := arg.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	}
	return 0
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, v := range list {
		if v == value {
			return list
		}
	}
	return append(list, value)
}

// validName reports whether s can be a name constant: a slash and at least
// one non-empty segment, no whitespace or quote.
func validName(s string) bool {
	if len(s) < 2 || s[0] != '/' || strings.ContainsAny(s, " \t\n\"") {
		return false
	}
	for _, part := range strings.Split(s[1:], "/") {
		if part == "" {
			return false
		}
	}
	return true
}
