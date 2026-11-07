package system

import (
	"testing"

	"codenerd/internal/core"
)

// Two boots analyse the same corpus and manifests, so the second reuses the
// first's map; a change to any input (a manifest's ownership here) builds a
// new one instead of serving a stale analysis.
func TestBuildKernelDerivationMap_SharesOneBuildPerInputSet(t *testing.T) {
	configs := defaultKernelShardConfigs(t.TempDir())
	first, err := buildKernelDerivationMap(configs)
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildKernelDerivationMap(defaultKernelShardConfigs(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("a second boot rebuilt the derivation map for unchanged inputs")
	}

	changed := append([]core.KernelShardConfig(nil), configs...)
	moved := changed[0]
	moved.OwnedPredicates = append([]string(nil), moved.OwnedPredicates...)
	moved.OwnedPredicates = append(moved.OwnedPredicates, "derivation_cache_probe_predicate")
	changed[0] = moved
	third, err := buildKernelDerivationMap(changed)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Fatal("a changed ownership table was served the old derivation map")
	}
	fourth, err := buildKernelDerivationMap(configs)
	if err != nil {
		t.Fatal(err)
	}
	if fourth == third {
		t.Fatal("the original inputs were served the changed inputs' map")
	}
}
