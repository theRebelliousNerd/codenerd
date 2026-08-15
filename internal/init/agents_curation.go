// Package init implements the "nerd init" cold-start initialization system.
// This file connects the agent curation surfaces — Type U definitions and
// interactive selection — to the phase that actually builds knowledge bases.
package init

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// mergeTypeUAgents folds `--define-agent` definitions into the auto-detected
// recommendations.
//
// ParseTypeUAgentFlag, ValidateTypeUAgentDefinition, ToRecommendedAgent and
// TypeUAgentsToRecommended were fully implemented and fully tested, but nothing
// outside the test binary ever called them: a user could pass --define-agent,
// see it validate, and get no agent. The merge happens before knowledge-base
// creation so a Type U agent gets the same KB, prompts.yaml, registry entry and
// shard registration as a detected one.
//
// A name that collides with an auto-detected agent replaces it. The user asked
// for that name explicitly; silently keeping the built-in and dropping theirs
// would be the surprising outcome, and creating both would fight over
// .nerd/shards/{name}_knowledge.db, which is keyed on the lowercased name.
func (i *Initializer) mergeTypeUAgents(recommended []RecommendedAgent) []RecommendedAgent {
	if len(i.config.TypeUAgents) == 0 {
		return recommended
	}

	userAgents := TypeUAgentsToRecommended(i.config.TypeUAgents)
	byName := make(map[string]int, len(recommended))
	for idx, agent := range recommended {
		byName[strings.ToLower(agent.Name)] = idx
	}

	merged := recommended
	for _, agent := range userAgents {
		if idx, clash := byName[strings.ToLower(agent.Name)]; clash {
			logging.Boot("Type U agent %s overrides the auto-detected agent of the same name", agent.Name)
			fmt.Printf("   • %s: user definition overrides the auto-detected agent\n", agent.Name)
			merged[idx] = agent
			continue
		}
		byName[strings.ToLower(agent.Name)] = len(merged)
		merged = append(merged, agent)
		fmt.Printf("   • %s: %s\n", agent.Name, agent.Reason)
	}
	return merged
}

// curateAgents runs interactive agent selection when the run is interactive and
// there is a real terminal to prompt on, and records the outcome in
// .nerd/preferences.json.
//
// InteractiveAgentSelection existed with a full customize loop and
// DefaultInitConfig has always set Interactive: true, but no caller ever
// reached it, so the flag was decorative and OPEN-QUESTIONS #1 ("default or
// opt-in?") had no answer in code. The answer implemented here is: on by
// default, gated on an actual terminal, with `--no-interactive` as the escape
// hatch. A cold start that silently installs a dozen specialists the user never
// saw is the worse default, and the TTY gate means CI, `nerd init < /dev/null`
// and the chat `/init` path (which sets Interactive: false) are unaffected.
//
// Any failure to read the user's answer degrades to the recommended set rather
// than failing init — a broken stdin must not cost the user their workspace.
func (i *Initializer) curateAgents(ctx context.Context, recommended []RecommendedAgent, profile ProjectProfile, result *InitResult) []RecommendedAgent {
	if !i.config.Interactive || len(recommended) == 0 {
		return recommended
	}

	interactiveCfg, ok := i.resolveInteractiveConfig()
	if !ok {
		logging.Boot("Interactive agent selection skipped: stdin/stdout is not a terminal")
		return recommended
	}

	previous, err := LoadAgentPreferences(i.config.Workspace)
	if err != nil {
		// A malformed preferences file must not block curation; we just lose
		// the "auto accept" shortcut for this run.
		logging.Boot("Could not load previous agent preferences: %v", err)
	}
	interactiveCfg.PreviousPrefs = previous

	if previous != nil && previous.AutoAcceptRecommended {
		logging.Boot("Agent selection auto-accepted from saved preferences")
		return recommended
	}

	detected := ConvertToDetectedAgents(recommended, profile)
	selected, err := InteractiveAgentSelection(ctx, detected, interactiveCfg)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Interactive agent selection failed, keeping recommended agents: %v", err))
		return recommended
	}
	if len(selected) == 0 {
		result.Warnings = append(result.Warnings, "Interactive agent selection produced no agents; keeping recommended agents")
		return recommended
	}

	curated := ConvertToRecommendedAgents(selected)
	i.persistAgentSelection(recommended, curated, result)
	return curated
}

// persistAgentSelection records which agents the user kept and which they
// dropped so a later `nerd init --force` can honor the same choices. Phase 8
// merges into this same file rather than truncating it, so the record survives
// the rest of the run.
func (i *Initializer) persistAgentSelection(offered, kept []RecommendedAgent, result *InitResult) {
	keptNames := make(map[string]bool, len(kept))
	accepted := make([]string, 0, len(kept))
	for _, agent := range kept {
		keptNames[agent.Name] = true
		accepted = append(accepted, agent.Name)
	}
	rejected := make([]string, 0, len(offered))
	for _, agent := range offered {
		if !keptNames[agent.Name] {
			rejected = append(rejected, agent.Name)
		}
	}

	prefs := &AgentSelectionPreferences{
		AcceptedAgents:  accepted,
		RejectedAgents:  rejected,
		LastInteractive: time.Now(),
	}
	if err := SaveAgentPreferences(i.config.Workspace, prefs); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save agent selection preferences: %v", err))
	}
}

// resolveInteractiveConfig returns the reader/writer to prompt on, or false
// when this run must not prompt. An explicitly injected InteractiveIO always
// wins so tests (and any future non-stdin front end) can drive the selection
// without owning the process's terminal.
func (i *Initializer) resolveInteractiveConfig() (InteractiveConfig, bool) {
	if i.config.InteractiveIO != nil {
		return *i.config.InteractiveIO, true
	}
	if !stdioIsTerminal() {
		return InteractiveConfig{}, false
	}
	return DefaultInteractiveConfig(), true
}

// stdioIsTerminal reports whether both stdin and stdout are character devices.
//
// Requiring both narrows it usefully — `go test`, cron and most CI runners
// redirect stdout to a pipe or file, so the conjunction is false for them and
// true for a human at a terminal. Pure stdlib on purpose: the alternatives are
// a new module dependency or per-OS ioctl build tags, and `nerd` ships on
// Windows too.
//
// What this does NOT do, despite reading like it might: distinguish a terminal
// from /dev/null. /dev/null is a character device on both ends, so
// `nerd init < /dev/null > /dev/null` passes this gate. That case is harmless
// (the read gets an immediate EOF and curation degrades to the recommended
// set), but the gap is real and it is wider than /dev/null: `docker run -t`,
// `docker compose run`, `script -c` and CI wrappers allocate a genuine pty and
// then send no input, which passes the gate AND never returns EOF.
//
// That combination used to hang `nerd init` forever. The bound now lives where
// it belongs — on the read itself (see readInput) — rather than in a gate that
// cannot see the difference. This function is a heuristic for "probably worth
// asking"; it is not a safety property, and nothing downstream may treat it as
// one.
func stdioIsTerminal() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout)
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
