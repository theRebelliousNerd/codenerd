package core

import (
	"path"
	"slices"
	"sort"
	"strings"
)

// Stable policy-set identifiers used by JIT runtime configuration. They are
// identities, not filenames; DefaultAgentPolicySetFiles resolves them against
// the embedded policy inventory.
const (
	PolicySetBase          = "base"
	PolicySetCoder         = "coder"
	PolicySetTester        = "tester"
	PolicySetReviewer      = "reviewer"
	PolicySetResearcher    = "researcher"
	PolicySetNemesis       = "nemesis"
	PolicySetToolGenerator = "tool_generator"
)

// defaultCorePolicyModules are non-schema root modules loaded into the policy
// builder after defaults/policy/*.mg. Keep kernel boot and the public inventory
// on this single list.
var defaultCorePolicyModules = []string{
	"doc_taxonomy.mg",
	"topology_planner.mg",
	"build_topology.mg",
	"campaign_rules.mg",
	"selection_policy.mg",
	"taxonomy.mg",
	"inference.mg",
	"jit_compiler.mg",
	"reviewer.mg",
	"tester.mg",
	"go_safety.mg",
	"benchmarks.mg",
}

var defaultAgentPolicySetExtras = map[string][]string{
	PolicySetBase: {},
	PolicySetCoder: {
		"policy/coder_classification.mg",
		"policy/coder_language.mg",
		"policy/coder_impact.mg",
		"policy/coder_safety.mg",
		"policy/coder_diagnostics.mg",
		"policy/coder_workflow.mg",
		"policy/coder_context.mg",
		"policy/coder_tdd.mg",
		"policy/coder_quality.mg",
		"policy/coder_learning.mg",
		"policy/coder_campaign.mg",
		"policy/coder_observability.mg",
		"policy/coder_patterns.mg",
	},
	PolicySetTester: {
		"tester.mg",
	},
	PolicySetReviewer: {
		"reviewer.mg",
	},
	PolicySetResearcher: {
		"policy/knowledge.mg",
		"policy/delegation.mg",
	},
	PolicySetNemesis: {
		"policy/browser_honeypot.mg",
		"policy/codedom_safety.mg",
	},
	PolicySetToolGenerator: {
		"policy/capabilities.mg",
		"policy/tool_routing.mg",
	},
}

var defaultAgentBasePolicyFiles = []string{
	"policy/constitution.mg",
	"policy/validation.mg",
}

// DefaultCorePolicyModules returns a caller-owned copy of the ordered root
// modules loaded into the default policy builder.
func DefaultCorePolicyModules() []string {
	return slices.Clone(defaultCorePolicyModules)
}

// DefaultPolicyFiles returns every embedded file that is loaded as default
// policy: defaults/policy/*.mg followed by the root policy modules.
func DefaultPolicyFiles() []string {
	files := make([]string, 0, 96)
	if entries, err := coreLogic.ReadDir("defaults/policy"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mg") {
				files = append(files, "policy/"+entry.Name())
			}
		}
	}
	sort.Strings(files)
	files = append(files, defaultCorePolicyModules...)
	return files
}

// IsDefaultPolicyFile reports whether ref is a canonical embedded policy path.
// Aliases such as base.mg and coder.mg intentionally fail: they do not exist in
// the live embedded inventory.
func IsDefaultPolicyFile(ref string) bool {
	if ref == "" || ref != strings.TrimSpace(ref) || strings.Contains(ref, "\\") {
		return false
	}
	cleaned := path.Clean(ref)
	if cleaned != ref || cleaned == "." || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "../") {
		return false
	}
	return slices.Contains(DefaultPolicyFiles(), ref)
}

// DefaultAgentPolicySetFiles resolves a stable set ID to canonical files in the
// embedded default policy inventory. Every set includes the constitutional base.
func DefaultAgentPolicySetFiles(setID string) ([]string, bool) {
	extras, ok := defaultAgentPolicySetExtras[setID]
	if !ok {
		return nil, false
	}
	files := make([]string, 0, len(defaultAgentBasePolicyFiles)+len(extras))
	files = append(files, defaultAgentBasePolicyFiles...)
	files = append(files, extras...)
	return files, true
}

// DefaultAgentPolicySetIDs returns the stable policy-set vocabulary.
func DefaultAgentPolicySetIDs() []string {
	ids := make([]string, 0, len(defaultAgentPolicySetExtras))
	for id := range defaultAgentPolicySetExtras {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
