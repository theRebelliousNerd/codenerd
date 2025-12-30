// Package prompt - Embedded corpus loader for baked-in prompt atoms.
// This file uses go:embed to bake prompt atoms into the binary at compile time,
// eliminating filesystem dependencies for built-in prompts.
package prompt

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// embeddedAtoms contains all YAML files from atoms/ baked into the binary.
// The atoms directory is a subdirectory of this package.
//
//go:embed atoms
var embeddedAtoms embed.FS

// LoadEmbeddedCorpus loads the baked-in prompt atoms from the embedded filesystem.
// This is called at startup to initialize the JIT compiler with built-in atoms.
// Returns an EmbeddedCorpus containing all atoms from internal/prompt/atoms/.
func LoadEmbeddedCorpus() (*EmbeddedCorpus, error) {
	timer := logging.StartTimer(logging.CategoryStore, "LoadEmbeddedCorpus")
	defer timer.Stop()

	logging.Get(logging.CategoryStore).Info("Loading embedded prompt corpus")

	var allAtoms []*PromptAtom

	// Walk the embedded filesystem
	err := fs.WalkDir(embeddedAtoms, "atoms", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Read and parse the file
		atoms, parseErr := parseEmbeddedYAML(path)
		if parseErr != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to parse embedded YAML %s: %v", path, parseErr)
			return nil // Continue with other files
		}

		allAtoms = append(allAtoms, atoms...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk embedded atoms: %w", err)
	}

	logging.Get(logging.CategoryStore).Info("Loaded %d atoms from embedded corpus", len(allAtoms))

	return NewEmbeddedCorpus(allAtoms), nil
}

// parseEmbeddedYAML parses a YAML file from the embedded filesystem.
func parseEmbeddedYAML(path string) ([]*PromptAtom, error) {
	data, err := embeddedAtoms.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded file: %w", err)
	}

	// Parse as array of atoms
	var rawAtoms []embeddedYAMLAtom
	if err := yaml.Unmarshal(data, &rawAtoms); err != nil {
		// Try parsing as single atom
		var single embeddedYAMLAtom
		if singleErr := yaml.Unmarshal(data, &single); singleErr != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		rawAtoms = []embeddedYAMLAtom{single}
	}

	// Convert to PromptAtom structs
	var atoms []*PromptAtom
	for _, raw := range rawAtoms {
                atom, err := convertEmbeddedAtom(raw, path)
                if err != nil {
                        logging.Get(logging.CategoryStore).Error("Skipping invalid atom in %s: %v", path, err)
                        continue
                }
		atoms = append(atoms, atom)
	}

	return atoms, nil
}

// embeddedYAMLAtom matches the YAML structure in atoms/*.yaml.
// This is a copy of yamlAtomDefinition to avoid import cycles.
type embeddedYAMLAtom struct {
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

	// Polymorphism / semantic embedding helpers
	Description    string `yaml:"description,omitempty"`
	ContentConcise string `yaml:"content_concise,omitempty"`
	ContentMin     string `yaml:"content_min,omitempty"`

	Priority      int      `yaml:"priority"`
	IsMandatory   bool     `yaml:"is_mandatory"`
	IsExclusive   string   `yaml:"is_exclusive,omitempty"`
	DependsOn     []string `yaml:"depends_on,omitempty"`
	ConflictsWith []string `yaml:"conflicts_with,omitempty"`

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

	// Content can be inline or reference a file relative to this YAML file.
	Content     string `yaml:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty"`
}

// convertEmbeddedAtom converts an embedded YAML atom definition to a PromptAtom.
func convertEmbeddedAtom(raw embeddedYAMLAtom, sourcePath string) (*PromptAtom, error) {
	if raw.ID == "" {
		return nil, fmt.Errorf("atom missing ID")
	}

	if raw.Category == "" {
		return nil, fmt.Errorf("atom %s missing category", raw.ID)
	}

	// Resolve content (inline or referenced file)
	content := raw.Content
	if raw.ContentFile != "" && strings.TrimSpace(content) == "" {
		// Use slash-separated paths for embedded FS access.
		contentPath := path.Join(path.Dir(sourcePath), raw.ContentFile)
		contentData, err := embeddedAtoms.ReadFile(contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded content file %s: %w", raw.ContentFile, err)
		}
		content = string(contentData)
	}

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("atom %s has no content", raw.ID)
	}

	// Compute token count and hash
	tokenCount := EstimateTokens(content)
	contentHash := HashContent(content)

	atom := &PromptAtom{
		ID:               raw.ID,
		Version:          1,
		Category:         AtomCategory(raw.Category),
		Subcategory:      raw.Subcategory,
		Content:          content,
		TokenCount:       tokenCount,
		ContentHash:      contentHash,
		Description:      raw.Description,
		ContentConcise:   raw.ContentConcise,
		ContentMin:       raw.ContentMin,
		Priority:         raw.Priority,
		IsMandatory:      raw.IsMandatory,
		IsExclusive:      raw.IsExclusive,
		DependsOn:        raw.DependsOn,
		ConflictsWith:    raw.ConflictsWith,
		OperationalModes: raw.OperationalModes,
		CampaignPhases:   raw.CampaignPhases,
		BuildLayers:      raw.BuildLayers,
		InitPhases:       raw.InitPhases,
		NorthstarPhases:  raw.NorthstarPhases,
		OuroborosStages:  raw.OuroborosStages,
		IntentVerbs:      raw.IntentVerbs,
		ShardTypes:       raw.ShardTypes,
		Languages:        raw.Languages,
		Frameworks:       raw.Frameworks,
		WorldStates:      raw.WorldStates,
		CreatedAt:        time.Now(),
	}

	// Validate
	if err := atom.Validate(); err != nil {
		return nil, fmt.Errorf("invalid atom: %w", err)
	}

	return atom, nil
}

// MustLoadEmbeddedCorpus loads the embedded corpus and panics on error.
// Use this for initialization where failure is unrecoverable.
func MustLoadEmbeddedCorpus() *EmbeddedCorpus {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		panic(fmt.Sprintf("failed to load embedded corpus: %v", err))
	}
	return corpus
}

// GetEmbeddedAtomCount returns the number of atoms in the embedded corpus.
// Useful for diagnostics and testing.
func GetEmbeddedAtomCount() (int, error) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return 0, err
	}
	return corpus.Count(), nil
}
