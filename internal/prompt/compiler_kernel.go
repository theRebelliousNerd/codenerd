package prompt

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	userconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

func acquireCompilationKernel(base KernelQuerier) (KernelQuerier, func(), error) {
	if base == nil {
		return nil, func() {}, nil
	}

	if provider, ok := base.(KernelScopeProvider); ok {
		scope, err := provider.NewCompilationScope()
		if err != nil {
			return nil, nil, err
		}
		if scope == nil {
			return nil, nil, fmt.Errorf("kernel scope provider returned nil scope")
		}
		return scope, func() {
			if err := scope.Close(); err != nil {
				logging.Get(logging.CategoryJIT).Warn("Failed to close prompt compilation scope: %v", err)
			}
		}, nil
	}

	// Compatibility path for lightweight adapters. Production uses an isolated
	// scope above; legacy kernels still get complete best-effort cleanup on
	// success, error, cancellation, and panic via the caller's defer.
	retracter, ok := base.(KernelRetracter)
	if !ok {
		return base, func() {}, nil
	}
	return base, func() {
		for _, predicate := range promptEphemeralPredicates {
			if err := retracter.Retract(predicate); err != nil {
				logging.Get(logging.CategoryJIT).Warn(
					"Failed to retract ephemeral prompt predicate %s: %v", predicate, err,
				)
			}
		}
	}, nil
}

// collectKernelInjectedAtoms queries runtime context predicates and turns them into ephemeral PromptAtoms.
// This lets JIT prompts include spreading-activation context and specialist knowledge without legacy injection.

// extractStringArgFast uses type switches for common primitives to avoid reflection
// and allocation overhead in hot loops, falling back to extractStringArg for complex types.
func extractStringArgFast(arg any) (string, error) {
	switch v := arg.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return extractStringArg(arg)
	}
}

// Kernel-injected atoms are synthesized from live fact rows, not from the
// reviewed corpus, so nothing upstream has ever looked at their size. They are
// also emitted with IsMandatory=true, which means budget.Fit skips category
// allocation for them entirely and only rejects the merged atom when it exceeds
// the ENTIRE budget (see budget.go, "Mandatory atom %s rejected"). Two failure
// modes follow from that, and both were live:
//
//  1. Growth. prompt_northstar.mg emits one injectable_context row per mission
//     constraint and per risk with no limit, so a project that keeps declaring
//     risks keeps growing a mandatory prompt section that outranks every
//     optional atom in the corpus.
//  2. Cliff. Because it is one merged atom, crossing the budget does not shed
//     the least important row — it drops the whole block, silently, and the
//     model loses the northstar entirely with no marker saying so.
//
// Capping at construction converts both into a visible, graduated loss. The
// caps are tunables owned by jit.* in .nerd/config.json (internal/config),
// resolved per compile by kernelInjectionLimits -- not literals.
// kernelInjectionLimits are the resolved bounds for the two kernel-injected
// atoms. Values originate in .nerd/config.json under jit.* (see
// DefaultJITConfig); CompilerConfig carries boot's copy and this struct is
// the per-compile resolution with unset fields filled in.
type kernelInjectionLimits struct {
	contextRows          int
	contextRowChars      int
	injectedAtomChars    int
	specialistBlocks     int
	specialistTopicChars int
	specialistBlockChars int
}

// kernelInjectionLimits resolves c's injection bounds, filling any unset
// (non-positive) field from the JIT defaults. The fill matters because
// WithConfig accepts partially-filled structs -- compiler_test.go builds
// CompilerConfig{DefaultTokenBudget: 50000} -- and an unfilled zero would
// read as "show nothing" instead of "unset".
func (c CompilerConfig) kernelInjectionLimits() kernelInjectionLimits {
	d := userconfig.DefaultJITConfig()
	lim := kernelInjectionLimits{
		contextRows:          d.KernelContextRows,
		contextRowChars:      d.KernelContextRowChars,
		injectedAtomChars:    d.KernelInjectedAtomChars,
		specialistBlocks:     d.SpecialistKnowledgeBlocks,
		specialistTopicChars: d.SpecialistTopicChars,
		specialistBlockChars: d.SpecialistBlockChars,
	}
	if c.KernelContextRows > 0 {
		lim.contextRows = c.KernelContextRows
	}
	if c.KernelContextRowChars > 0 {
		lim.contextRowChars = c.KernelContextRowChars
	}
	if c.KernelInjectedAtomChars > 0 {
		lim.injectedAtomChars = c.KernelInjectedAtomChars
	}
	if c.SpecialistKnowledgeBlocks > 0 {
		lim.specialistBlocks = c.SpecialistKnowledgeBlocks
	}
	if c.SpecialistTopicChars > 0 {
		lim.specialistTopicChars = c.SpecialistTopicChars
	}
	if c.SpecialistBlockChars > 0 {
		lim.specialistBlockChars = c.SpecialistBlockChars
	}
	return lim
}

// renderKernelContextBlock merges injectable_context rows under lim, leaving
// markers so the model can tell "this is everything" from "this is the first
// N of M".
func renderKernelContextBlock(rows []string, lim kernelInjectionLimits) string {
	var sb strings.Builder
	sb.WriteString("// KERNEL-INJECTED CONTEXT (from spreading activation)\n")
	shown := len(rows)
	if shown > lim.contextRows {
		shown = lim.contextRows
	}
	for _, row := range rows[:shown] {
		sb.WriteString("- ")
		sb.WriteString(types.ClampHead(row, lim.contextRowChars, "injectable_context row"))
		sb.WriteString("\n")
	}
	if notice := types.TruncationNotice(shown, len(rows), "injectable_context rows"); notice != "" {
		sb.WriteString(notice)
		sb.WriteString("\n")
	}
	return types.ClampText(sb.String(), lim.injectedAtomChars, "injectable_context")
}

func (c *JITPromptCompiler) collectDynamicKernelSnapshotWithConfig(kernel KernelQuerier, cc *CompilationContext, config CompilerConfig) (atoms []*PromptAtom, fingerprint string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			atoms = nil
			fingerprint = ""
			err = fmt.Errorf("dynamic prompt snapshot panic: %v", recovered)
		}
	}()
	atoms, err = c.collectKernelInjectedAtomsFromConfig(kernel, cc, config)
	if err != nil {
		return nil, "", err
	}
	records := make([]string, 0, len(atoms))
	if snapshot, available := kernel.(KernelFactSnapshotQuerier); available {
		facts, queryErr := snapshot.QueryAll()
		if queryErr != nil {
			return nil, "", fmt.Errorf("query complete kernel snapshot: %w", queryErr)
		}
		for predicate, rows := range facts {
			encodedPredicate, encodeErr := json.Marshal([]string{"predicate", predicate})
			if encodeErr != nil {
				return nil, "", encodeErr
			}
			records = append(records, string(encodedPredicate))
			for _, fact := range rows {
				arguments := make([]any, 0, len(fact.Args))
				for _, argument := range fact.Args {
					value := argument
					if printable, supported := argument.(fmt.Stringer); supported {
						value = printable.String()
					}
					arguments = append(arguments, []any{fmt.Sprintf("%T", argument), value})
				}
				encoded, encodeErr := json.Marshal([]any{"fact", predicate, fact.Predicate, arguments})
				if encodeErr != nil {
					return nil, "", fmt.Errorf("encode kernel snapshot %s: %w", predicate, encodeErr)
				}
				records = append(records, string(encoded))
			}
		}
	} else if _, scoped := c.kernel.(KernelScopeProvider); scoped {
		return nil, "", fmt.Errorf("isolated kernel scope must expose QueryAll for dynamic cache identity")
	}
	for _, atom := range atoms {
		encoded, encodeErr := json.Marshal([]string{"injection", atom.ID, atom.Content})
		if encodeErr != nil {
			return nil, "", encodeErr
		}
		records = append(records, string(encoded))
	}
	encoded, err := json.Marshal(canonicalStringSet(records))
	if err != nil {
		return nil, "", err
	}
	return atoms, HashContent(string(encoded)), nil
}

func (c *JITPromptCompiler) collectKernelInjectedAtomsFromConfig(kernel KernelQuerier, cc *CompilationContext, config CompilerConfig) ([]*PromptAtom, error) {
	if kernel == nil || cc == nil {
		return nil, nil
	}
	lim := config.kernelInjectionLimits()

	matchesShard := func(raw string) bool {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return false
		}
		if raw == "*" || raw == "/_all" {
			return true
		}
		rawTrim := strings.TrimPrefix(raw, "/")
		if cc.ShardInstanceID != "" && rawTrim == cc.ShardInstanceID {
			return true
		}
		return rawTrim == cc.ShardID
	}

	dynamic := make([]*PromptAtom, 0, 2)

	// Injectable context atoms
	ctxFacts, err := kernel.Query("injectable_context")
	if err != nil {
		return nil, err
	}
	ctxAtoms := make([]string, 0, len(ctxFacts))
	for _, fact := range ctxFacts {
		if len(fact.Args) < 2 {
			continue
		}
		factShardID, err := extractStringArgFast(fact.Args[0])
		if err != nil || !matchesShard(factShardID) {
			continue
		}
		if atom, ok := fact.Args[1].(string); ok && strings.TrimSpace(atom) != "" {
			ctxAtoms = append(ctxAtoms, atom)
		} else if atomStr, err := extractStringArgFast(fact.Args[1]); err == nil && strings.TrimSpace(atomStr) != "" {
			ctxAtoms = append(ctxAtoms, atomStr)
		}
	}
	if len(ctxAtoms) > 0 {
		ctxAtoms = canonicalStringSet(ctxAtoms)
		content := renderKernelContextBlock(ctxAtoms, lim)
		id := "kernel/context/" + HashContent(content)[:8]
		pa := NewPromptAtom(id, CategoryContext, content)
		pa.IsMandatory = true
		pa.KernelInjected = true
		pa.Priority = 95
		pa.ShardTypes = []string{cc.ShardID}
		dynamic = append(dynamic, pa)
	}

	// Specialist knowledge blocks
	knowledgeFacts, err := kernel.Query("specialist_knowledge")
	if err != nil {
		return nil, fmt.Errorf("query specialist_knowledge: %w", err)
	}
	{
		type block struct {
			topic   string
			content string
		}
		blocks := make([]block, 0, len(knowledgeFacts))
		for _, fact := range knowledgeFacts {
			if len(fact.Args) < 3 {
				continue
			}
			factShardID, err := extractStringArgFast(fact.Args[0])
			if err != nil || !matchesShard(factShardID) {
				continue
			}
			topic, err1 := extractStringArgFast(fact.Args[1])
			body, err2 := extractStringArgFast(fact.Args[2])
			if err1 == nil && err2 == nil && strings.TrimSpace(topic) != "" && strings.TrimSpace(body) != "" {
				blocks = append(blocks, block{topic: topic, content: body})
			}
		}
		if len(blocks) > 0 {
			sort.Slice(blocks, func(left, right int) bool {
				if blocks[left].topic != blocks[right].topic {
					return blocks[left].topic < blocks[right].topic
				}
				return blocks[left].content < blocks[right].content
			})
			uniqueBlocks := blocks[:0]
			for _, knowledgeBlock := range blocks {
				if len(uniqueBlocks) == 0 || uniqueBlocks[len(uniqueBlocks)-1] != knowledgeBlock {
					uniqueBlocks = append(uniqueBlocks, knowledgeBlock)
				}
			}
			blocks = uniqueBlocks
			var sb strings.Builder
			sb.WriteString("// SPECIALIST KNOWLEDGE (Type B/U expertise)\n")
			shown := len(blocks)
			if shown > lim.specialistBlocks {
				shown = lim.specialistBlocks
			}
			for _, b := range blocks[:shown] {
				sb.WriteString("## ")
				sb.WriteString(types.ClampHead(b.topic, lim.specialistTopicChars, "specialist_knowledge topic"))
				sb.WriteString("\n")
				sb.WriteString(types.ClampText(b.content, lim.specialistBlockChars, "specialist_knowledge body"))
				sb.WriteString("\n\n")
			}
			if notice := types.TruncationNotice(shown, len(blocks), "specialist_knowledge blocks"); notice != "" {
				sb.WriteString(notice)
				sb.WriteString("\n")
			}
			content := types.ClampText(strings.TrimRight(sb.String(), "\n"), lim.injectedAtomChars, "specialist_knowledge")
			id := "kernel/knowledge/" + HashContent(content)[:8]
			pa := NewPromptAtom(id, CategoryKnowledge, content)
			pa.IsMandatory = true
			pa.KernelInjected = true
			pa.Priority = 90
			pa.ShardTypes = []string{cc.ShardID}
			dynamic = append(dynamic, pa)
		}
	}

	return dynamic, nil
}
