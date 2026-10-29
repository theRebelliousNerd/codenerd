package defaults

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// declPattern captures the predicate name and its argument list from a
// `Decl name(A, B)` line.
var declPattern = regexp.MustCompile(`(?m)^\s*Decl\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(([^)]*)\)`)

// declKey renders a Decl as name/arity.
//
// Arity is part of the identity: Mangle keys predicates by name AND arity, so
// task_complexity/1 and task_complexity/2 are two different predicates and
// declaring both is correct. A name-only check flags those as duplicates and
// trains people to ignore it.
func declKey(match []string) string {
	name, args := match[1], strings.TrimSpace(match[2])
	arity := 0
	if args != "" {
		arity = strings.Count(args, ",") + 1
	}
	return name + "/" + itoa(arity)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// Declaring one predicate in two schema modules is not a warning. Mangle's
// analyzer rejects the whole program with "predicate X declared more than
// once", the kernel fails to boot its embedded constitution, and EVERY kernel
// operation fails — a schema module added in one corner takes down perception,
// policy, and the JIT compiler at once.
//
// This is a purely textual scan on purpose: it runs in milliseconds and names
// the two files, whereas the symptom is 100+ unrelated test failures whose
// messages mention neither file. Adding schemas_projectdoc.mg with a
// project_language Decl that already existed in schemas_project.mg cost exactly
// that debugging round-trip.
func TestSchemas_NoPredicateIsDeclaredTwice(t *testing.T) {
	files, err := filepath.Glob("*.mg")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no schema modules found next to this test")
	}

	// predicate -> files that declare it
	declaredIn := map[string][]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		seenHere := map[string]bool{}
		for _, m := range declPattern.FindAllStringSubmatch(string(data), -1) {
			name := declKey(m)
			if seenHere[name] {
				// Same file twice is equally fatal.
				declaredIn[name] = append(declaredIn[name], file)
				continue
			}
			seenHere[name] = true
			declaredIn[name] = append(declaredIn[name], file)
		}
	}

	var dupes []string
	for name, locations := range declaredIn {
		if len(locations) > 1 {
			dupes = append(dupes, name+" in "+strings.Join(locations, ", "))
		}
	}
	sort.Strings(dupes)

	if len(dupes) > 0 {
		t.Errorf("%d predicate(s) are declared more than once; Mangle's analyzer rejects the whole "+
			"program for this and the kernel will not boot:\n  %s",
			len(dupes), strings.Join(dupes, "\n  "))
	}
}

// Policy modules hold rules; schema modules hold declarations. A Decl in a
// policy file is legal, but it is also where duplicates hide from the check
// above — so the ones that exist should be deliberate and few.
func TestPolicy_DeclsDoNotCollideWithSchemas(t *testing.T) {
	schemaFiles, err := filepath.Glob("*.mg")
	if err != nil {
		t.Fatalf("glob schemas: %v", err)
	}
	policyFiles, err := filepath.Glob(filepath.Join("policy", "*.mg"))
	if err != nil {
		t.Fatalf("glob policy: %v", err)
	}

	schemaDecls := map[string]string{}
	for _, file := range schemaFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, m := range declPattern.FindAllStringSubmatch(string(data), -1) {
			schemaDecls[declKey(m)] = file
		}
	}

	var collisions []string
	for _, file := range policyFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, m := range declPattern.FindAllStringSubmatch(string(data), -1) {
			if origin, ok := schemaDecls[declKey(m)]; ok {
				collisions = append(collisions, declKey(m)+" declared in both "+origin+" and "+file)
			}
		}
	}
	sort.Strings(collisions)

	if len(collisions) > 0 {
		t.Errorf("%d predicate(s) are declared in a schema module AND a policy module; "+
			"keep the Decl in the schema and delete the policy copy:\n  %s",
			len(collisions), strings.Join(collisions, "\n  "))
	}
}
