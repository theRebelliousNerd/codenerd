package campaign

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// checkMissionWrite measures canonical containment. Policy admits the measured
// path, alongside ordinary constitutional permission and the campaign lease.
func (o *Orchestrator) checkMissionWrite(ctx context.Context, paths []string) error {
	o.mu.RLock()
	var receipt *MissionReceipt
	if o.campaign != nil {
		receipt = o.campaign.Mission
	}
	o.mu.RUnlock()
	if receipt == nil {
		return nil
	}
	for _, path := range paths {
		abs, err := tools.ResolveWorkspacePath(ctx, o.workspace, path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(o.workspace, abs)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		inside, protected := types.MangleAtom("/outside"), types.MangleAtom("/no")
		for _, scope := range receipt.Scope {
			if missionPathWithin(rel, scope) {
				inside = "/inside"
			}
		}
		for _, scope := range receipt.ProtectedPaths {
			if missionPathWithin(rel, scope) {
				protected = "/yes"
			}
		}
		if receipt.VerificationInputs == nil {
			protected = "/unknown"
		}
		if _, pinned := receipt.VerificationInputs[abs]; pinned {
			protected = "/yes"
		}
		f := types.Fact{Predicate: "mission_write_candidate", Args: []any{receipt.MissionID, rel, inside, protected}}
		if err := o.kernel.Assert(f); err != nil {
			return err
		}
		rows, queryErr := o.kernel.Query("mission_write_allowed")
		retractErr := o.kernel.RetractExactFactsBatch([]types.Fact{f})
		if queryErr != nil {
			return queryErr
		}
		if retractErr != nil {
			return retractErr
		}
		allowed := false
		for _, row := range rows {
			if len(row.Args) == 2 && types.ExtractString(row.Args[0]) == receipt.MissionID && types.ExtractString(row.Args[1]) == rel {
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("mission %s has no write admission for %s", receipt.MissionID, rel)
		}
	}
	return nil
}

func missionPathWithin(path, scope string) bool {
	path, scope = filepath.ToSlash(filepath.Clean(path)), filepath.ToSlash(filepath.Clean(scope))
	if filepath.Separator == '\\' {
		path, scope = strings.ToLower(path), strings.ToLower(scope)
	}
	return scope == "." || path == scope || strings.HasPrefix(path, scope+"/")
}
