package gates

import "testing"

// pytest exits 5 when it collects nothing. The detected gate passes that exit
// (a node without tests has not failed) but says it executed no test; a gate
// the project declared with the same ok exits keeps its declared meaning.
func TestDetectedPytestSaysExitFiveExecutedNoTest(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"pyproject.toml": "[project]\nname = \"x\"\n"})
	var pytest Gate
	for _, g := range detect(root) {
		if g.ID == "python:pytest" {
			pytest = g
		}
	}
	if pytest.ID == "" {
		t.Fatal("no python:pytest gate detected")
	}
	if !pytest.Passed(5) || !pytest.NoTests(5) {
		t.Errorf("exit 5: passed=%v no_tests=%v, want both", pytest.Passed(5), pytest.NoTests(5))
	}
	if pytest.NoTests(0) || pytest.NoTests(1) {
		t.Error("exit 0 ran tests and exit 1 failed; neither is an empty collection")
	}
	declared := Gate{OKExitCodes: []int{0, 5}}
	if !declared.Passed(5) || declared.NoTests(5) {
		t.Error("a project-declared ok exit is a pass, not an empty collection")
	}
}
