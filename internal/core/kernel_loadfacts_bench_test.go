package core

// buildSpecShapedLoadFacts returns 150k facts shaped like the spec-pairing
// corpus (see specGenFacts). It backs BenchmarkKernelLoadFacts in
// performance_bench_test.go.
func buildSpecShapedLoadFacts() []Fact {
	links, elements, sections, confirmed := specGenFacts(0, 37500, 37500, 37500)
	facts := make([]Fact, 0, len(links)+len(elements)+len(sections)+len(confirmed))
	facts = append(facts, links...)
	facts = append(facts, elements...)
	facts = append(facts, sections...)
	facts = append(facts, confirmed...)
	return facts
}
