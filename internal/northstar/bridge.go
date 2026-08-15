package northstar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// SINGLE VISION AUTHORITY
// =============================================================================
//
// DECISION (northstar TODO P0 "Single vision authority").
//
// There were three artifacts holding the same truth and no arrow between them:
//
//	.nerd/northstar.json  written by the chat wizard and `nerd northstar load`
//	.nerd/northstar.mg    written by the same two writers
//	.nerd/northstar_knowledge.db  written by Store.SaveVision, read by the
//	                              Guardian, /alignment and the campaign risk gate
//
// A user who ran the wizard configured the JSON. The Guardian read the SQLite
// row, which nothing wrote. /alignment therefore reported "No vision defined"
// against a project that had one, and campaigns scored risk against an empty
// vision. Reverse case: `nerd northstar load` wrote the DB, then the wizard's
// JSON reload showed stale content.
//
// The authority is now stated once, here, and enforced by SyncVisionAuthority:
//
//	The Mangle kernel is the executive: everything that DECIDES reads
//	northstar_* facts. Those facts are projected from exactly one place, the
//	SQLite Store, which is therefore the durable record. .nerd/northstar.json
//	and .nerd/northstar.mg are an IMPORT surface (a human or a wizard edits
//	them) and an EXPORT surface (so operators can read and diff the vision) --
//	never an authority anything reads to decide.
//
// Reconciliation is bidirectional because both surfaces have legitimate
// writers, and last-writer-wins on wall clock is the only ordering available
// between a file and a row. Ties and equal content resolve to "do nothing" so
// that a boot never rewrites files it did not change.
//
// Guardian.Initialize calls SyncVisionAuthority, so every boot path -- chat,
// shared chat, campaign observer, CLI -- converges on the same vision without
// each one remembering to.

// VisionSyncDirection reports which way SyncVisionAuthority moved the vision.
type VisionSyncDirection string

const (
	// SyncNoop means both surfaces already agreed (or neither exists).
	SyncNoop VisionSyncDirection = "noop"
	// SyncImported means the JSON file was newer and was written into the store.
	SyncImported VisionSyncDirection = "imported"
	// SyncExported means the store was newer and was written out to JSON + .mg.
	SyncExported VisionSyncDirection = "exported"
)

// VisionSyncResult describes what SyncVisionAuthority did.
type VisionSyncResult struct {
	Direction VisionSyncDirection
	Vision    *Vision
	JSONPath  string
	ManglePat string
}

// VisionJSONFileName / VisionMangleFileName are the operator-facing surfaces.
const (
	VisionJSONFileName   = "northstar.json"
	VisionMangleFileName = "northstar.mg"
)

// WizardDocument is the on-disk shape of .nerd/northstar.json.
//
// It mirrors chat.NorthstarWizardState (capitalised keys, no capability/risk
// IDs) because that is what the wizard marshals. Keeping the decoder here --
// rather than a third private copy in cmd/nerd -- is the point: every reader of
// the JSON surface now agrees on its shape.
type WizardDocument struct {
	Mission        string              `json:"Mission"`
	Problem        string              `json:"Problem"`
	Vision         string              `json:"Vision"`
	Personas       []WizardPersona     `json:"Personas"`
	Capabilities   []WizardCapability  `json:"Capabilities"`
	Risks          []WizardRisk        `json:"Risks"`
	Requirements   []WizardRequirement `json:"Requirements"`
	Constraints    []string            `json:"Constraints"`
	ResearchDocs   []string            `json:"ResearchDocs,omitempty"`
	ExtractedFacts []string            `json:"ExtractedFacts,omitempty"`
	Meta           *WizardDocumentMeta `json:"Meta,omitempty"`
}

// WizardDocumentMeta carries the store timestamps through the JSON surface so
// an export followed by an import is a fixed point rather than a clock reset.
type WizardDocumentMeta struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WizardPersona mirrors the wizard's persona shape.
type WizardPersona struct {
	Name       string   `json:"name"`
	PainPoints []string `json:"pain_points"`
	Needs      []string `json:"needs"`
}

// WizardCapability mirrors the wizard's capability shape. ID is optional: the
// wizard does not write one, so importers synthesise cap_<n>.
type WizardCapability struct {
	ID          string   `json:"id,omitempty"`
	Description string   `json:"description"`
	Timeline    string   `json:"timeline"`
	Priority    string   `json:"priority"`
	Serves      []string `json:"serves,omitempty"`
}

// WizardRisk mirrors the wizard's risk shape.
type WizardRisk struct {
	ID          string `json:"id,omitempty"`
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation"`
}

// WizardRequirement mirrors the wizard's requirement shape.
type WizardRequirement struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Priority    string   `json:"priority"`
	Source      string   `json:"source,omitempty"`
	Supports    []string `json:"supports,omitempty"`
	Addresses   []string `json:"addresses,omitempty"`
}

// ToVision converts the on-disk document into the canonical domain model,
// synthesising the IDs the JSON surface does not carry.
func (d *WizardDocument) ToVision() *Vision {
	if d == nil {
		return nil
	}
	v := &Vision{
		Mission:      strings.TrimSpace(d.Mission),
		Problem:      strings.TrimSpace(d.Problem),
		VisionStmt:   strings.TrimSpace(d.Vision),
		Personas:     make([]Persona, 0, len(d.Personas)),
		Capabilities: make([]Capability, 0, len(d.Capabilities)),
		Risks:        make([]Risk, 0, len(d.Risks)),
		Requirements: make([]Requirement, 0, len(d.Requirements)),
		Constraints:  append([]string(nil), d.Constraints...),
	}
	if v.Constraints == nil {
		v.Constraints = []string{}
	}
	for _, p := range d.Personas {
		v.Personas = append(v.Personas, Persona{
			Name:       p.Name,
			PainPoints: append([]string(nil), p.PainPoints...),
			Needs:      append([]string(nil), p.Needs...),
		})
	}
	for i, c := range d.Capabilities {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			id = fmt.Sprintf("cap_%d", i+1)
		}
		v.Capabilities = append(v.Capabilities, Capability{
			ID:          id,
			Description: c.Description,
			Timeline:    c.Timeline,
			Priority:    c.Priority,
			Serves:      append([]string(nil), c.Serves...),
		})
	}
	for i, r := range d.Risks {
		id := strings.TrimSpace(r.ID)
		if id == "" {
			id = fmt.Sprintf("risk_%d", i+1)
		}
		v.Risks = append(v.Risks, Risk{
			ID:          id,
			Description: r.Description,
			Likelihood:  r.Likelihood,
			Impact:      r.Impact,
			Mitigation:  r.Mitigation,
		})
	}
	for i, r := range d.Requirements {
		id := strings.ToLower(strings.TrimSpace(r.ID))
		if id == "" {
			id = fmt.Sprintf("req_%d", i+1)
		}
		v.Requirements = append(v.Requirements, Requirement{
			ID:          id,
			Type:        r.Type,
			Description: r.Description,
			Priority:    r.Priority,
			Supports:    append([]string(nil), r.Supports...),
			Addresses:   append([]string(nil), r.Addresses...),
		})
	}
	if d.Meta != nil {
		v.CreatedAt = d.Meta.CreatedAt
		v.UpdatedAt = d.Meta.UpdatedAt
	}
	return v
}

// WizardDocumentFromVision renders the canonical model back onto the JSON
// surface. IDs are preserved so an export/import round trip is stable.
func WizardDocumentFromVision(v *Vision) *WizardDocument {
	if v == nil {
		return nil
	}
	d := &WizardDocument{
		Mission:     v.Mission,
		Problem:     v.Problem,
		Vision:      v.VisionStmt,
		Constraints: append([]string(nil), v.Constraints...),
		Meta:        &WizardDocumentMeta{CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt},
	}
	if d.Constraints == nil {
		d.Constraints = []string{}
	}
	for _, p := range v.Personas {
		d.Personas = append(d.Personas, WizardPersona{
			Name:       p.Name,
			PainPoints: append([]string(nil), p.PainPoints...),
			Needs:      append([]string(nil), p.Needs...),
		})
	}
	for _, c := range v.Capabilities {
		d.Capabilities = append(d.Capabilities, WizardCapability{
			ID:          c.ID,
			Description: c.Description,
			Timeline:    c.Timeline,
			Priority:    c.Priority,
			Serves:      append([]string(nil), c.Serves...),
		})
	}
	for _, r := range v.Risks {
		d.Risks = append(d.Risks, WizardRisk{
			ID:          r.ID,
			Description: r.Description,
			Likelihood:  r.Likelihood,
			Impact:      r.Impact,
			Mitigation:  r.Mitigation,
		})
	}
	for _, r := range v.Requirements {
		d.Requirements = append(d.Requirements, WizardRequirement{
			ID:          r.ID,
			Type:        r.Type,
			Description: r.Description,
			Priority:    r.Priority,
			Supports:    append([]string(nil), r.Supports...),
			Addresses:   append([]string(nil), r.Addresses...),
		})
	}
	if d.Personas == nil {
		d.Personas = []WizardPersona{}
	}
	if d.Capabilities == nil {
		d.Capabilities = []WizardCapability{}
	}
	if d.Risks == nil {
		d.Risks = []WizardRisk{}
	}
	if d.Requirements == nil {
		d.Requirements = []WizardRequirement{}
	}
	return d
}

// LoadVisionJSON reads .nerd/northstar.json and returns the vision it holds.
// Returns (nil, nil) when the file does not exist -- an absent import surface is
// not an error, it just means the store is the only writer.
func LoadVisionJSON(nerdDir string) (*Vision, error) {
	path := filepath.Join(nerdDir, VisionJSONFileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc WizardDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	v := doc.ToVision()
	if v == nil || v.Mission == "" {
		// A JSON file with no mission is a half-finished wizard run, not a
		// vision. Importing it would flip northstar_defined() on and start
		// gating campaigns against nothing.
		return nil, nil
	}
	return v, nil
}

// WriteVisionJSON writes the export surface. It is a no-op when the file
// already holds an equivalent vision, so a boot does not churn the mtime and
// re-trigger an import on the next boot.
func WriteVisionJSON(nerdDir string, v *Vision) (bool, error) {
	if v == nil {
		return false, fmt.Errorf("vision is nil")
	}
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return false, fmt.Errorf("create %s: %w", nerdDir, err)
	}
	path := filepath.Join(nerdDir, VisionJSONFileName)
	if existing, err := LoadVisionJSON(nerdDir); err == nil && existing != nil && VisionsEquivalent(existing, v) {
		return false, nil
	}
	data, err := json.MarshalIndent(WizardDocumentFromVision(v), "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal vision: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// WriteVisionMangle writes .nerd/northstar.mg from the vision's own ToFacts.
//
// Rendering the file from ToFacts rather than a hand-written generator is what
// keeps the file and the kernel in agreement: previously three independent
// generators (chat wizard, cmd_northstar, ToFacts) each decided how to encode a
// capability, and they had already drifted on persona IDs.
func WriteVisionMangle(nerdDir string, v *Vision) error {
	if v == nil {
		return fmt.Errorf("vision is nil")
	}
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", nerdDir, err)
	}
	path := filepath.Join(nerdDir, VisionMangleFileName)
	return os.WriteFile(path, []byte(RenderVisionMangle(v)), 0644)
}

// RenderVisionMangle renders a vision as a loadable .mg fact file.
func RenderVisionMangle(v *Vision) string {
	var sb strings.Builder
	sb.WriteString("# Northstar Vision Facts\n")
	sb.WriteString("# GENERATED from the northstar knowledge store - do not hand-edit.\n")
	sb.WriteString("# Edit .nerd/northstar.json (or run the /northstar wizard) and reboot;\n")
	sb.WriteString("# northstar.SyncVisionAuthority reconciles the two on every boot.\n")
	sb.WriteString("# Schema declarations live in internal/core/defaults/schemas_misc.mg\n\n")
	for _, fact := range v.ToFacts() {
		sb.WriteString(fact.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// VisionsEquivalent compares two visions ignoring timestamps.
//
// Timestamps are excluded deliberately: they are bookkeeping, and including
// them would make every comparison unequal and every boot rewrite both files.
func VisionsEquivalent(a, b *Vision) bool {
	if a == nil || b == nil {
		return a == b
	}
	return reflect.DeepEqual(normalizeVision(a), normalizeVision(b))
}

func normalizeVision(v *Vision) Vision {
	c := *cloneVision(v)
	c.CreatedAt = time.Time{}
	c.UpdatedAt = time.Time{}
	if c.Personas == nil {
		c.Personas = []Persona{}
	}
	if c.Capabilities == nil {
		c.Capabilities = []Capability{}
	}
	if c.Risks == nil {
		c.Risks = []Risk{}
	}
	if c.Requirements == nil {
		c.Requirements = []Requirement{}
	}
	if c.Constraints == nil {
		c.Constraints = []string{}
	}
	for i := range c.Personas {
		if c.Personas[i].PainPoints == nil {
			c.Personas[i].PainPoints = []string{}
		}
		if c.Personas[i].Needs == nil {
			c.Personas[i].Needs = []string{}
		}
	}
	for i := range c.Capabilities {
		if c.Capabilities[i].Serves == nil {
			c.Capabilities[i].Serves = []string{}
		}
	}
	for i := range c.Requirements {
		if c.Requirements[i].Supports == nil {
			c.Requirements[i].Supports = []string{}
		}
		if c.Requirements[i].Addresses == nil {
			c.Requirements[i].Addresses = []string{}
		}
	}
	return c
}

// SyncVisionAuthority reconciles .nerd/northstar.json with the store and
// returns the vision that won.
//
// Ordering rules, in the order they are applied:
//
//  1. Neither surface has a vision -> noop.
//  2. Only the JSON has one -> import it (this is the wizard-then-/alignment
//     path that used to silently report "no vision defined").
//  3. Only the store has one -> export JSON and .mg (this is the
//     `nerd northstar load`-then-wizard-reload path).
//  4. Both, semantically equal -> noop, but the .mg is (re)written if missing.
//  5. Both, different -> the JSON file's mtime versus the store's updated_at
//     decides. Wall clock is the only ordering that exists between a file and a
//     row; ties go to the store because the store is what the kernel projects.
func SyncVisionAuthority(store *Store, nerdDir string) (VisionSyncResult, error) {
	res := VisionSyncResult{
		Direction: SyncNoop,
		JSONPath:  filepath.Join(nerdDir, VisionJSONFileName),
		ManglePat: filepath.Join(nerdDir, VisionMangleFileName),
	}
	if store == nil {
		return res, fmt.Errorf("store is nil")
	}

	stored, err := store.LoadVision()
	if err != nil {
		return res, fmt.Errorf("load stored vision: %w", err)
	}
	fileVision, err := LoadVisionJSON(nerdDir)
	if err != nil {
		// A corrupt import surface must not take the guardian down; the store
		// stays authoritative and the operator gets a loud line.
		logging.Get(logging.CategoryNorthstar).Warn(
			"northstar.json is unreadable (%v); continuing with the store as sole authority", err)
		fileVision = nil
	}

	switch {
	case stored == nil && fileVision == nil:
		return res, nil

	case stored == nil && fileVision != nil:
		if err := store.SaveVision(fileVision); err != nil {
			return res, fmt.Errorf("import vision from %s: %w", res.JSONPath, err)
		}
		if err := WriteVisionMangle(nerdDir, fileVision); err != nil {
			logging.Get(logging.CategoryNorthstar).Warn("failed to refresh %s: %v", res.ManglePat, err)
		}
		res.Direction = SyncImported
		res.Vision = fileVision
		logging.Get(logging.CategoryNorthstar).Info(
			"Imported vision from %s into %s (store had none)", res.JSONPath, store.Path())
		return res, nil

	case stored != nil && fileVision == nil:
		if _, err := WriteVisionJSON(nerdDir, stored); err != nil {
			return res, fmt.Errorf("export vision to %s: %w", res.JSONPath, err)
		}
		if err := WriteVisionMangle(nerdDir, stored); err != nil {
			logging.Get(logging.CategoryNorthstar).Warn("failed to refresh %s: %v", res.ManglePat, err)
		}
		res.Direction = SyncExported
		res.Vision = stored
		logging.Get(logging.CategoryNorthstar).Info(
			"Exported vision from %s to %s (no JSON surface existed)", store.Path(), res.JSONPath)
		return res, nil
	}

	if VisionsEquivalent(stored, fileVision) {
		res.Vision = stored
		if _, statErr := os.Stat(res.ManglePat); statErr != nil {
			if err := WriteVisionMangle(nerdDir, stored); err != nil {
				logging.Get(logging.CategoryNorthstar).Warn("failed to write %s: %v", res.ManglePat, err)
			}
		}
		return res, nil
	}

	jsonModTime := time.Time{}
	if info, statErr := os.Stat(res.JSONPath); statErr == nil {
		jsonModTime = info.ModTime()
	}
	if jsonModTime.After(stored.UpdatedAt) {
		imported := cloneVision(fileVision)
		imported.CreatedAt = stored.CreatedAt
		imported.UpdatedAt = time.Time{}
		if err := store.SaveVision(imported); err != nil {
			return res, fmt.Errorf("import newer vision from %s: %w", res.JSONPath, err)
		}
		if err := WriteVisionMangle(nerdDir, imported); err != nil {
			logging.Get(logging.CategoryNorthstar).Warn("failed to refresh %s: %v", res.ManglePat, err)
		}
		res.Direction = SyncImported
		res.Vision = imported
		logging.Get(logging.CategoryNorthstar).Info(
			"Imported newer vision from %s (file %s > store %s)",
			res.JSONPath, jsonModTime.Format(time.RFC3339), stored.UpdatedAt.Format(time.RFC3339))
		return res, nil
	}

	if _, err := WriteVisionJSON(nerdDir, stored); err != nil {
		return res, fmt.Errorf("export newer vision to %s: %w", res.JSONPath, err)
	}
	if err := WriteVisionMangle(nerdDir, stored); err != nil {
		logging.Get(logging.CategoryNorthstar).Warn("failed to refresh %s: %v", res.ManglePat, err)
	}
	res.Direction = SyncExported
	res.Vision = stored
	logging.Get(logging.CategoryNorthstar).Info(
		"Exported newer vision to %s (store %s >= file %s)",
		res.JSONPath, stored.UpdatedAt.Format(time.RFC3339), jsonModTime.Format(time.RFC3339))
	return res, nil
}

// FactStrings renders a vision's facts as sorted Datalog lines. Used by the CLI
// so `nerd northstar facts` reports what the kernel will actually hold rather
// than whatever a stale .mg file says.
func FactStrings(v *Vision) []string {
	if v == nil {
		return nil
	}
	facts := v.ToFacts()
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, f.String())
	}
	sort.Strings(out)
	return out
}
