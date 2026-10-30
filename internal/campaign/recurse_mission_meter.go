package campaign

import (
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"math"
	"sync"
)

type missionMeter struct {
	mu      sync.Mutex
	spend   int64
	unknown bool
	reports int
	byPrice map[string]int64
}

func (b *missionMeter) UsageUnknown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unknown = true
}

func (b *missionMeter) Observed(r usage.Report) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reports++
	u := r.Usage
	cost, version, priced := usage.EstimateCostVersioned(r.Model, int64(u.InputTokens), int64(u.OutputTokens), int64(u.CachedContentTokens), int64(u.CacheWriteTokens))
	if !priced || u.Partial || !u.Has(types.UsageTokens) || u.InputTokens < 0 || u.OutputTokens < 0 || cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) || cost*usage.MicroUSDPerUSD >= float64(math.MaxInt64-b.spend) {
		b.unknown = true
		return
	}
	micros := int64(math.Ceil(cost * usage.MicroUSDPerUSD))
	b.spend += micros
	if b.byPrice == nil {
		b.byPrice = map[string]int64{}
	}
	b.byPrice[version] += micros
}

func (b *missionMeter) snapshot() (int64, bool, int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spend, b.unknown, b.reports
}
func (b *missionMeter) phaseEnded(before int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.reports == before {
		b.unknown = true
	}
}
func (b *missionMeter) prices() map[string]int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[string]int64, len(b.byPrice))
	for key, value := range b.byPrice {
		out[key] = value
	}
	return out
}
