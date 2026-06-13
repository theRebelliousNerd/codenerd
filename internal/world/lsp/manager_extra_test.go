package lsp

import (
	"testing"

	"codenerd/internal/mangle"
)

func TestDiagnosticSeverityToAtomMapping(t *testing.T) {
	cases := map[mangle.DiagnosticSeverity]string{
		mangle.DiagError:              "/error",
		mangle.DiagWarning:            "/warning",
		mangle.DiagInformation:        "/info",
		mangle.DiagHint:               "/hint",
		mangle.DiagnosticSeverity(99): "/unknown",
	}
	for sev, want := range cases {
		if got := diagnosticSeverityToAtom(sev); got != want {
			t.Errorf("diagnosticSeverityToAtom(%v)=%q, want %q", sev, got, want)
		}
	}
}

func TestProjectionsNilServer(t *testing.T) {
	// With no mangle LSP server wired, projections must fail closed (empty) rather
	// than panic.
	m := &Manager{}
	if got := m.projectDefinitions(); len(got) != 0 {
		t.Errorf("projectDefinitions with nil server should be empty, got %d", len(got))
	}
	if got := m.projectReferences(); len(got) != 0 {
		t.Errorf("projectReferences with nil server should be empty, got %d", len(got))
	}
	if got := m.projectDiagnostics(); len(got) != 0 {
		t.Errorf("projectDiagnostics with nil server should be empty, got %d", len(got))
	}
}
