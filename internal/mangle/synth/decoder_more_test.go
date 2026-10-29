package synth

import (
	"strings"
	"testing"
)

func TestDecodeSpec_DirectJSON(t *testing.T) {
	raw := `{"format":"mangle_synth_v1","program":{"clauses":[]}}`
	spec, err := DecodeSpec(raw)
	if err != nil {
		t.Fatalf("DecodeSpec failed: %v", err)
	}
	if spec.Format != FormatV1 {
		t.Errorf("expected format %q, got %q", FormatV1, spec.Format)
	}
}

func TestDecodeSpec_Empty(t *testing.T) {
	_, err := DecodeSpec("   \n\t   ")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if err != ErrEmptyResponse {
		t.Errorf("expected ErrEmptyResponse, got %v", err)
	}
}

func TestDecodeSpec_MissingJSON(t *testing.T) {
	_, err := DecodeSpec("just some random text without curly braces")
	if err == nil {
		t.Fatal("expected error for missing JSON")
	}
	if err != ErrMissingJSON {
		t.Errorf("expected ErrMissingJSON, got %v", err)
	}
}

func TestDecodeSpec_MarkdownCodeBlock(t *testing.T) {
	raw := "Here is the spec:\n```\n{\"format\":\"mangle_synth_v1\",\"program\":{}}\n```\nSome other text"
	spec, err := DecodeSpec(raw)
	if err != nil {
		t.Fatalf("DecodeSpec failed: %v", err)
	}
	if spec.Format != FormatV1 {
		t.Errorf("expected format %q, got %q", FormatV1, spec.Format)
	}
}

func TestDecodeSpec_DecodeError(t *testing.T) {
	// A valid JSON object, but not a valid Spec (e.g. unknown field)
	raw := `{"format":"mangle_synth_v1", "unknown_field": "bad", "program":{}}`
	_, err := DecodeSpec(raw)
	if err == nil {
		t.Fatal("expected error due to unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected error to mention 'unknown field', got: %v", err)
	}
}

func TestDecodeSpec_PiggybackSurfaceWithEmbeddedObject(t *testing.T) {
	// Piggyback payload where the surface is just text with an embedded json object
	surfaceText := `I have synthesized the program: {"format":"mangle_synth_v1","program":{"clauses":[]}} Enjoy!`
	// JSON encode the surface text
	raw := `{"surface_response": "` + strings.ReplaceAll(surfaceText, `"`, `\"`) + `"}`

	spec, err := DecodeSpec(raw)
	if err != nil {
		t.Fatalf("DecodeSpec failed: %v", err)
	}
	if spec.Format != FormatV1 {
		t.Errorf("expected format %q, got %q", FormatV1, spec.Format)
	}
}

func TestDecodeSpec_PiggybackSurfaceButBadJSON(t *testing.T) {
	raw := `{"surface_response": "I have synthesized the program: {\"format\":\"mangle_synth_v1\",\"unknown_field\":\"bad\",\"program\":{\"clauses\":[]}} Enjoy!"}`
	_, err := DecodeSpec(raw)
	if err == nil {
		t.Fatal("expected error due to unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected error to mention 'unknown field', got: %v", err)
	}
}

func TestDecodeSpec_TrailingJSONContent(t *testing.T) {
	_, err := decodeSpecPayload(`{"format":"mangle_synth_v1","program":{}}  "extra"`)
	if err == nil {
		t.Fatal("expected error for trailing content")
	}
	if !strings.Contains(err.Error(), "unexpected trailing JSON content") {
		t.Errorf("expected trailing JSON content error, got: %v", err)
	}
}

func TestEnsureEOF_Valid(t *testing.T) {
	_, err := decodeSpecPayload(`{"format":"mangle_synth_v1","program":{}}`)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPiggybackEmpty(t *testing.T) {
	_, ok := extractPiggybackSurface(`{bad json}`)
	if ok {
		t.Fatal("expected false")
	}

	_, ok = extractPiggybackSurface(`{}`)
	if ok {
		t.Fatal("expected false")
	}

	_, ok = extractPiggybackSurface(`{"surface_response": "    "}`)
	if ok {
		t.Fatal("expected false")
	}

	surfaceRaw, ok := extractPiggybackSurface(`{"surface_response": {"a":"b"}}`)
	if !ok {
		t.Fatal("expected true")
	}
	if !strings.Contains(surfaceRaw, "a") {
		t.Fatal("expected a in surface Raw")
	}
}

func TestExtractJSONPayload_Prefix(t *testing.T) {
	_, err := extractJSONPayload("```json\n{\"format\":\"mangle_synth_v1\",\"program\":{}}\n```")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	_, err = extractJSONPayload("```\n{\"format\":\"mangle_synth_v1\",\"program\":{}}\n```")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	res, err := extractJSONPayload("```json\n{\"format\":\"mangle_synth_v1\",\"program\":{}}")
	if err != nil {
		t.Fatalf("expected no error because fallback will catch it, got: %v", err)
	}
	if !strings.Contains(res, "mangle_synth_v1") {
		t.Fatalf("expected to extract the json")
	}
}

func TestExtractJSONPayload_CodeBlock(t *testing.T) {
	rawNoNewline := "```{\"format\":\"mangle_synth_v1\",\"program\":{}}```"
	res, err := extractJSONPayload(rawNoNewline)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if !strings.Contains(res, "mangle_synth_v1") {
		t.Fatalf("did not extract correctly")
	}
}

func TestEnsureEOF_Errors(t *testing.T) {
	_, err := decodeSpecPayload(`{"format":"mangle_synth_v1","program":{}}  [`)
	if err == nil {
		t.Fatal("expected error for invalid trailing content")
	}
}

func TestFindJSONObject_Escapes(t *testing.T) {
	_, ok := findJSONObject(`{"a": "b\"c\\d"}`)
	if !ok {
		t.Fatal("expected true")
	}
}
