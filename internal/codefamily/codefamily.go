// Package codefamily loads code family contracts: reviewed declarations that
// several independently located implementations share obligations
// (Docs/audits/codedom-governed-code-replication.md).
//
// A family is not a dependency and not a copy-sync rule. It says that when one
// member changes, every other member must be reconsidered for each shared
// requirement — edited, or explicitly decided to need no change. The kernel
// derives those obligations (policy/code_families.mg); this package only reads
// the operator's file and projects it as facts.
package codefamily

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// FileName is the contract file under .nerd.
const FileName = "code_families.json"

// SchemaVersion is the only version this loader accepts.
const SchemaVersion = 1

// Document is the on-disk contract file.
type Document struct {
	SchemaVersion int      `json:"schema_version"`
	Families      []Family `json:"families"`
}

// Family is one reviewed contract.
type Family struct {
	ID string `json:"family_id"`
	// Members are CodeDOM element refs (fn:pkg.Name, method:pkg.Type.Name).
	Members []string `json:"members"`
	// SharedRequirements are what every member must keep satisfying; each is
	// one review obligation per peer when a member changes.
	SharedRequirements []string `json:"shared_requirements"`
	// AllowedDifferences are documented, not enforced: they tell the
	// reviewer what is meant to vary.
	AllowedDifferences []string `json:"allowed_differences,omitempty"`
}

// Load reads nerdDir/code_families.json. A missing file is no families, not an
// error: families are opt-in.
func Load(nerdDir string) ([]Family, error) {
	data, err := os.ReadFile(filepath.Join(nerdDir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", FileName, err)
	}
	return Parse(data)
}

// Parse decodes and validates a contract file. Unknown fields are refused so a
// misspelled key cannot silently drop a requirement.
func Parse(data []byte) ([]Family, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if doc.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%s: schema_version %d, want %d", FileName, doc.SchemaVersion, SchemaVersion)
	}
	if err := Validate(doc.Families); err != nil {
		return nil, err
	}
	return doc.Families, nil
}

// Validate refuses a contract the kernel could not reason about soundly:
// duplicate or empty IDs, fewer than two members, duplicate members, and a
// family with no shared requirement (it would derive no obligation and so
// claim nothing).
func Validate(families []Family) error {
	var problems []string
	seen := map[string]bool{}
	for i, f := range families {
		id := strings.TrimSpace(f.ID)
		switch {
		case id == "":
			problems = append(problems, fmt.Sprintf("families[%d]: family_id is empty", i))
			continue
		case seen[id]:
			problems = append(problems, fmt.Sprintf("families[%d]: duplicate family_id %q", i, id))
		}
		seen[id] = true
		members := map[string]bool{}
		for _, m := range f.Members {
			m = strings.TrimSpace(m)
			if m == "" || members[m] {
				problems = append(problems, fmt.Sprintf("family %q: empty or duplicate member %q", id, m))
			}
			members[m] = true
		}
		if len(members) < 2 {
			problems = append(problems, fmt.Sprintf("family %q: needs at least two members", id))
		}
		if len(f.SharedRequirements) == 0 {
			problems = append(problems, fmt.Sprintf("family %q: no shared_requirements, so it would oblige nothing", id))
		}
		for _, r := range f.SharedRequirements {
			if strings.TrimSpace(r) == "" {
				problems = append(problems, fmt.Sprintf("family %q: empty shared requirement", id))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%s: %s", FileName, strings.Join(problems, "; "))
	}
	return nil
}

// Facts projects families as code_family / code_family_member /
// code_family_requirement rows (policy/code_families.mg).
func Facts(families []Family) []types.Fact {
	var out []types.Fact
	for _, f := range families {
		id := types.MangleString(strings.TrimSpace(f.ID))
		out = append(out, types.Fact{Predicate: "code_family", Args: []any{id}})
		for _, m := range f.Members {
			out = append(out, types.Fact{Predicate: "code_family_member", Args: []any{id, types.MangleString(strings.TrimSpace(m))}})
		}
		for _, r := range f.SharedRequirements {
			out = append(out, types.Fact{Predicate: "code_family_requirement", Args: []any{id, types.MangleString(strings.TrimSpace(r))}})
		}
	}
	return out
}

// Predicates are the base predicates Facts produces, for retract-and-replace.
var Predicates = []string{"code_family", "code_family_member", "code_family_requirement"}
