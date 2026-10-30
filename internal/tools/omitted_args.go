package tools

import "strings"

// IsOmittedSentinel reports whether s is a model's spelling of "no value":
// empty, JSON null, Python None, Go nil (also "<nil>" from fmt), JS
// undefined, or "n/a". A model that serialises a missing optional argument as
// one of those sends it as a string, and a tool that takes it at its word
// looks for a directory named "null" (run_build, 2026-10-29) or a revision
// named "None" (edit_element, 2026-10-29). This is the one list: the registry
// drops a typed optional argument that carries one (withoutOmittedOptionals),
// and the string arguments whose "not given" is a sentinel -- a path, a
// revision -- ask it too.
func IsOmittedSentinel(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "null", "none", "nil", "<nil>", "undefined", "n/a":
		return true
	}
	return false
}

// withoutOmittedOptionals is args less every optional argument (one the
// tool's schema does not require) that the model sent as "no value": nil, or,
// for an argument declared as an array, object, number or boolean, an omitted
// sentinel string -- no such argument has a string value, so "null" there is
// only the model's way of leaving it out. A string argument keeps a sentinel
// it was given: "None" is a Python value an edit may well mean, so whether it
// means "not given" is that argument's to say (IsOmittedOptionalPath, a
// revision check). A required argument is left as given, so its absence is
// still reported by validation. args is not modified; the result is a copy
// when anything is dropped.
func withoutOmittedOptionals(tool *Tool, args map[string]any) map[string]any {
	var drop []string
	for key, value := range args {
		if requiredArg(tool, key) {
			continue
		}
		switch v := value.(type) {
		case nil:
			drop = append(drop, key)
		case string:
			prop, declared := tool.Schema.Properties[key]
			if declared && prop.Type != "" && prop.Type != "string" && IsOmittedSentinel(v) {
				drop = append(drop, key)
			}
		}
	}
	if len(drop) == 0 {
		return args
	}
	out := make(map[string]any, len(args)-len(drop))
	for key, value := range args {
		out[key] = value
	}
	for _, key := range drop {
		delete(out, key)
	}
	return out
}

func requiredArg(tool *Tool, key string) bool {
	for _, required := range tool.Schema.Required {
		if required == key {
			return true
		}
	}
	return false
}

// RevisionPrecondition reads a revision argument the way every element tool
// and every guard projecting one must: an omission sentinel ("None", "null",
// empty) carries no precondition, and a revision is compared trimmed. It
// returns the revision to compare and whether one applies. The build-exclusion
// projector skipped its check for a "None" it took for a stale revision while
// the handler applied the edit without a precondition.
func RevisionPrecondition(rev string) (string, bool) {
	if IsOmittedSentinel(rev) {
		return "", false
	}
	return strings.TrimSpace(rev), true
}
