package projectdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

const validDoc = `---
schema: nerd/v1
project: codeNERD
language: Go
commands:
  build: go build -o nerd.exe ./cmd/nerd
  test: go test ./...
  env:
    CGO_CFLAGS: -IC:/CodeProjects/codeNERD/sqlite_headers
forbid:
  - match: .nerd/config.json
    reason: user-owned runtime config; edit by hand only
require:
  - run go test ./... before handoff
conventions:
  - id: conventional-commits
    rule: commit subjects use a conventional-commit prefix
---

Prose body that is advisory only.
`

func TestParse_Valid(t *testing.T) {
	doc, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Spec.Project != "codeNERD" {
		t.Errorf("Project = %q", doc.Spec.Project)
	}
	if doc.Spec.Commands.Build != "go build -o nerd.exe ./cmd/nerd" {
		t.Errorf("Build = %q", doc.Spec.Commands.Build)
	}
	if doc.Spec.Commands.Env["CGO_CFLAGS"] == "" {
		t.Error("command env was dropped; a build prerequisite invisible in the command string is exactly what this field is for")
	}
	if doc.Body != "Prose body that is advisory only." {
		t.Errorf("Body = %q", doc.Body)
	}
}

// Strictness is the feature. A directive the author believes is in force but
// which the parser silently dropped is worse than no directive.
func TestParse_UnknownKeyIsAHardError(t *testing.T) {
	doc := strings.Replace(validDoc, "project: codeNERD", "projekt: codeNERD", 1)
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("a misspelled key must fail the parse, not be ignored")
	}
	if !strings.Contains(err.Error(), "projekt") {
		t.Errorf("error must name the offending key so the author can fix it, got: %v", err)
	}
}

func TestParse_SchemaVersionIsPinned(t *testing.T) {
	cases := map[string]string{
		"missing": strings.Replace(validDoc, "schema: nerd/v1\n", "", 1),
		"future":  strings.Replace(validDoc, "nerd/v1", "nerd/v2", 1),
		"foreign": strings.Replace(validDoc, "nerd/v1", "agents/v1", 1),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(doc)); err == nil {
				t.Fatal("must refuse to half-apply a document written for a different contract")
			}
		})
	}
}

func TestParse_ForbidRuleMustExplainItself(t *testing.T) {
	doc := strings.Replace(validDoc,
		"    reason: user-owned runtime config; edit by hand only\n", "", 1)
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("a forbid rule with no reason must be rejected: an unexplained denial reads as a malfunction and invites a workaround")
	}
}

func TestParse_ForbidRuleWithEmptyMatchIsRejected(t *testing.T) {
	doc := strings.Replace(validDoc, "match: .nerd/config.json", `match: ""`, 1)
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("an empty match is a substring of every path and would deny every write")
	}
}

func TestParse_RequiresFrontmatter(t *testing.T) {
	if _, err := Parse([]byte("# Just prose\n\nno frontmatter here")); err == nil {
		t.Fatal("nerd.md without frontmatter must be rejected; the machine-readable half is the point")
	}
	if _, err := Parse([]byte("---\nschema: nerd/v1\nproject: x\n")); err == nil {
		t.Fatal("an unterminated frontmatter block must be rejected")
	}
	if _, err := Parse(nil); err == nil {
		t.Fatal("an empty file must be rejected")
	}
}

func TestForbidsPath(t *testing.T) {
	doc, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"exact relative path", ".nerd/config.json", true},
		{"absolute path", "C:/CodeProjects/codeNERD/.nerd/config.json", true},
		{"windows separators", `.nerd\config.json`, true},
		{"different case", ".NERD/Config.JSON", true},
		{"unprotected sibling", ".nerd/agents.json", false},
		{"unrelated", "internal/session/executor.go", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, got := doc.ForbidsPath(tc.path)
			if got != tc.want {
				t.Fatalf("ForbidsPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
			if got && reason == "" {
				t.Error("a denial must carry its reason")
			}
		})
	}
}

// A separator or case difference walking through the gate would make write
// protection trivially bypassable on Windows, where both are routine.
func TestForbidsPath_NormalizationIsNotBypassable(t *testing.T) {
	doc, err := Parse([]byte(strings.Replace(validDoc,
		"match: .nerd/config.json", `match: Secrets/Prod`, 1)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for _, path := range []string{
		"secrets/prod/keys.yaml",
		`SECRETS\PROD\keys.yaml`,
		"./app/Secrets/Prod/keys.yaml",
	} {
		if _, forbidden := doc.ForbidsPath(path); !forbidden {
			t.Errorf("ForbidsPath(%q) = false; separator and case differences must not open the gate", path)
		}
	}
}

func TestFacts(t *testing.T) {
	doc, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc.Path = "nerd.md"

	byPred := map[string][]types.Fact{}
	for _, f := range doc.Facts() {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}

	for _, pred := range []string{
		PredPresent, PredName, PredLanguage, PredCommand,
		PredCommandEnv, PredForbiddenPath, PredRequirement, PredConvention,
	} {
		if len(byPred[pred]) == 0 {
			t.Errorf("no %s fact emitted", pred)
		}
	}

	// /lang is an atom dimension. A quoted "go" is a disjoint type in Mangle and
	// would never unify with current_context(/lang, /go), so the fact would look
	// present and match nothing.
	langArg := byPred[PredLanguage][0].Args[0]
	atom, ok := langArg.(types.MangleAtom)
	if !ok {
		t.Fatalf("%s arg is %T, want types.MangleAtom", PredLanguage, langArg)
	}
	if string(atom) != "/go" {
		t.Errorf("language atom = %q, want %q (lowercased and slash-prefixed)", atom, "/go")
	}
}

// Every emitted fact must survive ToAtom. A fact that cannot be converted is
// evicted at the kernel boundary, so a write-protection rule would silently not
// exist.
func TestFacts_AllConvertToAtoms(t *testing.T) {
	doc, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc.Path = "nerd.md"

	for _, f := range doc.Facts() {
		if _, err := f.ToAtom(); err != nil {
			t.Errorf("fact %s%v does not convert to a Mangle atom: %v", f.Predicate, f.Args, err)
		}
	}
}

func TestFacts_NilDocumentIsSafe(t *testing.T) {
	var doc *Document
	if got := doc.Facts(); got != nil {
		t.Errorf("Facts() on nil = %v, want nil", got)
	}
	if got := doc.PromptSection(); got != "" {
		t.Errorf("PromptSection() on nil = %q, want empty", got)
	}
	if _, forbidden := doc.ForbidsPath("anything"); forbidden {
		t.Error("a nil document must forbid nothing")
	}
}

func TestPromptSection_StatesThatProtectionIsEnforced(t *testing.T) {
	doc, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	doc.Path = "nerd.md"
	section := doc.PromptSection()

	for _, want := range []string{
		"go build -o nerd.exe ./cmd/nerd", // the real command, not an inferred one
		"CGO_CFLAGS",
		".nerd/config.json",
		"ENFORCED", // the model must know this is not advice it can weigh
		"conventional-commits",
		"Prose body that is advisory only.",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("prompt section is missing %q:\n%s", want, section)
		}
	}
}

func TestLoad_AbsentFileIsNotAnError(t *testing.T) {
	doc, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("a workspace with no nerd.md must not error: %v", err)
	}
	if doc != nil {
		t.Errorf("doc = %v, want nil", doc)
	}
}

// An unreadable directive must never degrade to "no directive". Load surfaces
// the error so the boot path can say out loud that no rules are in force.
func TestLoad_MalformedFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("---\nschema: nerd/v9\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("a malformed nerd.md must be reported, not silently ignored")
	}
}

func TestFind_PrefersWorkspaceRootOverNerdDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, ".nerd")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, FileName), []byte(validDoc), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Only .nerd/nerd.md exists — it should be found.
	if got := Find(dir); got == "" {
		t.Fatal("Find must fall back to .nerd/")
	}

	// Root wins once present.
	root := filepath.Join(dir, FileName)
	if err := os.WriteFile(root, []byte(validDoc), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := Find(dir); got != root {
		t.Errorf("Find = %q, want the workspace-root copy %q", got, root)
	}
}
