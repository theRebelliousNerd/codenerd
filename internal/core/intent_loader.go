package core

import "codenerd/internal/logging"

func (k *RealKernel) loadEmbeddedIntentFacts() {
	allowed := defaultIntentFactPredicates()
	intentFiles := DefaultIntentSchemaFiles()
	loadedFacts := 0

	for _, file := range intentFiles {
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
			if _, ok := allowed[fact.Predicate]; !ok {
				continue
			}
			k.bootFacts = append(k.bootFacts, fact)
			loadedFacts++
		}
	}

	if loadedFacts > 0 {
		logging.Kernel("Loaded %d intent corpus facts into boot EDB", loadedFacts)
	}
}
