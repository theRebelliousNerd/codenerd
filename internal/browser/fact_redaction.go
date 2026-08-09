package browser

import (
	"codenerd/internal/browser/security"
	"codenerd/internal/mangle"
)

func (m *SessionManager) addFacts(facts []mangle.Fact) error {
	safeFacts := m.sanitizeFacts(facts)
	m.recordFlightFacts(safeFacts)
	if m.engine == nil {
		return nil
	}
	return m.engine.AddFacts(safeFacts)
}

func (m *SessionManager) sanitizeFacts(facts []mangle.Fact) []mangle.Fact {
	redactor := m.redactor
	if redactor == nil {
		redactor = security.NewRedactor(nil)
	}
	result := make([]mangle.Fact, len(facts))
	for i, fact := range facts {
		result[i] = fact
		result[i].Args = append([]any(nil), fact.Args...)
		for argIndex, arg := range result[i].Args {
			if value, ok := arg.(string); ok {
				result[i].Args[argIndex] = redactor.SanitizeString(value)
			}
		}

		switch fact.Predicate {
		case "net_header":
			if len(result[i].Args) >= 5 {
				name, nameOK := result[i].Args[3].(string)
				value, valueOK := fact.Args[4].(string)
				if nameOK && valueOK {
					result[i].Args[4] = redactor.RedactHeader(name, value)
				}
			}
		case "input_event":
			if len(result[i].Args) >= 3 {
				descriptor, descriptorOK := result[i].Args[1].(string)
				value, valueOK := fact.Args[2].(string)
				if descriptorOK && valueOK {
					result[i].Args[2] = redactor.RedactInputValue(descriptor, value)
				}
			}
		case "dom_attr", "attribute":
			if len(result[i].Args) >= 3 {
				name, nameOK := result[i].Args[1].(string)
				if nameOK && redactor.IsSensitiveKey(name) {
					result[i].Args[2] = security.Redacted
				}
			}
		case "react_prop":
			if len(result[i].Args) >= 3 {
				name, nameOK := result[i].Args[1].(string)
				if nameOK && redactor.IsSensitiveKey(name) {
					result[i].Args[2] = security.Redacted
				}
			}
		}
	}
	return result
}
