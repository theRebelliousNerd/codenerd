package codedom

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/tools"
)

// FamilyNoChangeTool records that a member of a declared code family needs no
// change after a peer changed (Docs/audits/codedom-governed-code-replication.md).
// It writes nothing: the host asserts the decision as a turn fact, which
// settles that member's review in policy/code_families.mg.
func FamilyNoChangeTool() *tools.Tool {
	return &tools.Tool{
		Name: "family_no_change",
		Description: "Record that a member of a declared code family needs no change after you changed a peer. " +
			"Give the family id, the member's element ref (as the open review names it) and the concrete reason the shared requirement still holds for it. " +
			"Use it only after reading the member; editing the member settles the review without this.",
		Category: tools.CategoryCode,
		Priority: 40,
		Effect:   tools.EffectRead,
		Execute:  executeFamilyNoChange,
		Schema: tools.ToolSchema{
			Required: []string{"family", "member", "reason"},
			Properties: map[string]tools.Property{
				"family": {Type: "string", Description: "Code family id"},
				"member": {Type: "string", Description: "Element ref of the member that needs no change"},
				"reason": {Type: "string", Description: "Why the shared requirement still holds for this member without an edit"},
			},
		},
	}
}

func executeFamilyNoChange(ctx context.Context, args map[string]any) (string, error) {
	get := func(k string) string { v, _ := args[k].(string); return strings.TrimSpace(v) }
	d := tools.FamilyDecision{Family: get("family"), Member: get("member"), Reason: get("reason")}
	if d.Family == "" || d.Member == "" {
		return "", fmt.Errorf("family_no_change: family and member are required")
	}
	if len(d.Reason) < 20 {
		return "", fmt.Errorf("family_no_change: give the concrete reason the shared requirement still holds for %s (at least a sentence)", d.Member)
	}
	if !tools.RecordFamilyDecision(ctx, d) {
		return "", fmt.Errorf("family_no_change: no turn is open to record the decision")
	}
	return fmt.Sprintf("Recorded: %s in family %s needs no change. The review gate re-evaluates at the end of the turn.", d.Member, d.Family), nil
}
