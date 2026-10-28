package config

import (
	"fmt"
	"sort"
	"strings"
)

// Treatment names an override may set. They are the policy's /name values
// without the slash (internal/orient/treatment.mg).
const (
	TreatmentIndexAndParse  = "index_and_parse"
	TreatmentIndexNamesOnly = "index_names_only"
	TreatmentExclude        = "exclude"
)

// Override bases. An access restriction is a hard limit no inferred or task
// promotion may widen; an operational override is an indexing choice a more
// specific operator override may refine.
const (
	TreatmentBasisAccessRestriction = "access_restriction"
	TreatmentBasisOperational       = "operational"
)

// OrientTreatmentConfig is orient.treatment: the bounds of the directory
// census orientation takes for semantic treatment, the thresholds the
// treatment policy reads, and the operator's overrides. A zero is absent
// (WithDefaults). Treatment is shadow-only in this release: the world scanner
// records what a decision would skip and still hashes and parses every
// member file.
type OrientTreatmentConfig struct {
	// CensusMaxDepth is the deepest member directory the census rows describe.
	// A deeper file counts toward each of its ancestors up to this depth.
	CensusMaxDepth int `json:"census_max_depth,omitempty"`
	// HeaderReadBudget is how many file headers one census may open, in
	// total, to count generated-code markers. Reads are counted and recorded.
	HeaderReadBudget int `json:"header_read_budget,omitempty"`
	// HeaderSamplesPerDir is how many files of one directory's subtree are
	// sampled for generated markers, evenly spaced over its sorted members.
	HeaderSamplesPerDir int `json:"header_samples_per_dir,omitempty"`
	// HeaderBytes is how much of each sampled file is read.
	HeaderBytes int `json:"header_bytes,omitempty"`
	// SourceSharePermille is the share of source-language files at which a
	// directory is source evidence (/orient_treatment_source_share_permille).
	SourceSharePermille int `json:"source_share_permille,omitempty"`
	// DataSharePermille and DataMinBytes make a directory seed data: that
	// share of data-format files and at least that many bytes.
	DataSharePermille int   `json:"data_share_permille,omitempty"`
	DataMinBytes      int64 `json:"data_min_bytes,omitempty"`
	// GeneratedSharePermille of at least GeneratedMinSamples header reads
	// carrying a generated marker make a directory generated.
	GeneratedSharePermille int `json:"generated_share_permille,omitempty"`
	GeneratedMinSamples    int `json:"generated_min_samples,omitempty"`
	// Overrides are operator decisions for workspace-relative directories.
	Overrides []TreatmentOverride `json:"overrides,omitempty"`
}

// TreatmentOverride is one operator decision for a member directory and the
// directories below it.
type TreatmentOverride struct {
	// Path is a canonical workspace-relative directory: slash-separated, no
	// leading or trailing slash, no "." or ".." segment.
	Path string `json:"path"`
	// Treatment is index_and_parse, index_names_only or exclude.
	Treatment string `json:"treatment"`
	// Basis is access_restriction or operational. index_and_parse grants
	// work, so it cannot be an access restriction.
	Basis string `json:"basis"`
}

// DefaultOrientTreatmentConfig is orient.treatment with every bound written
// down. Depth 4 describes a monorepo's packages without a row per leaf. 2048
// header reads of 1KiB, 4 per directory, cost well under a second. A
// directory is source evidence at 60% source files, seed data at 80% data
// files and 1MiB, and generated when 3 of 4 sampled headers say so.
func DefaultOrientTreatmentConfig() OrientTreatmentConfig {
	return OrientTreatmentConfig{
		CensusMaxDepth:         4,
		HeaderReadBudget:       2048,
		HeaderSamplesPerDir:    4,
		HeaderBytes:            1024,
		SourceSharePermille:    600,
		DataSharePermille:      800,
		DataMinBytes:           1 << 20,
		GeneratedSharePermille: 750,
		GeneratedMinSamples:    3,
	}
}

// TreatmentPolicy is the treatment section in force: the defaults when the
// orient section does not name one.
func (c OrientConfig) TreatmentPolicy() OrientTreatmentConfig {
	if c.Treatment == nil {
		return DefaultOrientTreatmentConfig()
	}
	return c.Treatment.WithDefaults()
}

// WithDefaults fills every absent bound. Overrides are kept as written.
func (c OrientTreatmentConfig) WithDefaults() OrientTreatmentConfig {
	d := DefaultOrientTreatmentConfig()
	if c.CensusMaxDepth == 0 {
		c.CensusMaxDepth = d.CensusMaxDepth
	}
	if c.HeaderReadBudget == 0 {
		c.HeaderReadBudget = d.HeaderReadBudget
	}
	if c.HeaderSamplesPerDir == 0 {
		c.HeaderSamplesPerDir = d.HeaderSamplesPerDir
	}
	if c.HeaderBytes == 0 {
		c.HeaderBytes = d.HeaderBytes
	}
	if c.SourceSharePermille == 0 {
		c.SourceSharePermille = d.SourceSharePermille
	}
	if c.DataSharePermille == 0 {
		c.DataSharePermille = d.DataSharePermille
	}
	if c.DataMinBytes == 0 {
		c.DataMinBytes = d.DataMinBytes
	}
	if c.GeneratedSharePermille == 0 {
		c.GeneratedSharePermille = d.GeneratedSharePermille
	}
	if c.GeneratedMinSamples == 0 {
		c.GeneratedMinSamples = d.GeneratedMinSamples
	}
	return c
}

// SortedOverrides is the overrides in path order, so a digest over them does
// not depend on the order the file lists them in.
func (c OrientTreatmentConfig) SortedOverrides() []TreatmentOverride {
	out := append([]TreatmentOverride(nil), c.Overrides...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Check reports the contradictions in orient.treatment under prefix.
func (c OrientTreatmentConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	bad := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	rangeOf := func(field string, v, lo, hi int64, why string) {
		if v < lo || v > hi {
			bad(field, fmt.Sprintf("%d is outside %d..%d: %s", v, lo, hi, why),
				fmt.Sprintf("a value from %d to %d, or remove the key for the default", lo, hi))
		}
	}
	rangeOf("census_max_depth", int64(c.CensusMaxDepth), 1, 32, "rows describe directories down to this depth")
	rangeOf("header_read_budget", int64(c.HeaderReadBudget), 1, 1<<20, "every header read is a file open the census pays for")
	rangeOf("header_samples_per_dir", int64(c.HeaderSamplesPerDir), 1, 64, "headers sampled per directory subtree")
	rangeOf("header_bytes", int64(c.HeaderBytes), 64, 64<<10, "a generated marker sits in a file's first lines")
	rangeOf("source_share_permille", int64(c.SourceSharePermille), 1, 1000, "share of source-language files")
	rangeOf("data_share_permille", int64(c.DataSharePermille), 1, 1000, "share of data-format files")
	rangeOf("data_min_bytes", c.DataMinBytes, 1, 1<<50, "a seed-data directory is large")
	rangeOf("generated_share_permille", int64(c.GeneratedSharePermille), 1, 1000, "share of sampled headers with a generated marker")
	rangeOf("generated_min_samples", int64(c.GeneratedMinSamples), 1, 64, "headers a generated judgment needs")
	if c.GeneratedMinSamples > c.HeaderSamplesPerDir {
		bad("generated_min_samples",
			fmt.Sprintf("%d samples are required but only %d are read per directory, so no directory could be judged generated", c.GeneratedMinSamples, c.HeaderSamplesPerDir),
			"generated_min_samples <= header_samples_per_dir")
	}
	seen := map[string]bool{}
	for i, o := range c.Overrides {
		field := fmt.Sprintf("overrides[%d]", i)
		if msg := treatmentPathProblem(o.Path); msg != "" {
			bad(field+".path", msg, "a workspace-relative directory such as \"testdata/golden\"")
		} else if seen[o.Path] {
			bad(field+".path", fmt.Sprintf("%q has more than one override; the decision would depend on list order", o.Path), "keep one override per directory")
		}
		seen[o.Path] = true
		switch o.Treatment {
		case TreatmentIndexAndParse, TreatmentIndexNamesOnly, TreatmentExclude:
		default:
			bad(field+".treatment", fmt.Sprintf("%q is not a treatment", o.Treatment),
				strings.Join([]string{TreatmentIndexAndParse, TreatmentIndexNamesOnly, TreatmentExclude}, ", "))
		}
		switch o.Basis {
		case TreatmentBasisOperational:
		case TreatmentBasisAccessRestriction:
			if o.Treatment == TreatmentIndexAndParse {
				bad(field+".basis", "an access restriction cannot grant index_and_parse", "basis \"operational\"")
			}
		default:
			bad(field+".basis", fmt.Sprintf("%q is not a basis: say whether this is a hard access restriction or an operational omission", o.Basis),
				TreatmentBasisAccessRestriction+" or "+TreatmentBasisOperational)
		}
	}
	return out
}

// treatmentPathProblem is empty for a canonical member-directory path.
func treatmentPathProblem(p string) string {
	switch {
	case p == "":
		return "path is empty; the workspace root is not a treatment scope"
	case strings.ContainsAny(p, "\\\x00"):
		return fmt.Sprintf("%q must use forward slashes and no NUL", p)
	case strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/"):
		return fmt.Sprintf("%q must be relative, with no leading or trailing slash", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Sprintf("%q has an empty, \".\" or \"..\" segment", p)
		}
	}
	return ""
}

// Params are the treatment thresholds the policy reads, as config_param rows
// (config_param_required(/orient, Key) in treatment.mg). The census
// bounds are Go's and are not rows.
func (c OrientTreatmentConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/orient_treatment_source_share_permille", Value: int64(c.SourceSharePermille)},
		{Key: "/orient_treatment_data_share_permille", Value: int64(c.DataSharePermille)},
		{Key: "/orient_treatment_data_min_bytes", Value: c.DataMinBytes},
		{Key: "/orient_treatment_generated_share_permille", Value: int64(c.GeneratedSharePermille)},
		{Key: "/orient_treatment_generated_min_samples", Value: int64(c.GeneratedMinSamples)},
	}
}
