// Package main implements the codeNERD CLI commands.
// This file contains the runInstruction function implementing the OODA loop.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"codenerd/internal/world"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// =============================================================================
// RUN INSTRUCTION - OODA Loop Implementation
// =============================================================================

// runCmd executes a single instruction
var runCmd = &cobra.Command{
	Use:   "run [instruction]",
	Short: "Execute a single instruction through the OODA loop",
	Long: `Processes a natural language instruction through the full Cortex pipeline:
  1. Perception: Transduce input to intent atoms
  2. Orient: Load facts, activate context via spreading activation
  3. Decide: Derive next_action via Mangle policy rules
  4. Act: Execute via VirtualStore, report via Articulation layer`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInstruction,
}

// runInstruction executes a single instruction through the OODA loop
func runInstruction(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received shutdown signal")
		cancel()
	}()

	userInput := joinArgs(args)
	logger.Info("Processing instruction", zap.String("input", userInput))

	// Resolve API key
	key := resolveAPIKey(apiKey, workspace)

	// Boot Cortex (System Stabilization)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Add usage tracker to context if available
	if cortex.UsageTracker != nil {
		ctx = usage.NewContext(ctx, cortex.UsageTracker)
	}

	baseRouting, baseExec := systemResultBaselines(cortex.Kernel)

	emitter := articulation.NewEmitter()

	// 2. Perception Layer: Transduce Input -> Intent
	logger.Debug("Transducing user input to intent atoms")
	intent, err := cortex.Transducer.ParseIntent(ctx, userInput)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}
	logger.Info("Intent parsed",
		zap.String("verb", intent.Verb),
		zap.String("target", intent.Target))

	// /stats is deterministic and should not require running shards or policy.
	if intent.Verb == "/stats" {
		stats, err := computeStats(ctx, cortex.Workspace, intent.Target)
		if err != nil {
			stats = fmt.Sprintf("Stats error: %v", err)
		}
		emitter.Emit(articulation.PiggybackEnvelope{
			Surface: stats,
			Control: articulation.ControlPacket{
				IntentClassification: articulation.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates: []string{fmt.Sprintf("observation(/stats, %q)", stats)},
			},
		})
		return nil
	}

	// 3. World Model: Incremental Scan Workspace (fast)
	logger.Debug("Scanning workspace incrementally", zap.String("path", cortex.Workspace))
	scanRes, err := cortex.Scanner.ScanWorkspaceIncremental(ctx, cortex.Workspace, cortex.LocalDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		return fmt.Errorf("world model error: %w", err)
	}
	if scanRes != nil && !scanRes.Unchanged {
		if err := world.ApplyIncrementalResult(cortex.Kernel, scanRes); err != nil {
			return fmt.Errorf("world model apply error: %w", err)
		}
		logger.Debug("Workspace scan applied", zap.Int("facts", len(scanRes.NewFacts)))
	} else {
		logger.Debug("Workspace unchanged, using cached facts")
	}

	// 4. Load Facts into Hollow Kernel
	if err := cortex.Kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
		return fmt.Errorf("kernel load error: %w", err)
	}

	// Update system facts (Time, etc.)
	if err := cortex.Kernel.UpdateSystemFacts(); err != nil {
		return fmt.Errorf("system facts update error: %w", err)
	}

	// 5. Query Executive Policy (Decide) and actually execute.
	// Previously next_action(/delegate_coder) was printed and the command
	// exited 0 without ever spawning work — classic hollow success.
	logger.Debug("Querying executive policy")
	var output string
	var actionErr error
	actionExecuted := false

	// One-shot CLI is explicit user interaction — lift the boot guard so
	// VirtualStore RouteAction can run for non-delegate next_actions.
	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}

	// Check for explicit delegate_task facts first
	delegateFacts, _ := cortex.Kernel.Query("delegate_task")
	if len(delegateFacts) > 0 {
		fact := delegateFacts[0]
		shardType := types.ExtractString(fact.Args[0])
		task := types.ExtractString(fact.Args[1])
		if strings.TrimSpace(task) == "" {
			task = userInput
		}
		logger.Info("Delegating to shard", zap.String("type", shardType), zap.String("task", task))

		if shardType == "/tool_generator" || shardType == "tool_generator" {
			if cortex.Orchestrator == nil {
				actionErr = fmt.Errorf("tool_generator delegation requires autopoiesis orchestrator")
				output = actionErr.Error()
			} else {
				count, err := cortex.Orchestrator.ProcessKernelDelegations(ctx)
				if err != nil {
					actionErr = err
					output = fmt.Sprintf("Tool generation failed: %v", err)
				} else if count == 0 {
					// Fall back to a direct ouroboros generation from the task text
					need, detErr := cortex.Orchestrator.DetectToolNeed(ctx, task)
					if detErr != nil {
						actionErr = detErr
						output = fmt.Sprintf("Tool generation failed: %v", detErr)
					} else {
						if need == nil {
							need = buildCLIToolNeed(task)
						}
						loopRes := cortex.Orchestrator.ExecuteOuroborosLoop(ctx, need)
						if loopRes == nil || !loopRes.Success {
							errMsg := "unknown"
							if loopRes != nil && loopRes.Error != "" {
								errMsg = loopRes.Error
							}
							actionErr = fmt.Errorf("tool generation failed: %s", errMsg)
							output = actionErr.Error()
						} else {
							actionExecuted = true
							output = fmt.Sprintf("Autopoiesis: Generated tool %s", loopRes.ToolName)
						}
					}
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Autopoiesis: Generated %d tools", count)
				}
			}
		} else {
			result, err := cortex.SpawnTask(ctx, shardType, task)
			if err != nil {
				actionErr = err
				output = fmt.Sprintf("Shard execution failed: %v", err)
				if strings.TrimSpace(result) != "" {
					output += "\n" + result
				}
			} else {
				actionExecuted = true
				output = fmt.Sprintf("Shard Result: %s", result)
			}
		}
	} else {
		// No delegate_task — try next_action and execute handoffs.
		actionFacts, _ := cortex.Kernel.Query("next_action")
		if len(actionFacts) == 0 {
			actionErr = fmt.Errorf("no action derived from policy")
			output = actionErr.Error()
		} else {
			action := types.ExtractString(actionFacts[0].Args[0])
			logger.Info("Derived next_action", zap.String("action", action))

			if shard := nextActionToShardType(action); shard != "" {
				// Execute the handoff that policy derived but did not surface
				// as delegate_task (e.g. /create → next_action(/delegate_coder)).
				logger.Info("Executing next_action handoff", zap.String("shard", shard))
				result, err := cortex.SpawnTask(ctx, shard, userInput)
				if err != nil {
					actionErr = err
					output = fmt.Sprintf("Handoff %s failed: %v", action, err)
					if strings.TrimSpace(result) != "" {
						output += "\n" + result
					}
				} else if strings.TrimSpace(result) == "" {
					// An empty result is not success.
					//
					// This branch set actionExecuted = true on any nil error,
					// so a shard returning "" was reported as executed and the
					// envelope asserted task_status(/manual_instruction,
					// /complete). Observed live on a two-part instruction: the
					// coder shard did the first half, dropped the second, and
					// returned an empty string — output read "Executed
					// /delegate_coder via coder:" with nothing after the colon,
					// and the command exited 0.
					//
					// The hollow-success guard below already exists for exactly
					// this class of failure; it just could not see this case,
					// because actionExecuted was already true by the time it
					// ran. Same family as the BaseShardAgent stub that made
					// `nerd spawn <anything>` succeed.
					actionErr = fmt.Errorf("shard %s returned an empty result for %s: nothing was reported as done, so nothing can be verified", shard, action)
					output = actionErr.Error()
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Executed %s via %s:\n%s", action, shard, result)
				}
			} else if cortex.VirtualStore != nil {
				// Non-delegate actions: route through VirtualStore.
				// routePermittedAction files the constitutional permission
				// request first (constitution.mg default-deny derives
				// permitted/3 only from a matching pending_action/5; the
				// one-shot path has no executive shard to file it), routes,
				// and retracts. The kernel still decides — safe_action and
				// !dangerous_content gates apply unchanged.
				fact := nextActionFact(action, userInput)
				vsResult, vsErr := routePermittedAction(ctx, cortex.VirtualStore, cortex.Kernel, fact)
				if vsErr != nil {
					actionErr = vsErr
					output = fmt.Sprintf("RouteAction(%s) failed: %v", action, vsErr)
				} else {
					actionExecuted = true
					output = fmt.Sprintf("Executed %s: %v", action, vsResult)
				}
			} else {
				actionErr = fmt.Errorf("derived next_action %s but no executor handled it (no delegate handoff or virtual store)", action)
				output = actionErr.Error()
			}
		}
	}

	routingNew, execNew := waitForSystemResults(ctx, cortex.Kernel, baseRouting, baseExec, 3*time.Second)
	if summary := formatSystemResults(routingNew, execNew); summary != "" {
		output = output + "\n\n" + summary
	}

	// Compound instructions must account for every requested subtask before we
	// can claim /complete. The existing hollow-success guard catches empty
	// results, but a shard can return a plausible non-empty summary that
	// describes only the part it did (observed live: TurnStart/TurnEnd done,
	// IntentParsed silently dropped with no mention). Detect the instruction's
	// subtasks and require the result to evidence each one. Do not weaken the
	// hollow-success guard: this is an additional check that can only
	// downgrade /complete to /partial or /failed.
	subtasks := extractRequestedSubtasks(userInput)
	var unaccounted []string
	if len(subtasks) >= 2 && actionExecuted && actionErr == nil {
		unaccounted = findUnaccountedSubtasks(subtasks, output)
		if len(unaccounted) > 0 {
			gapMsg := fmt.Sprintf("PARTIAL: %d of %d requested subtasks not evidenced in result: %s",
				len(unaccounted), len(subtasks), strings.Join(unaccounted, "; "))
			logger.Warn("Compound instruction incomplete",
				zap.Int("total", len(subtasks)),
				zap.Strings("missing", unaccounted),
				zap.Strings("subtasks", subtasks))
			output = output + "\n\n" + gapMsg + "\nMissing subtasks: " + strings.Join(unaccounted, " | ")
			actionErr = fmt.Errorf("compound instruction incomplete: %d of %d subtasks not evidenced — missing: %s",
				len(unaccounted), len(subtasks), strings.Join(unaccounted, ", "))
		}
	}

	// 6. Articulation Layer: Report
	status := "/complete"
	if actionErr != nil || !actionExecuted {
		// A subtask gap after a real action is a partial result, not a total
		// failure — the distinction matters to policy and to the transcript.
		if len(unaccounted) > 0 && actionExecuted {
			status = "/partial"
		} else {
			status = "/failed"
		}
	}
	mangleUpdates := []string{
		fmt.Sprintf("task_status(/manual_instruction, %s)", status),
		fmt.Sprintf("observation(/result, %q)", output),
	}
	if len(unaccounted) > 0 {
		// Name the gap explicitly so policy and transcripts can see it even
		// when the surface is truncated.
		mangleUpdates = append(mangleUpdates,
			fmt.Sprintf("observation(/subtask_gap, %q)", strings.Join(unaccounted, " | ")))
	}
	if len(subtasks) >= 2 && status == "/complete" {
		mangleUpdates = append(mangleUpdates,
			fmt.Sprintf("observation(/subtask_accounting, %q)", fmt.Sprintf("all %d subtasks evidenced", len(subtasks))))
	}
	payload := articulation.PiggybackEnvelope{
		Surface: fmt.Sprintf("Processed: %s\nResult: %s", userInput, output),
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   intent.Category,
				Verb:       intent.Verb,
				Target:     intent.Target,
				Constraint: intent.Constraint,
				Confidence: intent.Confidence,
			},
			MangleUpdates: mangleUpdates,
		},
	}
	emitter.Emit(payload)

	if actionErr != nil {
		return actionErr
	}
	if !actionExecuted {
		return fmt.Errorf("hollow success blocked: no side-effecting action executed for %q", userInput)
	}
	return nil
}

// nextActionFact builds the VirtualStore-routable fact for a policy-derived
// non-delegate next_action. VirtualStore.parseActionFact requires exactly
// (ActionID, Type, Target); the previous 2-arg {action, input} shape failed
// parsing for EVERY non-delegate verb (F-ROUTE-1: "invalid action fact:
// requires at least 3 arguments"), so /explain → /analyze_code never executed.
// Mirrors the tdd_loop.go action-fact pattern.
func nextActionFact(action, target string) core.Fact {
	return core.Fact{
		Predicate: "next_action",
		Args: []any{
			fmt.Sprintf("cli-%d", time.Now().UnixNano()),
			types.MangleAtom(action),
			target,
		},
	}
}

// nextActionToShardType maps policy next_action atoms onto domain shard types
// that SpawnTask can run. Empty means "not a shard handoff".
// nextActionToShardType maps policy next_action atoms onto domain shard types
// that SpawnTask can run. Empty means "not a shard handoff".
func nextActionToShardType(action string) string {
	action = strings.TrimSpace(strings.TrimPrefix(action, "/"))
	switch strings.ToLower(action) {
	case "delegate_coder", "delegate_coder_shard":
		return "coder"
	case "delegate_tester":
		return "tester"
	case "delegate_reviewer":
		return "reviewer"
	case "delegate_researcher":
		return "researcher"
	case "delegate_tool_generator":
		return "tool_generator"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// F-RUN-1: per-subtask accounting for compound instructions
// ---------------------------------------------------------------------------

// compoundSplitRe splits a compound instruction into candidate subtask
// fragments. It looks for strong separators: semicolons, newlines, " and ",
// " then ", " also ", ", and ", " plus ", numbered/bulleted lists. A plain
// comma alone is NOT a splitter — it would over-split single tasks.
var compoundSplitRe = regexp.MustCompile(`(?i)\s*(?:;|\n)+\s*|\s+and\s+also\s+|\s+then\s+|\s+also\s+|\s*,\s*and\s+|\s+and\s+|\s+plus\s+`)

var numberedListRe = regexp.MustCompile(`(?m)^\s*(?:\d+[.):]|[-*])\s+`)

// extractRequestedSubtasks decomposes a user instruction into discrete
// subtasks. It returns nil/empty for single-task instructions. A compound
// instruction is defined as yielding >=2 non-trivial fragments after
// splitting. The original fragment text is preserved so downstream
// accounting can report which part was not evidenced.
func extractRequestedSubtasks(input string) []string {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil
	}
	// Normalise whitespace for detection but keep original punctuation.
	// First handle numbered/bulleted lists explicitly: they are unambiguous
	// subtask separators.
	if numberedListRe.MatchString(s) {
		// Split on bullet/number markers. Keep the content after each marker.
		parts := numberedListRe.Split(s, -1)
		var out []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// Further split each bullet on compound separators in case
			// "1. Do X and also do Y" appears.
			if p == "" {
				continue
			}
			sub := compoundSplitRe.Split(p, -1)
			for _, q := range sub {
				q = strings.TrimSpace(strings.Trim(q, ".,; "))
				if len(q) >= 3 {
					out = append(out, q)
				}
			}
		}
		// A prohibition is satisfied by the ABSENCE of an action, so
		// requiring lexical evidence of it in the output can never succeed
		// and would fail the runs that complied best. Filter constraints
		// before they reach the evidence check.
		out = filterProhibitionClauses(out)
		// A deliverable is imperative; a clause that describes the world is context, and context cannot be evidenced.
		out = filterDeclarativeContextClauses(out)
		if len(out) >= 2 {
			return dedupSubtasks(out)
		}
		// Fall through — not actually a multi-bullet instruction.
	}

	parts := compoundSplitRe.Split(s, -1)
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.Trim(p, ".,; "))
		// Strip leading discourse markers that would otherwise pollute
		// keyword matching ("please ", "also ", "then ").
		lower := strings.ToLower(p)
		for _, prefix := range []string{"please ", "also ", "then ", "and "} {
			if strings.HasPrefix(lower, prefix) {
				p = strings.TrimSpace(p[len(prefix):])
				lower = strings.ToLower(p)
			}
		}
		if len(p) >= 3 {
			cleaned = append(cleaned, p)
		}
	}
	cleaned = mergeModifierFragments(cleaned)
	// A prohibition is satisfied by the ABSENCE of an action, so requiring
	// lexical evidence of it in the output can never succeed and would fail
	// the runs that complied best. Filter constraints before they reach the
	// evidence check.
	cleaned = filterProhibitionClauses(cleaned)
	// A deliverable is imperative; a clause that describes the world is context, and context cannot be evidenced.
	cleaned = filterDeclarativeContextClauses(cleaned)
	if len(cleaned) < 2 {
		return nil
	}
	return dedupSubtasks(cleaned)
}

// filterProhibitionClauses removes clauses that are constraints rather than
// deliverables. A prohibition is satisfied by the ABSENCE of an action, so
// requiring lexical evidence of it in the output can never succeed and would
// fail the runs that complied best. The guard must only check positive
// deliverables.
func filterProhibitionClauses(in []string) []string {
	var out []string
	for _, s := range in {
		// Handle embedded prohibition sentences appended with a period
		// ("do B. Do not change any files") — strip the trailing
		// constraint so the deliverable can be evidenced without mentioning
		// the prohibition.
		s = stripTrailingProhibition(s)
		if s == "" {
			continue
		}
		if isProhibitionClause(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isProhibitionClause(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	lower := strings.ToLower(t)
	// Small, readable, explicitly listed set — not a clever regex.
	prohibitionPrefixes := []string{
		"do not",
		"don't",
		"don’t",
		"dont",
		"never",
		"avoid",
		"without",
		"no ",
	}
	for _, p := range prohibitionPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

func stripTrailingProhibition(s string) string {
	lower := strings.ToLower(s)
	earliest := -1
	// Look for a sentence boundary followed by a prohibition prefix.
	for _, prefix := range []string{"do not", "don't", "don’t", "dont", "never", "avoid", "without", "no "} {
		for _, sep := range []string{". ", "! ", "? "} {
			needle := sep + prefix
			if idx := strings.Index(lower, needle); idx != -1 {
				if earliest == -1 || idx < earliest {
					earliest = idx
				}
			}
		}
	}
	if earliest != -1 {
		return strings.TrimSpace(strings.Trim(s[:earliest], ".,; "))
	}
	return s
}

// declarativeContextStarters marks the first meaningful token of a clause that
// describes the world rather than requesting work. A deliverable is imperative;
// a clause that describes the world is context, and context cannot be evidenced.
var declarativeContextStarters = map[string]bool{
	// articles and determiners
	"the": true, "a": true, "an": true, "this": true, "that": true, "these": true, "those": true, "its": true, "their": true, "our": true, "your": true, "my": true, "his": true, "her": true, "each": true, "every": true, "some": true, "any": true, "all": true, "both": true,
	// pronouns
	"it": true, "they": true, "he": true, "she": true, "we": true, "you": true, "i": true, "there": true, "here": true,
	// prepositions and subordinators
	"in": true, "on": true, "at": true, "for": true, "from": true, "with": true, "by": true, "of": true, "as": true, "because": true, "since": true, "when": true, "while": true, "where": true, "if": true, "although": true, "though": true,
	// forms of to be and bare auxiliaries
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true, "being": true, "has": true, "have": true, "had": true, "will": true, "would": true, "can": true, "could": true, "may": true, "might": true, "should": true,
}

var leadingAdverbs = map[string]bool{
	"then": true, "also": true, "next": true, "finally": true, "first": true, "second": true,
}

func firstMeaningfulToken(s string) string {
	toks := tokenRe.FindAllString(s, -1)
	idx := 0
	for idx < len(toks) {
		low := strings.ToLower(toks[idx])
		if leadingAdverbs[low] {
			idx++
			continue
		}
		break
	}
	if idx < len(toks) {
		return strings.ToLower(toks[idx])
	}
	return ""
}

func isDeclarativeContextClause(s string) bool {
	tok := firstMeaningfulToken(s)
	if tok == "" {
		return false
	}
	return declarativeContextStarters[tok]
}

func filterDeclarativeContextClauses(in []string) []string {
	var out []string
	for _, s := range in {
		sentences := splitSentences(s)
		for _, sent := range sentences {
			sent = strings.TrimSpace(sent)
			if sent == "" {
				continue
			}
			if isDeclarativeContextClause(sent) {
				continue
			}
			out = append(out, sent)
		}
	}
	return out
}

func splitSentences(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '!' || s[i] == '?' {
			// Sentence boundary is punctuation followed by whitespace or end.
			if i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\n' || s[i+1] == '\t' || s[i+1] == '\r') {
				seg := strings.TrimSpace(s[start : i+1])
				if seg != "" {
					trimmed := strings.Trim(seg, ".,; ")
					if trimmed != "" {
						out = append(out, trimmed)
					}
				}
				j := i + 1
				for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\t' || s[j] == '\r') {
					j++
				}
				start = j
				i = j - 1
			} else if i+1 == len(s) {
				seg := strings.TrimSpace(s[start : i+1])
				if seg != "" {
					trimmed := strings.Trim(seg, ".,; ")
					if trimmed != "" {
						out = append(out, trimmed)
					}
				}
				start = len(s)
				break
			}
		}
	}
	if start < len(s) {
		seg := strings.TrimSpace(s[start:])
		if seg != "" {
			trimmed := strings.Trim(seg, ".,; ")
			if trimmed != "" {
				out = append(out, trimmed)
			}
		}
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

// mergeModifierFragments folds single-token fragments back into the fragment
// they were split from.
//
// " and " is a weak separator: it joins subtasks ("wire X and wire Y") but it
// also joins coordinated modifiers inside ONE task ("reads cleanly and
// consistently"). Splitting the latter invents a subtask named "consistently"
// that no result will ever evidence, which would permanently downgrade an
// ordinary single-task instruction to /partial. A false gap report is worse
// than the miss this whole mechanism exists to catch, because it is the kind
// of noise that gets a check switched off.
//
// The discriminator is grammatical, not statistical: a subtask is a clause and
// has at least a verb and an object. A lone word is a modifier. Fragments are
// merged rather than dropped so the reported subtask text still matches what
// the user wrote.
func mergeModifierFragments(parts []string) []string {
	var out []string
	for _, p := range parts {
		if len(tokenRe.FindAllString(p, -1)) < 2 && len(out) > 0 {
			out[len(out)-1] = out[len(out)-1] + " and " + p
			continue
		}
		out = append(out, p)
	}
	return out
}

func dedupSubtasks(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		key := strings.ToLower(strings.TrimSpace(s))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// subtaskGenericStopWords are filtered when building the distinctive token
// set so "add IntentParsed support" collapses to ["intentparsed"] rather
// than matching on the generic "support".
var subtaskGenericStopWords = map[string]bool{
	"and": true, "or": true, "the": true, "a": true, "an": true,
	"to": true, "for": true, "in": true, "of": true, "on": true,
	"with": true, "please": true, "then": true, "also": true,
	"that": true, "this": true, "these": true, "those": true,
	"is": true, "are": true, "be": true, "should": true, "will": true,
	"would": true, "could": true, "do": true, "does": true, "it": true,
	"its": true, "as": true, "by": true, "from": true, "at": true,
	"if": true, "when": true, "while": true, "into": true, "out": true,
	"over": true, "under": true, "up": true, "down": true,
	"implement": true, "add": true, "create": true, "fix": true, "make": true,
	"update": true, "ensure": true, "handle": true, "handling": true,
	"support": true, "include": true, "including": true, "remove": true,
	"delete": true, "feature": true, "code": true, "file": true,
	// interrogatives and pronouns — question shape, not subject; no evidence of work done
	"which": true, "what": true, "why": true, "how": true, "where": true,
	"who": true, "whom": true, "whose": true, "them": true, "they": true,
	"their": true, "there": true, "here": true, "each": true, "any": true,
	"all": true, "some": true, "both": true, "other": true, "others": true,
	// reporting and presentation verbs — delivery, not content; answer shows rather than announces
	"state": true, "list": true, "name": true, "explain": true, "describe": true,
	"show": true, "tell": true, "identify": true, "report": true, "summarize": true,
	"summarise": true, "provide": true, "give": true, "return": true, "output": true,
	"print": true, "display": true, "mention": true, "note": true, "detail": true,
	"specify": true,
}

// tokenRe extracts alphanumeric tokens (including underscores).
var tokenRe = regexp.MustCompile(`[A-Za-z0-9_]+`)

// findUnaccountedSubtasks returns the subset of subtasks whose distinctive
// keywords are not evidenced in output. Matching is case-insensitive and
// substring-based on the lowercased output. A subtask is considered
// accounted for if every distinctive token appears in output, or — for
// longer keyword sets — at least half of them appear. Generic stop words
// are excluded so the check is driven by identifiers like TurnStart or
// IntentParsed, not by filler.
func findUnaccountedSubtasks(subtasks []string, output string) []string {
	if len(subtasks) == 0 {
		return nil
	}
	lowerOut := strings.ToLower(output)

	// Tokens shared by several subtasks carry no discriminating information.
	// "wire the TurnStart audit event and wire the TurnEnd audit event and wire
	// the IntentParsed audit event" gives every subtask the tokens wire/audit/
	// event; a result that mentions two of the three satisfies all of them.
	// That is precisely how the live miss survived: for the dropped subtask,
	// three shared tokens matched and only `intentparsed` did not, so the
	// proportional rule scored it 3-of-4 and called it done.
	//
	// The tokens that actually identify a subtask are the ones its siblings do
	// not have. Where those exist they are required in full — a result that
	// never says "IntentParsed" is not evidence that IntentParsed was wired,
	// no matter how much shared vocabulary it shares with its neighbours.
	tokenOwners := make(map[string]int)
	for _, st := range subtasks {
		for _, kw := range distinctiveTokens(st) {
			tokenOwners[kw]++
		}
	}

	var missing []string
	for _, st := range subtasks {
		keywords := distinctiveTokens(st)

		var unique []string
		for _, kw := range keywords {
			if tokenOwners[kw] == 1 {
				unique = append(unique, kw)
			}
		}
		if len(unique) > 0 {
			for _, kw := range unique {
				if !strings.Contains(lowerOut, kw) {
					missing = append(missing, st)
					break
				}
			}
			continue
		}

		// No token separates this subtask from its siblings — fall back to the
		// proportional rule, which is the best available signal when the
		// instruction genuinely repeats itself.
		if len(keywords) == 0 {
			// No distinctive keywords — fall back to raw tokens of length >=3.
			raw := tokenRe.FindAllString(st, -1)
			for _, t := range raw {
				if len(t) >= 3 {
					keywords = append(keywords, strings.ToLower(t))
				}
			}
			if len(keywords) == 0 {
				continue
			}
		}
		matched := 0
		for _, kw := range keywords {
			if strings.Contains(lowerOut, kw) {
				matched++
			}
		}
		// Require all keywords for 1-2 keyword subtasks; otherwise require
		// at least half (ceil) to allow minor paraphrasing without letting a
		// wholly-dropped subtask slip through.
		required := len(keywords)
		if len(keywords) > 2 {
			required = (len(keywords) + 1) / 2
		}
		if matched < required {
			missing = append(missing, st)
		}
	}
	return missing
}

func distinctiveTokens(s string) []string {
	toks := tokenRe.FindAllString(s, -1)
	var out []string
	seen := make(map[string]bool)
	for _, t := range toks {
		low := strings.ToLower(t)
		if len(low) < 3 {
			continue
		}
		if subtaskGenericStopWords[low] {
			continue
		}
		if seen[low] {
			continue
		}
		seen[low] = true
		out = append(out, low)
	}
	// If filtering left us empty but there were tokens, keep the longest
	// raw token so single-generic subtasks like "fix it" don't vanish.
	if len(out) == 0 && len(toks) > 0 {
		longest := ""
		for _, t := range toks {
			if len(t) > len(longest) {
				longest = t
			}
		}
		if len(longest) >= 2 {
			return []string{strings.ToLower(longest)}
		}
	}
	return out
}
