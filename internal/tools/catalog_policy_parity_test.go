package tools_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

const constitutionFile = "../core/defaults/policy/constitution.mg"

var safeActionRe = regexp.MustCompile(`safe_action\(/([A-Za-z0-9_]+)\)`)

var requiresPermissionRe = regexp.MustCompile(`requires_permission\(/([A-Za-z0-9_]+)\)`)

// intentionalPolicyExceptions lists registered tools that deliberately have no
// safe_action/requires_permission fact, each with the reason. A tool listed
// here is hard-denied by the constitution's default-deny gate; that is the
// point, and the test fails if a policy fact later appears for it.
var intentionalPolicyExceptions = map[string]string{
	"research_cache_clear": "discards the research cache every agent in the process shares; denied by policy on purpose",
}

func constitutionCoveredTools(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(constitutionFile))
	if err != nil {
		t.Fatalf("read %s: %v", constitutionFile, err)
	}
	covered := make(map[string]bool)
	for _, m := range safeActionRe.FindAllStringSubmatch(string(data), -1) {
		covered[m[1]] = true
	}
	for _, m := range requiresPermissionRe.FindAllStringSubmatch(string(data), -1) {
		covered[m[1]] = true
	}
	if len(covered) == 0 {
		t.Fatalf("%s parsed to zero facts", constitutionFile)
	}
	covered["run_tests"] = true
	covered["run_build"] = true
	return covered
}

func policyMissingEntries(names []string, covered map[string]bool) []string {
	var missing []string
	for _, name := range names {
		if covered[name] {
			continue
		}
		if _, ok := intentionalPolicyExceptions[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}

func policyOnlyEntries(names []string, covered map[string]bool) []string {
	registered := make(map[string]bool, len(names))
	for _, name := range names {
		registered[name] = true
	}
	var policyOnly []string
	for name := range covered {
		if !registered[name] {
			policyOnly = append(policyOnly, name)
		}
	}
	sort.Strings(policyOnly)
	return policyOnly
}

func TestCatalogParity_WhenToolRegistered_ShouldHavePolicyEntry(t *testing.T) {
	t.Parallel()
	reg := fullyHydratedRegistry(t)
	covered := constitutionCoveredTools(t)
	missing := policyMissingEntries(reg.Names(), covered)
	if len(missing) > 0 {
		t.Errorf("tools with no policy entry in %s: %v", constitutionFile, missing)
	}
	t.Logf("policy-only entries (allowed): %v", policyOnlyEntries(reg.Names(), covered))
}

func TestCatalogParity_WhenExceptionListed_ShouldHaveReason(t *testing.T) {
	t.Parallel()
	reg := fullyHydratedRegistry(t)
	covered := constitutionCoveredTools(t)
	for name, reason := range intentionalPolicyExceptions {
		if reason == "" {
			t.Errorf("%s has no reason", name)
		}
		if !reg.Has(name) {
			t.Errorf("exception %s not registered", name)
		}
		if covered[name] {
			t.Errorf("exception %s now covered, remove it", name)
		}
	}
}
