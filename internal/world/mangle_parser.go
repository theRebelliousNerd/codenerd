package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// MangleCodeParser implements CodeParser for Mangle (.mg, .dl) source files.
// It parses Mangle declarations, rules, facts, and queries.
type MangleCodeParser struct {
	projectRoot string
}

// NewMangleCodeParser creates a new Mangle parser with the given project root.
func NewMangleCodeParser(projectRoot string) *MangleCodeParser {
	return &MangleCodeParser{
		projectRoot: projectRoot,
	}
}

// Language returns "mg" for Ref URI generation.
func (p *MangleCodeParser) Language() string {
	return "mg"
}

// SupportedExtensions returns [".mg", ".dl", ".mangle"].
func (p *MangleCodeParser) SupportedExtensions() []string {
	return []string{".mg", ".dl", ".mangle"}
}

// Parse extracts CodeElements from Mangle source code.
func (p *MangleCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("MangleCodeParser: parsing file: %s", filepath.Base(path))

	statements := splitMangleStatements(string(content))
	logging.WorldDebug("MangleCodeParser: mangle statements in %s: %d", filepath.Base(path), len(statements))

	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}

	// Track ordinals for multiple declarations of same predicate
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

	logging.WorldDebug("MangleCodeParser: parsed %s - %d elements in %v",
		filepath.Base(path), len(elements), time.Since(start))
	return elements, nil
}

// EmitLanguageFacts generates Mangle-specific Stratum 0 facts.
// For Mangle files, we emit facts about the structure of the Mangle code itself.
func (p *MangleCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact

	for _, elem := range elements {
		switch elem.Type {
		case ElementMangleDecl:
			// mg_decl(Ref, PredicateName)
			facts = append(facts, core.Fact{
				Predicate: "mg_decl",
				Args:      []interface{}{elem.Ref, elem.Name},
			})

		case ElementMangleRule:
			// mg_rule(Ref, HeadPredicate)
			facts = append(facts, core.Fact{
				Predicate: "mg_rule",
				Args:      []interface{}{elem.Ref, elem.Name},
			})

			// Detect recursive rules
			if isRecursiveRule(elem.Body, elem.Name) {
				facts = append(facts, core.Fact{
					Predicate: "mg_recursive_rule",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Detect negation in rule body
			if containsNegation(elem.Body) {
				facts = append(facts, core.Fact{
					Predicate: "mg_negation_rule",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Detect aggregation in rule
			if containsAggregation(elem.Body) {
				facts = append(facts, core.Fact{
					Predicate: "mg_aggregation_rule",
					Args:      []interface{}{elem.Ref},
				})
			}

		case ElementMangleFact:
			// mg_fact(Ref, PredicateName)
			facts = append(facts, core.Fact{
				Predicate: "mg_fact",
				Args:      []interface{}{elem.Ref, elem.Name},
			})

		case ElementMangleQuery:
			// mg_query(Ref, PredicateName)
			facts = append(facts, core.Fact{
				Predicate: "mg_query",
				Args:      []interface{}{elem.Ref, elem.Name},
			})
		}
	}

	return facts
}

// isRecursiveRule checks if a rule body contains the head predicate.
func isRecursiveRule(body, headPred string) bool {
	// Look for the predicate name followed by '(' in the body after ":-"
	idx := strings.Index(body, ":-")
	if idx == -1 {
		return false
	}
	ruleBody := body[idx+2:]
	// Check if head predicate appears in body
	return strings.Contains(ruleBody, headPred+"(")
}

// containsNegation checks if a rule body contains negation.
func containsNegation(body string) bool {
	// Look for "not " or "!" patterns in the body
	idx := strings.Index(body, ":-")
	if idx == -1 {
		return false
	}
	ruleBody := body[idx+2:]
	return strings.Contains(ruleBody, "not ") || strings.Contains(ruleBody, "!")
}

// containsAggregation checks if a rule body contains aggregation.
func containsAggregation(body string) bool {
	// Look for "|>" pipe operator or fn:collect, fn:sum, etc.
	return strings.Contains(body, "|>") ||
		strings.Contains(body, "fn:collect") ||
		strings.Contains(body, "fn:sum") ||
		strings.Contains(body, "fn:count") ||
		strings.Contains(body, "fn:max") ||
		strings.Contains(body, "fn:min") ||
		strings.Contains(body, "fn:group_by")
}
