package observation

import (
	"fmt"
	"strings"
)

// PartialState says what a failed run had already done, from the fields the
// producer measured: the files it changed, the checks it ran, the acceptance
// verdict, the tool failures it recorded and the kernel's account of what the
// turn still owed. It reads nothing out of Output and claims nothing the
// producer did not observe.
//
// A run that fails after tool work has changed a workspace. The error alone
// tells its reader the run stopped; this tells them what is on disk and what
// nobody checked. It returns "" when the producer measured nothing -- which is
// not a claim that nothing happened, only that this return cannot say.
//
// Every changed path and every note is listed: the reader is deciding what to
// keep or undo, and a list cut for tidiness is the list they cannot decide
// from.
func (r Return) PartialState() string {
	var lines []string
	if len(r.Changed) > 0 {
		lines = append(lines, fmt.Sprintf("Before it stopped it changed %d file(s): %s", len(r.Changed), strings.Join(r.Changed, ", ")))
	}
	if r.Build != nil {
		lines = append(lines, "Build "+strings.TrimPrefix(r.Build.Text(), "build "))
	}
	if r.Tests != nil {
		lines = append(lines, "Tests "+strings.TrimPrefix(r.Tests.Text(), "tests "))
	}
	if r.Acceptance != nil {
		lines = append(lines, "Acceptance: "+r.Acceptance.Status+" ("+r.Acceptance.Summary+")")
	}
	if len(r.Missing) > 0 {
		lines = append(lines, "Not established: "+strings.Join(r.Missing, ", "))
	}
	for _, f := range r.Findings {
		lines = append(lines, fmt.Sprintf("Finding %s:%d %s", f.File, f.Line, f.Message))
	}
	lines = append(lines, r.Notes...)
	if len(lines) == 0 {
		return ""
	}
	return "Partial state (measured; nothing here is verified complete):\n- " + strings.Join(lines, "\n- ")
}
