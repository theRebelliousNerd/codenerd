// Package specs delivers bounded workspace browser specifications.
//
// The document contract is adapted from BrowserNERD under Apache-2.0; see
// THIRD_PARTY_NOTICES.md. codeNERD confines all source roots to the active
// workspace and leaves invariant evaluation to the live Cortex kernel.
package specs

const (
	DefaultRoot              = ".nerd/browser/specs"
	defaultMaxFiles          = 2000
	defaultMaxFileBytes      = 2 << 20
	defaultMaxResults        = 12
	defaultMaxExcerptBytes   = 1200
	hardMaxFiles             = 5000
	hardMaxFileBytes         = 8 << 20
	hardMaxResults           = 50
	hardMaxExcerptBytes      = 8 << 10
	hardMaxCatalogBytes      = 32 << 20
	hardMaxCatalogWarnings   = 50
	hardMaxCatalogEntries    = 100000
	hardMaxInvariantsPerSpec = 500
)

// Source is one named workspace documentation corpus.
type Source struct {
	Name    string   `json:"name" yaml:"name"`
	Roots   []string `json:"roots" yaml:"roots"`
	Indexes []string `json:"indexes,omitempty" yaml:"indexes,omitempty"`
	Include []string `json:"include,omitempty" yaml:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// Config controls bounded spec discovery and delivery.
type Config struct {
	Enabled         *bool    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Sources         []Source `json:"sources,omitempty" yaml:"sources,omitempty"`
	MaxFiles        int      `json:"max_files,omitempty" yaml:"max_files,omitempty"`
	MaxFileBytes    int64    `json:"max_file_bytes,omitempty" yaml:"max_file_bytes,omitempty"`
	MaxResults      int      `json:"max_results,omitempty" yaml:"max_results,omitempty"`
	MaxExcerptBytes int      `json:"max_excerpt_bytes,omitempty" yaml:"max_excerpt_bytes,omitempty"`
}

// DefaultConfig returns the native workspace spec defaults.
func DefaultConfig() Config {
	enabled := true
	return Config{
		Enabled: &enabled, MaxFiles: defaultMaxFiles, MaxFileBytes: defaultMaxFileBytes,
		MaxResults: defaultMaxResults, MaxExcerptBytes: defaultMaxExcerptBytes,
	}
}

// Normalize fills defaults and clamps every caller-controlled resource limit.
func (c Config) Normalize() Config {
	defaults := DefaultConfig()
	if c.Enabled == nil {
		c.Enabled = defaults.Enabled
	}
	c.MaxFiles = boundedInt(c.MaxFiles, defaultMaxFiles, hardMaxFiles)
	c.MaxFileBytes = boundedInt64(c.MaxFileBytes, defaultMaxFileBytes, hardMaxFileBytes)
	c.MaxResults = boundedInt(c.MaxResults, defaultMaxResults, hardMaxResults)
	c.MaxExcerptBytes = boundedInt(c.MaxExcerptBytes, defaultMaxExcerptBytes, hardMaxExcerptBytes)
	if len(c.Sources) == 0 {
		c.Sources = []Source{{Name: "workspace", Roots: []string{DefaultRoot}, Include: []string{"**/*.md"}}}
	}
	return c
}

// IsEnabled reports whether native spec delivery is enabled.
func (c Config) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

func boundedInt(value, fallback, ceiling int) int {
	if value <= 0 {
		return fallback
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

func boundedInt64(value, fallback, ceiling int64) int64 {
	if value <= 0 {
		return fallback
	}
	if value > ceiling {
		return ceiling
	}
	return value
}

// Binding ties a spec to a browser-observable feature.
type Binding struct {
	Kind   string `json:"kind" yaml:"kind"`
	Target string `json:"target" yaml:"target"`
}

// Invariant is one read-only live-kernel browser assertion.
type Invariant struct {
	Name   string `json:"name" yaml:"name"`
	Query  string `json:"query,omitempty" yaml:"query"`
	Expect string `json:"expect,omitempty" yaml:"expect"`
	Prose  string `json:"prose,omitempty" yaml:"-"`
	File   string `json:"file,omitempty" yaml:"-"`
	From   int    `json:"from,omitempty" yaml:"-"`
	To     int    `json:"to,omitempty" yaml:"-"`
	Inline bool   `json:"inline,omitempty"`
}

// Covers reports whether the invariant governs any line in [from,to].
func (i Invariant) Covers(file string, from, to int) bool {
	return sameFile(i.File, file) && i.From > 0 && i.To > 0 && from > 0 && to > 0 && i.From <= to && from <= i.To
}

// InFile reports whether the invariant governs file.
func (i Invariant) InFile(file string) bool { return sameFile(i.File, file) }

// Spec is one parsed Markdown specification.
type Spec struct {
	Name       string      `json:"name"`
	Title      string      `json:"title,omitempty"`
	Path       string      `json:"path"`
	Corpus     string      `json:"corpus"`
	Source     string      `json:"source,omitempty"`
	Summary    string      `json:"summary,omitempty"`
	ReadWhen   string      `json:"read_when,omitempty"`
	DocType    string      `json:"doc_type,omitempty"`
	Subsystem  string      `json:"subsystem,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Bindings   []Binding   `json:"bindings,omitempty"`
	Invariants []Invariant `json:"invariants,omitempty"`
	Body       string      `json:"-"`
}

// LoadResult describes bounded catalog scope.
type LoadResult struct {
	Specs          []Spec   `json:"-"`
	FilesScanned   int      `json:"files_scanned"`
	EntriesScanned int      `json:"entries_scanned"`
	BytesLoaded    int64    `json:"bytes_loaded"`
	Warnings       []string `json:"warnings,omitempty"`
	Truncated      bool     `json:"truncated"`
}

// MatchInput describes relevant development and page context.
type MatchInput struct {
	Corpus    string
	File      string
	From      int
	To        int
	Component string
	Route     string
	Selector  string
	Terms     []string
	Max       int
}

// Match is a compact ranked spec excerpt.
type Match struct {
	Name           string    `json:"name"`
	Title          string    `json:"title"`
	Path           string    `json:"path"`
	Corpus         string    `json:"corpus"`
	Summary        string    `json:"summary,omitempty"`
	ReadWhen       string    `json:"read_when,omitempty"`
	DocType        string    `json:"doc_type,omitempty"`
	Subsystem      string    `json:"subsystem,omitempty"`
	Bindings       []Binding `json:"bindings,omitempty"`
	InvariantCount int       `json:"invariant_count"`
	Score          int       `json:"score"`
	Excerpt        string    `json:"excerpt,omitempty"`
}

// SelectedInvariant pairs an invariant with its declaring spec.
type SelectedInvariant struct {
	Spec     string    `json:"spec"`
	Path     string    `json:"path"`
	Corpus   string    `json:"corpus"`
	Bindings []Binding `json:"bindings,omitempty"`
	Invariant
}
