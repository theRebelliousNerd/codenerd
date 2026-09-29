package northstar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/types"
)

type deriveScriptClient struct {
	complete func(context.Context, string, string) (string, error)
}

func (c *deriveScriptClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return c.complete(ctx, system, user)
}

type deriveFrame struct {
	path, summary, body string
	start, end          int
	final               bool
}

// Decode lengths independently of production framing, including embedded
// newlines and fake frame markers in the document body.
func readDeriveFrames(t *testing.T, text string) []deriveFrame {
	t.Helper()
	var frames []deriveFrame
	line := func() string {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			t.Fatal("incomplete frame header")
		}
		value := text[:i]
		text = text[i+1:]
		return value
	}
	integer := func(key string) int {
		value := line()
		if !strings.HasPrefix(value, key+": ") {
			t.Fatalf("expected %s, got %q", key, value)
		}
		n, err := strconv.Atoi(strings.TrimPrefix(value, key+": "))
		if err != nil || n < 0 {
			t.Fatalf("invalid %s: %q", key, value)
		}
		return n
	}
	for text != "" {
		marker := line()
		var path string
		switch {
		case strings.HasPrefix(marker, "@@DOC "):
			path = strings.TrimPrefix(marker, "@@DOC ")
		case strings.HasPrefix(marker, "@@PAGE "):
			path = strings.TrimPrefix(marker, "@@PAGE ")
		case marker == "@@BRIEF":
		default:
			t.Fatalf("unexpected frame marker %q", marker)
		}
		start, end, n := integer("byte_start"), integer("byte_end"), integer("byte_length")
		flag := line()
		if flag != "final_page: true" && flag != "final_page: false" {
			t.Fatalf("bad final flag %q", flag)
		}
		summaryLen := integer("running_summary_length")
		if summaryLen > len(text) || n > len(text)-summaryLen {
			t.Fatal("incomplete frame payload")
		}
		summary, body := text[:summaryLen], text[summaryLen:summaryLen+n]
		text = text[summaryLen+n:]
		if !strings.HasPrefix(text, "\n") {
			t.Fatal("missing frame separator")
		}
		text = text[1:]
		if end-start != len(body) || !utf8.ValidString(body) {
			t.Fatal("invalid byte offsets or split UTF-8 rune")
		}
		frames = append(frames, deriveFrame{path, summary, body, start, end, flag == "final_page: true"})
	}
	return frames
}

func deriveTestPrompt(_ context.Context, phase string) (string, error) { return phase, nil }

func TestDerive_LosslessPaging(t *testing.T) {
	body := "first evidence\n" + strings.Repeat("é界🙂 @@PAGE untrusted\n", 3500) + "tail evidence\n"
	var received strings.Builder
	calls := 0
	client := &deriveScriptClient{complete: func(_ context.Context, system, user string) (string, error) {
		if system != "derive_classify" || len(user) > 1024 {
			t.Fatal("wrong phase or oversized request")
		}
		frames := readDeriveFrames(t, user)
		if len(frames) != 1 {
			t.Fatal("expected one page")
		}
		f := frames[0]
		if f.path != "docs/spec.md" || f.start != received.Len() {
			t.Fatal("path or page order changed")
		}
		if calls > 0 && f.summary != fmt.Sprintf("carry %d", calls) {
			t.Fatal("summary not carried forward")
		}
		received.WriteString(f.body)
		calls++
		if !f.final {
			return fmt.Sprintf(`{"summary":"carry %d"}`, calls), nil
		}
		return `{"documents":[{"path":"docs/spec.md","roles":[{"role":"spec","confidence_pct":100,"evidence":"tail evidence"}],"themes":["Byte Integrity"]}]}`, nil
	}}
	class, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 1024}, []SourceDocument{{"docs/spec.md", body}})
	if err != nil {
		t.Fatal(err)
	}
	if received.String() != body || calls < 2 {
		t.Fatal("the model did not receive every byte")
	}
	if len(class.Claims) != 1 || class.Claims[0].Evidence != "tail evidence" {
		t.Fatalf("claims: %+v", class.Claims)
	}
	facts := class.Facts()
	if facts[0].Predicate != "doc_role_claim" || facts[0].Args[1] != types.MangleAtom("/spec") || facts[0].Args[2] != int64(100) {
		t.Fatalf("role fact: %+v", facts[0])
	}
	if facts[1].Predicate != "doc_theme" || facts[1].Args[1] != "byte integrity" {
		t.Fatalf("theme fact: %+v", facts[1])
	}
}

func TestDerive_BatchesSmallDocumentsAndReportsOmissions(t *testing.T) {
	docs := []SourceDocument{{"a.md", "A evidence"}, {"b.md", "B evidence"}, {"empty.md", ""}}
	calls := 0
	client := &deriveScriptClient{complete: func(_ context.Context, _, user string) (string, error) {
		calls++
		frames := readDeriveFrames(t, user)
		if len(frames) != 3 || frames[2].body != "" {
			t.Fatal("small or empty document omitted from the request")
		}
		return `{"documents":[{"path":"a.md","roles":[{"role":"readme","confidence_pct":90,"evidence":"A evidence"}],"themes":["README"]},{"path":"invented.md","roles":[]}]}`, nil
	}}
	class, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 1024}, docs)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(class.Unread) != 2 || len(class.Rejected) != 1 || len(class.Claims) != 1 {
		t.Fatalf("calls=%d class=%+v", calls, class)
	}
}

func TestDerive_RoleParsing(t *testing.T) {
	for _, tc := range []struct {
		name, role, confidence, evidence string
		accepted                         bool
	}{
		{"valid", "north_star_draft", "92", "Vision draft", true},
		{"unknown", "best", "92", "Vision draft", false},
		{"atom prefix", "/vision", "92", "Vision draft", false},
		{"fraction", "vision", "92.5", "Vision draft", false},
		{"string", "vision", `"92"`, "Vision draft", false},
		{"exponent", "vision", "1e2", "Vision draft", false},
		{"negative", "vision", "-1", "Vision draft", false},
		{"overflow", "vision", "101", "Vision draft", false},
		{"null", "vision", "null", "Vision draft", false},
		{"invented evidence", "vision", "92", "Different line", false},
		{"multiple lines", "vision", "92", "Vision draft\nOther line", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := json.Marshal(map[string]any{"documents": []any{map[string]any{"path": "vision.md", "roles": []any{map[string]any{"role": tc.role, "confidence_pct": json.RawMessage(tc.confidence), "evidence": tc.evidence}}}}})
			if err != nil {
				t.Fatal(err)
			}
			client := &deriveScriptClient{complete: func(context.Context, string, string) (string, error) { return string(payload), nil }}
			class, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 512}, []SourceDocument{{"vision.md", "Vision draft\nOther line"}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(class.Claims) == 1) != tc.accepted || (len(class.Rejected) == 1) == tc.accepted {
				t.Fatalf("class=%+v", class)
			}
			if !tc.accepted && len(class.Facts()) != 0 {
				t.Fatal("rejected claim became a fact")
			}
		})
	}
}

func TestDerive_PagingFailuresAndCancellation(t *testing.T) {
	for _, response := range []string{`{}`, `{"summary":" "}`, `{"summary":"` + strings.Repeat("x", 1024) + `"}`, "refused"} {
		client := &deriveScriptClient{complete: func(context.Context, string, string) (string, error) { return response, nil }}
		if _, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 256}, []SourceDocument{{"long.md", strings.Repeat("x", 1024)}}); err == nil {
			t.Fatalf("accepted bad summary %q", response)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	client := &deriveScriptClient{complete: func(context.Context, string, string) (string, error) { cancel(); return `{"summary":"carry"}`, nil }}
	_, err := ClassifyDocuments(ctx, client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 256}, []SourceDocument{{"long.md", strings.Repeat("x", 1024)}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

const deriveDraftResponse = `{"Mission":"Build reliable tools","Problem":"Context drifts","Vision":"Ground every decision","Personas":[{"name":"Engineer","pain_points":["Drift"],"needs":["Evidence"],"source":"origin.md"}],"Capabilities":[{"id":"cap_1","description":"Ground decisions","timeline":"now","priority":"critical","serves":["Engineer","Ghost"],"source":"vision.md"}],"Risks":[{"id":"risk_1","description":"False claims","likelihood":"high","impact":"high","mitigation":"Check evidence","source":"origin.md"}],"Requirements":[{"id":"REQ-001","type":"functional","description":"Check source paths","priority":"must-have","supports":["cap_1","absent"],"addresses":["risk_1","absent"],"source":"vision.md"}],"Constraints":["No invented sources"],"FieldSources":{"Mission":"origin.md","Problem":"origin.md","Vision":"vision.md","Constraints":"origin.md"}}`

func TestDerive_WizardDocumentLinkIntegrity(t *testing.T) {
	brief := OrientationBrief{
		Months:  []MonthView{{Index: 0, Label: "2026-01", Commits: 2}},
		Origins: []BriefSource{{Path: "origin.md", Body: "ORIGIN FULL TEXT\n" + strings.Repeat("origin byte\n", 100)}},
		Evolved: []EvolutionLink{{Old: "origin.md", New: "vision.md"}},
		Vision:  []BriefSource{{Path: "older.md", Body: "LOW WEIGHT FULL TEXT", Weight: 30}, {Path: "vision.md", Body: "WIP\nHIGH WEIGHT FULL TEXT\n" + strings.Repeat("vision byte\n", 100), Weight: 90, Roles: []string{"/north_star_draft"}}},
	}
	var received strings.Builder
	previousSummary := ""
	client := &deriveScriptClient{complete: func(_ context.Context, phase, user string) (string, error) {
		if phase != "derive_vision" || len(user) > 512 {
			t.Fatal("wrong phase or oversized request")
		}
		f := readDeriveFrames(t, user)[0]
		if f.start != received.Len() || f.summary != previousSummary {
			t.Fatal("brief pages or summary out of order")
		}
		received.WriteString(f.body)
		if !f.final {
			previousSummary = "origin.md and vision.md, WIP retained"
			return `{"summary":"origin.md and vision.md, WIP retained"}`, nil
		}
		return deriveDraftResponse, nil
	}}
	draft, err := DraftVision(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 512}, brief)
	if err != nil {
		t.Fatal(err)
	}
	text := received.String()
	for _, full := range []string{brief.Origins[0].Body, brief.Vision[0].Body, brief.Vision[1].Body, "2026-01"} {
		if !strings.Contains(text, full) {
			t.Fatalf("missing complete evidence %q", full)
		}
	}
	if !(strings.Index(text, "## Timeline") < strings.Index(text, "## Origin sources") && strings.Index(text, "## Origin sources") < strings.Index(text, "## Evolution") && strings.Index(text, "## Evolution") < strings.Index(text, "## Vision sources")) {
		t.Fatal("brief hierarchy changed")
	}
	if strings.Index(text, "HIGH WEIGHT FULL TEXT") > strings.Index(text, "LOW WEIGHT FULL TEXT") {
		t.Fatal("vision sources not in weight order")
	}
	v := draft.Document.ToVision()
	if !textMarksDraft(v.VisionStmt) || !reflect.DeepEqual(v.Capabilities[0].Serves, []string{"Engineer"}) || !reflect.DeepEqual(v.Requirements[0].Supports, []string{"cap_1"}) || !reflect.DeepEqual(v.Requirements[0].Addresses, []string{"risk_1"}) {
		t.Fatalf("vision: %+v", v)
	}
	if len(draft.Notes) < 4 || !reflect.DeepEqual(draft.FieldSources["Capabilities[0]"], []string{"vision.md"}) {
		t.Fatalf("provenance or pruning not reported: %+v", draft)
	}
}

func TestDerive_DraftRejectsMissingOrInventedProvenance(t *testing.T) {
	for _, response := range []string{
		strings.Replace(deriveDraftResponse, `"Mission":"origin.md",`, "", 1),
		strings.Replace(deriveDraftResponse, `"source":"vision.md"`, `"source":"invented.md"`, 1),
		strings.Replace(deriveDraftResponse, `"timeline":"now"`, `"timeline":"yesterday"`, 1),
		strings.Replace(deriveDraftResponse, `"Mission":"Build reliable tools"`, `"Mission":""`, 1),
		deriveDraftResponse + ` {"ignored":true}`,
	} {
		if _, err := parseDraft(response, []BriefSource{{Path: "origin.md"}, {Path: "vision.md"}}); err == nil {
			t.Fatal("accepted invalid draft")
		}
	}
}

func TestDerive_RequirementsKeepAllFieldsAndObjects(t *testing.T) {
	state := WizardRequirementsInput{Mission: "Mission", Personas: []WizardPersona{{Name: "Engineer", PainPoints: []string{"pain"}, Needs: []string{"need"}}}, Capabilities: []WizardCapability{{ID: "cap_1", Description: "cap"}}, ExistingRequirements: []WizardRequirement{{ID: "REQ-001"}}, ExtractedFacts: []string{strings.Repeat("research bytes ", 100)}}
	var received strings.Builder
	var reqs []draftRequirement
	for i := 0; i < 20; i++ {
		reqs = append(reqs, draftRequirement{Type: "functional", Description: fmt.Sprintf("requirement %d", i), Priority: "must-have", Source: "Mission"})
	}
	reqs[1].ID = "REQ-002"
	payload, err := json.Marshal(map[string]any{"requirements": reqs})
	if err != nil {
		t.Fatal(err)
	}
	client := &deriveScriptClient{complete: func(_ context.Context, phase, user string) (string, error) {
		if phase != "derive_requirements" || len(user) > 512 {
			t.Fatal("wrong phase or bound")
		}
		f := readDeriveFrames(t, user)[0]
		received.WriteString(f.body)
		if !f.final {
			return `{"summary":"retain all research and existing REQ-001"}`, nil
		}
		return string(payload), nil
	}}
	got, err := DeriveRequirements(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 512}, state)
	if err != nil {
		t.Fatal(err)
	}
	var decoded WizardRequirementsInput
	if err := json.Unmarshal([]byte(received.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, state) || len(got) != 20 || got[0].ID != "REQ-003" || got[1].ID != "REQ-002" {
		t.Fatalf("lost state, object or explicit ID: %+v", got)
	}
	if _, err := parseRequirements(`{"requirements":[{"id":"REQ-001","description":"duplicate","type":"functional","priority":"must-have"}]}`, state.ExistingRequirements); err == nil {
		t.Fatal("reused existing ID")
	}
}

func TestInstallDerivedVision_PreservesExistingAuthority(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	existing := &Vision{Mission: "Human vision", Problem: "Existing problem", VisionStmt: "Existing commitment"}
	if err := store.SaveVision(existing); err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadVision()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteVisionJSON(dir, before); err != nil {
		t.Fatal(err)
	}
	if err := WriteVisionMangle(dir, before); err != nil {
		t.Fatal(err)
	}
	jsonBefore, err := os.ReadFile(filepath.Join(dir, VisionJSONFileName))
	if err != nil {
		t.Fatal(err)
	}
	mgBefore, err := os.ReadFile(filepath.Join(dir, VisionMangleFileName))
	if err != nil {
		t.Fatal(err)
	}
	draft := &Draft{Document: WizardDocument{Mission: "Candidate", Vision: "New draft"}, FieldSources: map[string][]string{"Mission": {"vision.md"}}}
	result, err := InstallDerivedVision(store, dir, draft)
	if err != nil {
		t.Fatal(err)
	}
	after, err := store.LoadVision()
	if err != nil {
		t.Fatal(err)
	}
	jsonAfter, err := os.ReadFile(filepath.Join(dir, VisionJSONFileName))
	if err != nil {
		t.Fatal(err)
	}
	mgAfter, err := os.ReadFile(filepath.Join(dir, VisionMangleFileName))
	if err != nil {
		t.Fatal(err)
	}
	if result.Installed || !reflect.DeepEqual(before, after) || string(jsonBefore) != string(jsonAfter) || string(mgBefore) != string(mgAfter) {
		t.Fatal("existing authority changed")
	}
	data, err := os.ReadFile(result.DerivedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "vision.md") || !strings.Contains(string(data), "Candidate") {
		t.Fatal("candidate or provenance missing")
	}
}

func TestCompilePhasePrompt_DeriveAtomsOnly(t *testing.T) {
	for _, phase := range []string{"derive_classify", "derive_vision", "derive_requirements"} {
		text, err := CompilePhasePrompt(context.Background(), phase)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(text, `"summary"`) || strings.Contains(text, "control_packet") {
			t.Fatalf("wrong protocol for %s", phase)
		}
	}
}

func TestDerive_BatchBudgetBoundary(t *testing.T) {
	docs := []SourceDocument{{"a.md", strings.Repeat("a", 200)}, {"b.md", strings.Repeat("b", 200)}, {"c.md", strings.Repeat("c", 200)}}
	calls := 0
	client := &deriveScriptClient{complete: func(_ context.Context, _, user string) (string, error) {
		if len(user) > 512 {
			t.Fatal("batch over configured request bound")
		}
		frames := readDeriveFrames(t, user)
		if len(frames) != 1 || frames[0].path != docs[calls].Path || frames[0].body != docs[calls].Body {
			t.Fatal("batch boundary lost or reordered bytes")
		}
		calls++
		payload, err := json.Marshal(map[string]any{"documents": []any{map[string]any{"path": frames[0].path, "roles": []any{map[string]any{"role": "reference", "confidence_pct": 100, "evidence": frames[0].body}}}}})
		return string(payload), err
	}}
	class, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 512}, docs)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 || len(class.Claims) != 3 {
		t.Fatal("batch flush missed documents")
	}
}

func TestDerive_InvalidInputsNeverReachTheModel(t *testing.T) {
	client := &deriveScriptClient{complete: func(context.Context, string, string) (string, error) {
		t.Fatal("invalid document reached the model")
		return "", nil
	}}
	for _, docs := range [][]SourceDocument{
		{{Path: "invalid.md", Body: string([]byte{0xff})}},
		{{Path: "bad\npath", Body: "valid UTF-8"}},
		{{Path: "same.md", Body: "first"}, {Path: "same.md", Body: "second"}},
	} {
		if _, err := ClassifyDocuments(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 512}, docs); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestInstallDerivedVision_FreshAuthorityBootsWithoutReimport(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	draft := &Draft{Document: WizardDocument{Mission: "Derived mission", Problem: "Evidence loss", Vision: "A sourced draft (work in progress)"}}
	result, err := InstallDerivedVision(store, dir, draft)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Installed || result.DerivedPath != "" {
		t.Fatal("fresh vision was not installed")
	}
	for _, name := range []string{VisionJSONFileName, VisionMangleFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	sync, err := SyncVisionAuthority(store, dir)
	if err != nil {
		t.Fatal(err)
	}
	if sync.Direction != SyncNoop {
		t.Fatalf("first boot would reimport: %+v", sync)
	}
}

func TestInstallDerivedVision_ConcurrentDraftsDoNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	start := make(chan struct{})
	type outcome struct {
		result InstallResult
		err    error
	}
	out := make(chan outcome, 2)
	for _, mission := range []string{"First candidate", "Second candidate"} {
		go func(mission string) {
			<-start
			result, err := InstallDerivedVision(store, dir, &Draft{Document: WizardDocument{Mission: mission}})
			out <- outcome{result, err}
		}(mission)
	}
	close(start)
	installed := 0
	var winner string
	for i := 0; i < 2; i++ {
		got := <-out
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.result.Installed {
			installed++
			winner = got.result.Vision.Mission
		}
	}
	vision, err := store.LoadVision()
	if err != nil {
		t.Fatal(err)
	}
	if installed != 1 || vision.Mission != winner {
		t.Fatal("concurrent derivation replaced existing authority")
	}
}

func TestInstallDerivedVision_ExportFailureReportsInstalledAuthority(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	blocked := filepath.Join(dir, "blocked-export-directory")
	if err := os.WriteFile(blocked, []byte("this is a file"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := InstallDerivedVision(store, blocked, &Draft{Document: WizardDocument{Mission: "Installed before export"}})
	if err == nil || !result.Installed || result.Vision == nil || !strings.Contains(err.Error(), "installed") {
		t.Fatalf("partial persistence was not reported: result=%+v error=%v", result, err)
	}
	vision, err := store.LoadVision()
	if err != nil {
		t.Fatal(err)
	}
	if vision == nil || vision.Mission != "Installed before export" {
		t.Fatal("authority was not preserved after export failure")
	}
}

func TestDerive_DraftMarkerCannotBeEscapedByFieldCitation(t *testing.T) {
	brief := OrientationBrief{Origins: []BriefSource{{Path: "origin.md", Body: "Origins"}}, Vision: []BriefSource{
		{Path: "vision.md", Body: "Established vision", Weight: 80},
		{Path: "emerging.md", Body: "Work in progress", Weight: 90, Roles: []string{"north_star_draft"}},
	}}
	client := &deriveScriptClient{complete: func(context.Context, string, string) (string, error) { return deriveDraftResponse, nil }}
	draft, err := DraftVision(context.Background(), client, deriveTestPrompt, DeriveBudget{MaxRequestBytes: 4096}, brief)
	if err != nil {
		t.Fatal(err)
	}
	if !textMarksDraft(draft.Document.ToVision().VisionStmt) {
		t.Fatal("model promoted a policy-selected draft source by citing another path")
	}
}
