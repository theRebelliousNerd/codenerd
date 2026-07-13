package system

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"codenerd/internal/core"
)

func TestDefaultCortexPermissionEnvelopeRoutesToPolicyShard(t *testing.T) {
	workspace := t.TempDir()
	configs := defaultKernelShardConfigs(workspace)

	policyPredicates := map[string]bool{}
	for _, config := range configs {
		for _, predicate := range config.OwnedPredicates {
			if predicate == "pending_action" || predicate == "permitted_action" ||
				predicate == "permission_check_result" || predicate == "permitted" {
				if config.Domain != "policy" {
					t.Fatalf("authorization predicate %q owned by %q, want policy", predicate, config.Domain)
				}
				policyPredicates[predicate] = true
			}
		}
	}
	for _, predicate := range []string{"pending_action", "permitted_action", "permission_check_result", "permitted"} {
		if !policyPredicates[predicate] {
			t.Fatalf("policy shard does not own authorization predicate %q", predicate)
		}
	}

	cortex := core.NewCortexKernel("cortex")
	for _, config := range configs {
		if config.Domain != "policy" && config.Domain != "cortex" {
			continue
		}
		shard, err := core.NewKernelShard(config)
		if err != nil {
			t.Fatalf("NewKernelShard(%q) error = %v", config.Domain, err)
		}
		if err := cortex.RegisterShard(shard); err != nil {
			t.Fatalf("RegisterShard(%q) error = %v", config.Domain, err)
		}
	}
	if err := cortex.Evaluate(); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	payload := map[string]any{"encoding": "utf-8"}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(payload) error = %v", err)
	}
	if err := cortex.Assert(core.Fact{
		Predicate: "pending_action",
		Args: []any{
			"cortex-action-1",
			core.MangleAtom("/read_file"),
			"hello.txt",
			string(payloadJSON),
			time.Now().Unix(),
		},
	}); err != nil {
		t.Fatalf("Assert(pending_action) error = %v", err)
	}

	permitted, err := cortex.Query("permitted")
	if err != nil {
		t.Fatalf("Query(permitted) error = %v", err)
	}
	if !slices.ContainsFunc(permitted, func(fact core.Fact) bool {
		return len(fact.Args) == 3 &&
			fmtSprint(fact.Args[0]) == "/read_file" &&
			fmtSprint(fact.Args[1]) == "hello.txt" &&
			fmtSprint(fact.Args[2]) == string(payloadJSON)
	}) {
		t.Fatalf("exact permitted envelope not derived: %v", permitted)
	}

	virtualStore := core.NewVirtualStoreWithConfig(nil, core.VirtualStoreConfig{WorkingDir: workspace})
	virtualStore.SetKernel(cortex)
	if !virtualStore.CheckKernelPermitted("read_file", "hello.txt", payload) {
		t.Fatal("exact Cortex authorization envelope was denied")
	}
	if virtualStore.CheckKernelPermitted("read_file", "other.txt", payload) {
		t.Fatal("mismatched target was permitted")
	}
	if virtualStore.CheckKernelPermitted("read_file", "hello.txt", map[string]any{"encoding": "ascii"}) {
		t.Fatal("mismatched payload was permitted")
	}
}

func fmtSprint(value any) string {
	if atom, ok := value.(core.MangleAtom); ok {
		return string(atom)
	}
	return value.(string)
}
