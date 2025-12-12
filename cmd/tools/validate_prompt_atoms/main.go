// Package main implements a validator for prompt atom YAML files used by the JIT prompt compiler.
//
// This tool is intentionally strict about schema:
// - unknown YAML fields are treated as errors (to catch typos like init_phase vs init_phases)
// - required fields are enforced (id, category, priority, is_mandatory, content/content_file)
//
// Usage:
//
//	go run ./cmd/tools/validate_prompt_atoms -root internal/prompt/atoms
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/prompt"

	"gopkg.in/yaml.v3"
)

type atomDefinition struct {
	// Core identity
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

	// Polymorphism / semantic embedding helpers
	Description    string `yaml:"description,omitempty"`
	ContentConcise string `yaml:"content_concise,omitempty"`
	ContentMin     string `yaml:"content_min,omitempty"`

	// Composition
	Priority      *int     `yaml:"priority"`
	IsMandatory   *bool    `yaml:"is_mandatory"`
	IsExclusive   string   `yaml:"is_exclusive,omitempty"`
	DependsOn     []string `yaml:"depends_on,omitempty"`
	ConflictsWith []string `yaml:"conflicts_with,omitempty"`

	// Contextual Selectors
	OperationalModes []string `yaml:"operational_modes,omitempty"`
	CampaignPhases   []string `yaml:"campaign_phases,omitempty"`
	BuildLayers      []string `yaml:"build_layers,omitempty"`
	InitPhases       []string `yaml:"init_phases,omitempty"`
	NorthstarPhases  []string `yaml:"northstar_phases,omitempty"`
	OuroborosStages  []string `yaml:"ouroboros_stages,omitempty"`
	IntentVerbs      []string `yaml:"intent_verbs,omitempty"`
	ShardTypes       []string `yaml:"shard_types,omitempty"`
	Languages        []string `yaml:"languages,omitempty"`
	Frameworks       []string `yaml:"frameworks,omitempty"`
	WorldStates      []string `yaml:"world_states,omitempty"`

	// Content (inline or file-backed)
	Content     string `yaml:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty"`
}

type issueSeverity string

const (
	severityError   issueSeverity = "error"
	severityWarning issueSeverity = "warning"
)

type issue struct {
	Severity issueSeverity
	File     string
	AtomID   string
	Message  string
}

func main() {
	root := flag.String("root", "internal/prompt/atoms", "Root directory containing YAML atom files")
	failOnWarn := flag.Bool("fail-on-warn", false, "Exit non-zero if warnings are present")
	warnNoncanonicalSelectors := flag.Bool("warn-noncanonical-selectors", false, "Warn on non-canonical selector tags (e.g., missing leading '/')")
	enforceCanonicalSelectors := flag.Bool("enforce-canonical-selectors", false, "Treat non-canonical selector tags as errors")
	checkRecommendedSelectors := flag.Bool("check-recommended-selectors", true, "Warn when directory-scoped selector fields are missing (e.g., campaign_phases under /campaign)")
	flag.Parse()

	opts := validationOptions{
		WarnNoncanonicalSelectors: *warnNoncanonicalSelectors,
		EnforceCanonicalSelectors: *enforceCanonicalSelectors,
		CheckRecommendedSelectors: *checkRecommendedSelectors,
	}

	issues, stats, err := validateAtomTree(*root, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate_prompt_atoms: %v\n", err)
		os.Exit(2)
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].File != issues[j].File {
			return issues[i].File < issues[j].File
		}
		if issues[i].AtomID != issues[j].AtomID {
			return issues[i].AtomID < issues[j].AtomID
		}
		return issues[i].Message < issues[j].Message
	})

	var errCount, warnCount int
	for _, it := range issues {
		switch it.Severity {
		case severityError:
			errCount++
		case severityWarning:
			warnCount++
		}
	}

	fmt.Printf("Validated %d YAML files, %d atoms\n", stats.Files, stats.Atoms)
	if errCount == 0 && warnCount == 0 {
		fmt.Println("OK: no issues found")
		return
	}
	fmt.Printf("Issues: %d errors, %d warnings\n", errCount, warnCount)
	for _, it := range issues {
		loc := it.File
		if it.AtomID != "" {
			loc = fmt.Sprintf("%s (%s)", it.File, it.AtomID)
		}
		fmt.Printf("- %s: %s: %s\n", it.Severity, loc, it.Message)
	}

	if errCount > 0 || (*failOnWarn && warnCount > 0) {
		os.Exit(1)
	}
}

type validationStats struct {
	Files int
	Atoms int
}

type atomRecord struct {
	File string
	Atom atomDefinition
}

type validationOptions struct {
	WarnNoncanonicalSelectors bool
	EnforceCanonicalSelectors bool
	CheckRecommendedSelectors bool
}

func validateAtomTree(root string, opts validationOptions) ([]issue, validationStats, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, validationStats{}, err
	}
	if !info.IsDir() {
		return nil, validationStats{}, fmt.Errorf("root is not a directory: %s", root)
	}

	validCategories := make(map[string]struct{})
	for _, cat := range prompt.AllCategories() {
		validCategories[string(cat)] = struct{}{}
	}

	records := make([]atomRecord, 0, 512)
	issues := make([]issue, 0, 128)
	stats := validationStats{}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			issues = append(issues, issue{Severity: severityError, File: path, Message: walkErr.Error()})
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		stats.Files++
		defs, parseIssues := parseAtomYAMLFile(path)
		issues = append(issues, parseIssues...)
		relPath, _ := filepath.Rel(root, path)
		for _, def := range defs {
			stats.Atoms++
			issues = append(issues, validateAtomDef(path, relPath, def, validCategories, opts)...)
			records = append(records, atomRecord{File: path, Atom: def})
		}
		return nil
	})
	if err != nil {
		return nil, validationStats{}, err
	}

	// Cross-file validation: duplicate IDs, dependencies, conflicts.
	idToFile := make(map[string]string, len(records))
	for _, rec := range records {
		if rec.Atom.ID == "" {
			continue
		}
		if existing, ok := idToFile[rec.Atom.ID]; ok {
			issues = append(issues, issue{
				Severity: severityError,
				File:     rec.File,
				AtomID:   rec.Atom.ID,
				Message:  fmt.Sprintf("duplicate atom id (also in %s)", existing),
			})
			continue
		}
		idToFile[rec.Atom.ID] = rec.File
	}

	for _, rec := range records {
		id := rec.Atom.ID
		for _, dep := range rec.Atom.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				issues = append(issues, issue{Severity: severityError, File: rec.File, AtomID: id, Message: "depends_on contains empty id"})
				continue
			}
			if dep == id {
				issues = append(issues, issue{Severity: severityError, File: rec.File, AtomID: id, Message: "depends_on contains self"})
				continue
			}
			if _, ok := idToFile[dep]; !ok {
				issues = append(issues, issue{Severity: severityError, File: rec.File, AtomID: id, Message: fmt.Sprintf("depends_on references missing atom %q", dep)})
			}
		}
		for _, conflict := range rec.Atom.ConflictsWith {
			conflict = strings.TrimSpace(conflict)
			if conflict == "" {
				issues = append(issues, issue{Severity: severityError, File: rec.File, AtomID: id, Message: "conflicts_with contains empty id"})
				continue
			}
			if conflict == id {
				issues = append(issues, issue{Severity: severityError, File: rec.File, AtomID: id, Message: "conflicts_with contains self"})
				continue
			}
			if _, ok := idToFile[conflict]; !ok {
				issues = append(issues, issue{Severity: severityWarning, File: rec.File, AtomID: id, Message: fmt.Sprintf("conflicts_with references missing atom %q", conflict)})
			}
		}
	}

	return issues, stats, nil
}

func parseAtomYAMLFile(path string) ([]atomDefinition, []issue) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []issue{{Severity: severityError, File: path, Message: fmt.Sprintf("read failed: %v", err)}}
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, []issue{{Severity: severityWarning, File: path, Message: "empty YAML file"}}
	}

	// First try sequence form (most files).
	var defs []atomDefinition
	if err := decodeKnownFields(data, &defs); err == nil {
		return defs, nil
	}

	// Then try single atom mapping.
	var single atomDefinition
	if err2 := decodeKnownFields(data, &single); err2 == nil {
		return []atomDefinition{single}, nil
	}

	// Provide a single consolidated parse error.
	return nil, []issue{{Severity: severityError, File: path, Message: "YAML parse failed (check unknown fields, types, or structure)"}}
}

func decodeKnownFields(data []byte, out interface{}) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		// YAML library returns io.EOF when there is no document.
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	// Disallow multiple YAML documents in a single file (hard to lint consistently).
	var extra interface{}
	if err := dec.Decode(&extra); err == nil {
		return fmt.Errorf("multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed after first YAML document: %w", err)
	}
	return nil
}

func validateAtomDef(path, relPath string, def atomDefinition, validCategories map[string]struct{}, opts validationOptions) []issue {
	var issues []issue
	atomID := strings.TrimSpace(def.ID)

	if atomID == "" {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "missing required field: id"})
	} else if strings.ContainsAny(atomID, " \t\r\n") {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "id contains whitespace"})
	}

	cat := strings.TrimSpace(def.Category)
	if cat == "" {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "missing required field: category"})
	} else if _, ok := validCategories[cat]; !ok {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: fmt.Sprintf("unknown category %q", cat)})
	}

	if def.Priority == nil {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "missing required field: priority"})
	} else if *def.Priority < 0 || *def.Priority > 100 {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: fmt.Sprintf("priority out of range (expected 0..100): %d", *def.Priority)})
	}

	if def.IsMandatory == nil {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "missing required field: is_mandatory"})
	}

	// Content: require either inline content or a content_file.
	inline := strings.TrimSpace(def.Content)
	contentFile := strings.TrimSpace(def.ContentFile)

	if inline == "" && contentFile == "" {
		issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "missing required field: content or content_file"})
	} else if inline != "" && contentFile != "" {
		issues = append(issues, issue{Severity: severityWarning, File: path, AtomID: atomID, Message: "both content and content_file are set; content_file will be ignored"})
	}

	if inline == "" && contentFile != "" {
		// Validate referenced content file exists.
		if filepath.IsAbs(contentFile) {
			issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "content_file must be relative (not absolute)"})
		} else {
			full := filepath.Join(filepath.Dir(path), contentFile)
			b, err := os.ReadFile(full)
			if err != nil {
				issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: fmt.Sprintf("content_file read failed: %v", err)})
			} else if strings.TrimSpace(string(b)) == "" {
				issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: "content_file is empty"})
			}
		}
	}

	issues = append(issues, validateSelectorList(path, atomID, "operational_modes", def.OperationalModes, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "campaign_phases", def.CampaignPhases, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "build_layers", def.BuildLayers, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "init_phases", def.InitPhases, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "northstar_phases", def.NorthstarPhases, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "ouroboros_stages", def.OuroborosStages, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "intent_verbs", def.IntentVerbs, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "shard_types", def.ShardTypes, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "languages", def.Languages, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "frameworks", def.Frameworks, selectorStyleSlashPref, opts)...)
	issues = append(issues, validateSelectorList(path, atomID, "world_states", def.WorldStates, selectorStyleNoSlash, opts)...)
	issues = append(issues, validateWorldStatesKnownSet(path, atomID, def.WorldStates)...)

	if opts.CheckRecommendedSelectors {
		issues = append(issues, validateRecommendedSelectors(path, relPath, atomID, def)...)
	}

	return issues
}

type selectorStyle int

const (
	selectorStyleAny selectorStyle = iota
	selectorStyleSlashPref
	selectorStyleNoSlash
)

func validateSelectorList(path, atomID, field string, values []string, canonical selectorStyle, opts validationOptions) []issue {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	var issues []issue
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: fmt.Sprintf("%s contains empty value", field)})
			continue
		}
		if _, ok := seen[v]; ok {
			issues = append(issues, issue{Severity: severityWarning, File: path, AtomID: atomID, Message: fmt.Sprintf("%s contains duplicate value %q", field, v)})
			continue
		}
		seen[v] = struct{}{}

		hasSlash := strings.HasPrefix(v, "/")

		noncanonical := false
		switch canonical {
		case selectorStyleSlashPref:
			noncanonical = !hasSlash
		case selectorStyleNoSlash:
			noncanonical = hasSlash
		default:
			noncanonical = false
		}

		if noncanonical {
			want := "any"
			switch canonical {
			case selectorStyleSlashPref:
				want = "leading '/'"
			case selectorStyleNoSlash:
				want = "no leading '/'"
			}

			if opts.EnforceCanonicalSelectors {
				issues = append(issues, issue{Severity: severityError, File: path, AtomID: atomID, Message: fmt.Sprintf("%s value is non-canonical (%s): %q", field, want, v)})
			} else if opts.WarnNoncanonicalSelectors {
				issues = append(issues, issue{Severity: severityWarning, File: path, AtomID: atomID, Message: fmt.Sprintf("%s value is non-canonical (%s): %q", field, want, v)})
			}
		}
	}
	return issues
}

func validateWorldStatesKnownSet(path, atomID string, values []string) []issue {
	if len(values) == 0 {
		return nil
	}
	known := map[string]struct{}{
		"failing_tests":   {},
		"diagnostics":     {},
		"large_refactor":  {},
		"security_issues": {},
		"new_files":       {},
		"high_churn":      {},
	}
	var issues []issue
	for _, raw := range values {
		v := strings.TrimPrefix(strings.TrimSpace(raw), "/")
		if v == "" {
			continue
		}
		if _, ok := known[v]; !ok {
			issues = append(issues, issue{Severity: severityWarning, File: path, AtomID: atomID, Message: fmt.Sprintf("world_states contains unknown value %q", v)})
		}
	}
	return issues
}

func validateRecommendedSelectors(path, relPath, atomID string, def atomDefinition) []issue {
	if atomID == "" {
		return nil
	}

	// Normalize to forward slashes for portable checks.
	p := strings.ToLower(filepath.ToSlash(relPath))

	var issues []issue
	warnMissing := func(field, hint string) {
		issues = append(issues, issue{
			Severity: severityWarning,
			File:     path,
			AtomID:   atomID,
			Message:  fmt.Sprintf("missing recommended field: %s (%s)", field, hint),
		})
	}

	// Directory-scoped expectations. These are warnings by default because some
	// atoms are intentionally global.
	switch {
	case strings.Contains(p, "/campaign/") && len(def.CampaignPhases) == 0:
		warnMissing("campaign_phases", "atoms under /campaign should scope to phases")
	case strings.Contains(p, "/init/") && len(def.InitPhases) == 0:
		warnMissing("init_phases", "atoms under /init should scope to init phases")
	case strings.Contains(p, "/northstar/") && len(def.NorthstarPhases) == 0:
		warnMissing("northstar_phases", "atoms under /northstar should scope to phases")
	case strings.Contains(p, "/ouroboros/") && len(def.OuroborosStages) == 0:
		warnMissing("ouroboros_stages", "atoms under /ouroboros should scope to stages")
	case strings.Contains(p, "/build_layer/") && len(def.BuildLayers) == 0:
		warnMissing("build_layers", "atoms under /build_layer should scope to layers")
	case strings.Contains(p, "/intent/") && len(def.IntentVerbs) == 0:
		warnMissing("intent_verbs", "atoms under /intent should scope to verbs")
	case strings.Contains(p, "/language/") && len(def.Languages) == 0:
		warnMissing("languages", "atoms under /language should scope to languages")
	case strings.Contains(p, "/framework/") && len(def.Frameworks) == 0:
		warnMissing("frameworks", "atoms under /framework should scope to frameworks")
	case strings.Contains(p, "/world_state/") && len(def.WorldStates) == 0:
		warnMissing("world_states", "atoms under /world_state should scope to states")
	case strings.Contains(p, "/shards/") && len(def.ShardTypes) == 0:
		warnMissing("shard_types", "atoms under /shards should scope to shard types")
	}

	return issues
}
