package verification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/observation"
)

// The judge/v2 reply contract (judge_contract.go): JSON syntax is not a
// verification contract. Each refused reply below decoded without error under
// the old parser.
func TestParseJudgment_RefusesWhatIsNotAJudgment(t *testing.T) {
	for name, reply := range map[string]string{
		"empty":                    ``,
		"prose":                    `looks good`,
		"empty object":             `{}`,
		"null":                     `null`,
		"array":                    `[]`,
		"missing contract":         `{"status":"pass","confidence":0.9,"explanation":"ok"}`,
		"other contract":           `{"contract":"judge/v1","status":"pass","confidence":0.9,"explanation":"ok"}`,
		"missing status":           `{"contract":"judge/v2","confidence":0.9,"explanation":"ok"}`,
		"unknown status":           `{"contract":"judge/v2","status":"fail","confidence":0.9,"explanation":"ok"}`,
		"missing confidence":       `{"contract":"judge/v2","status":"pass","explanation":"ok"}`,
		"null confidence":          `{"contract":"judge/v2","status":"pass","confidence":null,"explanation":"ok"}`,
		"confidence above 1":       `{"contract":"judge/v2","status":"pass","confidence":1.5,"explanation":"ok"}`,
		"negative confidence":      `{"contract":"judge/v2","status":"pass","confidence":-0.1,"explanation":"ok"}`,
		"confidence as a string":   `{"contract":"judge/v2","status":"pass","confidence":"high","explanation":"ok"}`,
		"blank explanation":        `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"  "}`,
		"unknown key":              `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok","success":true}`,
		"duplicate status":         `{"contract":"judge/v2","status":"defect","status":"pass","confidence":0.9,"explanation":"ok"}`,
		"duplicate nested key":     `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"x","citations":["E1"],"corrective_action":{"type":"docs","type":"tool","query":"q"}}`,
		"two values":               `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok"} {}`,
		"pass with a violation":    `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok","violations":["mock_code"]}`,
		"pass with a citation":     `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok","citations":["E1"]}`,
		"defect with no grounds":   `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"bad"}`,
		"defect requesting more":   `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"bad","citations":["E1"],"requested_evidence":["E2"]}`,
		"unknown violation":        `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"bad","citations":["E1"],"violations":["ugly"]}`,
		"insufficient + violation": `{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"?","violations":["mock_code"]}`,
		"corrective without query": `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"bad","citations":["E1"],"corrective_action":{"type":"docs","query":""}}`,
		"corrective naming shard":  `{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"bad","citations":["E1"],"corrective_action":{"type":"docs","query":"q","shard_hint":"researcher"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if j, err := parseJudgment(reply, 16384); err == nil {
				t.Fatalf("parseJudgment(%s) = %+v, want it refused", reply, j)
			}
		})
	}
}

func TestParseJudgment_AcceptsTheContract(t *testing.T) {
	for name, reply := range map[string]string{
		"pass":                  `{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok"}`,
		"fenced pass":           "```json\n{\"contract\":\"judge/v2\",\"status\":\"pass\",\"confidence\":1,\"explanation\":\"ok\"}\n```",
		"zero-confidence pass":  `{"contract":"judge/v2","status":"pass","confidence":0,"explanation":"ok","violations":[],"suggestions":[]}`,
		"grounded defect":       `{"contract":"judge/v2","status":"defect","confidence":0.8,"explanation":"stub","violations":["placeholder"],"citations":["E2"],"corrective_action":{"type":"docs","query":"read the API","reason":"wrong call"}}`,
		"labelled suspicion":    `{"contract":"judge/v2","status":"defect","confidence":0.4,"explanation":"smells","unsupported_suspicion":"probably mocked"}`,
		"insufficient evidence": `{"contract":"judge/v2","status":"insufficient_evidence","confidence":0.5,"explanation":"cut","requested_evidence":["E3"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJudgment(reply, 16384); err != nil {
				t.Fatalf("parseJudgment = %v, want it accepted", err)
			}
		})
	}
	if _, err := parseJudgment(`{"contract":"judge/v2","status":"pass","confidence":0.9,"explanation":"ok"}`, 10); err == nil {
		t.Fatal("a reply over delegation.judge_reply_bytes was accepted")
	}
}

// Grounding: a defect stands only on evidence the packet still supports.
func TestJudgmentGround(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ret := observation.Return{Outcome: "/done", Output: "done", Writes: []observation.FileWrite{{
		Path: path, Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: "package a\n"},
	}}}
	packet := buildEvidencePacket(1, "/implementation", ret, 24000)

	judge := func(reply string) *Judgment {
		t.Helper()
		j, err := parseJudgment(reply, 16384)
		if err != nil {
			t.Fatal(err)
		}
		j.ground(packet)
		return j
	}
	if j := judge(`{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"x","citations":["E2","E9"]}`); j.Status != JudgeDefect || len(j.Grounded) != 1 || len(j.Ungrounded) != 1 {
		t.Fatalf("one usable citation = %+v, want a grounded defect noting the unresolved one", j)
	}
	if j := judge(`{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"x","unsupported_suspicion":"maybe mocked"}`); j.Status != JudgeInsufficientEvidence || j.Claimed != JudgeDefect {
		t.Fatalf("a bare suspicion = %+v, want insufficient evidence", j)
	}
	if err := os.WriteFile(path, []byte("package a // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if j := judge(`{"contract":"judge/v2","status":"defect","confidence":0.9,"explanation":"x","citations":["E2"]}`); j.Status != JudgeInsufficientEvidence {
		t.Fatalf("a citation of a superseded revision = %+v, want insufficient evidence", j)
	}
}

func TestEvidencePacket_BoundsAndProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.go")
	content := strings.Repeat("x := 1\n", 2000)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ret := observation.Return{
		Outcome: "/done", Output: strings.Repeat("narrative ", 2000), Stage: "checks_passed",
		Build: &observation.Verification{Kind: "build", Source: observation.SourceObserved, Ran: true, OK: true, Outcome: "passed"},
		Tests: &observation.Verification{Kind: "tests", Source: observation.SourceReported, Ran: true, OK: true},
		Writes: []observation.FileWrite{{
			Path: path, Before: observation.FileState{Known: true}, After: observation.FileState{Known: true, Exists: true, Content: content},
		}},
	}
	p := buildEvidencePacket(1, "/implementation", ret, 4096)
	kinds := map[string]EvidenceItem{}
	for _, it := range p.Items {
		kinds[it.Kind+"/"+it.Source] = it
	}
	for _, want := range []string{"verdict/kernel", "stage/observed", "check/observed", "check/reported", "change/observed", "narrative/reported"} {
		if _, ok := kinds[want]; !ok {
			t.Errorf("packet lacks a %s item: %+v", want, p.Items)
		}
	}
	change := kinds["change/observed"]
	if change.Omitted == 0 || !p.expandable(change.ID) {
		t.Fatalf("an over-budget change = %+v, want it elided and expandable", change)
	}
	if n := kinds["narrative/reported"]; n.Omitted == 0 || len(n.Body) > 4096/4 {
		t.Fatalf("narrative = %d bytes (omitted %d), want it held to a quarter of the budget", len(n.Body), n.Omitted)
	}
	if got := p.expand([]string{change.ID}); !strings.Contains(got, "x := 1") || strings.Count(got, "x := 1") > 4096/len("x := 1\n")+1 {
		t.Fatalf("expansion not bounded by the budget (%d bytes)", len(got))
	}
	if p.expandable("E1") {
		t.Fatal("an unelided item is expandable")
	}
}
