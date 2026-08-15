package logging

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Call-site audit for AuditLogger.SafetyCheck, encoded as a ratchet.
//
// The safety_allow / safety_block event families were declared in audit.go and,
// for most of this repo's life, written by nobody: every constitutional verdict
// the system made left no durable trace, so an unattended run could not be
// audited for what it was allowed to do. The gates that DO record their verdict
// now must keep doing so, and a new gate must not appear without one.
//
// The audit is a classification of every site that asks the kernel whether an
// action is permitted:
//
//   - gate     — the answer decides whether an action executes. Its package
//     must record the verdict via logging.Audit().SafetyCheck.
//   - notGate  — the query is read-only (display, rule comparison) or consumes
//     an already-cleared decision. No audit obligation.
//   - knownGap — a real gate in a package this test cannot fix from here.
//     Listed with a reason; allowed to be missing, never required to be.
//
// A permitted-query site in none of the three buckets fails the test. That is
// the ratchet: classification is mandatory, and dropping the audit call from a
// classified gate turns it into an unclassified violation.

// permittedQueryMarkers are the shapes a kernel permission query takes in Go.
var permittedQueryMarkers = []string{
	`Query("permitted`,
	`Sprintf("permitted(`,
}

type gateClass int

const (
	classGate gateClass = iota
	classNotGate
	classKnownGap
)

// safetyGateInventory is the audit result, keyed by slash-separated repo path.
//
// auditedIn names the file that must contain the SafetyCheck call for a gate.
// It is usually the gate's own file, but not always: a gate function can
// legitimately return a bool and let its sole caller record the verdict, which
// is what CheckKernelPermitted does. Naming the file is what keeps this honest.
// The check used to ask only whether SOME file in the gate's *directory* called
// SafetyCheck, which credited internal/core/virtual_store.go for four unrelated
// guards in sibling files while CheckKernelPermitted's own verdict path could
// have gone unrecorded without the test noticing.
var safetyGateInventory = map[string]struct {
	class     gateClass
	reason    string
	auditedIn string
}{
	"internal/session/executor_tools.go": {
		classGate,
		"tool executor safety gate: queries permitted/3 and refuses the call on no match",
		"internal/session/executor_tools.go",
	},
	"internal/core/virtual_store.go": {
		classGate,
		"CheckKernelPermitted: default-deny authorization for every VirtualStore action. " +
			"The function itself only returns the verdict; both branches are recorded by its " +
			"sole caller, routeAction",
		"internal/core/virtual_store_routing.go",
	},
	"internal/shards/system/constitution.go": {
		classKnownGap,
		"ConstitutionGateShard.CheckAction decides allow/deny but records no audit event; owned by internal/shards",
		"",
	},
	"internal/shards/system/router.go": {
		classNotGate,
		"queries permitted_action, the post-verdict routing queue, not the verdict itself",
		"",
	},
	"internal/core/rule_court.go": {
		classNotGate,
		"compares permitted derivations between a sandbox and the live kernel to score a candidate rule",
		"",
	},
	"cmd/nerd/chat/model_session_context.go": {
		classNotGate,
		"read-only: lists permitted actions for the session context display",
		"",
	},
	"scripts/probe_safety/main.go": {
		classNotGate,
		"standalone diagnostic in package main: asserts a pending_action, queries permitted/N " +
			"and prints the rows. Nothing branches on the verdict, so there is no decision to audit",
		"",
	},
}

// fileCallsSafetyCheck reports whether src contains a real call to
// logging.Audit().SafetyCheck(...) — parsed, not grepped.
//
// It requires the full shape: a call whose function is a selector named
// SafetyCheck, whose receiver is itself a call to Audit(). That rejects a
// mention in a comment or a string, an unrelated method that happens to be
// named SafetyCheck, and a bare identifier reference that never invokes
// anything. A file that cannot be parsed is treated as NOT auditing, so a
// syntax error fails the gate rather than silently exempting it.
func fileCallsSafetyCheck(src string) bool {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SafetyCheck" {
			return true
		}
		// The receiver must itself be a call — Audit().SafetyCheck(...), not a
		// field or a package-level identifier that merely reads that way.
		recv, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := recv.Fun.(type) {
		case *ast.Ident:
			found = fn.Name == "Audit"
		case *ast.SelectorExpr:
			found = fn.Sel.Name == "Audit"
		}
		return true
	})
	return found
}

func TestSafetyCheckCallSites_WhenKernelGateExists_ShouldAuditTheVerdict(t *testing.T) {
	root := repoRoot(t)

	gateFiles := map[string]bool{}
	safetyCheckFiles := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable trees are not this test's business
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "testdata", ".nerd", ".claude": // .claude holds agent worktrees (full repo copies); walking it duplicates every file under a non-checkout path
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/logging/") {
			return nil // this package defines SafetyCheck; it is not a call site
		}

		for _, marker := range permittedQueryMarkers {
			if strings.Contains(content, marker) {
				gateFiles[rel] = true
				break
			}
		}
		// A real call expression, not the text "SafetyCheck(" anywhere in the
		// file. The substring form was fully bypassable: an adversarial pass
		// deleted all three genuine logging.Audit().SafetyCheck calls from the
		// gate's auditing file, left a COMMENT containing "SafetyCheck(" behind,
		// and the test stayed green with grep counting zero real calls. A guard
		// over a safety property that a comment can satisfy is not a guard.
		if fileCallsSafetyCheck(content) {
			safetyCheckFiles[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking repo: %v", err)
	}

	if len(gateFiles) == 0 {
		t.Fatal("found no permitted-query sites at all — the scanner has drifted from the code")
	}

	var unclassified, unaudited, misdeclared []string
	for rel := range gateFiles {
		entry, known := safetyGateInventory[rel]
		if !known {
			unclassified = append(unclassified, rel)
			continue
		}
		if entry.class != classGate {
			continue
		}
		// An inventory entry that names no auditing file cannot be checked, so
		// the omission has to fail rather than pass quietly — otherwise the
		// cheapest way to silence this test is to blank the field.
		if entry.auditedIn == "" {
			misdeclared = append(misdeclared, rel+" (classGate with no auditedIn)")
			continue
		}
		if !safetyCheckFiles[entry.auditedIn] {
			unaudited = append(unaudited,
				rel+" — expected the verdict in "+entry.auditedIn+" ("+entry.reason+")")
		}
	}
	sort.Strings(unclassified)
	sort.Strings(unaudited)
	sort.Strings(misdeclared)

	if len(misdeclared) > 0 {
		t.Errorf("gate(s) in safetyGateInventory declare no auditedIn file: %v\n"+
			"Name the file that records this gate's verdict. It is the gate's own file unless "+
			"the gate returns a bool and a caller records it.", misdeclared)
	}

	if len(unclassified) > 0 {
		t.Errorf("new kernel-permission query site(s) with no classification: %v\n"+
			"Classify each in safetyGateInventory (this file). If it decides whether an action runs, "+
			"it is a gate and its package must call logging.Audit().SafetyCheck with the verdict.",
			unclassified)
	}
	if len(unaudited) > 0 {
		t.Errorf("constitutional gate(s) no longer record their verdict: %v\n"+
			"Every allow and every deny belongs in the audit trail — the safety_allow/safety_block "+
			"families exist so an unattended run can be audited afterwards.",
			unaudited)
	}
}

// TestSafetyCheck_WhenVerdictRecorded_ShouldWriteBothFamilies proves the other
// half: that a call site which does the right thing produces a queryable fact.
func TestSafetyCheck_WhenVerdictRecorded_ShouldWriteBothFamilies(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Audit().SafetyCheck("/edit main.go", true, "kernel policy permitted")
	Audit().SafetyCheck("/shell rm -rf /", false, "no permitted fact derived")
	CloseAll()

	audit := readLog(t, ws, "audit")
	for _, want := range []string{
		`"event":"safety_allow"`,
		`"event":"safety_block"`,
		`safety_check(`,
		`/safety_allow`,
		`/safety_block`,
		"no permitted fact derived",
	} {
		if !strings.Contains(audit, want) {
			t.Errorf("audit log missing %q\n---\n%s", want, audit)
		}
	}
}

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate the module root from the test working directory")
		}
		dir = parent
	}
}
