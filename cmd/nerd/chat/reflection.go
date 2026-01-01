package chat

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

const (
	reflectionQueryMaxLen   = 600
	reflectionQueryMinChars = 12
)

// ReflectionState captures the last System 2 recall pass for inspection.
type ReflectionState struct {
	Query         string
	TraceHits     []store.TraceRecallHit
	LearningHits  []store.LearningRecallHit
	ContextHits   []string
	UsedEmbedding bool
	Duration      time.Duration
	Warnings      []string
}

func (m *Model) performReflection(ctx context.Context, input string, intent perception.Intent) *ReflectionState {
	cfg := config.DefaultReflectionConfig()
	if m.Config != nil {
		cfg = m.Config.GetReflectionConfig()
	}
	if !cfg.Enabled {
		return nil
	}
	if m.localDB == nil && m.learningStore == nil {
		return nil
	}

	query := buildReflectionQuery(input, intent)
	if len(query) < reflectionQueryMinChars {
		return nil
	}

	state := &ReflectionState{
		Query: query,
	}
	start := time.Now()
	defer func() { state.Duration = time.Since(start) }()

	var (
		traceHits    []store.TraceRecallHit
		learningHits []store.LearningRecallHit
		warnings     []string
	)

	expectedModel := ""
	expectedDim := 0
	if m.embeddingEngine != nil {
		expectedModel = m.embeddingEngine.Name()
		expectedDim = m.embeddingEngine.Dimensions()
	}

	if m.embeddingEngine != nil {
		queryTask := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
		embedCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		queryEmbedding, err := embedReflectionQuery(embedCtx, m.embeddingEngine, queryTask, query)
		cancel()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Reflection embedding failed: %v", err))
		} else {
			state.UsedEmbedding = true

			if m.localDB != nil {
				hits, err := m.localDB.RecallTracesByEmbedding(queryEmbedding, cfg.TopK*3)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("Trace recall failed: %v", err))
				} else {
					expectedTask := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
					traceHits = filterTraceHits(hits, expectedModel, expectedDim, expectedTask, &warnings)
				}
			}

			if m.learningStore != nil {
				hits, err := m.learningStore.RecallLearningsByEmbedding(queryEmbedding, cfg.TopK*3)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("Learning recall failed: %v", err))
				} else {
					expectedTask := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
					learningHits = filterLearningHits(hits, expectedModel, expectedDim, expectedTask, &warnings)
				}
			}
		}
	}

	traceRanked := rankTraceHits(traceHits, cfg)
	learningRanked := rankLearningHits(learningHits, cfg)

	if len(traceRanked) == 0 && m.localDB != nil {
		hits, err := m.localDB.RecallTracesLexical(query, cfg.TopK)
		if err == nil {
			traceRanked = rankTraceHits(hits, cfg)
		}
	}
	if len(learningRanked) == 0 && m.learningStore != nil {
		hits, err := m.learningStore.RecallLearningsLexical(query, cfg.TopK)
		if err == nil {
			learningRanked = rankLearningHits(hits, cfg)
		}
	}

	state.TraceHits = extractTraceHits(traceRanked)
	state.LearningHits = extractLearningHits(learningRanked)
	state.ContextHits = formatReflectionHits(traceRanked, learningRanked)
	state.Warnings = warnings

	if m.kernel != nil {
		assertReflectionFacts(m.kernel, traceRanked, learningRanked)
	}

	if len(state.ContextHits) == 0 && len(state.Warnings) > 0 {
		logging.Get(logging.CategoryContext).Debug("Reflection warnings: %s", strings.Join(state.Warnings, "; "))
	}
	return state
}

func (m *Model) renderReflectionStatus() string {
	if m.lastReflection == nil {
		return "No reflection recall has run yet in this session."
	}

	state := m.lastReflection
	var sb strings.Builder
	sb.WriteString("**Reflection status**\n\n")
	sb.WriteString(fmt.Sprintf("Query: %s\n", truncateForContext(state.Query, 200)))
	sb.WriteString(fmt.Sprintf("Used embeddings: %t\n", state.UsedEmbedding))
	sb.WriteString(fmt.Sprintf("Duration: %s\n", state.Duration.Round(time.Millisecond)))
	if len(state.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range state.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", w))
		}
	}
	if len(state.ContextHits) > 0 {
		sb.WriteString("\nHits:\n")
		for _, hit := range state.ContextHits {
			sb.WriteString(fmt.Sprintf("- %s\n", hit))
		}
	} else {
		sb.WriteString("\nHits: none\n")
	}
	return sb.String()
}

type rankedTrace struct {
	hit   store.TraceRecallHit
	score float64
}

type rankedLearning struct {
	hit   store.LearningRecallHit
	score float64
}

func buildReflectionQuery(input string, intent perception.Intent) string {
	seen := make(map[string]struct{})
	var parts []string

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		parts = append(parts, value)
	}

	add(intent.Target)
	add(intent.Constraint)
	add(strings.TrimPrefix(intent.Verb, "/"))
	add(strings.TrimSpace(input))

	query := strings.TrimSpace(strings.Join(parts, "\n"))
	if len(query) > reflectionQueryMaxLen {
		query = query[:reflectionQueryMaxLen]
	}
	return query
}

func embedReflectionQuery(ctx context.Context, engine embedding.EmbeddingEngine, taskType string, query string) ([]float32, error) {
	if engine == nil {
		return nil, fmt.Errorf("no embedding engine configured")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty reflection query")
	}
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		return taskAware.EmbedWithTask(ctx, query, taskType)
	}
	return engine.Embed(ctx, query)
}

func filterTraceHits(hits []store.TraceRecallHit, expectedModel string, expectedDim int, expectedTask string, warnings *[]string) []store.TraceRecallHit {
	if expectedModel == "" && expectedDim == 0 && expectedTask == "" {
		return hits
	}
	filtered := make([]store.TraceRecallHit, 0, len(hits))
	dropped := 0
	for _, hit := range hits {
		if isEmbeddingMismatch(hit.EmbeddingModelID, hit.EmbeddingDim, hit.EmbeddingTask, expectedModel, expectedDim, expectedTask) {
			dropped++
			continue
		}
		filtered = append(filtered, hit)
	}
	if dropped > 0 && warnings != nil {
		*warnings = append(*warnings, fmt.Sprintf("Skipped %d trace hits with mismatched embedding metadata", dropped))
	}
	return filtered
}

func filterLearningHits(hits []store.LearningRecallHit, expectedModel string, expectedDim int, expectedTask string, warnings *[]string) []store.LearningRecallHit {
	if expectedModel == "" && expectedDim == 0 && expectedTask == "" {
		return hits
	}
	filtered := make([]store.LearningRecallHit, 0, len(hits))
	dropped := 0
	for _, hit := range hits {
		if isEmbeddingMismatch(hit.EmbeddingModelID, hit.EmbeddingDim, hit.EmbeddingTask, expectedModel, expectedDim, expectedTask) {
			dropped++
			continue
		}
		filtered = append(filtered, hit)
	}
	if dropped > 0 && warnings != nil {
		*warnings = append(*warnings, fmt.Sprintf("Skipped %d learning hits with mismatched embedding metadata", dropped))
	}
	return filtered
}

func isEmbeddingMismatch(modelID string, dim int, task string, expectedModel string, expectedDim int, expectedTask string) bool {
	if expectedModel != "" {
		if modelID == "" || modelID != expectedModel {
			return true
		}
	}
	if expectedDim > 0 {
		if dim == 0 || dim != expectedDim {
			return true
		}
	}
	if expectedTask != "" {
		if task == "" || task != expectedTask {
			return true
		}
	}
	return false
}

func rankTraceHits(hits []store.TraceRecallHit, cfg config.ReflectionConfig) []rankedTrace {
	if len(hits) == 0 {
		return nil
	}
	ranked := make([]rankedTrace, 0, len(hits))
	for _, hit := range hits {
		weighted := applyRecencyWeight(hit.Score, hit.CreatedAt, cfg.RecencyHalfLifeDays)
		if weighted < cfg.MinScore {
			continue
		}
		ranked = append(ranked, rankedTrace{hit: hit, score: weighted})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if cfg.TopK > 0 && len(ranked) > cfg.TopK {
		ranked = ranked[:cfg.TopK]
	}
	return ranked
}

func rankLearningHits(hits []store.LearningRecallHit, cfg config.ReflectionConfig) []rankedLearning {
	if len(hits) == 0 {
		return nil
	}
	ranked := make([]rankedLearning, 0, len(hits))
	for _, hit := range hits {
		weighted := applyRecencyWeight(hit.Score, hit.LearnedAt, cfg.RecencyHalfLifeDays)
		if weighted < cfg.MinScore {
			continue
		}
		ranked = append(ranked, rankedLearning{hit: hit, score: weighted})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if cfg.TopK > 0 && len(ranked) > cfg.TopK {
		ranked = ranked[:cfg.TopK]
	}
	return ranked
}

func extractTraceHits(ranked []rankedTrace) []store.TraceRecallHit {
	out := make([]store.TraceRecallHit, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.hit)
	}
	return out
}

func extractLearningHits(ranked []rankedLearning) []store.LearningRecallHit {
	out := make([]store.LearningRecallHit, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.hit)
	}
	return out
}

func formatReflectionHits(traceHits []rankedTrace, learningHits []rankedLearning) []string {
	var hits []string
	for _, r := range traceHits {
		hits = append(hits, formatTraceHit(r.hit, r.score))
	}
	for _, r := range learningHits {
		hits = append(hits, formatLearningHit(r.hit, r.score))
	}
	return hits
}

func formatTraceHit(hit store.TraceRecallHit, score float64) string {
	outcome := normalizeOutcome(hit.Outcome)
	summary := strings.TrimSpace(hit.Summary)
	if summary == "" {
		summary = hit.TraceID
	}
	return fmt.Sprintf("Trace %s %s (%d%%): %s",
		hit.ShardType,
		strings.TrimPrefix(outcome, "/"),
		scoreToPercent(score),
		truncateForContext(summary, 220),
	)
}

func formatLearningHit(hit store.LearningRecallHit, score float64) string {
	summary := strings.TrimSpace(hit.Summary)
	if summary == "" {
		summary = hit.Predicate
	}
	predicate := strings.TrimSpace(hit.Predicate)
	if predicate == "" {
		predicate = "learning"
	}
	return fmt.Sprintf("Learning %s %s (%d%%): %s",
		hit.ShardType,
		predicate,
		scoreToPercent(score),
		truncateForContext(summary, 220),
	)
}

func assertReflectionFacts(kernel *core.RealKernel, traceHits []rankedTrace, learningHits []rankedLearning) {
	for _, r := range traceHits {
		outcome := normalizeOutcome(r.hit.Outcome)
		summary := strings.TrimSpace(r.hit.Summary)
		if summary == "" {
			summary = r.hit.TraceID
		}
		summary = truncateForContext(summary, 300)
		score := scoreToPercent(r.score)
		_ = kernel.Assert(core.Fact{
			Predicate: "trace_recall_result",
			Args:      []interface{}{r.hit.TraceID, score, outcome, summary},
		})
	}

	for _, r := range learningHits {
		summary := strings.TrimSpace(r.hit.Summary)
		if summary == "" {
			summary = r.hit.Predicate
		}
		summary = truncateForContext(summary, 300)
		score := scoreToPercent(r.score)
		_ = kernel.Assert(core.Fact{
			Predicate: "learning_recall_result",
			Args:      []interface{}{r.hit.LearningID, score, r.hit.Predicate, summary},
		})
	}
}

func normalizeOutcome(outcome string) string {
	outcome = strings.TrimSpace(outcome)
	if outcome == "" {
		outcome = "success"
	}
	if !strings.HasPrefix(outcome, "/") {
		outcome = "/" + outcome
	}
	return outcome
}

func applyRecencyWeight(score float64, createdAt time.Time, halfLifeDays int) float64 {
	if score <= 0 {
		return score
	}
	if createdAt.IsZero() || halfLifeDays <= 0 {
		return clampScore(score)
	}
	ageDays := time.Since(createdAt).Hours() / 24
	if ageDays <= 0 {
		return clampScore(score)
	}
	decay := math.Pow(0.5, ageDays/float64(halfLifeDays))
	return clampScore(score * decay)
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func scoreToPercent(score float64) int {
	score = clampScore(score)
	return int(math.Round(score * 100))
}
