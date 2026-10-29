package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Brutal MCP handle-expansion probes: walk control, error shapes, budgets.

func testHandleStore() *HandleStore {
	return NewHandleStore(DefaultHandleStoreConfig())
}

const nestedPayload = `{"tools":[{"name":"grep","score":9},{"name":"find","score":7}],"meta":{"q":"x"}}`

// Mint then Expand with no pointer returns a bounded digest that keeps the handle.
func TestExpandRoundTripKeepsHandle(t *testing.T) {
	s := testHandleStore()
	h := s.Mint("search", json.RawMessage(nestedPayload))
	if h == "" || !strings.HasPrefix(h, "mcp:h:") {
		t.Fatalf("bad handle %q", h)
	}
	d, err := s.Expand(h, "", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if d.Handle != h {
		t.Fatalf("digest handle %q, want %q", d.Handle, h)
	}
	if d.Data == nil {
		t.Fatal("digest has no data")
	}
	if d.FullBytes != len(nestedPayload) {
		t.Fatalf("FullBytes=%d, want %d", d.FullBytes, len(nestedPayload))
	}
}

// Pointer walks into arrays and objects, including escaped segments.
func TestExpandPointerWalk(t *testing.T) {
	s := testHandleStore()
	h := s.Mint("search", json.RawMessage(nestedPayload))
	budget := BudgetFor(ViewCompact)
	d, err := s.Expand(h, "/tools/1/name", ViewCompact, budget)
	if err != nil {
		t.Fatalf("Expand pointer failed: %v", err)
	}
	raw, _ := json.Marshal(d.Data)
	if strings.Trim(string(raw), `"`) != "find" {
		t.Fatalf("pointer slice = %s, want find", raw)
	}
}

// Every malformed expansion fails with a naming error, never a panic.
func TestExpandErrorShapes(t *testing.T) {
	s := testHandleStore()
	h := s.Mint("search", json.RawMessage(nestedPayload))
	budget := BudgetFor(ViewCompact)
	cases := map[string]struct {
		id, pointer string
		want        string
	}{
		"unknown handle":      {"mcp:h:deadbeefcafe", "", "not found"},
		"empty handle":        {"  ", "", "handle is required"},
		"missing key":         {h, "/nope", "no key"},
		"index out of range":  {h, "/tools/9", "out of range"},
		"index into object":   {h, "/meta/0", "no key"},
		"key into array":      {h, "/tools/name", "not an array index"},
		"address into scalar": {h, "/meta/q/deep", "scalar"},
		"bad syntax":          {h, "tools/0", "must be empty or start with /"},
	}
	for name, c := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s: panicked: %v", name, r)
				}
			}()
			_, err := s.Expand(c.id, c.pointer, ViewCompact, budget)
			if err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), c.want) {
				t.Fatalf("%s: error %q does not mention %q", name, err, c.want)
			}
		}()
	}
	if _, err := s.Expand("mcp:h:deadbeefcafe", "", ViewCompact, budget); !errors.Is(err, ErrHandleNotFound) {
		t.Fatalf("unknown handle err=%v, want ErrHandleNotFound", err)
	}
}

// A pointer into a non-JSON payload explains itself; a bare expansion still digests.
func TestExpandNonJSONPayload(t *testing.T) {
	s := testHandleStore()
	h := s.Mint("shell", json.RawMessage(`plain text, not json {{{`))
	if h == "" {
		t.Fatal("Mint stored nothing")
	}
	if _, err := s.Expand(h, "/a", ViewCompact, BudgetFor(ViewCompact)); err == nil {
		t.Fatal("pointer into non-JSON succeeded, want error")
	} else if !strings.Contains(err.Error(), "not JSON") {
		t.Fatalf("error %q should say not JSON", err)
	}
	d, err := s.Expand(h, "", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("bare Expand of non-JSON failed: %v", err)
	}
	if d.Data == nil {
		t.Fatal("bare digest of non-JSON has no data")
	}
}

// A zero budget fails safe to the view default instead of meaning unbounded:
// the digest must equal an explicit-default digest byte for byte.
func TestExpandZeroBudgetFailsSafe(t *testing.T) {
	s := testHandleStore()
	big := `{"wide":"` + strings.Repeat("w", 5000) + `"}`
	h := s.Mint("search", json.RawMessage(big))
	zero, err := s.Expand(h, "", ViewCompact, DigestBudget{})
	if err != nil {
		t.Fatalf("Expand with zero budget failed: %v", err)
	}
	explicit, err := s.Expand(h, "", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand with explicit budget failed: %v", err)
	}
	if zero.Bytes != explicit.Bytes || zero.Truncated != explicit.Truncated ||
		len(zero.Elided) != len(explicit.Elided) || zero.FullBytes != explicit.FullBytes {
		t.Fatalf("zero budget diverged from default: %+v vs %+v", zero, explicit)
	}
	if !zero.Truncated {
		t.Fatal("zero-budget digest of a 5KB payload claims nothing was truncated")
	}
	if zero.Bytes > 2400*2 {
		t.Fatalf("zero-budget digest escaped the default backstop: %d bytes", zero.Bytes)
	}
}

// Budget ceilings hold on a hostile payload: wide strings, deep nesting, many keys.
func TestExpandBudgetCeilingsHold(t *testing.T) {
	big := map[string]any{"wide": strings.Repeat("w", 100000)}
	deep := map[string]any{}
	cur := deep
	for i := 0; i < 50; i++ {
		next := map[string]any{}
		cur["n"] = next
		cur = next
	}
	big["deep"] = deep
	for i := 0; i < 200; i++ {
		big[strings.Repeat("k", 5)+string(rune('a'+i%26))+string(rune(i))] = i
	}
	raw, _ := json.Marshal(big)
	s := testHandleStore()
	h := s.Mint("evil", raw)
	if h == "" {
		t.Skip("hostile payload exceeds store budget; covered by retain tests")
	}
	d, err := s.Expand(h, "", ViewCompact, BudgetFor(ViewCompact))
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if d.Bytes > 2400*2 { // backstop MaxBytes with re-encode slack
		t.Fatalf("compact digest escaped its budget: %d bytes", d.Bytes)
	}
	if !d.Truncated {
		t.Fatal("hostile digest claims nothing was truncated")
	}
}
