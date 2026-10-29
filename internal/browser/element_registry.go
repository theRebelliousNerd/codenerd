package browser

import (
	"fmt"
	"strings"
	"sync"
)

// ElementFingerprint is the private, session-scoped identity used to resolve
// an element after ordinary DOM churn. Selectors are intentionally not exposed
// by progressive observations: callers receive only the opaque Ref.
type ElementFingerprint struct {
	Ref          string             `json:"ref"`
	Generation   int                `json:"generation"`
	Selector     string             `json:"-"`
	AltSelectors []string           `json:"-"`
	TagName      string             `json:"tag_name,omitempty"`
	ID           string             `json:"id,omitempty"`
	Name         string             `json:"name,omitempty"`
	Classes      []string           `json:"classes,omitempty"`
	TextContent  string             `json:"text_content,omitempty"`
	AriaLabel    string             `json:"aria_label,omitempty"`
	DataTestID   string             `json:"data_testid,omitempty"`
	Role         string             `json:"role,omitempty"`
	InputType    string             `json:"input_type,omitempty"`
	RowKey       string             `json:"row_key,omitempty"`
	RowIndex     string             `json:"row_index,omitempty"`
	BoundingBox  map[string]float64 `json:"bounding_box,omitempty"`
}

// ElementRegistry owns opaque refs for one session. A navigation creates a new
// generation and makes every prior ref unresolvable. Repeated observations in
// one generation reuse refs for the same fingerprint.
type ElementRegistry struct {
	mu         sync.RWMutex
	elements   map[string]ElementFingerprint
	refsByKey  map[string]string
	generation int
	next       int
}

func NewElementRegistry() *ElementRegistry {
	return &ElementRegistry{
		elements:   make(map[string]ElementFingerprint),
		refsByKey:  make(map[string]string),
		generation: 1,
	}
}

// RegisterBatch assigns stable refs to fingerprints and returns copies in the
// same order. Duplicate fingerprints in a batch intentionally share a ref.
func (r *ElementRegistry) RegisterBatch(fingerprints []ElementFingerprint) []ElementFingerprint {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.registerBatchLocked(fingerprints)
}

// RegisterBatchForGeneration atomically refuses a snapshot captured before a
// navigation. This prevents a racing observation from repopulating the new
// generation with fingerprints from the old document.
func (r *ElementRegistry) RegisterBatchForGeneration(generation int, fingerprints []ElementFingerprint) ([]ElementFingerprint, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.generation != generation {
		return nil, false
	}
	return r.registerBatchLocked(fingerprints), true
}

func (r *ElementRegistry) registerBatchLocked(fingerprints []ElementFingerprint) []ElementFingerprint {
	registered := make([]ElementFingerprint, len(fingerprints))
	for i := range fingerprints {
		fp := cloneFingerprint(fingerprints[i])
		key := fingerprintKey(fp)
		ref := r.refsByKey[key]
		if ref == "" {
			r.next++
			ref = fmt.Sprintf("e%d_%d", r.generation, r.next)
			r.refsByKey[key] = ref
		}
		fp.Ref = ref
		fp.Generation = r.generation
		r.elements[ref] = cloneFingerprint(fp)
		registered[i] = fp
	}
	return registered
}

// Get returns a copy so callers cannot mutate registry state without locking.
func (r *ElementRegistry) Get(ref string) (ElementFingerprint, bool) {
	if r == nil {
		return ElementFingerprint{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	fp, ok := r.elements[ref]
	return cloneFingerprint(fp), ok
}

func (r *ElementRegistry) Generation() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.generation
}

func (r *ElementRegistry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.elements)
}

// Clear advances the generation and invalidates all previously issued refs.
func (r *ElementRegistry) Clear() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generation++
	r.next = 0
	r.elements = make(map[string]ElementFingerprint)
	r.refsByKey = make(map[string]string)
}

func fingerprintKey(fp ElementFingerprint) string {
	parts := []string{strings.ToLower(fp.TagName)}
	switch {
	case fp.DataTestID != "":
		parts = append(parts, "testid", fp.DataTestID)
	case fp.ID != "":
		parts = append(parts, "id", fp.ID)
	case fp.Name != "":
		parts = append(parts, "name", fp.Name, fp.InputType)
	case fp.AriaLabel != "":
		parts = append(parts, "aria", fp.AriaLabel, fp.Role)
	case fp.RowKey != "":
		parts = append(parts, "rowkey", fp.RowKey)
	case fp.RowIndex != "":
		parts = append(parts, "rowindex", fp.RowIndex)
	default:
		parts = append(parts, "selector", fp.Selector, fp.TextContent)
	}
	// A selector disambiguates repeated labels/names without making it public.
	parts = append(parts, fp.Selector)
	return strings.Join(parts, "\x00")
}

func cloneFingerprint(fp ElementFingerprint) ElementFingerprint {
	fp.AltSelectors = append([]string(nil), fp.AltSelectors...)
	fp.Classes = append([]string(nil), fp.Classes...)
	if fp.BoundingBox != nil {
		original := fp.BoundingBox
		fp.BoundingBox = make(map[string]float64, len(original))
		for key, value := range original {
			fp.BoundingBox[key] = value
		}
	}
	return fp
}
