package sessionlog

import "codenerd/internal/usage"

// PriceUsage prices u at the usage price table's rates for model, filling
// u.Price. An unpriced model leaves Priced false: unknown cost, not free.
func PriceUsage(model string, u *Usage) {
	if u == nil {
		return
	}
	usd, version, ok := usage.EstimateCostVersioned(model,
		int64(u.InputTokens), int64(u.OutputTokens), int64(u.CacheReadTokens), int64(u.CacheWriteTokens))
	u.Price = &Price{USD: usd, Version: version, Priced: ok}
}
