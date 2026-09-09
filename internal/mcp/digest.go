package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// An MCP tool result is whatever the server felt like returning. It is the one
// part of the control plane whose size nobody on this side gets to design, and
// it arrives after the decision to call has already been made — so it cannot be
// budgeted by selecting differently, only by shaping what comes back.
//
// Shaping here is deliberately budgeted in BYTES as well as item counts.
// Clamping cardinality alone is the obvious approach and it fails on the most
// common real payload there is: one object with one enormous string in it — a
// stack trace, a file body, a base64 blob, a rendered diff. Ten items become
// three and the response is still forty thousand tokens.

// View is the disclosure depth of a rendered result. The three levels and the
// default match the browser tools' progressive surface, because an agent that
// has learned "start at compact" for one tool should not have to learn a
// different ladder for another.
type View string

const (
	// ViewSummary is the structural sketch plus a handful of representative
	// values: enough to decide what to do next, never enough to work from.
	ViewSummary View = "summary"
	// ViewCompact is the working default.
	ViewCompact View = "compact"
	// ViewFull is the diagnostic tier, still bounded.
	ViewFull View = "full"
)

// NormalizeView resolves a caller-supplied view.
//
// Unlike the browser tools, an unrecognized view is an error rather than a
// silent downgrade to compact: a caller who typed "detailed" and got compact
// has been answered with less than they asked for and told nothing, and the
// symptom shows up much later as "the tool keeps hiding things from me".
func NormalizeView(raw string) (View, error) {
	switch v := View(strings.ToLower(strings.TrimSpace(raw))); v {
	case "":
		return ViewCompact, nil
	case ViewSummary, ViewCompact, ViewFull:
		return v, nil
	default:
		return "", fmt.Errorf("unsupported view %q: expected summary, compact, or full", raw)
	}
}

// DigestBudget bounds one rendered result on four independent axes. All four
// are needed: a payload can be too big by being too long (items), too wide
// (string bytes), too deep (nesting), or simply too large overall (bytes), and
// clamping any three still lets the fourth through.
type DigestBudget struct {
	// MaxBytes caps the rendered JSON of the shaped payload. This is the
	// backstop that holds when the other three are individually satisfied.
	MaxBytes int
	// MaxItems caps elements kept per array.
	MaxItems int
	// MaxDepth caps nesting depth; deeper values collapse to their sketch.
	MaxDepth int
	// MaxStringBytes caps the width of any single string scalar.
	MaxStringBytes int
	// MaxObjectKeys caps how many keys of one object are kept.
	MaxObjectKeys int
}

// BudgetFor returns the default budget for a view.
//
// The numbers are chosen against what they buy in an LLM context window, at
// roughly four bytes per token: summary is ~150 tokens (an atlas entry you can
// afford on every turn), compact ~500 (a working answer), full ~4000 (a
// deliberate, expensive look). Full is bounded rather than unbounded on
// purpose — "full" means "everything within a budget you can afford", and an
// unbounded tier is how a 30 MB result reaches the model exactly once before
// the session is over.
func BudgetFor(view View) DigestBudget {
	switch view {
	case ViewSummary:
		return DigestBudget{MaxBytes: 600, MaxItems: 3, MaxDepth: 3, MaxStringBytes: 120, MaxObjectKeys: 12}
	case ViewFull:
		return DigestBudget{MaxBytes: 16000, MaxItems: 200, MaxDepth: 12, MaxStringBytes: 4000, MaxObjectKeys: 100}
	default:
		return DigestBudget{MaxBytes: 2400, MaxItems: 20, MaxDepth: 5, MaxStringBytes: 400, MaxObjectKeys: 32}
	}
}

// withCeiling clamps a caller-supplied item count into the view's budget. A
// caller may ask for fewer items than the tier allows, never more: otherwise
// max_items is an escape hatch around the whole budget.
func (b DigestBudget) withCeiling(maxItems int) DigestBudget {
	if maxItems > 0 && maxItems < b.MaxItems {
		b.MaxItems = maxItems
	}
	return b
}

// ElisionKind names why something was cut, so the caller can tell "there is
// more of this list" from "this string was clipped" without diffing anything.
type ElisionKind string

const (
	ElisionArrayTail  ElisionKind = "array_tail"
	ElisionStringTail ElisionKind = "string_tail"
	ElisionObjectKeys ElisionKind = "object_keys"
	ElisionDepth      ElisionKind = "depth"
	ElisionBudget     ElisionKind = "budget"
)

// Elision records one cut and how to undo it.
//
// Pointer is an RFC 6901 JSON pointer into the ORIGINAL payload, not into the
// shaped one, because the shaped one is the thing missing the data. That is
// what makes an elision actionable rather than merely honest.
type Elision struct {
	Pointer string      `json:"pointer"`
	Kind    ElisionKind `json:"kind"`
	Omitted int         `json:"omitted"`
	Unit    string      `json:"unit"` // "items" | "bytes" | "keys"
}

// Digest is a shaped result: what the caller sees, what was cut, and how to get
// the rest.
type Digest struct {
	View      View      `json:"view"`
	Shape     string    `json:"shape"`
	Data      any       `json:"data,omitzero"`
	Elided    []Elision `json:"elided,omitzero"`
	Handle    string    `json:"handle,omitzero"`
	Bytes     int       `json:"bytes"`
	FullBytes int       `json:"full_bytes"`
	Truncated bool      `json:"truncated"`
}

// digester carries the walk state. Elisions accumulate as the walk proceeds, so
// the pointer for each one is built from the path stack rather than
// reconstructed afterwards.
type digester struct {
	budget DigestBudget
	elided []Elision
	path   []string
}

// Shape renders a one-line structural sketch of a JSON value.
//
// This is the highest-value line in the whole control plane. "an array of 47
// objects with id, name and status" is about twenty tokens and tells an agent
// everything it needs to decide whether to look closer, page, or move on. The
// forty-seven records it describes are about four thousand.
func Shape(v any) string {
	return sketch(v, 0)
}

const (
	sketchMaxDepth = 4
	sketchMaxKeys  = 8
	// sketchLongString is where "a value" becomes "a body of text".
	sketchLongString = 256
)

func sketch(v any, depth int) string {
	switch value := v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64, int, int64, json.Number:
		return "num"
	case string:
		// Bucketed, not exact. An exact length makes two structurally identical
		// records compare unequal, so a homogeneous array of 200 rows sketches
		// as "mixed" purely because the titles differ in length — which is both
		// wrong and the least useful thing the sketch could say. Two buckets
		// keep the structural comparison stable while still flagging the case
		// that matters: one field carrying a wall of text.
		if len(value) > sketchLongString {
			return "text"
		}
		return "str"
	case []any:
		if len(value) == 0 {
			return "[]"
		}
		if depth >= sketchMaxDepth {
			return "[" + strconv.Itoa(len(value)) + " x ...]"
		}
		// Sketch the first element and check whether the rest agree. A
		// homogeneous array is the common case and collapses to one term; a
		// ragged one is worth saying so, because it changes how the caller
		// must consume it.
		head := sketch(value[0], depth+1)
		for _, item := range value[1:] {
			if sketch(item, depth+1) != head {
				return "[" + strconv.Itoa(len(value)) + " x mixed]"
			}
		}
		return "[" + strconv.Itoa(len(value)) + " x " + head + "]"
	case map[string]any:
		if len(value) == 0 {
			return "{}"
		}
		if depth >= sketchMaxDepth {
			return "{" + strconv.Itoa(len(value)) + " keys}"
		}
		keys := sortedKeys(value)
		var parts []string
		for i, k := range keys {
			if i >= sketchMaxKeys {
				parts = append(parts, "+"+strconv.Itoa(len(keys)-sketchMaxKeys)+" more")
				break
			}
			parts = append(parts, k+": "+sketch(value[k], depth+1))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return "any"
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Deterministic order: two identical payloads must sketch identically, or
	// the sketch reads as new information on every turn.
	sort.Strings(keys)
	return keys
}

// shrinkSteps is how many times shapeToFit halves the shaping budget before
// giving up. Five halvings take 20 items to 1 and 400 string bytes to 25, which
// is past the point where anything useful survives.
const shrinkSteps = 5

// shapeToFit shapes a value and, if the result overruns the byte budget,
// reshapes it smaller rather than discarding it.
//
// The naive backstop — null the whole payload when it does not fit — was what
// this code did first, and it is much worse than it looks. A compact view of a
// 200-row result would come back as literally nothing: the caller asked for a
// smaller answer and got no answer, with the shape sketch as the only surviving
// evidence that data existed. Halving the item and string budgets until the
// render fits returns as many whole rows as the budget can actually hold, which
// is what "compact" was always supposed to mean.
func shapeToFit(value any, budget DigestBudget) (*digester, any, []byte, error) {
	attempt := budget
	var last *digester
	var lastShaped any
	var lastRendered []byte

	for step := 0; step <= shrinkSteps; step++ {
		d := &digester{budget: attempt}
		shaped := d.walk(value, 0)
		rendered, err := json.Marshal(shaped)
		if err != nil {
			return d, nil, nil, err
		}
		last, lastShaped, lastRendered = d, shaped, rendered

		if budget.MaxBytes <= 0 || len(rendered) <= budget.MaxBytes {
			return d, shaped, rendered, nil
		}
		attempt = halveShapingBudget(attempt)
	}

	// Still over budget after every halving: the payload is pathologically wide
	// rather than long — thousands of distinct small keys, say — and there is
	// nothing left to shrink. Only here is discarding correct, and it is
	// reported as its own elision kind so it is never mistaken for a clean read.
	last.elided = append(last.elided, Elision{
		Pointer: "", Kind: ElisionBudget, Omitted: len(lastRendered), Unit: "bytes",
	})
	_ = lastShaped
	return last, nil, []byte("null"), nil
}

// halveShapingBudget tightens the axes a caller can actually feel, leaving
// MaxBytes alone: it is the target being fitted, not a dial.
func halveShapingBudget(b DigestBudget) DigestBudget {
	halve := func(n, floor int) int {
		if n <= floor {
			return floor
		}
		if n/2 < floor {
			return floor
		}
		return n / 2
	}
	b.MaxItems = halve(b.MaxItems, 1)
	b.MaxStringBytes = halve(b.MaxStringBytes, 40)
	b.MaxObjectKeys = halve(b.MaxObjectKeys, 4)
	if b.MaxDepth > 2 {
		b.MaxDepth--
	}
	return b
}

// DigestJSON shapes a raw JSON payload to a budget.
//
// A payload that does not parse as JSON is not an error — plenty of MCP servers
// return plain text — it is shaped as a single string instead, which is exactly
// what the string branch of the walker already does correctly.
func DigestJSON(raw json.RawMessage, view View, budget DigestBudget) Digest {
	full := len(raw)

	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		value = string(raw)
	}

	d, shaped, rendered, err := shapeToFit(value, budget)
	if err != nil {
		// A value that came out of encoding/json cannot normally fail to go
		// back in. If it somehow does, report the sketch rather than nothing:
		// a caller with a shape and a handle can still make progress.
		return Digest{
			View: view, Shape: Shape(value), FullBytes: full,
			Truncated: true,
			Elided:    []Elision{{Pointer: "", Kind: ElisionBudget, Omitted: full, Unit: "bytes"}},
		}
	}

	return Digest{
		View:      view,
		Shape:     Shape(value),
		Data:      shaped,
		Elided:    d.elided,
		Bytes:     len(rendered),
		FullBytes: full,
		Truncated: len(d.elided) > 0,
	}
}

// walk shapes one value, recording an elision for anything it drops.
func (d *digester) walk(v any, depth int) any {
	switch value := v.(type) {
	case string:
		return d.walkString(value)
	case []any:
		return d.walkArray(value, depth)
	case map[string]any:
		return d.walkObject(value, depth)
	default:
		// Scalars pass through: a number or a bool cannot blow a budget, and
		// rounding one would corrupt the answer rather than shrink it.
		return v
	}
}

func (d *digester) walkString(s string) any {
	limit := d.budget.MaxStringBytes
	if limit <= 0 || len(s) <= limit {
		return s
	}
	// Cut on a rune boundary so the result stays valid UTF-8; a broken final
	// rune renders as a replacement character and looks like corrupted data.
	cut := limit
	for cut > 0 && !utf8Boundary(s, cut) {
		cut--
	}
	d.note(ElisionStringTail, len(s)-cut, "bytes")
	return s[:cut] + "…"
}

// utf8Boundary reports whether index i begins a new rune.
func utf8Boundary(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return true
	}
	return s[i]&0xC0 != 0x80
}

func (d *digester) walkArray(items []any, depth int) any {
	if d.budget.MaxDepth > 0 && depth >= d.budget.MaxDepth {
		d.note(ElisionDepth, len(items), "items")
		return Shape(items)
	}

	keep := len(items)
	if d.budget.MaxItems > 0 && keep > d.budget.MaxItems {
		keep = d.budget.MaxItems
	}

	out := make([]any, 0, keep)
	for i := 0; i < keep; i++ {
		d.push(strconv.Itoa(i))
		out = append(out, d.walk(items[i], depth+1))
		d.pop()
	}
	if keep < len(items) {
		d.note(ElisionArrayTail, len(items)-keep, "items")
	}
	return out
}

func (d *digester) walkObject(obj map[string]any, depth int) any {
	if d.budget.MaxDepth > 0 && depth >= d.budget.MaxDepth {
		d.note(ElisionDepth, len(obj), "keys")
		return Shape(obj)
	}

	keys := sortedKeys(obj)
	keep := len(keys)
	if d.budget.MaxObjectKeys > 0 && keep > d.budget.MaxObjectKeys {
		keep = d.budget.MaxObjectKeys
	}

	out := make(map[string]any, keep)
	for i := 0; i < keep; i++ {
		k := keys[i]
		d.push(k)
		out[k] = d.walk(obj[k], depth+1)
		d.pop()
	}
	if keep < len(keys) {
		d.note(ElisionObjectKeys, len(keys)-keep, "keys")
	}
	return out
}

func (d *digester) push(seg string) { d.path = append(d.path, seg) }
func (d *digester) pop()            { d.path = d.path[:len(d.path)-1] }

// maxElisions bounds the elision list itself. A pathological payload can
// produce thousands of cuts, and a truncation report longer than the truncated
// data defeats its own purpose.
const maxElisions = 24

func (d *digester) note(kind ElisionKind, omitted int, unit string) {
	if len(d.elided) >= maxElisions {
		return
	}
	d.elided = append(d.elided, Elision{
		Pointer: jsonPointer(d.path),
		Kind:    kind,
		Omitted: omitted,
		Unit:    unit,
	})
}

// jsonPointer renders a path stack as RFC 6901, escaping the two characters
// that are structural in that grammar.
func jsonPointer(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, seg := range path {
		sb.WriteByte('/')
		sb.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(seg))
	}
	return sb.String()
}

// ResolvePointer walks an RFC 6901 pointer into a decoded JSON value.
//
// The empty pointer addresses the whole document, which is what makes "expand
// this handle" and "expand this slice of this handle" the same operation with
// one argument defaulted.
func ResolvePointer(root any, pointer string) (any, error) {
	pointer = strings.TrimSpace(pointer)
	if pointer == "" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON pointer %q: must be empty or start with /", pointer)
	}

	unescape := strings.NewReplacer("~1", "/", "~0", "~")
	current := root
	for _, rawSeg := range strings.Split(pointer[1:], "/") {
		seg := unescape.Replace(rawSeg)
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[seg]
			if !ok {
				return nil, fmt.Errorf("JSON pointer %q: no key %q", pointer, seg)
			}
			current = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("JSON pointer %q: %q is not an array index", pointer, seg)
			}
			if idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("JSON pointer %q: index %d out of range (len %d)", pointer, idx, len(node))
			}
			current = node[idx]
		default:
			return nil, fmt.Errorf("JSON pointer %q: %q addresses into a scalar", pointer, seg)
		}
	}
	return current, nil
}
