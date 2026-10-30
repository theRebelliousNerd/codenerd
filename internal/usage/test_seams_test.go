package usage

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// EstimateCost returns the estimated USD cost for a model and token counts, and
// whether the model was priced at all.
func EstimateCost(model string, input, output int64) (float64, bool) {
	cost, _, ok := EstimateCostVersioned(model, input, output, 0, 0)
	return cost, ok
}

// LookupPrice is LookupPriceVersion without the version.
func LookupPrice(model string) (Price, bool) {
	p, _, ok := LookupPriceVersion(model)
	return p, ok
}

// Cost returns the estimated USD cost of the given token counts.
func (p Price) Cost(input, output int64) float64 {
	return p.CostDetailed(input, output, 0, 0)
}
