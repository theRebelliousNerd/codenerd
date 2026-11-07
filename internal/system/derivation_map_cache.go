package system

import (
	"crypto/sha256"
	"sort"
	"sync"

	"codenerd/internal/core"
)

// The derivation map is a pure function of the embedded corpus text and the
// ownership, shared-predicate and consumer-route tables the shard manifests
// declare. Booting a Cortex used to re-parse and re-run the whole static
// analysis (about a quarter of a boot's CPU) for inputs that had not changed
// since the last boot in the process. The map is only read after it is built
// (CortexKernel routes queries through it), so one build per distinct input
// set is shared by every boot.
var derivationMapCache struct {
	mu  sync.Mutex
	key [sha256.Size]byte
	dm  *core.DerivationMap
}

// derivationMapKey fingerprints every input of the analysis.
func derivationMapKey(corpus string, programFacts map[string]struct{}, owners map[string]string, shared map[string]struct{}, routes []core.PredicateConsumerRoute, catchAll string) [sha256.Size]byte {
	h := sha256.New()
	field := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	set := func(m map[string]struct{}) {
		names := make([]string, 0, len(m))
		for name := range m {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			field(name)
		}
		field("")
	}
	field(corpus)
	set(programFacts)
	set(shared)
	ownerNames := make([]string, 0, len(owners))
	for name := range owners {
		ownerNames = append(ownerNames, name)
	}
	sort.Strings(ownerNames)
	for _, name := range ownerNames {
		field(name)
		field(owners[name])
	}
	field("")
	for _, route := range routes {
		field(route.Predicate)
		field(route.Consumer)
	}
	field(catchAll)
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))
	return key
}

// cachedDerivationMap returns the map for these inputs, building it only when
// they differ from the last build's.
func cachedDerivationMap(corpus string, programFacts map[string]struct{}, owners map[string]string, shared map[string]struct{}, routes []core.PredicateConsumerRoute, catchAll string) (*core.DerivationMap, error) {
	key := derivationMapKey(corpus, programFacts, owners, shared, routes, catchAll)
	derivationMapCache.mu.Lock()
	defer derivationMapCache.mu.Unlock()
	if derivationMapCache.dm != nil && derivationMapCache.key == key {
		return derivationMapCache.dm, nil
	}
	dm, err := core.BuildDerivationMapWithConsumerRoutes(corpus, programFacts, owners, shared, routes, catchAll)
	if err != nil {
		return nil, err
	}
	derivationMapCache.key, derivationMapCache.dm = key, dm
	return dm, nil
}
