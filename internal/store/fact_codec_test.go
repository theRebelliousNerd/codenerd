package store

import (
	"encoding/json"
	"testing"

	"codenerd/internal/types"
)

func TestColdStoragePreservesTypedArgs(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	args := []any{
		types.MangleAtom("/go"),
		int64(7),
		float64(3.5),
		"/tmp/file.go",
		true,
	}

	if err := store.StoreFact("typed_fact", args, "fact", 5); err != nil {
		t.Fatalf("StoreFact failed: %v", err)
	}

	facts, err := store.LoadFacts("typed_fact")
	if err != nil {
		t.Fatalf("LoadFacts failed: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("Expected 1 fact, got %d", len(facts))
	}

	got := facts[0].Args
	if atom, ok := got[0].(types.MangleAtom); !ok || atom != types.MangleAtom("/go") {
		t.Fatalf("Expected MangleAtom /go, got %#v", got[0])
	}
	if n, ok := got[1].(int64); !ok || n != 7 {
		t.Fatalf("Expected int64(7), got %#v", got[1])
	}
	if f, ok := got[2].(float64); !ok || f != 3.5 {
		t.Fatalf("Expected float64(3.5), got %#v", got[2])
	}
	if s, ok := got[3].(string); !ok || s != "/tmp/file.go" {
		t.Fatalf("Expected path string, got %#v", got[3])
	}
	if b, ok := got[4].(bool); !ok || !b {
		t.Fatalf("Expected bool true, got %#v", got[4])
	}
}

func TestWorldCachePreservesTypedArgs(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	facts := []WorldFactInput{
		{
			Predicate: "file_topology",
			Args: []any{
				"/workspace/main.go",
				types.MangleAtom("/go"),
				int64(12),
			},
		},
	}

	if err := store.ReplaceWorldFactsForFile("/workspace/main.go", "fast", "fp-1", facts); err != nil {
		t.Fatalf("ReplaceWorldFactsForFile failed: %v", err)
	}

	loaded, fp, err := store.LoadWorldFactsForFile("/workspace/main.go", "fast")
	if err != nil {
		t.Fatalf("LoadWorldFactsForFile failed: %v", err)
	}
	if fp != "fp-1" {
		t.Fatalf("Expected fingerprint fp-1, got %q", fp)
	}
	if len(loaded) != 1 {
		t.Fatalf("Expected 1 world fact, got %d", len(loaded))
	}

	got := loaded[0].Args
	if s, ok := got[0].(string); !ok || s != "/workspace/main.go" {
		t.Fatalf("Expected path string, got %#v", got[0])
	}
	if atom, ok := got[1].(types.MangleAtom); !ok || atom != types.MangleAtom("/go") {
		t.Fatalf("Expected MangleAtom /go, got %#v", got[1])
	}
	if n, ok := got[2].(int64); !ok || n != 12 {
		t.Fatalf("Expected int64(12), got %#v", got[2])
	}

	all, err := store.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatalf("LoadAllWorldFacts failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("Expected 1 cached world fact, got %d", len(all))
	}
}

func TestFactCodec_StabilizeDecodeErrorPaths(t *testing.T) {
	cases := map[string]string{
		"blank":     "   ",
		"invalid":   "{not json",
		"nonTagged": `[1,2,3]`,
		"emptyType": `[{"type":"","value":"x"}]`,
	}
	for name, data := range cases {
		args, err := decodeFactArgs(data)
		switch name {
		case "blank":
			if err != nil || args != nil {
				t.Fatalf("blank should decode to nil,nil got %#v err %v", args, err)
			}
		default:
			if err == nil {
				t.Fatalf("%s should fail, got %#v", name, args)
			}
		}
	}
}

func TestFactCodec_StabilizeUnknownTagCoercion(t *testing.T) {
	data := `[{"type":"future_tag_int","value":42},{"type":"future_tag_float","value":3.5},{"type":"future_tag_str","value":"abc"},{"type":"bool","value":"notbool"},{"type":"atom","value":42},{"type":"string","value":42},{"type":"int64","value":"oops"},{"type":"float64","value":"oops"}]`
	args, err := decodeFactArgs(data)
	if err != nil {
		t.Fatalf("decodeFactArgs failed: %v", err)
	}
	if len(args) != 8 {
		t.Fatalf("expected 8 args, got %d: %#v", len(args), args)
	}
	if n, ok := args[0].(int64); !ok || n != 42 {
		t.Fatalf("future int tag should coerce to int64(42), got %#v", args[0])
	}
	if f, ok := args[1].(float64); !ok || f != 3.5 {
		t.Fatalf("future float tag should coerce to 3.5, got %#v", args[1])
	}
	if s, ok := args[2].(string); !ok || s != "abc" {
		t.Fatalf("future string tag should pass through, got %#v", args[2])
	}
	if b, ok := args[3].(bool); !ok || b != false {
		t.Fatalf("non-bool bool tag should decode false, got %#v", args[3])
	}
	if args[4] != nil {
		t.Fatalf("non-string atom tag should decode nil, got %#v", args[4])
	}
	if s, ok := args[5].(string); !ok || s != "42" {
		t.Fatalf("non-string string tag should stringify, got %#v", args[5])
	}
}

func TestFactCodec_StabilizeEncodeEdgeTypes(t *testing.T) {
	type custom struct{ X int }
	encoded, err := encodeFactArgs([]any{nil, int(7), int32(8), uint64(9), uint64(18446744073709551615), float32(1.5), custom{X: 1}})
	if err != nil {
		t.Fatalf("encodeFactArgs failed: %v", err)
	}
	args, err := decodeFactArgs(encoded)
	if err != nil {
		t.Fatalf("round trip failed: %v", err)
	}
	if len(args) != 7 {
		t.Fatalf("expected 7 args, got %#v", args)
	}
	if args[0] != nil {
		t.Fatalf("nil should round trip, got %#v", args[0])
	}
	if n, ok := args[1].(int64); !ok || n != 7 {
		t.Fatalf("int should round trip as int64, got %#v", args[1])
	}
	if s, ok := args[4].(string); !ok || s != "18446744073709551615" {
		t.Fatalf("uint64 overflow should keep digits as string, got %#v", args[4])
	}
	if f, ok := args[5].(float64); !ok || f != float64(float32(1.5)) {
		t.Fatalf("float32 should round trip as float64, got %#v", args[5])
	}
}

func TestFactCodec_StabilizeCoerceAndTagged(t *testing.T) {
	if got := coerceUnknownFactValue("kept"); got != "kept" {
		t.Fatalf("non-Number should pass through, got %#v", got)
	}
	if got := coerceUnknownFactValue(json.Number("42")); got != int64(42) {
		t.Fatalf("json.Number int should coerce to int64, got %#v", got)
	}
	if got := coerceUnknownFactValue(json.Number("3.5")); got != 3.5 {
		t.Fatalf("json.Number float should coerce to float64, got %#v", got)
	}
	if got := coerceUnknownFactValue(json.Number("abc")); got != "abc" {
		t.Fatalf("unparsable Number should keep literal, got %#v", got)
	}
	if !isTaggedFactArgs(nil) {
		t.Fatalf("empty slice is tagged by vacuous truth")
	}
	if isTaggedFactArgs([]encodedFactArg{{Type: ""}}) {
		t.Fatalf("empty type must be untagged")
	}
	if !isTaggedFactArgs([]encodedFactArg{{Type: "string"}}) {
		t.Fatalf("non-empty type must be tagged")
	}
}

func TestStore_StabilizeLexicalLimitsAndCosine(t *testing.T) {
	if got := normalizeLexicalLimit(0); got != 5 {
		t.Fatalf("non-positive limit should default to 5, got %d", got)
	}
	if got := normalizeLexicalLimit(99); got != 10 {
		t.Fatalf("large limit should clamp to 10, got %d", got)
	}
	if got := normalizeLexicalLimit(4); got != 4 {
		t.Fatalf("in-range limit should pass through, got %d", got)
	}
	if got := lexicalFetchLimit(5); got != 15 {
		t.Fatalf("fetch should triple the limit, got %d", got)
	}
	if got := lexicalFetchLimit(20); got != 30 {
		t.Fatalf("fetch should cap at 30, got %d", got)
	}
	if got := float32Cosine([]float32{1}, []float32{1, 2}); got != 0 {
		t.Fatalf("mismatched lengths should score 0, got %v", got)
	}
	if got := float32Cosine(nil, nil); got != 0 {
		t.Fatalf("empty vectors should score 0, got %v", got)
	}
	if got := float32Cosine([]float32{0, 0}, []float32{1, 1}); got != 0 {
		t.Fatalf("zero vector should score 0, got %v", got)
	}
	if got := float32Cosine([]float32{1, 0}, []float32{1, 0}); got != 1 {
		t.Fatalf("identical vectors should score 1, got %v", got)
	}
	if got := decodeFloat32Blob(nil); got != nil {
		t.Fatalf("nil blob should decode nil, got %#v", got)
	}
	if got := decodeFloat32Blob([]byte{1, 2, 3}); got != nil {
		t.Fatalf("odd-length blob should decode nil, got %#v", got)
	}
	orig := []float32{1.5, -2.25, 0}
	if got := decodeFloat32Blob(encodeFloat32SliceToBlob(orig)); len(got) != 3 || got[0] != orig[0] || got[1] != orig[1] || got[2] != orig[2] {
		t.Fatalf("blob round trip failed, got %#v", got)
	}
}
