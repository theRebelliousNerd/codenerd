package codemodel

import (
	"fmt"
	"strings"

	"codenerd/internal/mangle"
)

// MangleStatement is one statement of a Mangle source file: a Decl, a rule,
// a fact or a query, terminated by its top-level '.'.
type MangleStatement struct {
	Text               string
	StartLine, EndLine int
	// Start and End are byte offsets [Start, End) of Text in the source the
	// statement was split from.
	Start, End int
}

// SplitMangleStatements splits Mangle source into statements. It is a text
// splitter, not the Mangle parser: it has to keep working on a file the
// parser rejects, which is exactly the file somebody needs to repair.
func SplitMangleStatements(content string) []MangleStatement {
	var out []MangleStatement

	parenDepth, bracketDepth, braceDepth := 0, 0, 0
	inString, escape, inComment := false, false, false
	line := 1
	stmtStartIdx, stmtStartLine := -1, 1
	lookingForStart := true

	for i := 0; i < len(content); i++ {
		b := content[i]
		if b == '\n' {
			line++
			inComment = false
			escape = false
		}
		if inComment {
			continue
		}
		if lookingForStart && !isMangleWhitespace(b) && b != '#' {
			stmtStartIdx = i
			stmtStartLine = line
			lookingForStart = false
		}
		if inString {
			if escape {
				escape = false
				continue
			}
			if b == '\\' {
				escape = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '#':
			inComment = true
			continue
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '.':
			if lookingForStart || parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
				continue
			}
			// A decimal point in 3.14 does not end a statement.
			if i > 0 && isMangleDigit(content[i-1]) && i+1 < len(content) && isMangleDigit(content[i+1]) {
				continue
			}
			end := i + 1
			if stmt := content[stmtStartIdx:end]; strings.TrimSpace(stmt) != "" {
				out = append(out, MangleStatement{
					Text: stmt, StartLine: stmtStartLine, EndLine: line,
					Start: stmtStartIdx, End: end,
				})
			}
			lookingForStart = true
			stmtStartIdx = -1
		}
	}
	return out
}

func isMangleWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	}
	return false
}

func isMangleDigit(b byte) bool { return b >= '0' && b <= '9' }

// MangleHead returns a statement's head (the text before a top-level ":-")
// and whether the statement is a rule.
func MangleHead(statement string) (head string, isRule bool) {
	stmt := strings.TrimSpace(statement)
	if before, ok := strings.CutSuffix(stmt, "."); ok {
		stmt = strings.TrimSpace(before)
	}
	inString, escape, inComment := false, false, false
	parenDepth, bracketDepth, braceDepth := 0, 0, 0
	for i := 0; i < len(stmt)-1; i++ {
		b := stmt[i]
		if b == '\n' {
			inComment = false
			escape = false
		}
		if inComment {
			continue
		}
		if inString {
			if escape {
				escape = false
				continue
			}
			if b == '\\' {
				escape = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '#':
			inComment = true
			continue
		case '"':
			inString = true
			continue
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		}
		if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && stmt[i] == ':' && stmt[i+1] == '-' {
			return strings.TrimSpace(stmt[:i]), true
		}
	}
	return strings.TrimSpace(stmt), false
}

// ManglePredicate returns the predicate name and arity a statement head
// names; "" when the head names none.
func ManglePredicate(head string) (pred string, arity int) {
	h := strings.TrimSpace(head)
	if h == "" {
		return "", 0
	}
	if after, ok := strings.CutPrefix(h, "?"); ok {
		h = strings.TrimSpace(after)
	}
	if after, ok := strings.CutPrefix(h, "Decl"); ok {
		h = strings.TrimSpace(after)
	}
	nameEnd := 0
	for nameEnd < len(h) && h[nameEnd] != '(' && !isMangleWhitespace(h[nameEnd]) {
		nameEnd++
	}
	pred = strings.TrimSpace(h[:nameEnd])
	rest := strings.TrimSpace(h[nameEnd:])
	if pred == "" {
		return "", 0
	}
	if !strings.HasPrefix(rest, "(") {
		return pred, 0
	}
	return pred, mangleArity(mangleArgs(rest))
}

func mangleArgs(rest string) string {
	inString, escape, depth := false, false, 0
	for i := 0; i < len(rest); i++ {
		b := rest[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if b == '\\' {
				escape = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return strings.TrimSpace(rest[1:i])
			}
		}
	}
	return ""
}

func mangleArity(args string) int {
	if args == "" {
		return 0
	}
	arity := 1
	angle, paren, bracket, brace := 0, 0, 0, 0
	inString, escape := false, false
	for i := 0; i < len(args); i++ {
		b := args[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if b == '\\' {
				escape = true
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			bracket++
		case ']':
			if bracket > 0 {
				bracket--
			}
		case '{':
			brace++
		case '}':
			if brace > 0 {
				brace--
			}
		case ',':
			if angle == 0 && paren == 0 && bracket == 0 && brace == 0 {
				arity++
			}
		}
	}
	return arity
}

// MangleBodyPredicates returns the predicates a rule's body reads, in order,
// each once. Built-in function calls (fn:..., :string:...) are not
// predicates and are left out.
func MangleBodyPredicates(statement string) []string {
	_, body, ok := strings.Cut(statement, ":-")
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	inString, inComment := false, false
	for i := 0; i < len(body); i++ {
		b := body[i]
		if b == '\n' {
			inComment = false
		}
		if inComment {
			continue
		}
		if inString {
			if b == '\\' {
				i++
				continue
			}
			if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			continue
		}
		if b == '#' {
			inComment = true
			continue
		}
		if b != '(' {
			continue
		}
		j := i
		for j > 0 && isMangleIdentByte(body[j-1]) {
			j--
		}
		name := body[j:i]
		// fn:count( and :string:contains( are built-ins, not predicates.
		if name == "" || (j > 0 && body[j-1] == ':') {
			continue
		}
		if c := name[0]; (c < 'a' || c > 'z') && c != '_' {
			continue
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func isMangleIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// ParseMangle builds the element model of a Mangle source file. Statements are
// keyed by kind, predicate and a hash of their own text, never by ordinal: an
// ordinal renumbers every statement below an insertion, so a key the model was
// shown would silently name a different rule after the next edit.
func ParseMangle(path, src string) *File {
	src = Normalize(src)
	f := &File{Path: path, Language: LangMangle, Source: src}
	if _, err := mangle.ParseUnit(strings.NewReader(src)); err != nil {
		f.Err = err
		f.Errors = append(f.Errors, SyntaxError{Line: 1, Column: 1, Msg: err.Error()})
	} else {
		f.Parsed = true
	}

	var elements []Element
	for _, st := range SplitMangleStatements(src) {
		head, isRule := MangleHead(st.Text)
		pred, arity := ManglePredicate(head)
		if pred == "" {
			continue
		}
		kind := KindFact
		trimmedHead := strings.TrimSpace(head)
		switch {
		case strings.HasPrefix(trimmedHead, "Decl"):
			kind = KindDecl
		case strings.HasPrefix(trimmedHead, "?"):
			kind = KindQuery
		case isRule:
			kind = KindRule
		}
		e := Element{
			Name: pred, Names: []string{pred}, Kind: kind,
			Start: mangleDocStart(src, st.Start), End: st.End,
			DeclLine: st.StartLine, Exported: true,
		}
		e.UnitStart, e.UnitEnd = e.Start, e.End
		e.Signature, _, _ = strings.Cut(strings.TrimSpace(st.Text), "\n")
		e.Doc = mangleDocLine(src[e.Start:st.Start])
		e.StartLine = f.LineOf(e.Start)
		e.EndLine = f.LineOf(max(e.End-1, e.Start))
		e.Revision = Revision(src[e.Start:e.End])
		base := fmt.Sprintf("%s:%s/%d", kind, pred, arity)
		if kind != KindDecl {
			base += "@" + Revision(st.Text)[:6]
		}
		e.Key = base
		elements = append(elements, e)
	}
	assignKeys(elements, func(e *Element) string { return e.Key })
	f.Elements = elements
	return f
}

// mangleDocStart walks back from a statement over the comment lines directly
// above it (no blank line between) and returns where they begin.
func mangleDocStart(src string, stmtStart int) int {
	lineStart := strings.LastIndexByte(src[:stmtStart], '\n') + 1
	start := stmtStart
	if strings.TrimSpace(src[lineStart:stmtStart]) != "" {
		return start
	}
	start = lineStart
	for start > 0 {
		prevEnd := start - 1
		prevStart := strings.LastIndexByte(src[:prevEnd], '\n') + 1
		line := strings.TrimSpace(src[prevStart:prevEnd])
		if !strings.HasPrefix(line, "#") {
			break
		}
		start = prevStart
	}
	return start
}

func mangleDocLine(doc string) string {
	for line := range strings.SplitSeq(doc, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return ""
}
