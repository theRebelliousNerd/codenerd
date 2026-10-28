package usage

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
)

// Price is the USD cost per one million tokens for a model family.
//
// Prices are list prices and change often. They exist to give an operator an
// order-of-magnitude answer to "what did this session cost", not an invoice.
// TokenCounts.Cost is always an estimate; the provider's billing is the record.
type Price struct {
	InputPerMTok  float64
	OutputPerMTok float64
	// CacheReadPerMTok/CacheWritePerMTok price the cached sub-counts of
	// input. Zero means "no separate cache rate on record": those tokens
	// are then priced at the plain input rate, which overestimates rather
	// than silently dropping spend.
	CacheReadPerMTok  float64
	CacheWritePerMTok float64
}

// CostDetailed prices input split into uncached, cache-read and cache-write
// parts. cacheRead/cacheWrite are sub-counts of input; any excess over
// input is clamped so a provider miscount can never price negative tokens.
func (p Price) CostDetailed(input, output, cacheRead, cacheWrite int64) float64 {
	if cacheRead < 0 {
		cacheRead = 0
	}
	if cacheWrite < 0 {
		cacheWrite = 0
	}
	if cacheRead+cacheWrite > input {
		cacheWrite = max(input-cacheRead, 0)
		if cacheRead > input {
			cacheRead = input
			cacheWrite = 0
		}
	}
	readRate := p.CacheReadPerMTok
	if readRate == 0 {
		readRate = p.InputPerMTok
	}
	writeRate := p.CacheWritePerMTok
	if writeRate == 0 {
		writeRate = p.InputPerMTok
	}
	uncached := input - cacheRead - cacheWrite
	return (float64(uncached)/1e6)*p.InputPerMTok +
		(float64(cacheRead)/1e6)*readRate +
		(float64(cacheWrite)/1e6)*writeRate +
		(float64(output)/1e6)*p.OutputPerMTok
}

// priceTable maps a lowercase model-name prefix to its price. Lookup picks the
// longest matching prefix, so "claude-opus-4-5-20260101" resolves through
// "claude-opus-4-5" rather than the shorter "claude-opus".
//
// Unknown models resolve to no price and contribute zero cost, which is why
// Stats reports CostKnown alongside Cost: a $0.00 total means "nothing priced",
// not "nothing spent".
var priceTable = map[string]Price{
	// Anthropic (Haiku 5.5's rate holds for prompts up to 100K tokens; above
	// that it is $0.50/$2.50, which this table does not model)
	"claude-fable-5":   {InputPerMTok: 10, OutputPerMTok: 50.00, CacheReadPerMTok: 1, CacheWritePerMTok: 12.5},
	"claude-opus-5-5":  {InputPerMTok: 4, OutputPerMTok: 20.00, CacheReadPerMTok: 0.4, CacheWritePerMTok: 5},
	"claude-opus-5":    {InputPerMTok: 5, OutputPerMTok: 25.00, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25},
	"claude-opus-4-8":  {InputPerMTok: 5, OutputPerMTok: 25.00, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25},
	"claude-opus-4-7":  {InputPerMTok: 5, OutputPerMTok: 25.00, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25},
	"claude-opus-4-6":  {InputPerMTok: 5, OutputPerMTok: 25.00, CacheReadPerMTok: 0.5, CacheWritePerMTok: 6.25},
	"claude-sonnet-5":  {InputPerMTok: 2, OutputPerMTok: 10.00, CacheReadPerMTok: 0.2, CacheWritePerMTok: 2.5},
	"claude-haiku-5-5": {InputPerMTok: 0.1, OutputPerMTok: 0.50, CacheReadPerMTok: 0.01, CacheWritePerMTok: 0.125},
	"claude-opus-4":    {InputPerMTok: 15, OutputPerMTok: 75.00, CacheReadPerMTok: 1.5, CacheWritePerMTok: 18.75},
	"claude-opus":      {InputPerMTok: 15, OutputPerMTok: 75.00, CacheReadPerMTok: 1.5, CacheWritePerMTok: 18.75},
	"claude-sonnet-4":  {InputPerMTok: 3, OutputPerMTok: 15.00, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75},
	"claude-sonnet":    {InputPerMTok: 3, OutputPerMTok: 15.00, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75},
	"claude-haiku-4":   {InputPerMTok: 1, OutputPerMTok: 5.00, CacheReadPerMTok: 0.1, CacheWritePerMTok: 1.25},
	"claude-haiku":     {InputPerMTok: 0.8, OutputPerMTok: 4.00, CacheReadPerMTok: 0.08, CacheWritePerMTok: 1},
	"claude-3-5-haiku": {InputPerMTok: 0.8, OutputPerMTok: 4.00, CacheReadPerMTok: 0.08, CacheWritePerMTok: 1},
	"claude-3-opus":    {InputPerMTok: 15, OutputPerMTok: 75.00, CacheReadPerMTok: 1.5, CacheWritePerMTok: 18.75},
	"claude-3-sonnet":  {InputPerMTok: 3, OutputPerMTok: 15.00, CacheReadPerMTok: 0.3, CacheWritePerMTok: 3.75},
	"claude-3-haiku":   {InputPerMTok: 0.25, OutputPerMTok: 1.25, CacheReadPerMTok: 0.025, CacheWritePerMTok: 0.3125},

	// OpenAI
	"gpt-4o-mini": {InputPerMTok: 0.15, OutputPerMTok: 0.60},
	"gpt-4o":      {InputPerMTok: 2.50, OutputPerMTok: 10.00},
	"gpt-4-turbo": {InputPerMTok: 10.00, OutputPerMTok: 30.00},
	"gpt-4":       {InputPerMTok: 30.00, OutputPerMTok: 60.00},
	"gpt-3.5":     {InputPerMTok: 0.50, OutputPerMTok: 1.50},
	"o1-mini":     {InputPerMTok: 3.00, OutputPerMTok: 12.00},
	"o1":          {InputPerMTok: 15.00, OutputPerMTok: 60.00},
	"o3-mini":     {InputPerMTok: 1.10, OutputPerMTok: 4.40},

	// Google
	"gemini-2.5-pro":   {InputPerMTok: 1.25, OutputPerMTok: 10.00},
	"gemini-2.5-flash": {InputPerMTok: 0.30, OutputPerMTok: 2.50},
	"gemini-2.0-flash": {InputPerMTok: 0.10, OutputPerMTok: 0.40},
	"gemini-1.5-pro":   {InputPerMTok: 1.25, OutputPerMTok: 5.00},
	"gemini-1.5-flash": {InputPerMTok: 0.075, OutputPerMTok: 0.30},

	// Meta. The contributor tier's cached input is $0.002/MTok (recorded as
	// CacheReadPerMTok below). The standard tier (muse-spark-1.3) is
	// unpriced: no source for its rate is on record.
	"muse-spark-1.3-contributor": {InputPerMTok: 0.10, OutputPerMTok: 0.20, CacheReadPerMTok: 0.002},

	// DeepSeek / Z.AI / open-weight hosted
	"deepseek-chat":     {InputPerMTok: 0.27, OutputPerMTok: 1.10},
	"deepseek-reasoner": {InputPerMTok: 0.55, OutputPerMTok: 2.19},
	"glm-4.6":           {InputPerMTok: 0.60, OutputPerMTok: 2.20},
	"glm-4.5":           {InputPerMTok: 0.60, OutputPerMTok: 2.20},
	"glm-4":             {InputPerMTok: 0.50, OutputPerMTok: 1.50},

	// Embeddings (output side is normally zero, but keep the shape uniform)
	"text-embedding-3-large": {InputPerMTok: 0.13},
	"text-embedding-3-small": {InputPerMTok: 0.02},
}

var (
	priceMu       sync.RWMutex
	priceKeysMemo []string
)

// priceKeys returns table prefixes sorted longest-first so LookupPrice can stop
// at the first match and still get the most specific one.
func priceKeys() []string {
	priceMu.RLock()
	if priceKeysMemo != nil {
		defer priceMu.RUnlock()
		return priceKeysMemo
	}
	priceMu.RUnlock()

	priceMu.Lock()
	defer priceMu.Unlock()
	if priceKeysMemo != nil {
		return priceKeysMemo
	}
	keys := make([]string, 0, len(priceTable))
	for k := range priceTable {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	priceKeysMemo = keys
	return priceKeysMemo
}

// LookupPrice resolves a model name to its price. The bool reports whether the
// model was found; callers must not treat a zero Price as "free".
//
// Matching is prefix-based on the lowercased name after stripping any provider
// route prefix ("openai/gpt-4o", "anthropic.claude-sonnet-4" → "gpt-4o",
// "claude-sonnet-4"), because engines name the same model several ways.
func LookupPrice(model string) (Price, bool) {
	p, _, ok := LookupPriceVersion(model)
	return p, ok
}

// LookupPriceVersion is LookupPrice plus the version of the entry that priced
// the model: the table key it matched and a digest of its five rates. Stamped
// on each usage event, it lets a cost export say which rates an estimate used
// -- the table is list price and changes, and RegisterPrice overrides it, so a
// dollar figure without its rates cannot be reproduced or compared.
func LookupPriceVersion(model string) (Price, string, bool) {
	name := normalizeModelName(model)
	if name == "" {
		return Price{}, "", false
	}
	for _, k := range priceKeys() {
		if strings.HasPrefix(name, k) {
			priceMu.RLock()
			p := priceTable[k]
			priceMu.RUnlock()
			return p, p.version(k), true
		}
	}
	return Price{}, "", false
}

// version names a price entry: its table key and a digest of every rate.
func (p Price) version(key string) string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%g/%g/%g/%g", p.InputPerMTok, p.OutputPerMTok, p.CacheReadPerMTok, p.CacheWritePerMTok)
	return fmt.Sprintf("%s@%08x", key, h.Sum32())
}

// RegisterPrice adds or overrides a price entry at runtime, for operators whose
// negotiated rates differ from list price. The key is matched as a prefix.
func RegisterPrice(modelPrefix string, p Price) {
	key := normalizeModelName(modelPrefix)
	if key == "" {
		return
	}
	priceMu.Lock()
	priceTable[key] = p
	priceKeysMemo = nil // force the sorted key list to be rebuilt
	priceMu.Unlock()
}

// normalizeModelName lowercases and strips provider routing prefixes and any
// deployment suffix noise so table lookup sees a bare model family name.
func normalizeModelName(model string) string {
	name := strings.ToLower(strings.TrimSpace(model))
	if name == "" {
		return ""
	}
	// "openai/gpt-4o", "vertex_ai/gemini-2.5-pro" → take the last path segment.
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	// "anthropic.claude-sonnet-4-v1:0" → strip a leading provider dot-prefix.
	for _, prov := range []string{"anthropic.", "openai.", "google.", "bedrock."} {
		name = strings.TrimPrefix(name, prov)
	}
	// "...-v1:0" bedrock-style version suffix.
	if i := strings.Index(name, ":"); i > 0 {
		name = name[:i]
	}
	return name
}

// EstimateCostDetailed is EstimateCost with the cache sub-counts of input.
func EstimateCostDetailed(model string, input, output, cacheRead, cacheWrite int64) (float64, bool) {
	cost, _, ok := EstimateCostVersioned(model, input, output, cacheRead, cacheWrite)
	return cost, ok
}

// EstimateCostVersioned is EstimateCostDetailed plus the price entry's version
// ("" when the model is unpriced, which is unpriced spend and not free spend).
func EstimateCostVersioned(model string, input, output, cacheRead, cacheWrite int64) (float64, string, bool) {
	p, version, ok := LookupPriceVersion(model)
	if !ok {
		return 0, "", false
	}
	return p.CostDetailed(input, output, cacheRead, cacheWrite), version, true
}
