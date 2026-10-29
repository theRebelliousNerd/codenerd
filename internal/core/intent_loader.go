package core

import (
	"sync"

	"codenerd/internal/logging"
)

// embeddedIntentFacts parses the embedded intent corpus once per process. The
// files are compiled into the binary, so the parse can never go stale; every
// kernel used to repeat it under the process-wide parser lock (mangle
// ParseUnit), so a domain cortex's seven kernels booting at once parsed it
// seven times in a row. Stored facts are immutable, so kernels share them.
var embeddedIntentFacts = sync.OnceValue(func() []Fact {
	allowed := defaultIntentFactPredicates()
	var out []Fact
	for _, file := range DefaultIntentSchemaFiles() {
		data, err := coreLogic.ReadFile("defaults/" + file)
		if err != nil {
			logging.KernelDebug("Intent corpus file not found: %s (%v)", file, err)
			continue
		}
		facts, err := ParseFactsFromString(string(data))
		if err != nil {
			logging.Get(logging.CategoryKernel).Warn("Failed to parse intent facts from %s: %v", file, err)
			continue
		}
		for _, fact := range facts {
			if _, ok := allowed[fact.Predicate]; ok {
				out = append(out, fact)
			}
		}
	}
	return out
})

func (k *RealKernel) loadEmbeddedIntentFacts() {
	facts := embeddedIntentFacts()
	k.bootFacts = append(k.bootFacts, facts...)
	if len(facts) > 0 {
		logging.Kernel("Loaded %d intent corpus facts into boot EDB", len(facts))
	}
}
