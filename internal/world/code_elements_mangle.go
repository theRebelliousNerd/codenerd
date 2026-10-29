package world

import (
	"codenerd/internal/logging"
	"codenerd/internal/world/codemodel"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (p *CodeElementParser) parseMangleFile(path string) ([]CodeElement, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	p.fileCache[path] = lines

	statements := codemodel.SplitMangleStatements(string(content))
	logging.WorldDebug("CodeElementParser: mangle statements in %s: %d", filepath.Base(path), len(statements))

	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}
	ruleOrdinal := make(map[string]int)
	factOrdinal := make(map[string]int)
	queryOrdinal := make(map[string]int)
	declOrdinal := make(map[string]int)

	var elements []CodeElement
	for _, st := range statements {
		head, isRule := codemodel.MangleHead(st.Text)
		pred, arity := codemodel.ManglePredicate(head)
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
