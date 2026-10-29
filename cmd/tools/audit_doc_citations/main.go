// Command audit_doc_citations is the tracked R6 citation grader.
//
// Docs/journeys/05-elite-harness-ladder.md (R6, 2026-09-20) says every path a
// doc cites must resolve and every line number must fall inside that file, and
// that the checker has to live in the tree before the rung is scored.
// scripts/doc_citation_check.py is that checker today, but scripts/ is
// gitignored, the script hardcodes ROOT to one Windows checkout, it never
// looks at a symbol or a Mangle Decl, and it always exits 0. A harness gate
// cannot depend on an untracked command that cannot fail.
//
// This command walks a docs tree for the citations that script recognises —
// repo paths under internal/ and cmd/ ending in .go or .mg, with an optional
// :line, range, or comma list — and for the symbol citations the docs
// actually write beside them: a backticked identifier, `pkg.Func`,
// `(*T).Method`, a `#Symbol` suffix, or a `name/arity` predicate. The repo
// root is a flag, or the module root found by walking up from the working
// directory for go.mod. Nothing here is an absolute path.
//
// A path must exist, a line must fall inside the file, a Go symbol must be
// declared in the cited file or its package, and a Mangle predicate must have
// a Decl somewhere in the .mg corpus. A backticked filename (`nerd.md`,
// `feedback.go`, `usage.json`) is not a symbol: the symbol grammar allows a
// dot, and those were being graded against a directory of the same base name.
// A name the standard library declares (`sync.Map`, `fmt.Errorf`,
// `filepath.Join`) is not a citation of a repo directory that happens to share
// the qualifier, and it is not a missing declaration of the file it sits beside.
// One line per broken citation, then a summary. Exit 1 when anything is broken,
// 0 when the tree is clean, 2 when the command itself cannot run. -json writes
// the same report as JSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("audit_doc_citations", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rootFlag := fs.String("root", "", "repository root (default: the module root found by walking up from the working directory for go.mod)")
	docsFlag := fs.String("docs", defaultDocs, "docs tree to scan, relative to the repository root unless absolute")
	asJSON := fs.Bool("json", false, "write a JSON report to stdout instead of text")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: audit_doc_citations [-root dir] [-docs dir] [-json]\n\n")
		fmt.Fprintf(stderr, "Checks internal/ and cmd/ citations in a docs tree.\n")
		fmt.Fprintf(stderr, "Exit 1 if any citation is broken, 0 if the tree is clean.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "audit_doc_citations: unexpected argument %q\n", fs.Arg(0))
		return 2
	}

	root := *rootFlag
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "audit_doc_citations: %v\n", err)
			return 2
		}
		root, err = findModuleRoot(cwd)
		if err != nil {
			fmt.Fprintf(stderr, "audit_doc_citations: %v\n", err)
			return 2
		}
	}

	rep, err := Audit(Options{RepoRoot: root, DocsRoot: *docsFlag})
	if err != nil {
		fmt.Fprintf(stderr, "audit_doc_citations: %v\n", err)
		return 2
	}
	var werr error
	if *asJSON {
		werr = writeJSON(stdout, rep)
	} else {
		werr = writeText(stdout, rep)
	}
	if werr != nil {
		fmt.Fprintf(stderr, "audit_doc_citations: %v\n", werr)
		return 2
	}
	if rep.Broken > 0 {
		return 1
	}
	return 0
}

func writeText(w io.Writer, r Report) error {
	for _, f := range r.Findings {
		if _, err := fmt.Fprintf(w, "%s:%d: %s: %s: %s\n", f.Doc, f.Line, f.Citation, f.Kind, f.Reason); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w,
		"summary: broken=%d citations=%d docs=%d missing-path=%d line-out-of-range=%d symbol-not-declared=%d predicate-not-declared=%d\n",
		r.Broken, r.Citations, r.DocsScanned,
		r.ByKind[kindMissingPath], r.ByKind[kindLineOutOfRange],
		r.ByKind[kindSymbolNotDeclared], r.ByKind[kindPredicateNotDeclared])
	return err
}

func writeJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// findModuleRoot walks up from start until it finds a go.mod file. The
// gitignored grader hardcodes C:/CodeProjects/codeNERD; a gate has to run
// against whatever checkout it was pointed at.
func findModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		st, err := os.Stat(gomod)
		if err == nil && !st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found from %s", start)
		}
		dir = parent
	}
}
