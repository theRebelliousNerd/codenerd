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

// logRelevance records what the judgment did to this compile's flesh: how
// many pooled atoms Jev read as applying, unsettled or not applying, and how
// many vector slots it refused or extra atoms it admitted.
func logRelevance(cc *CompilationContext, cfg AtomRelevance, out relevanceOutcome, pool, slots map[string]float64) {
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
	var applies, unsettled, refused, admittedExtra, refusedSlots int
	for id, pct := range out.Percent {
		_, slot := slots[id]
		switch {
		case pct >= int64(cfg.YesPercent):
			applies++
			if !slot {
				admittedExtra++
			}
		case pct <= int64(cfg.NoPercent):
			refused++
			if slot {
				refusedSlots++
			}
		default:
			unsettled++
		}
	}
	log.Info("JIT[%s] atom relevance: %d pooled (%d vector slots), %d asked + %d cached in %s: applies=%d unsettled=%d does_not_apply=%d; admitted %d beyond the vector slots, refused %d vector slots",
		shard, len(pool), len(slots), out.Judged, out.CacheHits, out.Elapsed.Round(time.Millisecond),
		applies, unsettled, refused, admittedExtra, refusedSlots)
}

// SetAtomRelevance turns the compiler's per-turn relevance on or off. Cached
// prompts compiled under the other setting are not served again.
func (c *JITPromptCompiler) SetAtomRelevance(r AtomRelevance) {
	c.selector.SetAtomRelevance(r)
	c.bumpSourceEpoch()
}
