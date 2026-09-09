package core

import (
	"fmt"
	"sort"
	"testing"
)

// registeredNames returns the concrete type names of every validator in r.
func registeredNames(r *ValidatorRegistry) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.validators))
	for _, v := range r.validators {
		names = append(names, fmt.Sprintf("%T", v))
	}
	sort.Strings(names)
	return names
}

// TestRegisterAllValidators_IsTheUnionOfTheGroups pins the fix for two copies of
// a safety list.
//
// RegisterAllValidators used to list all thirteen validators inline while the
// four grouped registrars listed the same thirteen again, and only
// RegisterAllValidators had a caller. A validator added to one of the groups
// would have been silently absent from every production VirtualStore, and
// post-action validators are exactly what catch a write that did not do what it
// claimed.
func TestRegisterAllValidators_IsTheUnionOfTheGroups(t *testing.T) {
	all := NewValidatorRegistry()
	RegisterAllValidators(all)

	union := NewValidatorRegistry()
	RegisterFileValidators(union)
	RegisterSyntaxValidators(union)
	RegisterExecutionValidators(union)
	RegisterCodeDOMValidators(union)

	got, want := registeredNames(all), registeredNames(union)
	if len(got) != len(want) {
		t.Fatalf("RegisterAllValidators registered %d validators, the groups register %d:\ngot:  %v\nwant: %v",
			len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("registration sets diverged at %d: %q vs %q\ngot:  %v\nwant: %v",
				i, got[i], want[i], got, want)
		}
	}
	if len(got) == 0 {
		t.Fatal("no validators registered at all; the test is vacuous")
	}
}

// TestRegisterAllValidators_CoversEveryCategory guards against a whole group
// being dropped from the composition — the failure the inline duplication made
// invisible.
func TestRegisterAllValidators_CoversEveryCategory(t *testing.T) {
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	names := registeredNames(r)

	present := func(substr string) bool {
		for _, n := range names {
			if len(n) >= len(substr) && contains(n, substr) {
				return true
			}
		}
		return false
	}

	for _, category := range []string{
		"FileWriteValidator",    // file group
		"ParanoidFileValidator", // file group, priority 100 backstop
		"SyntaxValidator",       // syntax group
		"BuildValidator",        // execution group
		"CodeDOMValidator",      // codedom group
	} {
		if !present(category) {
			t.Errorf("%s is not registered; a whole validator group is missing from RegisterAllValidators.\nregistered: %v", category, names)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
