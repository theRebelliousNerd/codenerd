package turnjudge

import (
	"strings"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// AuthorizationFrom is what the user's message allowed beyond the edits, as
// the kernel derives it from the turn's judgments (jev_authorization,
// policy/jev_evidence.mg). It is zero when the kernel derives nothing, when
// no judgments were recorded (Jev off, or the call failed: a missing judgment
// is no evidence, never a yes) and when the query fails. Only the user's own
// message reaches the judgment it rests on (AsksToCommit); this reads the
// kernel's conclusion and never decides.
func AuthorizationFrom(k Querier) perception.Authorization {
	if k == nil {
		return perception.Authorization{}
	}
	rows, err := k.Query("jev_authorization")
	if err != nil {
		return perception.Authorization{}
	}
	var kinds []perception.AuthorizationKind
	for _, r := range rows {
		if len(r.Args) == 1 {
			kinds = append(kinds, perception.AuthorizationKind(strings.TrimPrefix(types.ExtractString(r.Args[0]), "/")))
		}
	}
	return perception.AuthorizationOf(kinds...)
}
