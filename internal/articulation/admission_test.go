package articulation

import (
	"codenerd/internal/core"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

const admitControl = `"control_packet":{"intent_classification":{"category":"/query","verb":"/explain","target":"x","constraint":"none","confidence":1},"mangle_updates":[],"memory_operations":[]}`

// streamSurface feeds raw to a StreamParser in the given chunks.
func streamSurface(chunks []string) string {
	p := NewStreamParser()
	var sb strings.Builder
	for _, c := range chunks {
		sb.WriteString(p.ProcessChunk(c))
	}
	return sb.String()
}

func partitions(raw string, rng *rand.Rand) []string {
	var out []string
	for len(raw) > 0 {
		n := 1 + rng.Intn(9)
		if n > len(raw) {
			n = len(raw)
		}
		out = append(out, raw[:n])
		raw = raw[n:]
	}
	return out
}

// An envelope's member names are unique after decoding and case folding, in
// every spelling a name can take. A repeat is ambiguous: the stream parser
// keeps the first value and the final decoder keeps the last, so accepting
// either would show a person one statement and record another.
func TestAdmission_RepeatedMemberNamesAreAmbiguous(t *testing.T) {
	cases := map[string]string{
		"plain duplicate":          `{` + admitControl + `,"surface_response":"A","surface_response":"B"}`,
		"escaped spelling":         `{` + admitControl + `,"surface_response":"A","surface_respons\u0065":"B"}`,
		"case alias":               `{` + admitControl + `,"surface_response":"A","SURFACE_RESPONSE":"B"}`,
		"duplicate control_packet": `{` + admitControl + `,"control_packet":{},"surface_response":"A"}`,
		"duplicate inside control": `{"control_packet":{"mangle_updates":[],"mangle_updates":["a(1)."]},"surface_response":"A"}`,
		"duplicate tool argument":  `{"control_packet":{"tool_requests":[{"id":"1","tool_name":"read_file","tool_args":{"path":"a","path":"b"}}]},"surface_response":"A"}`,
		"duplicate in array item":  `{"control_packet":{"memory_operations":[{"op":"note","key":"k","key":"j","value":"v"}]},"surface_response":"A"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := NewResponseProcessor().Process(raw)
			if err != nil {
				t.Fatalf("Process: %v", err)
			}
			if res.Admission != AdmissionAmbiguous {
				t.Fatalf("admission = %q (surface %q), want ambiguous", res.Admission, res.Surface)
			}
			if got := ProcessLLMResponseAllowPlain(raw); got.Control != nil {
				t.Fatalf("an ambiguous reply must hand a consumer no control packet, got %+v", got.Control)
			}
			strict := NewResponseProcessor()
			strict.RequireValidJSON = true
			if _, err := strict.Process(raw); err == nil {
				t.Fatal("strict mode must fail an ambiguous reply")
			}
		})
	}
}

// Names that differ only in case are different tool arguments, and different
// objects may each use the same name.
func TestAdmission_ToolArgumentsKeepExactNames(t *testing.T) {
	raw := `{"control_packet":{"tool_requests":[{"id":"1","tool_name":"t","tool_args":{"Path":"a","path":"b"}},{"id":"2","tool_name":"t","tool_args":{"path":"c"}}]},"surface_response":"A"}`
	res, err := NewResponseProcessor().Process(raw)
	if err != nil || res.Admission != AdmissionAdmitted {
		t.Fatalf("admission = %q err = %v, want admitted", res.Admission, err)
	}
	if len(res.Control.ToolRequests) != 2 {
		t.Fatalf("tool requests = %d, want 2", len(res.Control.ToolRequests))
	}
}

// The live stream and the final parse read one key the same way, in every
// accepted spelling, at every split point and under random partitions.
func TestAdmission_StreamAndFinalAgreeOnAcceptedSpellings(t *testing.T) {
	cases := map[string]string{
		"literal":        `{` + admitControl + `,"surface_response":"hello \"there\" é"}`,
		"escaped key":    `{` + admitControl + `,"surface_respons\u0065":"escaped key"}`,
		"fully escaped":  `{` + admitControl + `,"\u0073urface_response":"first letter escaped"}`,
		"upper case key": `{` + admitControl + `,"SURFACE_RESPONSE":"upper"}`,
		"mixed case key": `{` + admitControl + `,"Surface_Response":"mixed"}`,
		"surface first":  `{"surface_response":"first","control_packet":{}}`,
		"spaced":         "{ \"control_packet\" : { } ,\n\t\"surface_response\"\n:\n\"spaced\" }",
	}
	rng := rand.New(rand.NewSource(7))
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			res, err := NewResponseProcessor().Process(raw)
			if err != nil || res.Admission != AdmissionAdmitted {
				t.Fatalf("final parse: admission = %q err = %v", res.Admission, err)
			}
			for split := 0; split <= len(raw); split++ {
				if got := streamSurface([]string{raw[:split], raw[split:]}); got != res.Surface {
					t.Fatalf("split %d: stream = %q, final = %q", split, got, res.Surface)
				}
			}
			for i := 0; i < 50; i++ {
				if got := streamSurface(partitions(raw, rng)); got != res.Surface {
					t.Fatalf("random partition %d: stream = %q, final = %q", i, got, res.Surface)
				}
			}
		})
	}
}

// A top-level key that is not the surface key but is long, escaped or merely
// similar must not capture the stream.
func TestStreamParser_OnlyTheSurfaceKeyCapturesTheStream(t *testing.T) {
	long := strings.Repeat("x", 300)
	raw := `{"` + long + `":"nope","surface_response_extra":"nope","surface_responses":"nope","surface_response":"yes"}`
	for split := 0; split <= len(raw); split += 7 {
		if got := streamSurface([]string{raw[:split], raw[split:]}); got != "yes" {
			t.Fatalf("split %d: stream = %q, want %q", split, got, "yes")
		}
	}
}

// Text around the envelope is a declared legacy framing: the reply is still
// admitted, but as "json_extracted" with a warning, never as a clean "json"
// parse that silently drops what came after.
func TestAdmission_TrailingTextIsReportedNotSilentlyDropped(t *testing.T) {
	env := `{` + admitControl + `,"surface_response":"The setting was saved."}`
	for name, raw := range map[string]string{
		"trailing qualification": env + "\n\nActually, it was not saved.",
		"leading prose":          "Here you go: " + env,
	} {
		t.Run(name, func(t *testing.T) {
			res, err := NewResponseProcessor().Process(raw)
			if err != nil {
				t.Fatalf("Process: %v", err)
			}
			if res.ParseMethod != "json_extracted" || res.Admission != AdmissionAdmitted {
				t.Fatalf("method = %q admission = %q, want an admitted json_extracted reply", res.ParseMethod, res.Admission)
			}
			if len(res.Warnings) == 0 || !strings.Contains(strings.Join(res.Warnings, " "), "outside the envelope") {
				t.Fatalf("warnings = %v, want one naming the dropped outside text", res.Warnings)
			}
		})
	}
}

// Two complete envelopes are ambiguous whichever comes first, and a second
// object that is not an envelope does not make the first one ambiguous.
func TestAdmission_SecondEnvelopeIsAmbiguousButOtherObjectsAreNot(t *testing.T) {
	a := `{` + admitControl + `,"surface_response":"A"}`
	b := `{` + admitControl + `,"surface_response":"B"}`
	for name, raw := range map[string]string{
		"back to back":     a + b,
		"example then not": "Example: " + a + "\nActual: " + b,
	} {
		res, err := NewResponseProcessor().Process(raw)
		if err != nil || res.Admission != AdmissionAmbiguous {
			t.Fatalf("%s: admission = %q err = %v, want ambiguous", name, res.Admission, err)
		}
	}
	res, err := NewResponseProcessor().Process(a + ` and the options were {"a": 1}`)
	if err != nil || res.Admission != AdmissionAdmitted || res.Surface != "A" {
		t.Fatalf("an envelope beside a non-envelope object: admission = %q surface = %q err = %v", res.Admission, res.Surface, err)
	}
}

func TestAdmission_ClassifiesPlainAndUnfinishedReplies(t *testing.T) {
	res, _ := NewResponseProcessor().Process("Just a sentence.")
	if res.Admission != AdmissionPlainOnly {
		t.Fatalf("plain reply admission = %q", res.Admission)
	}
	res, _ = NewResponseProcessor().Process(`{"control_packet":{"mangle_updates":["a(1)."]},"surface_response":"cut o`)
	if res.Admission != AdmissionMalformed {
		t.Fatalf("unfinished envelope admission = %q", res.Admission)
	}
	if got := ProcessLLMResponseAllowPlain(`{"control_packet":{"mangle_updates":["a(1)."]},"surface_response":"cut o`); got.Control != nil {
		t.Fatalf("an unfinished envelope must carry no control, got %+v", got.Control)
	}
}

// Every control the parser removes keeps a disposition at its original
// position, so a consumer can tell "removed" from "never sent".
func TestAdmission_ParserRemovalsKeepDispositions(t *testing.T) {
	raw := `{"control_packet":{"mangle_updates":["ok(1).","","no_paren.","bad(1)","evil(\"a\") && x."],"memory_operations":[]},"surface_response":"A"}`
	res, err := NewResponseProcessor().Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	wantIdx := []int{1, 2, 3, 4}
	if len(res.Dispositions) != len(wantIdx) {
		t.Fatalf("dispositions = %+v, want %d removals", res.Dispositions, len(wantIdx))
	}
	for i, d := range res.Dispositions {
		if d.Index != wantIdx[i] || d.Stage != StageRejectedParse || d.Kind != ControlMangleUpdate {
			t.Errorf("disposition %d = %+v, want rejected_parse of update #%d", i, d, wantIdx[i])
		}
		if d.Notice() == "" {
			t.Errorf("disposition %d has no notice", i)
		}
	}
	if got := ProcessLLMResponseAllowPlain(raw); len(got.Dispositions) != len(wantIdx) {
		t.Fatalf("ProcessedLLMResponse lost the dispositions: %+v", got.Dispositions)
	}
}

func TestAdmission_ListLimitsRecordTheRejectedTail(t *testing.T) {
	var ops []string
	for i := 0; i < 503; i++ {
		ops = append(ops, fmt.Sprintf(`{"op":"note","key":"k%d","value":"v"}`, i))
	}
	raw := `{"control_packet":{"memory_operations":[` + strings.Join(ops, ",") + `]},"surface_response":"A"}`
	res, err := NewResponseProcessor().Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Control.MemoryOperations) != 500 || len(res.Dispositions) != 3 {
		t.Fatalf("ops = %d dispositions = %d, want 500 kept and 3 recorded", len(res.Control.MemoryOperations), len(res.Dispositions))
	}
	if d := res.Dispositions[0]; d.Index != 500 || d.Kind != ControlMemoryOperation || d.Stage != StageRejectedParse {
		t.Fatalf("first tail disposition = %+v", d)
	}
}

func TestControlDispositionNoticeIsHostText(t *testing.T) {
	d := ControlDisposition{Kind: ControlMemoryOperation, Index: 2, Stage: StageFailed, Detail: "note k", Reason: "disk full"}
	if got, want := d.Notice(), "[Control] memory operation note k #3 failed: disk full"; got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
	if (ControlDisposition{Stage: StageApplied}).Notice() != "" {
		t.Fatal("an applied control needs no notice")
	}
}

func TestMangleUpdateDispositions_OneDispositionPerPositionInTheModelsList(t *testing.T) {
	// The parser removed the model's updates #0 and #2; the consumer filtered
	// the three that were left, and the kernel refused the second write.
	kept := []string{`observation("a", "b").`, `test_state(/passing).`, `observation("c", "d").`}
	origins := []int{1, 3, 4}
	_, blocked := core.FilterMangleUpdates(nil, kept, core.ModelObservationPolicy())
	ds := MangleUpdateDispositions(kept, origins, blocked, func(admitted int) error {
		if admitted == 1 {
			return fmt.Errorf("kernel full")
		}
		return nil
	})
	want := []struct {
		index int
		stage ControlStage
	}{{1, StageApplied}, {3, StageRejectedPolicy}, {4, StageFailed}}
	if len(ds) != len(want) {
		t.Fatalf("dispositions = %+v", ds)
	}
	for i, w := range want {
		if ds[i].Index != w.index || ds[i].Stage != w.stage {
			t.Errorf("disposition %d = %+v, want position %d %s", i, ds[i], w.index, w.stage)
		}
	}
}
