// Package projectdoc parses nerd.md, the per-project instruction file.
//
// nerd.md is to codeNERD what CLAUDE.md is to Claude Code and AGENTS.md is to
// other agents, with one deliberate difference: the machine-readable half is
// not advice.
//
// A CLAUDE.md line that says "never edit config.json" is prose appended to a
// prompt. The model usually honours it. codeNERD's thesis is that the model is
// the creative center and the Mangle kernel is the executive, so nerd.md splits
// along that line:
//
//   - The YAML frontmatter is a STRICT schema. It becomes kernel facts, and
//     policy derives enforcement from them. A forbidden path is a denied tool
//     call, not an instruction the model may reinterpret.
//   - The Markdown body is prose. It becomes a project prompt atom and is
//     advisory, exactly like CLAUDE.md.
//
// Strictness is the point of the frontmatter. An unknown key, a bad schema
// version, or a malformed entry is a hard parse error naming the line, not a
// silently ignored field — a directive the user believes is in force but which
// the parser dropped is worse than no directive at all.
package projectdoc

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the canonical project instruction file.
const FileName = "nerd.md"

// SchemaVersion is the only frontmatter schema this build accepts.
//
// It is pinned rather than range-checked so that a nerd.md written for a newer
// codeNERD fails loudly here instead of being half-understood. The failure
// message tells the user which version they have and which this binary speaks.
const SchemaVersion = "nerd/v1"

// Document is a parsed nerd.md.
type Document struct {
	// Path is the file this was read from, relative to the workspace when known.
	Path string

	// Spec is the strict machine-readable frontmatter.
	Spec Spec

	// Body is the Markdown prose after the frontmatter, verbatim and trimmed.
	// Advisory only.
	Body string
}

// Spec is the strict frontmatter schema.
//
// Every field is optional except Schema. Adding a field here is a schema
// change: bump SchemaVersion, because older binaries reject unknown keys by
// design.
type Spec struct {
	// Schema pins the frontmatter contract. Required, must equal SchemaVersion.
	Schema string `yaml:"schema"`

	// Project is a human-readable project name, surfaced in prompts.
	Project string `yaml:"project,omitempty"`

	// Language is the primary language tag (e.g. "go", "python"). It seeds the
	// JIT compilation context's /lang dimension so language-specific atoms are
	// selected without waiting for a file to be opened.
	Language string `yaml:"language,omitempty"`

	// Commands are the project's real build/test/lint invocations. Without
	// these the agent guesses, and a guess that compiles on the maintainer's
	// machine is the most expensive kind of wrong.
	Commands Commands `yaml:"commands,omitempty"`

	// Forbid lists paths the agent must not write to. ENFORCED: these become
	// project_forbidden_path facts, and the executor refuses any write-mutation
	// tool whose target matches, before the tool runs.
	Forbid []ForbidRule `yaml:"forbid,omitempty"`

	// Require lists non-negotiable steps (e.g. "run go test ./... before
	// handoff"). Surfaced to the model and available to policy as
	// project_requirement facts.
	Require []string `yaml:"require,omitempty"`

	// Conventions are named, checkable project rules.
	Conventions []Convention `yaml:"conventions,omitempty"`

	// Northstar is an optional block declaring the module's purpose and its
	// requirements.
	//
	// It does NOT bump SchemaVersion, deliberately deviating from the note
	// above that adding a field requires a bump. The reason: validation at
	// line 240 requires s.Schema to EQUAL SchemaVersion exactly, so bumping
	// to nerd/v2 would reject every existing nerd.md that declares schema
	// nerd/v1. The field is optional, so v1 files that do not use it are
	// completely unaffected; a v1 file that DOES use it fails loudly on an
	// older binary with an unknown-field error, which is the strictness that
	// decoder.KnownFields(true) at line 180 exists to provide.
	Northstar *Northstar `yaml:"northstar,omitempty"`
}

// Commands holds the project's canonical shell invocations.
type Commands struct {
	Build string `yaml:"build,omitempty"`
	Test  string `yaml:"test,omitempty"`
	Lint  string `yaml:"lint,omitempty"`
	Run   string `yaml:"run,omitempty"`
	// Env are environment variables that must be set for the commands above.
	// This exists because CGO_CFLAGS-style build prerequisites are invisible in
	// the command string and their absence produces a confusing failure far
	// from its cause.
	Env map[string]string `yaml:"env,omitempty"`
}

// ForbidRule denies writes to paths containing Match.
type ForbidRule struct {
	// Match is a substring matched against the slash-normalized target path.
	//
	// Substring, deliberately, not glob: the semantics have to be obvious to
	// someone writing the file and identical in Go and in Mangle. A glob engine
	// that disagrees with itself across those two layers is a safety gate that
	// sometimes opens.
	Match string `yaml:"match"`

	// Reason is shown to the model and to the user when a write is denied. It
	// is required — a denial the agent cannot explain looks like a malfunction
	// and invites a workaround.
	Reason string `yaml:"reason"`
}

// Convention is a named project rule.
type Convention struct {
	ID   string `yaml:"id"`
	Rule string `yaml:"rule"`
}

// Northstar declares the module's purpose and its requirements.
type Northstar struct {
	// Purpose is the module's high-level purpose.
	Purpose string `yaml:"purpose,omitempty"`

	// Requirements are the module's requirements.
	Requirements []NorthstarRequirement `yaml:"requirements,omitempty"`
}

// NorthstarRequirement is a single requirement within a Northstar block.
type NorthstarRequirement struct {
	// ID is the stable requirement identifier.
	ID string `yaml:"id"`

	// Statement is the human-readable requirement text.
	Statement string `yaml:"statement"`

	// Severity indicates the requirement's importance when present.
	Severity string `yaml:"severity,omitempty"`
}

// Find locates nerd.md for a workspace, searching the workspace root and then
// .nerd/. Returns "" when absent, which is not an error: nerd.md is optional.
func Find(workspace string) string {
	for _, candidate := range []string{
		filepath.Join(workspace, FileName),
		filepath.Join(workspace, ".nerd", FileName),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// Load reads and parses nerd.md from a workspace.
//
// Returns (nil, nil) when the file does not exist. Returns an error only when
// the file exists and is invalid — an unreadable directive must never degrade
// to "no directive".
func Load(workspace string) (*Document, error) {
	path := Find(workspace)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	doc, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if rel, relErr := filepath.Rel(workspace, path); relErr == nil {
		doc.Path = filepath.ToSlash(rel)
	} else {
		doc.Path = filepath.ToSlash(path)
	}
	return doc, nil
}

// LoadAll returns the root document (if any) followed by every module-level
// nerd.md found beneath the workspace.
func LoadAll(workspace string) ([]*Document, error) {
	var docs []*Document

	rootDoc, err := Load(workspace)
	if err != nil {
		return nil, err
	}
	if rootDoc != nil {
		docs = append(docs, rootDoc)
	}

	walkRoot := workspace
	if walkRoot == "" {
		walkRoot = "."
	}

	rootRel := ""
	if rootDoc != nil {
		rootRel = rootDoc.Path
	}

	var modules []*Document
	var walkErr error

	err = filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				if p != walkRoot {
					return fs.SkipDir
				}
			} else if name == "node_modules" || name == "vendor" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != FileName {
			return nil
		}
		rel, relErr := filepath.Rel(walkRoot, p)
		if relErr != nil {
			rel = p
		}
		relPOSIX := filepath.ToSlash(rel)
		if relPOSIX == rootRel {
			return nil
		}
		if filepath.ToSlash(filepath.Dir(relPOSIX)) == "." {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			walkErr = fmt.Errorf("read %s: %w", relPOSIX, readErr)
			return walkErr
		}
		doc, parseErr := Parse(data)
		if parseErr != nil {
			walkErr = fmt.Errorf("%s: %w", relPOSIX, parseErr)
			return walkErr
		}
		// Set workspace-relative POSIX path for module key derivation.
		wsForRel := workspace
		if wsForRel == "" {
			wsForRel = "."
		}
		if rp, rpErr := filepath.Rel(wsForRel, p); rpErr == nil {
			doc.Path = filepath.ToSlash(rp)
		} else {
			doc.Path = relPOSIX
		}
		modules = append(modules, doc)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if err != nil {
		// WalkDir failure unrelated to an invalid nerd.md (e.g., missing walkRoot)
		// must not surface as an error, matching Load's "absent is not an error".
		_ = err
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Path < modules[j].Path
	})
	docs = append(docs, modules...)
	return docs, nil
}

// Parse splits frontmatter from body and strictly decodes the frontmatter.
func Parse(data []byte) (*Document, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	var spec Spec
	decoder := yaml.NewDecoder(bytes.NewReader(front))
	// The whole point. An unknown key means the author wrote a directive this
	// binary will not honour; failing here is the only way they find out.
	decoder.KnownFields(true)
	if err := decoder.Decode(&spec); err != nil {
		return nil, fmt.Errorf("invalid frontmatter: %w", err)
	}

	if err := spec.validate(); err != nil {
		return nil, err
	}

	return &Document{Spec: spec, Body: strings.TrimSpace(body)}, nil
}

const frontmatterFence = "---"

// splitFrontmatter extracts the leading `---` fenced YAML block.
func splitFrontmatter(data []byte) (front []byte, body string, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// nerd.md bodies routinely embed long lines (tables, command strings), and
	// bufio's 64 KiB default would truncate mid-document and report a
	// frontmatter error for a body problem.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	if !scanner.Scan() {
		return nil, "", fmt.Errorf("file is empty; expected a %q frontmatter block", frontmatterFence)
	}
	if strings.TrimSpace(scanner.Text()) != frontmatterFence {
		return nil, "", fmt.Errorf(
			"first line must be %q to open the frontmatter block (got %q); "+
				"nerd.md requires machine-readable frontmatter, unlike CLAUDE.md",
			frontmatterFence, truncate(scanner.Text(), 40))
	}

	var frontLines []string
	closed := false
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == frontmatterFence {
			closed = true
			break
		}
		frontLines = append(frontLines, scanner.Text())
	}
	if !closed {
		return nil, "", fmt.Errorf("frontmatter block opened with %q was never closed", frontmatterFence)
	}

	var bodyLines []string
	for scanner.Scan() {
		bodyLines = append(bodyLines, scanner.Text())
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return nil, "", fmt.Errorf("read: %w", scanErr)
	}

	return []byte(strings.Join(frontLines, "\n")), strings.Join(bodyLines, "\n"), nil
}

func (s *Spec) validate() error {
	if strings.TrimSpace(s.Schema) == "" {
		return fmt.Errorf("frontmatter is missing the required %q key; expected %q", "schema", SchemaVersion)
	}
	if s.Schema != SchemaVersion {
		return fmt.Errorf(
			"unsupported schema %q; this build speaks %q. "+
				"Refusing to half-apply a document written for a different contract",
			s.Schema, SchemaVersion)
	}

	for i, rule := range s.Forbid {
		if strings.TrimSpace(rule.Match) == "" {
			return fmt.Errorf("forbid[%d] has an empty %q; a rule that matches every path would deny every write", i, "match")
		}
		if strings.TrimSpace(rule.Reason) == "" {
			return fmt.Errorf(
				"forbid[%d] (match %q) has no %q; a denial the agent cannot explain reads as a malfunction and invites a workaround",
				i, rule.Match, "reason")
		}
	}

	for i, c := range s.Conventions {
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("conventions[%d] has an empty %q", i, "id")
		}
		if strings.TrimSpace(c.Rule) == "" {
			return fmt.Errorf("conventions[%d] (id %q) has an empty %q", i, c.ID, "rule")
		}
	}

	for i, r := range s.Require {
		if strings.TrimSpace(r) == "" {
			return fmt.Errorf("require[%d] is empty", i)
		}
	}

	if s.Northstar != nil {
		seen := make(map[string]struct{}, len(s.Northstar.Requirements))
		for i, req := range s.Northstar.Requirements {
			if strings.TrimSpace(req.ID) == "" {
				return fmt.Errorf("northstar.requirements[%d] has an empty %q", i, "id")
			}
			if _, dup := seen[req.ID]; dup {
				return fmt.Errorf("northstar.requirements[%d] has duplicate %q %q", i, "id", req.ID)
			}
			seen[req.ID] = struct{}{}
			if strings.TrimSpace(req.Statement) == "" {
				return fmt.Errorf("northstar.requirements[%d] (id %q) has an empty %q", i, req.ID, "statement")
			}
			if sev := strings.TrimSpace(req.Severity); sev != "" && sev != "blocker" && sev != "major" && sev != "minor" {
				return fmt.Errorf("northstar.requirements[%d] (id %q) has invalid %q %q; expected one of %q, %q, %q", i, req.ID, "severity", req.Severity, "blocker", "major", "minor")
			}
		}
	}

	return nil
}

// ForbidsPath reports whether any forbid rule covers target, returning the
// reason.
//
// Matching is on the slash-normalized path and is case-insensitive, because a
// safety gate that a different capitalization walks through is not a gate.
func (d *Document) ForbidsPath(target string) (reason string, forbidden bool) {
	if d == nil || strings.TrimSpace(target) == "" {
		return "", false
	}
	// Route through the shared normalizer rather than repeating it: this used
	// its own filepath.ToSlash pair, which left backslashes untouched off
	// Windows and let `.nerd\config.json` past the gate on Linux and macOS.
	normalized := normalizeForMatch(target)
	for _, rule := range d.Spec.Forbid {
		match := normalizeForMatch(rule.Match)
		if match == "" {
			continue
		}
		if strings.Contains(normalized, match) {
			return rule.Reason, true
		}
	}
	return "", false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
