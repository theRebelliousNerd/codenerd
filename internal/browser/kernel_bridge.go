package browser

import (
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// KernelReader is the read side of a live Cortex kernel, matching the subset of
// types.Kernel the browser needs. Declaring it structurally keeps the browser
// package free of a dependency on the kernel's full surface.
type KernelReader interface {
	Query(predicate string) ([]types.Fact, error)
}

// KernelRetractor removes exact facts. types.Kernel satisfies it.
type KernelRetractor interface {
	RetractExactFactsBatch(facts []types.Fact) error
}

// kernelFactQuerier adapts a live kernel to FactQuerier.
//
// The Cortex hands the browser a write-only sink, which is why is_honeypot
// verdicts were unreachable from the manager: it could assert element evidence
// but never read the conclusion. One call to
// SetFactQuerier(NewKernelFactQuerier(kernel)) closes that loop.
type kernelFactQuerier struct {
	kernel KernelReader
}

// NewKernelFactQuerier returns a FactQuerier backed by a live kernel.
func NewKernelFactQuerier(kernel KernelReader) FactQuerier {
	if kernel == nil {
		return nil
	}
	return kernelFactQuerier{kernel: kernel}
}

func (q kernelFactQuerier) QueryFacts(predicate string, args ...string) []mangle.Fact {
	facts, err := q.kernel.Query(predicate)
	if err != nil {
		logging.BrowserDebug("kernel query %s failed: %v", predicate, err)
		return nil
	}
	results := make([]mangle.Fact, 0, len(facts))
	for _, fact := range facts {
		if !factMatchesArgs(fact.Args, args) {
			continue
		}
		results = append(results, mangle.Fact{
			Predicate: predicate,
			Args:      append([]any(nil), fact.Args...),
		})
	}
	return results
}

// factMatchesArgs applies the same prefix filter mangle.Engine.QueryFacts uses:
// an empty pattern slot matches anything, and a name constant compares equal to
// its slash-free spelling.
func factMatchesArgs(factArgs []any, pattern []string) bool {
	for i, want := range pattern {
		if want == "" {
			continue
		}
		if i >= len(factArgs) {
			return false
		}
		stored := types.ExtractString(factArgs[i])
		if stored == want || stored == "/"+want || strings.TrimPrefix(stored, "/") == want {
			continue
		}
		return false
	}
	return true
}

// kernelFactRetractor adapts a live kernel to FactRetractor.
type kernelFactRetractor struct {
	kernel KernelRetractor
}

// NewKernelFactRetractor returns a FactRetractor backed by a live kernel.
func NewKernelFactRetractor(kernel KernelRetractor) FactRetractor {
	if kernel == nil {
		return nil
	}
	return kernelFactRetractor{kernel: kernel}
}

func (r kernelFactRetractor) RetractFacts(facts []mangle.Fact) error {
	converted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, types.Fact{
			Predicate: fact.Predicate,
			Args:      append([]any(nil), fact.Args...),
		})
	}
	return r.kernel.RetractExactFactsBatch(converted)
}
