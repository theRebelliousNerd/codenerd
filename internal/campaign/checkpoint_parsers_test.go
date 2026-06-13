package campaign

import "testing"

func TestParseTestOutput(t *testing.T) {
	cr := &CheckpointRunner{}
	p, f := cr.parseTestOutput("--- PASS: TestA (0.01s)\n--- FAIL: TestB (0.02s)\n")
	if p != 1 || f != 1 {
		t.Errorf("parseTestOutput=(%d,%d), want (1,1)", p, f)
	}
	// Empty/unparseable output optimistically assumes a single pass.
	if p, f := cr.parseTestOutput(""); p != 1 || f != 0 {
		t.Errorf("parseTestOutput(empty)=(%d,%d), want (1,0)", p, f)
	}
}

func TestParseGoTestJSON(t *testing.T) {
	cr := &CheckpointRunner{}
	out := `{"Action":"pass","Test":"TestA","Elapsed":0.5}
{"Action":"fail","Test":"TestB","Elapsed":0.1}
{"Action":"pass","Test":"TestC","Elapsed":0.2}`
	p, f, dur := cr.parseGoTestJSON(out)
	if p != 2 || f != 1 {
		t.Errorf("parseGoTestJSON counts=(%d,%d), want (2,1)", p, f)
	}
	if dur <= 0 {
		t.Errorf("expected positive accumulated duration, got %v", dur)
	}
}

func TestParseJestJSON(t *testing.T) {
	cr := &CheckpointRunner{}
	if p, f := cr.parseJestJSON([]byte(`{"numPassedTests":5,"numFailedTests":2}`)); p != 5 || f != 2 {
		t.Errorf("parseJestJSON=(%d,%d), want (5,2)", p, f)
	}
	// Malformed input yields zeros rather than an error/panic.
	if p, f := cr.parseJestJSON([]byte("not json")); p != 0 || f != 0 {
		t.Errorf("parseJestJSON(bad)=(%d,%d), want (0,0)", p, f)
	}
}
