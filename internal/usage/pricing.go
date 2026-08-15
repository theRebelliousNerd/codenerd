package usage

import (
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
}

// Cost returns the estimated USD cost of the given token counts.
func (p Price) Cost(input, output int64) float64 {
	return (float64(input)/1e6)*p.InputPerMTok + (float64(output)/1e6)*p.OutputPerMTok
}

// priceTable maps a lowercase model-name prefix to its price. Lookup picks the
// longest matching prefix, so "claude-opus-4-5-20260101" resolves through
// "claude-opus-4-5" rather than the shorter "claude-opus".
//
// Unknown models resolve to no price and contribute zero cost, which is why
// Stats reports CostKnown alongside Cost: a $0.00 total means "nothing priced",
// not "nothing spent".
var priceTable = map[string]Price{
	// Anthropic
	"claude-opus-4":    {InputPerMTok: 15.00, OutputPerMTok: 75.00},
	"claude-opus":      {InputPerMTok: 15.00, OutputPerMTok: 75.00},
	"claude-sonnet-4":  {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-sonnet":    {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-haiku-4":   {InputPerMTok: 1.00, OutputPerMTok: 5.00},
	"claude-haiku":     {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	"claude-3-5-haiku": {InputPerMTok: 0.80, OutputPerMTok: 4.00},
	"claude-3-opus":    {InputPerMTok: 15.00, OutputPerMTok: 75.00},
	"claude-3-sonnet":  {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-3-haiku":   {InputPerMTok: 0.25, OutputPerMTok: 1.25},

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
	name := normalizeModelName(model)
	if name == "" {
		return Price{}, false
	}
	for _, k := range priceKeys() {
		if strings.HasPrefix(name, k) {
			priceMu.RLock()
			p := priceTable[k]
			priceMu.RUnlock()
			return p, true
		}
	}
	return Price{}, false
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

// EstimateCost returns the estimated USD cost for a model and token counts, and
// whether the model was priced at all.
func EstimateCost(model string, input, output int64) (float64, bool) {
	p, ok := LookupPrice(model)
	if !ok {
		return 0, false
	}
	return p.Cost(input, output), true
}
