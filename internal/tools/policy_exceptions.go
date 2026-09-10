package tools

// PolicyDeniedByDesign lists registered tools that deliberately have no
// safe_action or requires_permission fact in the constitution, each with the
// reason it is denied.
//
// A tool named here is hard-denied by the constitution's default-deny gate.
// That is the intent, not an oversight, and two separate checks read this map
// so they cannot reach opposite conclusions about the same tool:
//
//   - internal/tools/catalog_policy_parity_test.go fails if a policy fact
//     later appears for one of these, which would silently grant it.
//   - cmd/tools/action_linter skips them, instead of reporting a deliberate
//     policy decision as drift.
//
// That second consumer is why this is a package-level var rather than a
// test fixture. The linter used to report /research_cache_clear as an error --
// "registered but the constitution can never permit it" -- which was correct
// as a description and wrong as a verdict, and it was the only error the
// linter produced. A gate that fails on day one for a non-bug does not get
// switched on, and then the real drift it was written to catch goes unseen
// too. The exclusion was already recorded in three places (a comment in
// constitution.mg, the parity test's map, and the linter's unused exemptions
// file mechanism); three copies of one fact is how they end up disagreeing.
//
// Adding an entry here is a policy decision. It does not grant anything -- the
// constitution still denies the tool -- it records that the denial is meant.
var PolicyDeniedByDesign = map[string]string{
	"research_cache_clear": "discards the research cache every agent in the process shares; denied by policy on purpose",
}

// IsPolicyDeniedByDesign reports whether name is a tool the constitution
// denies deliberately. A leading "/" is accepted, so callers holding a Mangle
// action name do not have to trim it first.
func IsPolicyDeniedByDesign(name string) bool {
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	_, ok := PolicyDeniedByDesign[name]
	return ok
}
