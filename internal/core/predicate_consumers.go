package core

import (
	"fmt"
	"sort"
	"strings"
)

// PredicateConsumerRoute names one shard that receives a bounded copy of a
// predicate's EDB facts in addition to the canonical owner. The canonical
// owner remains the source of truth; consumer copies exist only so a local
// rule can join its complete body.
type PredicateConsumerRoute struct {
	Predicate string
	Consumer  string
}

func normalizedConsumerRouteIndex(routes []PredicateConsumerRoute) (map[string]map[string]struct{}, error) {
	indexed := make(map[string]map[string]struct{}, len(routes))
	seen := make(map[PredicateConsumerRoute]struct{}, len(routes))
	for _, route := range routes {
		predicate := strings.TrimSpace(route.Predicate)
		consumer := strings.TrimSpace(route.Consumer)
		if predicate == "" || barePredicate(predicate) != predicate {
			return nil, fmt.Errorf("predicate consumer route requires a bare predicate name, got %q", route.Predicate)
		}
		if consumer == "" {
			return nil, fmt.Errorf("predicate consumer route for %q has an empty consumer", predicate)
		}
		normalized := PredicateConsumerRoute{Predicate: predicate, Consumer: consumer}
		if _, duplicate := seen[normalized]; duplicate {
			return nil, fmt.Errorf("duplicate predicate consumer route %q -> %q", predicate, consumer)
		}
		seen[normalized] = struct{}{}
		if indexed[predicate] == nil {
			indexed[predicate] = make(map[string]struct{})
		}
		indexed[predicate][consumer] = struct{}{}
	}
	return indexed, nil
}

// validateConsumerRouteIndex ensures that every replica has a canonical
// source and a real, distinct consumer. An unowned fact's canonical source is
// the catch-all shard, matching CortexKernel.routeToShard.
func validateConsumerRouteIndex(
	indexed map[string]map[string]struct{},
	owners map[string]string,
	shared map[string]struct{},
	domains map[string]struct{},
	catchAll string,
) error {
	for predicate, consumers := range indexed {
		owner := owners[predicate]
		if owner == "" {
			owner = catchAll
		}
		if owner == "" {
			return fmt.Errorf("predicate consumer route for %q has no canonical owner or catch-all", predicate)
		}
		if _, ok := domains[owner]; !ok {
			return fmt.Errorf("predicate consumer route for %q names missing canonical owner %q", predicate, owner)
		}
		if _, isShared := shared[predicate]; isShared {
			return fmt.Errorf("predicate %q cannot be both shared and consumer-routed", predicate)
		}
		for consumer := range consumers {
			if _, ok := domains[consumer]; !ok {
				return fmt.Errorf("predicate consumer route for %q names missing consumer %q", predicate, consumer)
			}
			if consumer == owner {
				return fmt.Errorf("predicate consumer route for %q duplicates its canonical owner %q", predicate, owner)
			}
		}
	}
	return nil
}

func routeDomains(owners map[string]string, catchAll string) map[string]struct{} {
	domains := make(map[string]struct{}, len(owners)+1)
	for _, owner := range owners {
		if owner != "" {
			domains[owner] = struct{}{}
		}
	}
	if catchAll != "" {
		domains[catchAll] = struct{}{}
	}
	return domains
}

func consumerReplicaPresence(indexed map[string]map[string]struct{}) map[string]Presence {
	out := make(map[string]Presence, len(indexed))
	for predicate, consumers := range indexed {
		copy := make(map[string]struct{}, len(consumers))
		for consumer := range consumers {
			copy[consumer] = struct{}{}
		}
		out[predicate] = Presence{Shards: copy}
	}
	return out
}

func runtimeRouteDomains(c *CortexKernel) (map[string]string, map[string]struct{}, map[string]*KernelShard) {
	owners := make(map[string]string, len(c.predicateOwner))
	for predicate, owner := range c.predicateOwner {
		owners[predicate] = owner
	}
	domains := make(map[string]struct{}, len(c.shards))
	shards := make(map[string]*KernelShard, len(c.shards))
	for domain, shard := range c.shards {
		domains[domain] = struct{}{}
		shards[domain] = shard
	}
	return owners, domains, shards
}

func resolveConsumerTargets(indexed map[string]map[string]struct{}, owners map[string]string, shards map[string]*KernelShard, catchAll string) map[string][]*KernelShard {
	targets := make(map[string][]*KernelShard, len(indexed))
	for predicate, consumers := range indexed {
		owner := owners[predicate]
		if owner == "" {
			owner = catchAll
		}
		if shard := shards[owner]; shard != nil {
			targets[predicate] = append(targets[predicate], shard)
		}
		consumerDomains := make([]string, 0, len(consumers))
		for domain := range consumers {
			consumerDomains = append(consumerDomains, domain)
		}
		sort.Strings(consumerDomains)
		for _, domain := range consumerDomains {
			if shard := shards[domain]; shard != nil {
				targets[predicate] = append(targets[predicate], shard)
			}
		}
	}
	return targets
}

func copyConsumerRouteIndex(src map[string]map[string]struct{}) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(src))
	for predicate, consumers := range src {
		copy := make(map[string]struct{}, len(consumers))
		for consumer := range consumers {
			copy[consumer] = struct{}{}
		}
		out[predicate] = copy
	}
	return out
}

// SetPredicateConsumerRoutes installs explicit EDB replica destinations.
// Configure routes after registering every shard and before the first fact
// mutation, then pass the same routes to BuildDerivationMapWithConsumerRoutes.
func (c *CortexKernel) SetPredicateConsumerRoutes(routes []PredicateConsumerRoute) error {
	indexed, err := normalizedConsumerRouteIndex(routes)
	if err != nil {
		return err
	}

	c.mu.Lock()
	owners, domains, shards := runtimeRouteDomains(c)
	if err := validateConsumerRouteIndex(indexed, owners, c.sharedPredicates, domains, c.cortexDomain); err != nil {
		c.mu.Unlock()
		return err
	}
	c.predicateConsumers = indexed
	router := c.factRouter
	routerTargets := resolveConsumerTargets(indexed, owners, shards, c.cortexDomain)
	c.mu.Unlock()

	if router != nil {
		router.setConsumerTargets(routerTargets)
	}
	return nil
}
