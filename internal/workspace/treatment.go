package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Semantic treatment is decided by orientation (internal/orient/treatment.mg)
// and published as .nerd/orientation/membership.json. This file owns that
// artifact's schema and a read-only view of it. The view never changes what
// Includes, IncludesDir, Admit, Gate, Walk or Files answer: membership says
// whether a path is part of the workspace; treatment says how much of a
// member a consumer would ordinarily acquire.

// TreatmentArtifactSchema is the membership.json schema this build reads and
// orientation writes. Another version is refused, never reinterpreted.
const TreatmentArtifactSchema = 1

// Treatment is a policy treatment name, with its leading slash.
type Treatment string

const (
	// TreatmentIndexAndParse is ordinary structural indexing.
	TreatmentIndexAndParse Treatment = "/index_and_parse"
	// TreatmentIndexNamesOnly keeps names and metadata without body reads.
	TreatmentIndexNamesOnly Treatment = "/index_names_only"
	// TreatmentExclude skips ordinary traversal of the scope.
	TreatmentExclude Treatment = "/exclude"
)

// SkipsBodies is true for the treatments under which ordinary consumers do
// not read, hash, parse or embed a member's body.
func (t Treatment) SkipsBodies() bool {
	return t == TreatmentIndexNamesOnly || t == TreatmentExclude
}

// ErrTreatmentIdentity is the view's refusal of an artifact written for a
// different workspace or schema: a copied artifact installs nothing.
var ErrTreatmentIdentity = errors.New("workspace: treatment artifact belongs to another workspace or schema")

// TreatmentRow is the policy's effective treatment for one census directory.
type TreatmentRow struct {
	Dir       string    `json:"dir"`
	Treatment Treatment `json:"treatment"`
	// Authority is /access_restriction, /operator or /inferred: whether the
	// decision is a hard access limit, an operator's operational choice, or
	// orientation's inference from measurements.
	Authority string `json:"authority"`
	// Scope is the directory the governing claim was made on: Dir itself or
	// the ancestor Dir inherits from.
	Scope   string   `json:"scope"`
	Reasons []string `json:"reasons"`
}

// TreatmentConflict is a directory whose nearest equally authoritative claims
// disagree. It has no treatment; consumers keep their current behavior.
type TreatmentConflict struct {
	Dir        string      `json:"dir"`
	Scope      string      `json:"scope"`
	Treatments []Treatment `json:"treatments"`
}

// TreatmentCensus is what the measurement behind the rows cost and covered.
type TreatmentCensus struct {
	Directories      int   `json:"directories"`
	Files            int   `json:"files"`
	Bytes            int64 `json:"bytes"`
	MaxDepth         int   `json:"max_depth"`
	HeaderReads      int   `json:"header_reads"`
	HeaderReadBudget int   `json:"header_read_budget"`
	// UnsampledDirectories had fewer header samples than the per-directory
	// allowance because the budget ran out; no generated judgment covers them.
	UnsampledDirectories int `json:"unsampled_directories"`
}

// TreatmentArtifact is .nerd/orientation/membership.json. It carries no
// timestamps: the same generation, measurements and policy encode to the
// same bytes.
type TreatmentArtifact struct {
	SchemaVersion int `json:"schema_version"`
	// Workspace is the canonical membership root the rows describe.
	Workspace string `json:"workspace"`
	// Generation is Membership.Generation when the census ran.
	Generation        string              `json:"generation"`
	MeasurementDigest string              `json:"measurement_digest"`
	PolicyDigest      string              `json:"policy_digest"`
	Census            TreatmentCensus     `json:"census"`
	Treatments        []TreatmentRow      `json:"treatments"`
	Conflicts         []TreatmentConflict `json:"conflicts"`
	// UnmatchedOverrides are configured override paths that are not member
	// directories. They govern nothing.
	UnmatchedOverrides []string `json:"unmatched_overrides"`
}

// TreatmentArtifactPath is where root's artifact lives.
func TreatmentArtifactPath(root string) string {
	return filepath.Join(root, ".nerd", "orientation", "membership.json")
}

// Encode is the artifact's deterministic JSON: rows, reasons, conflicts and
// unmatched paths sorted, nil lists written as empty.
func (a TreatmentArtifact) Encode() ([]byte, error) {
	a.Treatments = append([]TreatmentRow{}, a.Treatments...)
	for i := range a.Treatments {
		a.Treatments[i].Reasons = append([]string{}, a.Treatments[i].Reasons...)
		sort.Strings(a.Treatments[i].Reasons)
	}
	sort.Slice(a.Treatments, func(i, j int) bool { return a.Treatments[i].Dir < a.Treatments[j].Dir })
	a.Conflicts = append([]TreatmentConflict{}, a.Conflicts...)
	for i := range a.Conflicts {
		a.Conflicts[i].Treatments = append([]Treatment{}, a.Conflicts[i].Treatments...)
		sort.Slice(a.Conflicts[i].Treatments, func(x, y int) bool { return a.Conflicts[i].Treatments[x] < a.Conflicts[i].Treatments[y] })
	}
	sort.Slice(a.Conflicts, func(i, j int) bool { return a.Conflicts[i].Dir < a.Conflicts[j].Dir })
	a.UnmatchedOverrides = append([]string{}, a.UnmatchedOverrides...)
	sort.Strings(a.UnmatchedOverrides)
	body, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

// ReadTreatmentArtifact decodes root's artifact. A missing file is an error
// satisfying errors.Is(err, os.ErrNotExist).
func ReadTreatmentArtifact(root string) (*TreatmentArtifact, error) {
	body, err := os.ReadFile(TreatmentArtifactPath(root))
	if err != nil {
		return nil, err
	}
	var a TreatmentArtifact
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("treatment artifact %s: %w", TreatmentArtifactPath(root), err)
	}
	return &a, nil
}

// GenerationOf is the durable identity of a member file list: SHA-256 over
// the sorted, NUL-separated paths. Unlike Revision it survives the process
// and names the members themselves, not the git stamps that produced them.
func GenerationOf(files []string) string {
	sorted := files
	if !sort.StringsAreSorted(files) {
		sorted = append([]string(nil), files...)
		sort.Strings(sorted)
	}
	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Generation is GenerationOf(Files()). A file edit that keeps the member set
// leaves it unchanged; an added, removed or newly ignored member moves it.
func (m *Membership) Generation() string {
	if m == nil {
		return ""
	}
	return GenerationOf(m.Files())
}

// TreatmentView is a frozen, read-only view of one treatment artifact.
type TreatmentView struct {
	generation   string
	policyDigest string
	current      bool
	rows         map[string]TreatmentRow
	conflicts    map[string]TreatmentConflict
}

// TreatmentDecision answers one path. A zero Treatment is unclassified:
// either no row governs the path or its nearest scope is in conflict, and
// consumers keep their current behavior.
type TreatmentDecision struct {
	Treatment Treatment
	// Dir is the census directory whose row or conflict governs the path;
	// empty when none does.
	Dir       string
	Authority string
	Conflict  bool
	// Stale is true when the artifact's generation is not the membership's
	// generation at the time the view was loaded.
	Stale bool
}

// TreatmentView loads this membership's artifact. A missing artifact is a nil
// view and a nil error: nothing is classified before orientation has run. An
// artifact for another root or schema is ErrTreatmentIdentity.
func (m *Membership) TreatmentView() (*TreatmentView, error) {
	if m == nil {
		return nil, errors.New("workspace: nil membership")
	}
	a, err := ReadTreatmentArtifact(m.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if a.SchemaVersion != TreatmentArtifactSchema || a.Workspace != m.root {
		return nil, fmt.Errorf("%w: schema %d for %q, want schema %d for %q",
			ErrTreatmentIdentity, a.SchemaVersion, a.Workspace, TreatmentArtifactSchema, m.root)
	}
	v := &TreatmentView{
		generation:   a.Generation,
		policyDigest: a.PolicyDigest,
		current:      a.Generation == m.Generation(),
		rows:         make(map[string]TreatmentRow, len(a.Treatments)),
		conflicts:    make(map[string]TreatmentConflict, len(a.Conflicts)),
	}
	for _, r := range a.Treatments {
		v.rows[r.Dir] = r
	}
	for _, c := range a.Conflicts {
		v.conflicts[c.Dir] = c
	}
	return v, nil
}

// Generation is the member generation the view's rows were derived for.
func (v *TreatmentView) Generation() string {
	if v == nil {
		return ""
	}
	return v.generation
}

// PolicyDigest identifies the policy and configuration behind the rows.
func (v *TreatmentView) PolicyDigest() string {
	if v == nil {
		return ""
	}
	return v.policyDigest
}

// Current reports whether the rows were derived for the member set the view
// was loaded against.
func (v *TreatmentView) Current() bool { return v != nil && v.current }

// Treatment resolves a workspace-relative path to the nearest census
// directory at or above it, segment by segment: a row for "data" never
// governs "database". The policy already resolved inheritance and authority
// for every census directory; this is only the lookup.
func (v *TreatmentView) Treatment(rel string) TreatmentDecision {
	if v == nil {
		return TreatmentDecision{}
	}
	rel, ok := cleanRel(rel)
	if !ok {
		return TreatmentDecision{}
	}
	for dir := rel; dir != ""; dir = parentRel(dir) {
		if c, ok := v.conflicts[dir]; ok {
			return TreatmentDecision{Dir: c.Dir, Conflict: true, Stale: !v.current}
		}
		if r, ok := v.rows[dir]; ok {
			return TreatmentDecision{Treatment: r.Treatment, Dir: r.Dir, Authority: r.Authority, Stale: !v.current}
		}
	}
	return TreatmentDecision{}
}
