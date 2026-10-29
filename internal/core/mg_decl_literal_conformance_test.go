package core

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This guard exists because the same defect has now shipped three times, in
// three unrelated subsystems, and none of them failed loudly.
//
// Mangle distinguishes the name constant /read_file from the string constant
// "read_file". They are different values and never unify. A Decl's bound list
// is the contract that says which one a slot holds — and when a rule head puts
// the other kind of literal there, the program usually still analyzes and the
// facts still derive. Nothing is reported. The cost lands later and elsewhere:
//
//   - MCP tool selection queried a /name-declared predicate with a quoted Go
//     string, matched nothing, and silently fell back to the Go heuristic. The
//     kernel was never consulted and no error said so.
//   - The browser honeypot detector pushed fmt.Sprintf("%.0f", …) strings into
//     the /number slots of position/5. That one is worse than dead rules: a
//     single type-wrong fact aborts whole-program evaluation, so every other
//     query in the kernel returns zero rows too.
//   - modular_tool_allowed was declared [/string, /name] while every rule head
//     in intent_routing_rules.mg emitted /name into the first slot. It derived
//     fine, which is exactly what made it a trap for the first Go consumer.
//
// The common thread is that the mismatch is invisible at the site where it is
// written and only observable at a distant call. So it gets a static check.
//
// Scope note: this compares literal arguments in .mg heads against their own
// Decl. It cannot see the Go side, where the other half of the bug lives —
// that is covered by the fact-convention guards in internal/types. Between
// them, a slot's type has to be wrong in the same way in both places to escape.

// declLine matches the single-line form every Decl in defaults/ uses:
//
//	Decl pred(A, B) bound [/string, /name].
var declLine = regexp.MustCompile(`^Decl\s+([a-z][A-Za-z0-9_]*)\s*\(([^)]*)\)\s*(bound\s*\[[^\]]*\](?:\s*bound\s*\[[^\]]*\])*)`)

// boundList pulls each alternative out of a Decl's bound clause. Mangle allows
// a UNION — several bound lists on one Decl — and exactly one predicate uses it:
//
//	Decl compile_context(Dimension, Value) bound [/name, /name] bound [/name, /string].
//
// because GenerateFacts emits a name when the value is slash-prefixed and a
// string otherwise. Matching only the first alternative would flag the perfectly
// legal "failing_tests" as a violation and push whoever hit it to "fix" a
// correct literal — a false positive is worse than a miss here, because it
// spends someone's trust in the guard.
var boundList = regexp.MustCompile(`bound\s*\[([^\]]*)\]`)

// atomStart matches any predicate application on a line — a fact, a rule head,
// or a body literal. The capture group is the predicate name; the match ends at
// its opening paren, so the parser can pick up the arguments from there.
//
// Anchoring to the line start would only find heads, and heads are not where
// this bug class hides best. A predicate that is pure EDB — asserted only from
// Go, never written as a fact in the corpus — has no head anywhere in .mg, so a
// head-only scan is structurally blind to it. user_intent/5 is exactly that:
// declared with a /string first slot, matched as `/current_intent` in a dozen
// rule bodies, and asserted from Go as MangleAtom("/current_intent"). The Decl
// was the only thing that disagreed, and nothing could see it.
//
// The leading [^A-Za-z0-9_:] is what keeps fn: transforms out: `fn:max(X)` is
// not a predicate application, and `max` is not a declared predicate anyway,
// but excluding the `:` form means the scanner never has to rely on that.
var atomStart = regexp.MustCompile(`(^|[^A-Za-z0-9_:])([a-z][A-Za-z0-9_]*)\s*\(`)

func TestMangleDecls_WhenAHeadUsesALiteral_ShouldMatchTheDeclaredBoundType(t *testing.T) {
	root := "defaults"

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".mg") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) == 0 {
		t.Fatalf("no .mg files under %s — the corpus moved and this guard is now inert", root)
	}

	// Pass 1: the declared contracts.
	bounds := map[string][][]string{}
	for _, f := range files {
		for _, line := range readLogicalLines(t, f) {
			m := declLine.FindStringSubmatch(line.text)
			if m == nil {
				continue
			}
			var alts [][]string
			for _, bm := range boundList.FindAllStringSubmatch(m[3], -1) {
				var types []string
				for _, b := range strings.Split(bm[1], ",") {
					types = append(types, strings.TrimSpace(b))
				}
				alts = append(alts, types)
			}
			bounds[m[1]] = alts
		}
	}
	if len(bounds) == 0 {
		t.Fatal("parsed zero bound Decls — the Decl syntax changed and this guard is now inert")
	}

	// Pass 2: every head literal, checked against its own Decl.
	type violation struct{ file, detail string }
	var violations []violation

	for _, f := range files {
		for _, line := range readLogicalLines(t, f) {
			if strings.HasPrefix(line.text, "Decl ") {
				continue
			}
			for _, loc := range atomStart.FindAllStringSubmatchIndex(line.text, -1) {
				pred := line.text[loc[4]:loc[5]]
				want, declared := bounds[pred]
				if !declared {
					continue // untyped Decl, or a predicate declared elsewhere
				}
				// loc[1], not loc[5]: the regex ends with the opening paren, so
				// the FULL match end is the index just past it. loc[5] is the end
				// of the predicate NAME, and passing it made atomArgs reject every
				// atom — which turned this whole guard into a test that could not
				// fail. Caught only because a known violation kept passing.
				args, ok := atomArgs(line.text, loc[1])
				if !ok || len(args) != len(want[0]) {
					// Arity mismatches, and atoms whose parentheses do not close
					// on this line, are the kernel's own analysis to report — it
					// does so loudly at boot. Guessing here would produce noise
					// that teaches people to ignore this test.
					continue
				}
				for i, arg := range args {
					got := literalKind(arg)
					if got == "" {
						continue
					}
					// A union Decl is satisfied by ANY of its alternatives.
					permitted := false
					var declared []string
					for _, alt := range want {
						if i >= len(alt) {
							continue
						}
						declared = append(declared, alt[i])
						if got == alt[i] {
							permitted = true
							break
						}
					}
					if permitted {
						continue
					}
					violations = append(violations, violation{
						file: f,
						detail: fmt.Sprintf("line %d: %s arg %d is %s (%s), but Decl says %s",
							line.num, pred, i+1, got, arg, strings.Join(declared, " or ")),
					})
				}
			}
		}
	}

	if len(violations) > 0 {
		sort.Slice(violations, func(i, j int) bool {
			if violations[i].file != violations[j].file {
				return violations[i].file < violations[j].file
			}
			return violations[i].detail < violations[j].detail
		})
		var b strings.Builder
		b.WriteString("Mangle head literals disagree with their Decl's bound types.\n\n")
		b.WriteString("A /name and a /string never unify. The program will still analyze and\n")
		b.WriteString("the facts will still derive — the failure surfaces at whatever queries\n")
		b.WriteString("this predicate next, as zero rows with no error, or as a whole-program\n")
		b.WriteString("evaluation abort if a /number slot got a string.\n\n")
		b.WriteString("Fix the Decl if the heads are right, or the heads if the Decl is right.\n")
		b.WriteString("Do not silence this by loosening the Decl to an untyped one: the bound\n")
		b.WriteString("list is what the Go side reads when it decides how to build a query.\n\n")
		for _, v := range violations {
			fmt.Fprintf(&b, "  %s %s\n", v.file, v.detail)
		}
		t.Error(b.String())
	}
}

type mgLine struct {
	num  int
	text string
}

// readLogicalLines returns comment-stripped, trimmed-right lines with their
// original 1-based numbers. Comments are stripped outside of quotes only: a #
// inside a string literal is content, not a comment.
func readLogicalLines(t *testing.T, path string) []mgLine {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []mgLine
	for i, line := range strings.Split(string(raw), "\n") {
		inQuote := false
		cut := -1
		for j := 0; j < len(line); j++ {
			switch line[j] {
			case '"':
				if j == 0 || line[j-1] != '\\' {
					inQuote = !inQuote
				}
			case '#':
				if !inQuote {
					cut = j
				}
			}
			if cut >= 0 {
				break
			}
		}
		if cut >= 0 {
			line = line[:cut]
		}
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			continue
		}
		out = append(out, mgLine{num: i + 1, text: line})
	}
	return out
}

// atomArgs splits the top-level arguments of the atom whose opening paren is at
// index open. It returns ok=false when the parentheses do not balance on this
// line, which is the signal to leave the atom alone rather than guess at it.
func atomArgs(line string, open int) ([]string, bool) {
	if open <= 0 || open > len(line) || line[open-1] != '(' {
		return nil, false
	}
	open-- // the regex match ends just past the paren; step back onto it
	depth := 0
	inQuote := false
	var args []string
	var cur strings.Builder
	for i := open; i < len(line); i++ {
		c := line[i]
		if inQuote {
			cur.WriteByte(c)
			if c == '"' && line[i-1] != '\\' {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
			cur.WriteByte(c)
		case '(', '[':
			depth++
			if depth > 1 {
				cur.WriteByte(c)
			}
		case ')', ']':
			depth--
			if depth == 0 {
				args = append(args, strings.TrimSpace(cur.String()))
				return args, true
			}
			cur.WriteByte(c)
		case ',':
			if depth == 1 {
				args = append(args, strings.TrimSpace(cur.String()))
				cur.Reset()
			} else {
				cur.WriteByte(c)
			}
		default:
			cur.WriteByte(c)
		}
	}
	return nil, false
}

// literalKind classifies an argument, or returns "" for anything that is not a
// literal this guard can judge — variables, wildcards, fn: transforms, list
// syntax, expressions. Only literals carry a checkable type at the head.
func literalKind(arg string) string {
	if arg == "" || arg == "_" {
		return ""
	}
	switch {
	case strings.HasPrefix(arg, "/"):
		return "/name"
	case strings.HasPrefix(arg, `"`):
		return "/string"
	}
	if _, err := strconv.ParseInt(arg, 10, 64); err == nil {
		return "/number"
	}
	if _, err := strconv.ParseFloat(arg, 64); err == nil {
		return "/float64"
	}
	return ""
}
