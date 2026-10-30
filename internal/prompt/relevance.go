package prompt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

// Per-turn relevance of flesh atoms (Docs/architecture/jev-turn-judgments.md,
// "Atom relevance"). Vector search stays recall: it nominates a pool of
// eligible atoms wider than its own slots. Jev answers, per pooled atom,
// "Does this guidance apply to this request?", and the answers reach the
// kernel as atom_relevance(Atom, Percent). jit_compiler.mg reads them in
// bands: applies admits from anywhere in the pool, does-not-apply refuses even
// a vector slot, and unsettled or unjudged leaves the vector channel's slots
// in charge. A failed, late or disabled judgment asserts nothing, so the
// compile is exactly the vector channel's.

// AtomRelevanceRequest is the turn's state Jev judges against: what a person
// would read to decide whether a guidance block applies.
type AtomRelevanceRequest struct {
	// Request is the turn's request text (the compile's semantic query).
	Request string
	// Intent is the intent verb without its slash ("fix", "review").
	Intent string
	// Target is the file, function or package the intent is aimed at.
	Target string
	// Language is the language of the work, when known.
	Language string
	// Frameworks are the frameworks in play, when known.
	Frameworks []string
	// Agent is the shard type the prompt is for ("coder", "reviewer").
	Agent string
}

// key identifies the request for the per-turn cache. Every model call of one
// turn compiles with the same request, so later compiles reuse the answers.
func (r AtomRelevanceRequest) key() string {
	h := sha256.New()
	for _, part := range []string{r.Request, r.Intent, r.Target, r.Language, strings.Join(r.Frameworks, ","), r.Agent} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// AtomRelevanceJudge answers, for each atom, the probability that its
// guidance applies to the request, keyed by atom ID. Any failure is an error
// and no answers: a missing answer is never a "no".
type AtomRelevanceJudge interface {
	JudgeAtomRelevance(ctx context.Context, req AtomRelevanceRequest, atoms []*PromptAtom) (map[string]float64, error)
}

// AtomRelevance configures the per-turn judgment. A nil Judge is off.
type AtomRelevance struct {
	Judge AtomRelevanceJudge
	// Pool bounds the atoms judged per compile: the vector channel's slots
	// plus the next most similar eligible atoms up to this many.
	Pool int
	// Timeout bounds one compile's judgment; past it the compile proceeds on
	// the vector channel alone.
	Timeout time.Duration
	// YesPercent and NoPercent are the band edges (typesafe
	// judgment_yes_percent / judgment_no_percent), asserted as
	// atom_relevance_threshold rows beside the judgments.
	YesPercent, NoPercent int
}

// relevanceCacheLimit bounds the per-turn cache; crossing it starts over,
// which costs one re-judgment of the current turn at most.
const relevanceCacheLimit = 8192

// relevanceCache holds answers by request, atom and content, so the many
// compiles of one turn ask Jev once per atom.
type relevanceCache struct {
	mu      sync.Mutex
	entries map[string]float64
}

func relevanceCacheKey(reqKey string, atom *PromptAtom) string {
	return reqKey + "\x00" + atom.ID + "\x00" + atom.ContentHash
}

func (c *relevanceCache) get(key string) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.entries[key]
	return v, ok
}

func (c *relevanceCache) putAll(values map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries)+len(values) > relevanceCacheLimit {
		c.entries = make(map[string]float64, len(values))
	}
	for k, v := range values {
		c.entries[k] = v
	}
}

// carryLimit bounds the remembered tasks; crossing it starts over, which
// costs each task one compile without carried atoms.
const carryLimit = 512

// carryMemory remembers, per task, the judged flesh atoms the last
// completed prompt of the task carried, in their prompt order. The next
// compile of the same task carries them: they rejoin the recall pool and
// stay admitted unless Jev now reads them as not applying (jit_compiler.mg
// flesh_admitted(_, /carried)), and they keep their places ahead of newly
// admitted flesh (assembler tierCarried), so consecutive turns of one task
// share the longest cacheable prefix. A new task starts with nothing carried.
//
// Carry is prefix memory, so only a completed prompt publishes it (stage,
// then publish): an atom selected but shed by the token budget, or a compile
// that failed after selection, leaves it as it was. What one compile
// carries into the relevance pool is therefore bounded by the flesh the last
// prompt fit, not by every atom ever selected for the task.
type carryMemory struct {
	mu    sync.Mutex
	tasks map[string]carryEntry
}

// carryEntry is one task's published carry and the selection sequence it
// came from: a compile that selected earlier never overwrites a later one.
type carryEntry struct {
	ids []string
	seq uint64
}

// carryStage is a compile's candidate carry, staged at selection and
// published when its prompt completes.
type carryStage struct {
	key string
	ids []string
	seq uint64
}

// carrySeq orders selections across compiles.
var carrySeq atomic.Uint64

func (m *carryMemory) get(key string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.tasks[key].ids...)
}

// publish records the staged carry the completed prompt kept: the staged ids
// it includes, in staged order. A stage selected before the task's current
// entry is stale and changes nothing.
func (m *carryMemory) publish(stage *carryStage, included map[string]bool) {
	if stage == nil {
		return
	}
	ids := make([]string, 0, len(stage.ids))
	for _, id := range stage.ids {
		if included[id] {
			ids = append(ids, id)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.tasks[stage.key]; ok && cur.seq > stage.seq {
		return
	}
	if m.tasks == nil || len(m.tasks) >= carryLimit {
		m.tasks = make(map[string]carryEntry)
	}
	m.tasks[stage.key] = carryEntry{ids: ids, seq: stage.seq}
}

// carryKey names the task a compile serves: the agent and what the intent is
// aimed at. A different target or agent is a different task.
func carryKey(cc *CompilationContext) string {
	return strings.TrimPrefix(cc.ShardType, "/") + "\x00" + strings.TrimPrefix(cc.IntentVerb, "/") + "\x00" + strings.TrimSpace(cc.IntentTarget)
}

// carriedInto adds the task's carried atoms that are still eligible flesh to
// the pool (at their raw vector score, 0 when unscored this turn) and returns
// them in carried order.
func carriedInto(pool, raw map[string]float64, carried []string, fleshAtoms []*PromptAtom, cc *CompilationContext) []string {
	eligible := make(map[string]*PromptAtom, len(fleshAtoms))
	for _, a := range fleshAtoms {
		if a != nil && !a.IsMandatory && !a.RetrievedContext && a.MatchesContext(cc) {
			eligible[a.ID] = a
		}
	}
	var out []string
	for _, id := range carried {
		if _, ok := eligible[id]; !ok {
			continue
		}
		if _, in := pool[id]; !in {
			pool[id] = raw[id]
		}
		out = append(out, id)
	}
	return out
}

// carriedFacts mark the carried atoms for the kernel's /carried row.
func carriedFacts(carried []string) []any {
	facts := make([]any, 0, len(carried))
	for _, id := range carried {
		facts = append(facts, "flesh_carried("+mangleQuoteString(id)+")")
	}
	return facts
}

// relevanceOutcome is one compile's judgment, for facts and the log line.
type relevanceOutcome struct {
	// Percent is each judged atom's answer as a percent; nil when nothing was
	// judged (off, no query, failed or late).
	Percent   map[string]int64
	Judged    int
	CacheHits int
	Elapsed   time.Duration
	Err       error
}

// SetAtomRelevance turns per-turn relevance on (a non-nil Judge) or off.
func (s *AtomSelector) SetAtomRelevance(r AtomRelevance) {
	s.vectorConfigMu.Lock()
	defer s.vectorConfigMu.Unlock()
	s.relevance = r
}

func (s *AtomSelector) relevanceSnapshot() AtomRelevance {
	s.vectorConfigMu.RLock()
	defer s.vectorConfigMu.RUnlock()
	return s.relevance
}

// relevanceRequestFor is the compile's request as Jev sees it.
func relevanceRequestFor(cc *CompilationContext) AtomRelevanceRequest {
	trim := func(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "/") }
	req := AtomRelevanceRequest{
		Request:  strings.TrimSpace(cc.SemanticQuery),
		Intent:   trim(cc.IntentVerb),
		Target:   strings.TrimSpace(cc.IntentTarget),
		Language: trim(cc.Language),
		Agent:    trim(cc.ShardType),
	}
	for _, fw := range cc.Frameworks {
		if fw = trim(fw); fw != "" {
			req.Frameworks = append(req.Frameworks, fw)
		}
	}
	return req
}

// recallPool widens the vector channel's slots to the pool: the slots, then
// the most similar remaining eligible atoms (same eligibility as the slots,
// above the absolute floor, but not held to the relative floor that keeps the
// vector channel alone conservative), until the pool is full.
func (s *AtomSelector) recallPool(
	raw map[string]float64,
	slots map[string]float64,
	fleshAtoms []*PromptAtom,
	cc *CompilationContext,
	pool int,
) map[string]float64 {
	out := make(map[string]float64, max(pool, len(slots)))
	for id, score := range slots {
		out[id] = score
	}
	if len(raw) == 0 || pool <= len(out) {
		return out
	}
	floor := 0.0
	if s != nil {
		floor = s.minScoreThreshold
	}
	type candidate struct {
		id    string
		score float64
	}
	var rest []candidate
	for _, atom := range fleshAtoms {
		if atom == nil || atom.IsMandatory || atom.RetrievedContext {
			continue
		}
		if _, taken := out[atom.ID]; taken {
			continue
		}
		score, ok := raw[atom.ID]
		if !ok || math.IsNaN(score) || score < floor || !atom.MatchesContext(cc) {
			continue
		}
		rest = append(rest, candidate{atom.ID, score})
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].score != rest[j].score {
			return rest[i].score > rest[j].score
		}
		return rest[i].id < rest[j].id
	})
	for _, c := range rest {
		if len(out) >= pool {
			break
		}
		out[c.id] = c.score
	}
	return out
}

// judgeRelevance asks the judge about every pooled atom not already answered
// for this request, within the configured timeout. All or nothing: a failed
// or late call leaves the outcome without percents.
func (s *AtomSelector) judgeRelevance(
	ctx context.Context,
	cfg AtomRelevance,
	cc *CompilationContext,
	pool map[string]float64,
	fleshAtoms []*PromptAtom,
) relevanceOutcome {
	start := time.Now()
	req := relevanceRequestFor(cc)
	reqKey := req.key()
	byID := make(map[string]*PromptAtom, len(fleshAtoms))
	for _, a := range fleshAtoms {
		if a != nil {
			byID[a.ID] = a
		}
	}
	ids := make([]string, 0, len(pool))
	for id := range pool {
		if _, ok := byID[id]; ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	out := relevanceOutcome{Percent: make(map[string]int64, len(ids))}
	var ask []*PromptAtom
	for _, id := range ids {
		atom := byID[id]
		if p, ok := s.relevanceCache.get(relevanceCacheKey(reqKey, atom)); ok {
			out.Percent[id] = probabilityPercent(p)
			out.CacheHits++
			continue
		}
		ask = append(ask, atom)
	}
	if len(ask) > 0 {
		jctx := ctx
		if cfg.Timeout > 0 {
			var cancel context.CancelFunc
			jctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
			defer cancel()
		}
		answers, err := cfg.Judge.JudgeAtomRelevance(jctx, req, ask)
		if err == nil {
			for _, atom := range ask {
				if _, ok := answers[atom.ID]; !ok {
					err = errors.New("judge returned no answer for " + atom.ID)
					break
				}
			}
		}
		if err != nil {
			return relevanceOutcome{Elapsed: time.Since(start), Err: err, CacheHits: out.CacheHits}
		}
		fresh := make(map[string]float64, len(ask))
		for _, atom := range ask {
			p := answers[atom.ID]
			fresh[relevanceCacheKey(reqKey, atom)] = p
			out.Percent[atom.ID] = probabilityPercent(p)
		}
		s.relevanceCache.putAll(fresh)
		out.Judged = len(ask)
	}
	out.Elapsed = time.Since(start)
	return out
}

// probabilityPercent is a probability as the integer percent the kernel's
// /number bound takes, clamped to 0..100.
func probabilityPercent(p float64) int64 {
	if math.IsNaN(p) || p <= 0 {
		return 0
	}
	if p >= 1 {
		return 100
	}
	return int64(math.Round(p * 100))
}

// relevanceFacts are the judgment's kernel inputs: one atom_relevance per
// answer and the two thresholds that read them. No answers, no facts.
func relevanceFacts(out relevanceOutcome, cfg AtomRelevance) []any {
	if len(out.Percent) == 0 {
		return nil
	}
	facts := make([]any, 0, len(out.Percent)+2)
	ids := make([]string, 0, len(out.Percent))
	for id := range out.Percent {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		facts = append(facts, "atom_relevance("+mangleQuoteString(id)+", "+strconv.FormatInt(out.Percent[id], 10)+")")
	}
	facts = append(facts,
		"atom_relevance_threshold(/yes, "+strconv.Itoa(cfg.YesPercent)+")",
		"atom_relevance_threshold(/no, "+strconv.Itoa(cfg.NoPercent)+")",
	)
	return facts
}

// logRelevance records the judgment itself: the pool, how much was asked or
// served from the turn's cache, and how long it took. What the kernel did
// with it is logged after the query (logRelevanceAdmission).
func logRelevance(cc *CompilationContext, out relevanceOutcome, pool, slots map[string]float64) {
	log := logging.Get(logging.CategoryJIT)
	shard := ""
	if cc != nil {
		shard = cc.ShardID
	}
	if out.Err != nil {
		log.Warn("JIT[%s] atom relevance: Jev gave no judgment (%v) after %s; the vector channel decides alone (%d slots, %d pooled, %d cached answers unused)",
			shard, out.Err, out.Elapsed.Round(time.Millisecond), len(slots), len(pool), out.CacheHits)
		return
	}
	log.Info("JIT[%s] atom relevance: %d pooled (%d vector slots), %d asked + %d cached in %s",
		shard, len(pool), len(slots), out.Judged, out.CacheHits, out.Elapsed.Round(time.Millisecond))
}

// logRelevanceAdmission reads the kernel's admission table for this compile
// (jit_compiler.mg flesh_admitted / flesh_refused) and records which row
// admitted each flesh atom and which atoms Jev's reading refused. The
// decision is the kernel's; this only reports it.
func logRelevanceAdmission(kernel KernelQuerier, cc *CompilationContext) {
	shard := ""
	if cc != nil {
		shard = cc.ShardID
	}
	rows := func(predicate string, facts []Fact, err error) map[string][]string {
		if err != nil {
			logging.Get(logging.CategoryJIT).Warn("JIT[%s] %s: %v", shard, predicate, err)
			return nil
		}
		out := map[string][]string{}
		for _, f := range facts {
			if len(f.Args) != 2 {
				continue
			}
			id, err1 := extractStringArg(f.Args[0])
			row, err2 := extractStringArg(f.Args[1])
			if err1 != nil || err2 != nil {
				continue
			}
			out[row] = append(out[row], id)
		}
		for _, ids := range out {
			sort.Strings(ids)
		}
		return out
	}
	admittedFacts, admittedErr := kernel.Query("flesh_admitted(Atom, Row)")
	admitted := rows("flesh_admitted", admittedFacts, admittedErr)
	refusedFacts, refusedErr := kernel.Query("flesh_refused(Atom, Row)")
	refused := rows("flesh_refused", refusedFacts, refusedErr)
	logging.Get(logging.CategoryJIT).Info("JIT[%s] flesh admission: admitted %v; refused %v", shard, admitted, refused)
}

// SetAtomRelevance turns the compiler's per-turn relevance on or off. Cached
// prompts compiled under the other setting are not served again.
func (c *JITPromptCompiler) SetAtomRelevance(r AtomRelevance) {
	c.selector.SetAtomRelevance(r)
	c.bumpSourceEpoch()
}

// Combined scores for judged flesh (selector.go queryFleshAtoms): carried
// atoms above new ones, carried order kept by a step far below any gap.
const (
	judgedCarriedScore = 0.9
	judgedNewScore     = 0.5
	carriedRankStep    = 1e-4
)

// stageCarry stages this compile's judged flesh for the task's next compile:
// the carried atoms still admitted, in their order, then the newly admitted
// ones by ID, which is the order the prompt shows them in. The compiler
// publishes it when the prompt completes (publishCarry).
func (s *AtomSelector) stageCarry(cc *CompilationContext, selected []*ScoredAtom, carriedRank map[string]int) {
	var kept, fresh []string
	for _, sa := range selected {
		if sa == nil || sa.Atom == nil {
			continue
		}
		if _, ok := carriedRank[sa.Atom.ID]; ok {
			kept = append(kept, sa.Atom.ID)
		} else {
			fresh = append(fresh, sa.Atom.ID)
		}
	}
	sort.Slice(kept, func(i, j int) bool { return carriedRank[kept[i]] < carriedRank[kept[j]] })
	sort.Strings(fresh)
	cc.stagedCarry = &carryStage{key: carryKey(cc), ids: append(kept, fresh...), seq: carrySeq.Add(1)}
}

// publishCarry publishes the carry the compile staged, kept to the atoms its
// completed prompt includes. A compile that staged nothing (no complete
// judgment) changes nothing.
func (s *AtomSelector) publishCarry(cc *CompilationContext, fitted []*OrderedAtom) {
	if cc == nil || cc.stagedCarry == nil {
		return
	}
	included := make(map[string]bool, len(fitted))
	for _, oa := range fitted {
		if oa != nil && oa.Atom != nil {
			included[oa.Atom.ID] = true
		}
	}
	s.carry.publish(cc.stagedCarry, included)
	cc.stagedCarry = nil
}
