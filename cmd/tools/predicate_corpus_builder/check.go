package main

// =============================================================================
// DRIFT CHECK
// =============================================================================
// predicate_corpus.db is generated from the .mg corpus and embedded into the
// binary, where it feeds schema validation and the self-healing repair
// guidance — it is how the system tells a model which predicates exist and what
// shape they are. Being generated, it drifts, and a stale entry here is not a
// cosmetic problem.
//
// The 2026-09 rebuild found the committed corpus recording click_event/2
// against a Decl of click_event/3, and seven more browser predicates each one
// argument short: every one had gained a leading SessionID and the corpus never
// caught up. Handing a model an arity the schema does not have invites the one
// failure this repo warns about hardest — a duplicate Decl at a different arity
// takes the whole kernel down at boot, not just the rule that uses it. 150
// predicates declared over the preceding seven weeks were missing entirely.
//
// `git diff --exit-code` cannot gate this: the rows carry a build timestamp, so
// every rebuild differs byte for byte while saying the same thing. The check
// compares what the corpus is FOR — the (name, arity, type) of every predicate.

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"codenerd/internal/store"
)

// predicateKey identifies a predicate as its consumers see it.
type predicateKey struct {
	Name  string
	Arity int
	Type  string
}

func (k predicateKey) String() string {
	return fmt.Sprintf("%s/%d (%s)", k.Name, k.Arity, k.Type)
}

// runDriftCheck compares the committed corpus against a fresh parse of the .mg
// files and returns a non-zero exit code on any difference.
func runDriftCheck(fresh []PredicateEntry) int {
	committed, err := readCommittedPredicates(outputPath)
	if err != nil {
		fmt.Printf("ERROR: cannot read %s: %v\n", outputPath, err)
		fmt.Println("Build it with: go run ./cmd/tools/predicate_corpus_builder")
		return 1
	}

	want := make(map[predicateKey]struct{}, len(fresh))
	for _, p := range fresh {
		want[predicateKey{Name: p.Name, Arity: p.Arity, Type: p.Type}] = struct{}{}
	}

	var missing, extra []string
	for k := range want {
		if _, ok := committed[k]; !ok {
			missing = append(missing, k.String())
		}
	}
	for k := range committed {
		if _, ok := want[k]; !ok {
			extra = append(extra, k.String())
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) == 0 && len(extra) == 0 {
		fmt.Printf("Predicate corpus is current: %d predicates match the .mg declarations.\n", len(want))
		return 0
	}

	fmt.Println("Predicate corpus has drifted from the .mg corpus.")
	fmt.Println()
	if len(missing) > 0 {
		fmt.Printf("Declared in .mg but absent from the corpus (%d):\n", len(missing))
		fmt.Println("  " + strings.Join(truncateList(missing, 30), "\n  "))
		fmt.Println()
	}
	if len(extra) > 0 {
		fmt.Printf("In the corpus but not declared in .mg (%d):\n", len(extra))
		fmt.Println("  " + strings.Join(truncateList(extra, 30), "\n  "))
		fmt.Println()
	}
	fmt.Println("A wrong arity here is fed to the model as fact. Rebuild with:")
	fmt.Println("  go run ./cmd/tools/predicate_corpus_builder")
	return 1
}

// truncateList caps a report so a large drift stays readable.
func truncateList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	out := append([]string(nil), items[:max]...)
	return append(out, fmt.Sprintf("… and %d more", len(items)-max))
}

// readCommittedPredicates reads the (name, arity, type) set from the corpus.
func readCommittedPredicates(path string) (map[predicateKey]struct{}, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	// Opened through the store façade rather than sql.Open, so this read picks
	// up the same pragmas every other site in the repo does.
	// TestSQLOpenSites_WhenOpeningSQLite_ShouldApplyPragmasOrBeExempt enforces
	// it, and caught this file on its first commit.
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	store.ApplyDefaultPragmas(db, store.ProfileReadOnly)
	defer func() { _ = db.Close() }()

	rows, err := db.Query("SELECT name, arity, type FROM predicates")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make(map[predicateKey]struct{}, 2048)
	for rows.Next() {
		var k predicateKey
		if err := rows.Scan(&k.Name, &k.Arity, &k.Type); err != nil {
			return nil, err
		}
		out[k] = struct{}{}
	}
	return out, rows.Err()
}
