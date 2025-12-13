package world

import (
	"codenerd/internal/logging"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type mangleStatement struct {
	Text      string
	StartLine int
	EndLine   int
}

func splitMangleStatements(content string) []mangleStatement {
	var out []mangleStatement

	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0

	inString := false
	escape := false
	inComment := false

	line := 1

	stmtStartIdx := -1
	stmtStartLine := 1

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

		if lookingForStart {
			if !isMangleWhitespace(b) && b != '#' {
				stmtStartIdx = i
				stmtStartLine = line
				lookingForStart = false
			}
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
			if lookingForStart {
				continue
			}
			if parenDepth != 0 || bracketDepth != 0 || braceDepth != 0 {
				continue
			}

			// Avoid splitting on decimal points in numbers like 3.14
			prevIsDigit := i > 0 && isMangleDigit(content[i-1])
			nextIsDigit := i+1 < len(content) && isMangleDigit(content[i+1])
			if prevIsDigit && nextIsDigit {
				continue
			}

			end := i + 1
			stmt := strings.TrimSpace(content[stmtStartIdx:end])
			if stmt != "" {
				out = append(out, mangleStatement{
					Text:      stmt,
					StartLine: stmtStartLine,
					EndLine:   line,
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
	default:
		return false
	}
}

func isMangleDigit(b byte) bool { return b >= '0' && b <= '9' }

func splitMangleHead(statement string) (head string, isRule bool) {
	// Strip trailing '.' if present for parsing.
	stmt := strings.TrimSpace(statement)
	if strings.HasSuffix(stmt, ".") {
		stmt = strings.TrimSpace(strings.TrimSuffix(stmt, "."))
	}

	inString := false
	escape := false
	inComment := false
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0

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

func parseManglePredicateAndArity(head string) (pred string, arity int) {
	h := strings.TrimSpace(head)
	if h == "" {
		return "", 0
	}

	if strings.HasPrefix(h, "?") {
		h = strings.TrimSpace(strings.TrimPrefix(h, "?"))
	}
	if strings.HasPrefix(h, "Decl") {
		h = strings.TrimSpace(strings.TrimPrefix(h, "Decl"))
	}

	// Predicate name up to '(' or whitespace.
	nameEnd := 0
	for nameEnd < len(h) {
		ch := h[nameEnd]
		if ch == '(' || isMangleWhitespace(ch) {
			break
		}
		nameEnd++
	}
	pred = strings.TrimSpace(h[:nameEnd])
	if pred == "" {
		return "", 0
	}

	rest := strings.TrimSpace(h[nameEnd:])
	if !strings.HasPrefix(rest, "(") {
		return pred, 0
	}

	// Find matching ')'.
	inString := false
	escape := false
	depth := 0
	closeIdx := -1
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
		if b == '"' {
			inString = true
			continue
		}
		if b == '(' {
			depth++
			continue
		}
		if b == ')' {
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
	}
	if closeIdx == -1 {
		return pred, 0
	}

	args := strings.TrimSpace(rest[1:closeIdx])
	if args == "" {
		return pred, 0
	}

	// Count top-level commas in args, respecting nested structures/types.
	arity = 1
	angleDepth := 0
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inString = false
	escape = false
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
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
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
		case ',':
			if angleDepth == 0 && parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
				arity++
			}
		}
	}

	return pred, arity
}

func (p *CodeElementParser) parseMangleFile(path string) ([]CodeElement, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	p.fileCache[path] = lines

	statements := splitMangleStatements(string(content))
	logging.WorldDebug("CodeElementParser: mangle statements in %s: %d", filepath.Base(path), len(statements))

	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}
	ruleOrdinal := make(map[string]int)
	factOrdinal := make(map[string]int)
	queryOrdinal := make(map[string]int)
	declOrdinal := make(map[string]int)

	var elements []CodeElement
	for _, st := range statements {
		head, isRule := splitMangleHead(st.Text)
		pred, arity := parseManglePredicateAndArity(head)
		if pred == "" {
			continue
		}

		elemType := ElementMangleFact
		switch {
		case strings.HasPrefix(strings.TrimSpace(head), "Decl"):
			elemType = ElementMangleDecl
		case strings.HasPrefix(strings.TrimSpace(head), "?"):
			elemType = ElementMangleQuery
		case isRule:
			elemType = ElementMangleRule
		default:
			elemType = ElementMangleFact
		}

		key := fmt.Sprintf("%s/%d", pred, arity)
		ref := ""
		switch elemType {
		case ElementMangleDecl:
			declOrdinal[key]++
			ref = fmt.Sprintf("decl:%s", key)
			if declOrdinal[key] > 1 {
				ref = fmt.Sprintf("decl:%s#%d", key, declOrdinal[key])
			}
		case ElementMangleRule:
			ruleOrdinal[key]++
			ref = fmt.Sprintf("rule:%s#%d", key, ruleOrdinal[key])
		case ElementMangleQuery:
			queryOrdinal[key]++
			ref = fmt.Sprintf("query:%s#%d", key, queryOrdinal[key])
		default:
			factOrdinal[key]++
			ref = fmt.Sprintf("fact:%s#%d", key, factOrdinal[key])
		}

		signature := ""
		if firstLine, _, ok := strings.Cut(st.Text, "\n"); ok {
			signature = strings.TrimSpace(firstLine)
		} else {
			signature = strings.TrimSpace(st.Text)
		}

		actions := defaultActions
		if elemType == ElementMangleQuery {
			actions = []ActionType{ActionView}
		}

		elements = append(elements, CodeElement{
			Ref:        ref,
			Type:       elemType,
			File:       path,
			StartLine:  st.StartLine,
			EndLine:    st.EndLine,
			Signature:  signature,
			Body:       st.Text,
			Visibility: VisibilityPublic,
			Actions:    actions,
			Package:    "mangle",
			Name:       pred,
		})
	}

	return elements, nil
}

