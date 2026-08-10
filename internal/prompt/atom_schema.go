package prompt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PromptAtomSchemaVersion is the only YAML atom schema emitted by codeNERD.
// An omitted schema_version means version 1 so existing canonical corpora do
// not need a mechanical header on every atom. Explicit future versions fail
// closed until an adapter is added here.
const PromptAtomSchemaVersion = 1

// LegacyPromptAtomAliasesRemovalDate bounds the v0 adapter lifetime. Authors
// must migrate before this date; built-in atoms are already forbidden from
// using the adapter.
const LegacyPromptAtomAliasesRemovalDate = "2027-01-01"

// AtomDefinition is the single YAML contract used by built-in atoms, project
// atoms, agent synchronization, and validation. Pointer fields preserve the
// distinction between an explicit zero/false value and a missing required key.
type AtomDefinition struct {
	SchemaVersion int         `yaml:"schema_version,omitempty"`
	Version       AtomVersion `yaml:"version,omitempty"`

	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Subcategory string `yaml:"subcategory,omitempty"`

	Description    string `yaml:"description,omitempty"`
	ContentConcise string `yaml:"content_concise,omitempty"`
	ContentMin     string `yaml:"content_min,omitempty"`

	Priority      *int     `yaml:"priority"`
	IsMandatory   *bool    `yaml:"is_mandatory"`
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
	Models           []string `yaml:"models,omitempty"`
	Providers        []string `yaml:"providers,omitempty"`
	WorldStates      []string `yaml:"world_states,omitempty"`

	Content     string `yaml:"content,omitempty"`
	ContentFile string `yaml:"content_file,omitempty"`

	// Bounded v0 compatibility. These keys are accepted only through the
	// explicit migration path below and are never emitted by codeNERD.
	AgentTypes []string            `yaml:"agent_types,omitempty"`
	Name       string              `yaml:"name,omitempty"`
	Metadata   *legacyAtomMetadata `yaml:"metadata,omitempty"`
	Selectors  legacyAtomSelectors `yaml:"selectors,omitempty"`
}

// AtomVersion accepts the canonical positive integer atom version and the v0
// semver string used by one historical prompt document. The latter is surfaced
// as a migration, never silently coerced.
type AtomVersion struct {
	Value        int
	Present      bool
	LegacySemver bool
}

func (v AtomVersion) IsZero() bool { return !v.Present }

func (v AtomVersion) MarshalYAML() (any, error) {
	if !v.Present {
		return nil, nil
	}
	return v.Value, nil
}

func (v *AtomVersion) UnmarshalYAML(node *yaml.Node) error {
	v.Present = true
	switch node.Tag {
	case "!!int":
		value, err := strconv.Atoi(node.Value)
		if err != nil {
			return fmt.Errorf("invalid atom version %q: %w", node.Value, err)
		}
		v.Value = value
		return nil
	case "!!str":
		parts := strings.Split(node.Value, ".")
		if len(parts) != 3 {
			return fmt.Errorf("legacy atom version must be semantic x.y.z, got %q", node.Value)
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil || major < 1 {
			return fmt.Errorf("invalid legacy atom version %q", node.Value)
		}
		for _, part := range parts[1:] {
			if _, err := strconv.Atoi(part); err != nil {
				return fmt.Errorf("invalid legacy atom version %q", node.Value)
			}
		}
		v.Value = major
		v.LegacySemver = true
		return nil
	default:
		return fmt.Errorf("atom version must be a positive integer, got %s", node.Tag)
	}
}

type legacyAtomMetadata struct {
	Name        string `yaml:"name,omitempty"`
	Description string `yaml:"description,omitempty"`
	Version     string `yaml:"version,omitempty"`
}

type legacySelectorFields struct {
	Always           *bool    `yaml:"always,omitempty"`
	OperationalModes []string `yaml:"operational_modes,omitempty"`
	CampaignPhases   []string `yaml:"campaign_phases,omitempty"`
	BuildLayers      []string `yaml:"build_layers,omitempty"`
	InitPhases       []string `yaml:"init_phases,omitempty"`
	NorthstarPhases  []string `yaml:"northstar_phases,omitempty"`
	OuroborosStages  []string `yaml:"ouroboros_stages,omitempty"`
	IntentVerbs      []string `yaml:"intent_verbs,omitempty"`
	AgentTypes       []string `yaml:"agent_types,omitempty"`
	ShardTypes       []string `yaml:"shard_types,omitempty"`
	Languages        []string `yaml:"languages,omitempty"`
	Frameworks       []string `yaml:"frameworks,omitempty"`
	WorldStates      []string `yaml:"world_states,omitempty"`
}

type legacyAtomSelectors struct {
	Present bool
	legacySelectorFields
}

func (s legacyAtomSelectors) IsZero() bool { return !s.Present }

var legacySelectorKeys = map[string]struct{}{
	"always": {}, "operational_modes": {}, "campaign_phases": {},
	"build_layers": {}, "init_phases": {}, "northstar_phases": {},
	"ouroboros_stages": {}, "intent_verbs": {}, "agent_types": {},
	"shard_types": {}, "languages": {}, "frameworks": {}, "world_states": {},
}

func (s *legacyAtomSelectors) UnmarshalYAML(node *yaml.Node) error {
	s.Present = true
	nodes := []*yaml.Node{node}
	if node.Kind == yaml.SequenceNode {
		if len(node.Content) != 1 {
			return fmt.Errorf("legacy selectors list must contain exactly one mapping")
		}
		nodes = node.Content
	}
	selection := nodes[0]
	if selection.Kind != yaml.MappingNode {
		return fmt.Errorf("legacy selectors must be a mapping")
	}
	seen := make(map[string]struct{}, len(selection.Content)/2)
	for i := 0; i < len(selection.Content); i += 2 {
		key := selection.Content[i].Value
		if _, ok := legacySelectorKeys[key]; !ok {
			return fmt.Errorf("unknown legacy selector field %q", key)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate legacy selector field %q", key)
		}
		seen[key] = struct{}{}
	}
	return selection.Decode(&s.legacySelectorFields)
}

// AtomSchemaMigration reports an explicit compatibility transformation.
type AtomSchemaMigration struct {
	AtomID  string
	Code    string
	Message string
}

// ParsedPromptAtom retains the canonical definition beside its runtime form so
// validators can add policy checks without maintaining a second YAML schema.
type ParsedPromptAtom struct {
	SourcePath string
	Definition AtomDefinition
	Atom       *PromptAtom
}

// AtomContentReader resolves a content_file relative to its atom source.
type AtomContentReader func(sourcePath, contentFile string) ([]byte, error)

// ParsePromptAtomYAML strictly parses one YAML document in mapping or sequence
// form. Unknown keys, unsupported schema versions, invalid atoms, and partial
// sequence failures reject the complete document.
func ParsePromptAtomYAML(data []byte, sourcePath string, readContent AtomContentReader) ([]ParsedPromptAtom, []AtomSchemaMigration, error) {
	definitions, err := decodeAtomDefinitions(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", sourcePath, err)
	}
	if len(definitions) == 0 {
		return nil, nil, fmt.Errorf("%s: atom document is empty", sourcePath)
	}

	parsed := make([]ParsedPromptAtom, 0, len(definitions))
	var migrations []AtomSchemaMigration
	for i := range definitions {
		definition := definitions[i]
		atomMigrations, err := migrateAtomDefinition(&definition)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: atom %d: %w", sourcePath, i+1, err)
		}
		atom, err := definition.toPromptAtom(sourcePath, readContent)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: atom %d: %w", sourcePath, i+1, err)
		}
		for j := range atomMigrations {
			atomMigrations[j].AtomID = atom.ID
		}
		migrations = append(migrations, atomMigrations...)
		parsed = append(parsed, ParsedPromptAtom{
			SourcePath: sourcePath,
			Definition: definition,
			Atom:       atom,
		})
	}
	return parsed, migrations, nil
}

// ParsePromptAtomFile is the canonical filesystem adapter.
func ParsePromptAtomFile(path string) ([]ParsedPromptAtom, []AtomSchemaMigration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParsePromptAtomYAML(data, path, func(sourcePath, contentFile string) ([]byte, error) {
		return os.ReadFile(filepath.Join(filepath.Dir(sourcePath), contentFile))
	})
}

// ParsePromptAtomDirectory parses a tree in deterministic path/document order.
// Duplicate IDs are rejected before any caller can persist a partial corpus.
func ParsePromptAtomDirectory(root string) ([]ParsedPromptAtom, []AtomSchemaMigration, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("root is not a directory: %s", root)
	}

	var parsed []ParsedPromptAtom
	var migrations []AtomSchemaMigration
	seen := make(map[string]string)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		fileAtoms, fileMigrations, err := ParsePromptAtomFile(path)
		if err != nil {
			return err
		}
		for _, record := range fileAtoms {
			if previous, ok := seen[record.Atom.ID]; ok {
				return fmt.Errorf("duplicate atom id %q in %s and %s", record.Atom.ID, previous, path)
			}
			seen[record.Atom.ID] = path
		}
		parsed = append(parsed, fileAtoms...)
		migrations = append(migrations, fileMigrations...)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	atoms := make([]*PromptAtom, 0, len(parsed))
	for _, record := range parsed {
		atoms = append(atoms, record.Atom)
	}
	if err := ValidatePromptAtomSet(atoms); err != nil {
		return nil, nil, fmt.Errorf("validate atom directory %s: %w", root, err)
	}
	return parsed, migrations, nil
}

func decodeAtomDefinitions(data []byte) ([]AtomDefinition, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("empty YAML document")
	}

	var document yaml.Node
	shapeDecoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := shapeDecoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode YAML: %w", err)
	}
	var extra yaml.Node
	if err := shapeDecoder.Decode(&extra); err == nil {
		return nil, errors.New("multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode trailing YAML: %w", err)
	}
	if len(document.Content) == 0 {
		return nil, errors.New("empty YAML document")
	}

	root := document.Content[0]
	switch root.Kind {
	case yaml.SequenceNode:
		var definitions []AtomDefinition
		if err := decodeKnownFields(data, &definitions); err != nil {
			return nil, err
		}
		return definitions, nil
	case yaml.MappingNode:
		var definition AtomDefinition
		if err := decodeKnownFields(data, &definition); err != nil {
			return nil, err
		}
		return []AtomDefinition{definition}, nil
	default:
		return nil, fmt.Errorf("top-level YAML must be an atom mapping or atom sequence")
	}
}

func decodeKnownFields(data []byte, target any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("strict YAML decode failed: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing YAML: %w", err)
	}
	return nil
}

func migrateAtomDefinition(definition *AtomDefinition) ([]AtomSchemaMigration, error) {
	if definition.SchemaVersion == 0 {
		definition.SchemaVersion = PromptAtomSchemaVersion
	}
	if definition.SchemaVersion != PromptAtomSchemaVersion {
		return nil, fmt.Errorf("unsupported schema_version %d (supported: %d)", definition.SchemaVersion, PromptAtomSchemaVersion)
	}

	legacy := definition.AgentTypes != nil || definition.Name != "" || definition.Metadata != nil || definition.Selectors.Present || definition.Version.LegacySemver
	var migrations []AtomSchemaMigration
	addMigration := func(code, message string) {
		migrations = append(migrations, AtomSchemaMigration{
			Code:    code,
			Message: fmt.Sprintf("%s; compatibility alias removal date %s", message, LegacyPromptAtomAliasesRemovalDate),
		})
	}

	if definition.Version.LegacySemver {
		addMigration("legacy-semver-version", "converted semantic-string version to its integer major")
	}
	if definition.Name != "" {
		if definition.Subcategory == "" {
			definition.Subcategory = slugMetadataName(definition.Name)
		}
		addMigration("legacy-name", "mapped legacy name metadata to subcategory")
	}
	if metadata := definition.Metadata; metadata != nil {
		if metadata.Description != "" {
			if definition.Description != "" && definition.Description != metadata.Description {
				return nil, errors.New("legacy metadata.description conflicts with description")
			}
			definition.Description = metadata.Description
		}
		if metadata.Name != "" && definition.Subcategory == "" {
			definition.Subcategory = slugMetadataName(metadata.Name)
		}
		if metadata.Version != "" && !definition.Version.Present {
			major, err := legacyMajorVersion(metadata.Version)
			if err != nil {
				return nil, fmt.Errorf("legacy metadata.version: %w", err)
			}
			definition.Version = AtomVersion{Value: major, Present: true, LegacySemver: true}
		}
		addMigration("legacy-metadata", "mapped bounded legacy metadata fields to canonical atom fields")
	}

	if definition.AgentTypes != nil {
		if definition.ShardTypes != nil {
			return nil, errors.New("agent_types and shard_types cannot both be set")
		}
		definition.ShardTypes = append([]string(nil), definition.AgentTypes...)
		definition.AgentTypes = nil
		addMigration("agent-types-alias", "mapped legacy agent_types selector to shard_types")
	}
	if definition.Selectors.Present {
		if err := mergeLegacySelectors(definition); err != nil {
			return nil, err
		}
		addMigration("legacy-selectors", "flattened legacy selectors into canonical selector fields")
	}

	if legacy {
		if definition.Priority == nil {
			zero := 0
			definition.Priority = &zero
			addMigration("legacy-priority-default", "preserved legacy missing priority as zero")
		}
		if definition.IsMandatory == nil {
			mandatory := false
			definition.IsMandatory = &mandatory
			addMigration("legacy-mandatory-default", "preserved legacy missing is_mandatory as false")
		}
	}

	return migrations, nil
}

func mergeLegacySelectors(definition *AtomDefinition) error {
	legacy := definition.Selectors.legacySelectorFields
	if legacy.Always != nil {
		if !*legacy.Always {
			return errors.New("legacy selectors.always=false has no canonical selection meaning")
		}
		if legacySelectorFieldCount(legacy) > 0 {
			return errors.New("legacy selectors.always=true cannot be combined with contextual selectors")
		}
	}
	if legacy.AgentTypes != nil && legacy.ShardTypes != nil {
		return errors.New("legacy selectors.agent_types and selectors.shard_types cannot both be set")
	}
	if legacy.AgentTypes != nil {
		legacy.ShardTypes = legacy.AgentTypes
	}

	merges := []struct {
		name      string
		canonical *[]string
		legacy    []string
	}{
		{"operational_modes", &definition.OperationalModes, legacy.OperationalModes},
		{"campaign_phases", &definition.CampaignPhases, legacy.CampaignPhases},
		{"build_layers", &definition.BuildLayers, legacy.BuildLayers},
		{"init_phases", &definition.InitPhases, legacy.InitPhases},
		{"northstar_phases", &definition.NorthstarPhases, legacy.NorthstarPhases},
		{"ouroboros_stages", &definition.OuroborosStages, legacy.OuroborosStages},
		{"intent_verbs", &definition.IntentVerbs, legacy.IntentVerbs},
		{"shard_types", &definition.ShardTypes, legacy.ShardTypes},
		{"languages", &definition.Languages, legacy.Languages},
		{"frameworks", &definition.Frameworks, legacy.Frameworks},
		{"world_states", &definition.WorldStates, legacy.WorldStates},
	}
	for _, merge := range merges {
		if merge.legacy == nil {
			continue
		}
		if *merge.canonical != nil {
			return fmt.Errorf("legacy selectors.%s conflicts with top-level %s", merge.name, merge.name)
		}
		*merge.canonical = append([]string(nil), merge.legacy...)
	}
	return nil
}

func legacySelectorFieldCount(fields legacySelectorFields) int {
	return len(fields.OperationalModes) + len(fields.CampaignPhases) + len(fields.BuildLayers) +
		len(fields.InitPhases) + len(fields.NorthstarPhases) + len(fields.OuroborosStages) +
		len(fields.IntentVerbs) + len(fields.AgentTypes) + len(fields.ShardTypes) +
		len(fields.Languages) + len(fields.Frameworks) + len(fields.WorldStates)
}

func (definition AtomDefinition) toPromptAtom(sourcePath string, readContent AtomContentReader) (*PromptAtom, error) {
	definition.ID = strings.TrimSpace(definition.ID)
	if definition.ID == "" {
		return nil, errors.New("missing required field: id")
	}
	if strings.ContainsAny(definition.ID, " \t\r\n") {
		return nil, fmt.Errorf("atom id %q contains whitespace", definition.ID)
	}

	category := strings.ToLower(strings.TrimSpace(definition.Category))
	if category == "" {
		return nil, fmt.Errorf("atom %s missing required field: category", definition.ID)
	}
	if category == "domain_knowledge" {
		category = string(CategoryDomain)
	}

	if definition.Priority == nil {
		return nil, fmt.Errorf("atom %s missing required field: priority", definition.ID)
	}
	if *definition.Priority < 0 || *definition.Priority > 100 {
		return nil, fmt.Errorf("atom %s priority out of range 0..100: %d", definition.ID, *definition.Priority)
	}
	if definition.IsMandatory == nil {
		return nil, fmt.Errorf("atom %s missing required field: is_mandatory", definition.ID)
	}

	content := definition.Content
	contentFile := strings.TrimSpace(definition.ContentFile)
	if strings.TrimSpace(content) != "" && contentFile != "" {
		return nil, fmt.Errorf("atom %s sets both content and content_file", definition.ID)
	}
	if strings.TrimSpace(content) == "" && contentFile != "" {
		if filepath.IsAbs(contentFile) {
			return nil, fmt.Errorf("atom %s content_file must be relative", definition.ID)
		}
		if readContent == nil {
			return nil, fmt.Errorf("atom %s needs a content_file reader", definition.ID)
		}
		contentData, err := readContent(sourcePath, contentFile)
		if err != nil {
			return nil, fmt.Errorf("atom %s read content_file %q: %w", definition.ID, contentFile, err)
		}
		content = string(contentData)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("atom %s missing required field: content or content_file", definition.ID)
	}

	version := 1
	if definition.Version.Present {
		version = definition.Version.Value
	}
	if version < 1 {
		return nil, fmt.Errorf("atom %s version must be positive", definition.ID)
	}

	atom := &PromptAtom{
		ID:               definition.ID,
		Version:          version,
		Category:         AtomCategory(category),
		Subcategory:      strings.TrimSpace(definition.Subcategory),
		Content:          content,
		TokenCount:       EstimateTokens(content),
		ContentHash:      HashContent(content),
		Description:      definition.Description,
		ContentConcise:   definition.ContentConcise,
		ContentMin:       definition.ContentMin,
		Priority:         *definition.Priority,
		IsMandatory:      *definition.IsMandatory,
		IsExclusive:      definition.IsExclusive,
		DependsOn:        append([]string(nil), definition.DependsOn...),
		ConflictsWith:    append([]string(nil), definition.ConflictsWith...),
		OperationalModes: append([]string(nil), definition.OperationalModes...),
		CampaignPhases:   append([]string(nil), definition.CampaignPhases...),
		BuildLayers:      append([]string(nil), definition.BuildLayers...),
		InitPhases:       append([]string(nil), definition.InitPhases...),
		NorthstarPhases:  append([]string(nil), definition.NorthstarPhases...),
		OuroborosStages:  append([]string(nil), definition.OuroborosStages...),
		IntentVerbs:      append([]string(nil), definition.IntentVerbs...),
		ShardTypes:       append([]string(nil), definition.ShardTypes...),
		Languages:        append([]string(nil), definition.Languages...),
		Frameworks:       append([]string(nil), definition.Frameworks...),
		Models:           append([]string(nil), definition.Models...),
		Providers:        append([]string(nil), definition.Providers...),
		WorldStates:      append([]string(nil), definition.WorldStates...),
		CreatedAt:        time.Now(),
	}
	selectorLists := []struct {
		name   string
		values []string
		slash  bool
	}{
		{"operational_modes", atom.OperationalModes, true},
		{"campaign_phases", atom.CampaignPhases, true},
		{"build_layers", atom.BuildLayers, true},
		{"init_phases", atom.InitPhases, true},
		{"northstar_phases", atom.NorthstarPhases, true},
		{"ouroboros_stages", atom.OuroborosStages, true},
		{"intent_verbs", atom.IntentVerbs, true},
		{"shard_types", atom.ShardTypes, true},
		{"languages", atom.Languages, true},
		{"frameworks", atom.Frameworks, true},
		{"models", atom.Models, true},
		{"providers", atom.Providers, true},
		{"world_states", atom.WorldStates, false},
		{"depends_on", atom.DependsOn, false},
		{"conflicts_with", atom.ConflictsWith, false},
	}
	for _, list := range selectorLists {
		if err := validateAtomStringList(list.name, list.values, list.slash); err != nil {
			return nil, fmt.Errorf("atom %s: %w", atom.ID, err)
		}
	}
	if err := validateWorldStateSelectors(atom.WorldStates); err != nil {
		return nil, fmt.Errorf("atom %s: %w", atom.ID, err)
	}
	if err := atom.Validate(); err != nil {
		return nil, err
	}
	atom.NormalizeSelectors()
	return atom, nil
}

func validateAtomStringList(name string, values []string, normalizeSlash bool) error {
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if normalizeSlash {
			if name == "models" || name == "providers" {
				normalized := NormalizeSelectorAtom(value)
				if value != normalized {
					return fmt.Errorf("%s contains invalid value %q: must be a valid Mangle atom (lowercase, slash-prefixed, letters/digits/underscore only), did you mean %q?", name, raw, normalized)
				}
				value = strings.TrimPrefix(value, "/")
			} else {
				value = strings.TrimPrefix(value, "/")
			}
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%s contains duplicate value %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

// ValidatePromptAtomSet enforces invariants that require the complete corpus.
// Runtime built-ins and filesystem corpus loads call this before publication.
func ValidatePromptAtomSet(atoms []*PromptAtom) error {
	known := make(map[string]struct{}, len(atoms))
	for _, atom := range atoms {
		if atom == nil {
			return errors.New("atom set contains nil atom")
		}
		if _, ok := known[atom.ID]; ok {
			return fmt.Errorf("duplicate atom id %q", atom.ID)
		}
		known[atom.ID] = struct{}{}
	}
	for _, atom := range atoms {
		for _, dependency := range atom.DependsOn {
			if _, ok := known[dependency]; !ok {
				return fmt.Errorf("atom %q depends on missing atom %q", atom.ID, dependency)
			}
		}
		for _, conflict := range atom.ConflictsWith {
			if _, ok := known[conflict]; !ok {
				return fmt.Errorf("atom %q conflicts with missing atom %q", atom.ID, conflict)
			}
		}
	}
	return nil
}

func validateWorldStateSelectors(values []string) error {
	known := KnownWorldStates()
	for _, raw := range values {
		value := strings.TrimPrefix(strings.TrimSpace(raw), "/")
		if value == "" {
			return errors.New("world_states contains an empty value")
		}
		if _, ok := known[value]; !ok {
			return fmt.Errorf("world_states contains unknown value %q", value)
		}
	}
	return nil
}

// KnownWorldStates derives the closed world-state vocabulary from the live
// typed CompilationContext dimensions instead of duplicating it in tooling.
func KnownWorldStates() map[string]struct{} {
	known := make(map[string]struct{})
	for _, dimension := range AllContextDimensions() {
		if dimension.Name != "world_state" {
			continue
		}
		for _, value := range dimension.Values {
			known[strings.TrimPrefix(value, "/")] = struct{}{}
		}
	}
	return known
}

func legacyMajorVersion(value string) (int, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("must be semantic x.y.z, got %q", value)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 1 {
		return 0, fmt.Errorf("invalid semantic version %q", value)
	}
	return major, nil
}

var metadataSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugMetadataName(value string) string {
	value = strings.Trim(metadataSlugPattern.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "_"), "_")
	return value
}

// NormalizeSelectorAtom converts a free-form identifier such as a model name
// into the Mangle atom form used by atom selectors.
func NormalizeSelectorAtom(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	if strings.HasPrefix(s, "/") {
		s = s[1:]
	}
	var b strings.Builder
	b.Grow(len(s) + 1)
	prevUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			prevUnderscore = false
		} else {
			if !prevUnderscore {
				b.WriteRune('_')
				prevUnderscore = true
			}
		}
	}
	return "/" + b.String()
}
