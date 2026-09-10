package broker

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"time"
)

// This file answers Q1: what is the real distribution of calls per epoch?
//
// An epoch is a maximal run of consecutive calls, inside one scope, that shared
// the same cacheable prefix. It matters because every prefix-cache strategy —
// and the Phase 4 rebuild controller in particular — is an amortization bet:
// you pay a premium to write a prefix into the provider's cache, and you earn
// it back on each later call that reads it instead of resending. If the median
// epoch is one call long, the premium is never earned back and the whole
// controller is dead weight, however elegant it is.
//
// Nothing here decides anything. It measures, so the decision can be made from
// data instead of from architectural taste.

// ---------------------------------------------------------------------------
// Prefix fingerprinting
// ---------------------------------------------------------------------------

// prefixFingerprint returns a stable fingerprint of the cacheable head of a
// request: the tool definitions and the system prompt.
//
// Those two, in that order, are what a provider prefix cache actually covers.
// A prefix cache is a prefix of the *token stream*, and every provider here
// serializes tools ahead of the system block ahead of the messages. So a change
// to a tool definition invalidates the system block's cache entry as well, even
// though the system text did not change — order is load-bearing, not cosmetic,
// and hashing these in wire order is what makes the fingerprint mean
// "invalidation boundary" rather than merely "content identity".
//
// Messages are deliberately excluded. Appending a message extends the prefix
// rather than breaking it, which is exactly the property that lets one prefix
// serve a whole conversation.
//
// Returns "" when there is no cacheable head at all. That is a real and
// interesting state — a call with no system prompt and no tools can never
// benefit from prefix caching — so it is reported rather than hidden.
func prefixFingerprint(req *Request) string {
	if req == nil || (req.System == "" && len(req.Tools) == 0) {
		return ""
	}

	h := fnv.New64a()
	var scratch [8]byte

	writeField := func(s string) {
		binary.LittleEndian.PutUint64(scratch[:], uint64(len(s)))
		// Length-prefixing keeps ("ab","c") distinct from ("a","bc"). Without
		// it, moving a character between adjacent fields would be invisible to
		// the hash, and two genuinely different prefixes would be reported as
		// one epoch.
		_, _ = h.Write(scratch[:])
		_, _ = h.Write([]byte(s))
	}

	for i := range req.Tools {
		tool := &req.Tools[i]
		writeField(tool.Name)
		writeField(tool.Description)
		schema, _ := jsonBytes(tool.InputSchema)
		writeField(string(schema))
	}
	writeField(req.System)

	return strconv.FormatUint(h.Sum64(), 16)
}

// ---------------------------------------------------------------------------
// Cache economics
// ---------------------------------------------------------------------------

// CacheEconomics describes what a provider charges for prefix caching, relative
// to its own uncached input rate.
//
// Kept as data rather than as a constant so Phase 4's break-even is derived per
// provider. A hard-coded break-even is a number that is correct for exactly one
// provider and silently wrong for the rest, which is the failure mode this
// whole package exists to eliminate.
type CacheEconomics struct {
	// WriteMultiplier is the cost of writing a token into the cache, as a
	// multiple of the uncached input rate. Above 1.0 for every provider that
	// charges for cache writes.
	WriteMultiplier float64
	// ReadMultiplier is the cost of reading a cached token, as a multiple of
	// the uncached input rate. At or above 1.0 means caching cannot pay.
	ReadMultiplier float64
	// TTL is how long an unread entry survives. An epoch spanning longer than
	// this gets no reuse however many calls it contains, because the entry is
	// gone before the second call arrives.
	TTL time.Duration
}

// cacheEconomicsByProvider holds published rate ratios. Absent providers get
// noCaching, which reports honestly that no reuse is possible rather than
// inventing a plausible-looking number.
var cacheEconomicsByProvider = map[string]CacheEconomics{
	"anthropic": {WriteMultiplier: 1.25, ReadMultiplier: 0.10, TTL: 5 * time.Minute},
	"openai":    {WriteMultiplier: 1.00, ReadMultiplier: 0.50, TTL: 5 * time.Minute},
	"gemini":    {WriteMultiplier: 1.00, ReadMultiplier: 0.25, TTL: time.Hour},
	"google":    {WriteMultiplier: 1.00, ReadMultiplier: 0.25, TTL: time.Hour},
	"deepseek":  {WriteMultiplier: 1.00, ReadMultiplier: 0.10, TTL: time.Hour},
}

// noCaching is the fallback for providers with no prefix cache. Its break-even
// is +Inf, so no epoch is ever scored as paying.
var noCaching = CacheEconomics{WriteMultiplier: 1, ReadMultiplier: 1}

// EconomicsFor returns the cache economics for a provider, or noCaching.
func EconomicsFor(provider string) CacheEconomics {
	if e, ok := cacheEconomicsByProvider[provider]; ok {
		return e
	}
	return noCaching
}

// BreakEvenCalls returns how many calls a prefix must serve before caching it
// costs less than resending it.
//
// Serving a prefix of size P over N calls costs P*write + (N-1)*P*read with a
// cache, and N*P without one. Setting those equal and solving for N gives
// (write-read)/(1-read), independent of P — which is why the answer is a call
// count and not a token count, and why a big prefix does not make caching pay
// sooner. It makes each call cheaper, not the bet safer.
//
// Returns +Inf when reads are not discounted, because then no N repays the
// write.
func (e CacheEconomics) BreakEvenCalls() float64 {
	if e.ReadMultiplier >= 1 {
		return math.Inf(1)
	}
	n := (e.WriteMultiplier - e.ReadMultiplier) / (1 - e.ReadMultiplier)
	if n < 1 {
		// A provider that charges nothing extra to write still needs the call
		// that writes the entry before any call can read it.
		return 1
	}
	return n
}

// ---------------------------------------------------------------------------
// Epoch segmentation
// ---------------------------------------------------------------------------

// Epoch is one run of calls that shared a cacheable prefix.
type Epoch struct {
	Scope    string    `json:"scope"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	Prefix   string    `json:"prefix"`
	Calls    int       `json:"calls"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`

	// InputTokens and CachedTokens are billed totals across the epoch, summed
	// from provider usage reports.
	InputTokens  int64 `json:"input_tokens"`
	CachedTokens int64 `json:"cached_tokens"`

	// PrefixTokens is the estimated size of the cacheable head, taken from the
	// first receipt's System and Tools segments. Segment splits are
	// proportional attribution rather than separately measured quantities, so
	// treat this as an order of magnitude, not a precise count.
	PrefixTokens int `json:"prefix_tokens"`

	// Method is the call shape that opened the epoch. It is here because
	// without it the headline number is ambiguous in a way that matters.
	//
	// Reading the executor settles what an epoch actually is in this
	// architecture, and it is not a session. The system prompt handed to the
	// provider is the JIT compilation result plus the current target's file
	// context, so it changes from turn to turn by construction -- that is what
	// JIT context management means. What does NOT change is the system prompt
	// within one turn's native tool loop, where runToolLoop passes the same
	// string into every round while only the message history grows, and
	// messages are deliberately outside the fingerprint.
	//
	// So an epoch is one turn's tool loop, and its length is that turn's round
	// count. A p50 of 1 then has two completely different readings: turns that
	// used no tools at all, or a tool loop that is not reusing its prefix. The
	// first is a fact about how the agent is used and Phase 4 cannot fix it;
	// the second is a caching problem and Phase 4 is exactly the fix. Grouping
	// the histogram by method is what separates them.
	Method string `json:"method"`
}

// Span returns the wall-clock duration the epoch covered.
func (e Epoch) Span() time.Duration { return e.Ended.Sub(e.Started) }

// Segment groups receipts into epochs.
//
// Receipts are grouped by (scope, provider, model) and ordered by start time
// within each group, then cut wherever the prefix changes. Grouping first is
// what keeps two concurrent sessions from being spliced into one bogus epoch
// that alternates prefixes and therefore reports every call as a singleton.
//
// Refusals are excluded. A refused call never reached the provider, so it
// neither warmed a cache nor read one; counting it would inflate epoch lengths
// with calls that could not have benefited.
func Segment(receipts []Receipt) []Epoch {
	type key struct{ scope, provider, model string }

	groups := make(map[key][]Receipt)
	for _, r := range receipts {
		if !r.Decision.Allowed {
			continue
		}
		k := key{r.Scope, r.Provider, r.Model}
		groups[k] = append(groups[k], r)
	}

	var epochs []Epoch
	for k, rs := range groups {
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].Started.Before(rs[j].Started) })

		var cur *Epoch
		for _, r := range rs {
			if cur == nil || cur.Prefix != r.Prefix {
				if cur != nil {
					epochs = append(epochs, *cur)
				}
				cur = &Epoch{
					Scope:        k.scope,
					Provider:     k.provider,
					Model:        k.model,
					Prefix:       r.Prefix,
					Started:      r.Started,
					PrefixTokens: r.Estimated.Segments.System + r.Estimated.Segments.Tools,
					Method:       r.Method,
				}
			}
			cur.Calls++
			cur.Ended = r.Started.Add(r.Duration)
			cur.InputTokens += r.Actual.InputTokens
			cur.CachedTokens += r.Actual.CachedTokens
		}
		if cur != nil {
			epochs = append(epochs, *cur)
		}
	}

	sort.SliceStable(epochs, func(i, j int) bool { return epochs[i].Started.Before(epochs[j].Started) })
	return epochs
}

// ---------------------------------------------------------------------------
// Histogram
// ---------------------------------------------------------------------------

// Bucket is one bar of the calls-per-epoch histogram.
type Bucket struct {
	Label string  `json:"label"`
	Low   int     `json:"low"`
	High  int     `json:"high"` // inclusive; 0 means unbounded
	Count int     `json:"count"`
	Pct   float64 `json:"pct"`
}

// bucketBounds are geometric because the question is an order-of-magnitude one.
// The difference between an epoch of 1 and an epoch of 2 decides whether prefix
// caching can work at all; the difference between 33 and 34 decides nothing.
var bucketBounds = []struct {
	label     string
	low, high int
}{
	{"1", 1, 1},
	{"2", 2, 2},
	{"3-4", 3, 4},
	{"5-8", 5, 8},
	{"9-16", 9, 16},
	{"17-32", 17, 32},
	{"33+", 33, 0},
}

// EpochHistogram is the Q1 answer.
type EpochHistogram struct {
	Epochs  int      `json:"epochs"`
	Calls   int      `json:"calls"`
	Buckets []Bucket `json:"buckets"`

	Min  int     `json:"min"`
	Max  int     `json:"max"`
	Mean float64 `json:"mean"`
	P50  int     `json:"p50"`
	P90  int     `json:"p90"`

	// Singletons counts epochs of exactly one call. This is the number that
	// decides Phase 4: a prefix that serves one call can never repay a cache
	// write on any provider, at any price.
	Singletons   int     `json:"singletons"`
	SingletonPct float64 `json:"singleton_pct"`

	// NoPrefixCalls counts admitted calls with no cacheable head at all. They
	// are a separate failure mode from short epochs and need a different fix,
	// so they are reported separately rather than folded into the singletons.
	NoPrefixCalls int     `json:"no_prefix_calls"`
	NoPrefixPct   float64 `json:"no_prefix_pct"`

	// ReuseRatio is calls divided by epochs: how many calls an average prefix
	// serves.
	ReuseRatio float64 `json:"reuse_ratio"`

	// Provider break-even, and how many epochs cleared it. Reported per
	// provider because the threshold differs per provider; an aggregate would
	// average two different questions into one meaningless number.
	ByProvider []ProviderVerdict `json:"by_provider"`

	// ByMethod splits the same epochs by the call shape that opened them, so
	// a low median can be attributed rather than merely observed. See the
	// Method field on Epoch for why the two readings differ so much.
	ByMethod []MethodVerdict `json:"by_method"`
}

// ProviderVerdict is the amortization verdict for one provider.
type ProviderVerdict struct {
	Provider     string  `json:"provider"`
	Epochs       int     `json:"epochs"`
	Calls        int     `json:"calls"`
	BreakEven    float64 `json:"break_even_calls"`
	PayingEpochs int     `json:"paying_epochs"`
	PayingPct    float64 `json:"paying_pct"`
	// ExpiredEpochs counts epochs whose span exceeded the provider's cache TTL.
	// Those epochs look long enough to pay but cannot, because the entry was
	// evicted between calls — a trap that only shows up when you measure time
	// as well as count.
	ExpiredEpochs int `json:"expired_epochs"`
}

// MethodVerdict is the epoch distribution for one call shape.
//
// The split exists because "p50 = 1" answers two questions at once and they
// have opposite consequences. Epochs opened by a plain completion are single
// calls because there is nothing to loop over -- Phase 4 cannot lengthen them,
// and no amount of cache engineering will. Epochs opened by a tool-calling
// method are a turn's tool loop, and a singleton there means the loop ran one
// round or the prefix moved inside it, which is a caching problem Phase 4 is
// exactly the fix for.
//
// Reporting only the aggregate hides which of those the number is describing.
type MethodVerdict struct {
	Method     string  `json:"method"`
	Epochs     int     `json:"epochs"`
	Calls      int     `json:"calls"`
	Singletons int     `json:"singletons"`
	Mean       float64 `json:"mean"`
	MaxCalls   int     `json:"max_calls"`
}

// Histogram summarizes epochs into the Q1 distribution.
func Histogram(epochs []Epoch) EpochHistogram {
	h := EpochHistogram{Buckets: make([]Bucket, len(bucketBounds))}
	for i, b := range bucketBounds {
		h.Buckets[i] = Bucket{Label: b.label, Low: b.low, High: b.high}
	}
	if len(epochs) == 0 {
		return h
	}

	lengths := make([]int, 0, len(epochs))
	perProvider := make(map[string]*ProviderVerdict)
	var order []string
	perMethod := make(map[string]*MethodVerdict)
	var methodOrder []string

	for _, e := range epochs {
		h.Epochs++
		h.Calls += e.Calls
		lengths = append(lengths, e.Calls)

		if e.Calls == 1 {
			h.Singletons++
		}
		if e.Prefix == "" {
			h.NoPrefixCalls += e.Calls
		}

		for i, b := range bucketBounds {
			if e.Calls >= b.low && (b.high == 0 || e.Calls <= b.high) {
				h.Buckets[i].Count++
				break
			}
		}

		mv, ok := perMethod[e.Method]
		if !ok {
			mv = &MethodVerdict{Method: e.Method}
			perMethod[e.Method] = mv
			methodOrder = append(methodOrder, e.Method)
		}
		mv.Epochs++
		mv.Calls += e.Calls
		if e.Calls == 1 {
			mv.Singletons++
		}
		if e.Calls > mv.MaxCalls {
			mv.MaxCalls = e.Calls
		}

		pv, ok := perProvider[e.Provider]
		if !ok {
			econ := EconomicsFor(e.Provider)
			pv = &ProviderVerdict{Provider: e.Provider, BreakEven: econ.BreakEvenCalls()}
			perProvider[e.Provider] = pv
			order = append(order, e.Provider)
		}
		pv.Epochs++
		pv.Calls += e.Calls

		econ := EconomicsFor(e.Provider)
		expired := econ.TTL > 0 && e.Span() > econ.TTL
		if expired {
			pv.ExpiredEpochs++
		}
		// An epoch only pays if it clears the break-even *and* fits inside the
		// TTL. Counting an expired epoch as paying is how a cache strategy gets
		// approved on paper and loses money in production.
		if !expired && e.Prefix != "" && float64(e.Calls) >= pv.BreakEven {
			pv.PayingEpochs++
		}
	}

	sort.Ints(lengths)
	h.Min = lengths[0]
	h.Max = lengths[len(lengths)-1]
	h.Mean = float64(h.Calls) / float64(h.Epochs)
	h.P50 = percentileInts(lengths, 50)
	h.P90 = percentileInts(lengths, 90)
	h.SingletonPct = pct(h.Singletons, h.Epochs)
	h.NoPrefixPct = pct(h.NoPrefixCalls, h.Calls)
	h.ReuseRatio = h.Mean

	for i := range h.Buckets {
		h.Buckets[i].Pct = pct(h.Buckets[i].Count, h.Epochs)
	}

	sort.Strings(order)
	for _, p := range order {
		pv := perProvider[p]
		pv.PayingPct = pct(pv.PayingEpochs, pv.Epochs)
		h.ByProvider = append(h.ByProvider, *pv)
	}

	// Most calls first: the shape carrying the spend is the one whose epoch
	// length decides whether Phase 4 is worth building.
	sort.SliceStable(methodOrder, func(i, j int) bool {
		a, b := perMethod[methodOrder[i]], perMethod[methodOrder[j]]
		if a.Calls != b.Calls {
			return a.Calls > b.Calls
		}
		return a.Method < b.Method
	})
	for _, m := range methodOrder {
		mv := perMethod[m]
		mv.Mean = float64(mv.Calls) / float64(mv.Epochs)
		h.ByMethod = append(h.ByMethod, *mv)
	}

	return h
}

// percentileInts returns the p-th percentile of a sorted slice using nearest
// rank, which keeps the result an actual observed value rather than an
// interpolation between two call counts that never happened.
func percentileInts(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(float64(p) / 100 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}
