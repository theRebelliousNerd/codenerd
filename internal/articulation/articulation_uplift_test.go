package articulation

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/types"
)

func feed(t *testing.T, chunks []string) (string, *StreamParser) {
	t.Helper()
	p := NewStreamParser()
	var out string
	for _, c := range chunks {
		part := p.ProcessChunk(c)
		if !utf8.ValidString(part) {
			t.Fatalf("incremental emit is invalid UTF-8: %q", part)
		}
		out += part
	}
	return out, p
}

// A "surface_response" mentioned inside reasoning text is a decoy, not the
// key: the streamer must lock onto the top-level key only. (Closes the
// TEST_GAP decoy-marker item in stream_parser_test.go.)
func TestStreamParser_IgnoresDecoyMarker(t *testing.T) {
	out, _ := feed(t, []string{
		`{"control_packet":{"reasoning_trace":"I will write \"surface_response\": \"decoy\" later"},`,
		`"surface_response":"real deal"}`,
	})
	if out != "real deal" {
		t.Errorf("streamed = %q, want %q", out, "real deal")
	}
}

// \uXXXX escapes decode — even when the escape itself straddles two chunks —
// including astral-plane surrogate pairs.
func TestStreamParser_DecodesUnicodeEscapes(t *testing.T) {
	out, _ := feed(t, []string{
		`{"surface_response":"caf`,
		`é \u00e9 \uD83D`,
		`\uDE00 done"}`,
	})
	if out != "café é 😀 done" {
		t.Errorf("streamed = %q, want %q", out, "café é 😀 done")
	}
}

// A multi-byte rune split across chunks emits whole or not at all: every
// incremental return is valid UTF-8 (asserted by feed) and the whole
// reassembles exactly.
func TestStreamParser_HoldsSplitRune(t *testing.T) {
	raw := "界日常"
	var chunks []string
	for i := 0; i < len(raw); i++ {
		chunks = append(chunks, raw[i:i+1])
	}
	full := append([]string{`{"surface_response":"`}, append(chunks, `"}`)...)
	out, p := feed(t, full)
	if out != raw {
		t.Errorf("streamed = %q, want %q", out, raw)
	}
	if !p.IsComplete() {
		t.Error("parser should be complete after the closing quote")
	}
}

// A non-string surface value releases the match instead of streaming some
// later field's string under its name.
func TestStreamParser_NonStringValueStreamsNothing(t *testing.T) {
	out, _ := feed(t, []string{`{"surface_response": 123, "other": "nope"}`})
	if out != "" {
		t.Errorf("streamed = %q, want nothing", out)
	}
}

func TestScanJSONString_EscapesAndTruncation(t *testing.T) {
	raw := `{"k":"a\"b\né\u00e9\ud83d\ude00"}`
	open := strings.Index(raw, `:"`) + 1
	val, end, ok := scanJSONString(raw, open)
	if !ok {
		t.Fatal("valid string failed to scan")
	}
	if val != "a\"b\néé😀" {
		t.Errorf("scanned = %q, want decoded escapes", val)
	}
	if raw[end-1] != '"' {
		t.Errorf("end index %d does not sit past the closing quote", end)
	}
	if _, _, ok := scanJSONString(`{"k":"abc`, strings.Index(`{"k":"abc`, `:"`)+1); ok {
		t.Error("unterminated string: expected ok=false")
	}
	if _, _, ok := scanJSONString(`{"k":"ab\u00`, strings.Index(`{"k":"ab\u00`, `:"`)+1); ok {
		t.Error("truncated escape: expected ok=false")
	}
	if val, ok := scanJSONStringPrefix(`{"k":"ab\u00`, strings.Index(`{"k":"ab\u00`, `:"`)+1); !ok || val != "ab" {
		t.Errorf("prefix = %q, %v; want ab, true", val, ok)
	}
}

func TestFindKeyOutsideStrings_SkipsDecoys(t *testing.T) {
	raw := `{"reasoning_trace":"note: \"surface_response\": \"decoy\" here","surface_response":"real"}`
	idx := findKeyOutsideStrings(raw, "surface_response")
	if idx == -1 {
		t.Fatal("real key not found")
	}
	if !strings.HasPrefix(raw[idx:], `"surface_response":"real"`) {
		t.Errorf("matched at %d: %.40q; want the real key", idx, raw[idx:])
	}
	if got := findKeyOutsideStrings(`{"a":"no key here"}`, "surface_response"); got != -1 {
		t.Errorf("absent key matched at %d", got)
	}
}

func TestSalvageSurface_SkipsDecoyAndDecodesEscapes(t *testing.T) {
	raw := `{"control_packet":{"reasoning_trace":"say \"surface_response\": \"decoy\""},"surface_response":"caf\u00e9 au lait is ready and`
	got := salvageSurfaceFromPartial(raw)
	if !strings.HasPrefix(got, "café au lait") || !strings.HasSuffix(got, "[…truncated]") {
		t.Errorf("salvaged = %q, want the café prefix with truncation marker", got)
	}
	if strings.Contains(got, "decoy") {
		t.Errorf("salvaged the decoy: %q", got)
	}
}

// Ten thousand tiny envelopes must not become ten thousand parse attempts:
// the scanner keeps the tail and drops the head.
func TestFindJSONCandidates_CapsRichCandidates(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString(`{"surface_response":"x"} `)
	}
	got := findJSONCandidates(sb.String())
	if len(got) != maxRichCandidates {
		t.Errorf("retained %d rich candidates, want %d", len(got), maxRichCandidates)
	}
}

// Around an atom's strings the emitter refuses the extended characters too;
// inside them it is the kernel gate's call (core's
// TestFilterMangleUpdates_ExtendedMetacharsInActionStrings).
func TestApplyCaps_RejectsExtendedShellMetachars(t *testing.T) {
	for _, atom := range []string{
		`run("a") && b.`,
		`run("a") > /tmp/x.`,
		`run("a") < /etc/passwd.`,
		`run("a") & b.`,
	} {
		raw := `{"control_packet":{` +
			`"intent_classification":{"category":"/query","verb":"/read","target":"t","confidence":0.9},` +
			`"mangle_updates":[` + fmt.Sprintf("%q", atom) + `],` +
			`"memory_operations":[],"self_correction":{"triggered":false},` +
			`"knowledge_requests":[],"reasoning_trace":"r",` +
			`"tool_requests":[]},` +
			`"surface_response":"ok"}`
		res := ProcessLLMResponse(raw)
		if res.Control == nil {
			t.Fatalf("atom %s: control is nil", atom)
		}
		for _, kept := range res.Control.MangleUpdates {
			if strings.Contains(kept, atom[:3]) {
				t.Errorf("atom %s survived caps; updates=%v warnings=%v", atom, res.Control.MangleUpdates, res.Warnings)
			}
		}
	}
}

func TestApplyConstitutionalOverride_NilEnvelope(t *testing.T) {
	if got := ApplyConstitutionalOverride(nil, []string{"a"}, "r"); got != nil {
		t.Errorf("nil envelope override = %+v, want nil", got)
	}
}

// The Ouroboros merge must not write into the caller's shared SessionContext.
func TestMapToPromptContext_DoesNotMutateCallerSession(t *testing.T) {
	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatal(err)
	}
	shared := &types.SessionContext{ExtraContext: map[string]string{"keep": "v"}}
	_, err = pa.mapToPromptContext(map[string]any{
		"shard_id": "x-1", "shard_type": "coder",
		"session_ctx": shared, "tool_name": "counter",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(shared.ExtraContext) != 1 || shared.ExtraContext["keep"] != "v" {
		t.Errorf("caller session mutated: %v", shared.ExtraContext)
	}
}

func TestPromptAssemblerAdapter_NilSafety(t *testing.T) {
	var nilAdapter *PromptAssemblerAdapter
	if _, err := nilAdapter.AssembleSystemPrompt(t.Context(), "a", "b"); err == nil {
		t.Error("nil adapter: expected an error, got nil")
	}
	if NewPromptAssemblerAdapter(nil).JITReady() {
		t.Error("nil assembler: JITReady should be false")
	}
}

// Both "coder" and "/coder" compile to the same tag: "//coder" matches
// nothing downstream.
func TestToCompilationContext_ToleratesPrefixedShardType(t *testing.T) {
	pa, err := NewPromptAssembler(newMockKernel())
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"coder", "/coder"} {
		cc := pa.toCompilationContext(&PromptContext{ShardID: "coder-1", ShardType: in})
		if cc.ShardType != "/coder" {
			t.Errorf("input %q -> ShardType %q, want /coder", in, cc.ShardType)
		}
	}
}
