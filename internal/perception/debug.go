package perception

// DebugTaxonomy exposes the internal classification logic for verification tools.
// It returns the selected verb, category, confidence, and shard type.
func DebugTaxonomy(input string) (string, string, float64, string) {
	return matchVerbFromCorpus(input)
}
