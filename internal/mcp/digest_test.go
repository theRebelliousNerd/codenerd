package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeView_ShouldRejectUnknownRatherThanDowngrade(t *testing.T) {
	t.Parallel()

	if v, err := NormalizeView(""); err != nil || v != ViewCompact {
		t.Errorf("empty view = %q, %v; want compact, nil", v, err)
	}
	if v, err := NormalizeView("FULL"); err != nil || v != ViewFull {
		t.Errorf("FULL = %q, %v; want full, nil", v, err)
	}
	// Silently downgrading "detailed" to compact answers the caller with less
	// than they asked for and tells them nothing; the symptom surfaces much
	// later as "the tool keeps hiding things".
	if _, err := NormalizeView("detailed"); err == nil {
		t.Error("unknown view was accepted; it must be an error, not a silent downgrade")
	}
}

func TestShape_ShouldSketchHomogeneousArrayInOneLine(t *testing.T) {
	t.Parallel()

	var payload any
	if err := json.Unmarshal([]byte(`{
		"items":[{"id":"a","name":"x","status":"open"},{"id":"b","name":"y","status":"shut"}],
		"next_cursor":"abc"
	}`), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := Shape(payload)
	if !strings.Contains(got, "2 x {") {
		t.Errorf("shape = %q; want a collapsed homogeneous array term", got)
	}
	if !strings.Contains(got, "next_cursor") {
		t.Errorf("shape = %q; want sibling keys named", got)
	}
}

func TestShape_WhenArrayRagged_ShouldSayMixed(t *testing.T) {
	t.Parallel()

	var payload any
	_ = json.Unmarshal([]byte(`[{"a":1},"str",7]`), &payload)
	if got := Shape(payload); !strings.Contains(got, "mixed") {
		t.Errorf("shape = %q; a ragged array must be reported as such, it changes how it is consumed", got)
	}
}

func TestDigestJSON_WhenArrayLong_ShouldClampAndPointAtTheTail(t *testing.T) {
	t.Parallel()

	var items []map[string]any
	for i := 0; i < 100; i++ {
		items = append(items, map[string]any{"id": i})
	}
	raw, _ := json.Marshal(map[string]any{"items": items})

	digest := DigestJSON(raw, ViewCompact, BudgetFor(ViewCompact))

	if !digest.Truncated {
		t.Fatal("100 items at compact should truncate")
	}
	var found bool
	for _, e := range digest.Elided {
		if e.Kind == ElisionArrayTail && e.Pointer == "/items" {
			found = true
			if e.Omitted != 100-BudgetFor(ViewCompact).MaxItems {
				t.Errorf("omitted = %d, want %d", e.Omitted, 100-BudgetFor(ViewCompact).MaxItems)
			}
		}
	}
	if !found {
		t.Errorf("no array_tail elision pointing at /items; elisions = %+v", digest.Elided)
	}
}

func TestDigestJSON_WhenOneEnormousString_ShouldStillFitBudget(t *testing.T) {
	t.Parallel()

	// This is the payload that defeats a shaper which only clamps item counts:
	// a single object, a single key, one gigantic value. Nothing to slice, and
	// a cardinality-only budget passes it straight through.
	raw, _ := json.Marshal(map[string]any{"trace": strings.Repeat("x", 200_000)})

	budget := BudgetFor(ViewCompact)
	digest := DigestJSON(raw, ViewCompact, budget)

	if digest.Bytes > budget.MaxBytes {
		t.Errorf("rendered %d bytes over a %d budget; a single wide scalar escaped the budget",
			digest.Bytes, budget.MaxBytes)
	}
	if !digest.Truncated {
		t.Error("truncation must be reported")
	}
	if digest.FullBytes < 200_000 {
		t.Errorf("full_bytes = %d; the original size must be reported so the caller can judge", digest.FullBytes)
	}
}

func TestDigestJSON_WhenSummaryView_ShouldCostFarLessThanFull(t *testing.T) {
	t.Parallel()

	var items []map[string]any
	for i := 0; i < 60; i++ {
		items = append(items, map[string]any{
			"id": i, "name": strings.Repeat("n", 40), "status": "open",
		})
	}
	raw, _ := json.Marshal(map[string]any{"items": items})

	summary := DigestJSON(raw, ViewSummary, BudgetFor(ViewSummary))
	full := DigestJSON(raw, ViewFull, BudgetFor(ViewFull))

	if summary.Bytes >= full.Bytes {
		t.Errorf("summary %d bytes >= full %d bytes; the ladder must actually descend",
			summary.Bytes, full.Bytes)
	}
	// The sketch is the point of summary: it must describe what it withheld.
	if summary.Shape == "" {
		t.Error("summary lost the shape sketch, which is the only thing making it actionable")
	}
}

func TestDigestJSON_WhenNotJSON_ShouldShapeAsText(t *testing.T) {
	t.Parallel()

	// Plenty of MCP servers return plain text. That is not an error.
	digest := DigestJSON(json.RawMessage("not json at all"), ViewCompact, BudgetFor(ViewCompact))
	if digest.Data == nil {
		t.Error("non-JSON payload produced no data; it should be shaped as a string")
	}
}

func TestDigestJSON_WhenStringClipped_ShouldStayValidUTF8(t *testing.T) {
	t.Parallel()

	// Multi-byte runes straddling the cut render as replacement characters and
	// read as corrupted data rather than as truncation.
	raw, _ := json.Marshal(map[string]any{"text": strings.Repeat("é", 4000)})
	digest := DigestJSON(raw, ViewCompact, BudgetFor(ViewCompact))

	encoded, err := json.Marshal(digest.Data)
	if err != nil {
		t.Fatalf("marshal shaped data: %v", err)
	}
	if strings.Contains(string(encoded), `�`) {
		t.Error("clipped string contains a replacement character; the cut was not on a rune boundary")
	}
}

func TestResolvePointer_ShouldWalkObjectsAndArrays(t *testing.T) {
	t.Parallel()

	var root any
	_ = json.Unmarshal([]byte(`{"items":[{"id":"a"},{"id":"b"}],"a~b":1,"c/d":2}`), &root)

	got, err := ResolvePointer(root, "/items/1/id")
	if err != nil || got != "b" {
		t.Errorf("/items/1/id = %v, %v; want b", got, err)
	}
	if _, err := ResolvePointer(root, ""); err != nil {
		t.Errorf("empty pointer must address the whole document: %v", err)
	}
	// RFC 6901 escapes: ~0 is a tilde, ~1 is a slash.
	if _, err := ResolvePointer(root, "/a~0b"); err != nil {
		t.Errorf("~0 escape failed: %v", err)
	}
	if _, err := ResolvePointer(root, "/c~1d"); err != nil {
		t.Errorf("~1 escape failed: %v", err)
	}
	if _, err := ResolvePointer(root, "/items/9"); err == nil {
		t.Error("out-of-range index must error rather than return nil")
	}
	if _, err := ResolvePointer(root, "items"); err == nil {
		t.Error("a pointer not starting with / must be rejected")
	}
}

func TestDigestBudget_WithCeiling_ShouldOnlyNarrow(t *testing.T) {
	t.Parallel()

	budget := BudgetFor(ViewCompact)
	if got := budget.withCeiling(5); got.MaxItems != 5 {
		t.Errorf("MaxItems = %d, want 5", got.MaxItems)
	}
	// max_items must not be an escape hatch around the tier's own ceiling.
	if got := budget.withCeiling(10_000); got.MaxItems != budget.MaxItems {
		t.Errorf("MaxItems = %d; a caller must not be able to raise the tier ceiling", got.MaxItems)
	}
}

func TestDigestJSON_WhenRowsAreWide_ShouldReturnFewerRowsNotNothing(t *testing.T) {
	t.Parallel()

	// Rows wide enough that the compact item ceiling alone overruns the byte
	// budget. The wrong answer here — and the first thing this code did — is to
	// null the whole payload: the caller asked for a smaller answer and got no
	// answer at all.
	var items []map[string]any
	for i := 0; i < 200; i++ {
		items = append(items, map[string]any{
			"id":     i,
			"title":  strings.Repeat("t", 60),
			"body":   strings.Repeat("detail ", 40),
			"status": "open",
		})
	}
	raw, _ := json.Marshal(map[string]any{"items": items})

	budget := BudgetFor(ViewCompact)
	digest := DigestJSON(raw, ViewCompact, budget)

	if digest.Data == nil {
		t.Fatal("compact view discarded the entire payload instead of returning fewer rows")
	}
	if digest.Bytes > budget.MaxBytes {
		t.Errorf("rendered %d bytes over a %d budget", digest.Bytes, budget.MaxBytes)
	}
	if !digest.Truncated {
		t.Error("truncation must be reported")
	}

	// And what survives has to be usable: whole rows, not fragments.
	encoded, _ := json.Marshal(digest.Data)
	if !strings.Contains(string(encoded), `"status":"open"`) {
		t.Errorf("shaped data = %s; want at least one complete row", encoded)
	}
}

func TestShape_WhenRowsDifferOnlyInStringLength_ShouldStayHomogeneous(t *testing.T) {
	t.Parallel()

	// Structurally identical records whose string values differ in length must
	// not sketch as "mixed" — that is both wrong and the least useful thing the
	// sketch could report about a uniform result set.
	var payload any
	_ = json.Unmarshal([]byte(`[
		{"id":"a","title":"short"},
		{"id":"bbbb","title":"a considerably longer title than the first"}
	]`), &payload)

	got := Shape(payload)
	if strings.Contains(got, "mixed") {
		t.Errorf("shape = %q; structurally identical rows must collapse to one term", got)
	}
	if !strings.Contains(got, "2 x {") {
		t.Errorf("shape = %q; want a collapsed homogeneous array", got)
	}
}

func TestShape_ShouldStillFlagAWallOfText(t *testing.T) {
	t.Parallel()

	var payload any
	_ = json.Unmarshal([]byte(`{"trace":"`+strings.Repeat("x", 900)+`","id":"a"}`), &payload)
	if got := Shape(payload); !strings.Contains(got, "text") {
		t.Errorf("shape = %q; a large string field must still be distinguishable from a small one", got)
	}
}
