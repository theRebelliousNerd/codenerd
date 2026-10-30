package campaign

import "codenerd/internal/northstar"

func campaignAlignmentScope(workspace string, c *Campaign) northstar.AlignmentScope {
	scope := northstar.AlignmentScope{Workspace: workspace, Disposition: northstar.ScopeUnresolved}
	if c == nil {
		return scope
	}
	for _, p := range c.SourceMaterial {
		scope.Paths = append(scope.Paths, northstar.AlignmentPath{Path: p, Role: northstar.PathContextSource})
	}
	for i := range c.Phases {
		appendPhaseAlignmentPaths(&scope, &c.Phases[i])
	}
	if len(scope.Paths) > 0 {
		scope.Disposition = northstar.ScopePaths
	}
	return scope
}

func phaseAlignmentScope(workspace string, phase *Phase) northstar.AlignmentScope {
	scope := northstar.AlignmentScope{Workspace: workspace, Disposition: northstar.ScopeUnresolved}
	appendPhaseAlignmentPaths(&scope, phase)
	if len(scope.Paths) > 0 {
		scope.Disposition = northstar.ScopePaths
	}
	return scope
}

func appendPhaseAlignmentPaths(scope *northstar.AlignmentScope, phase *Phase) {
	if phase == nil {
		return
	}
	for i := range phase.Tasks {
		appendTaskAlignmentPaths(scope, &phase.Tasks[i])
	}
}

func appendTaskAlignmentPaths(scope *northstar.AlignmentScope, task *Task) {
	if task == nil {
		return
	}
	// Artifacts are planned or contextual declarations, never observed writes.
	// Do not use DeterministicWriteSet's artifact fallback to label them writes.
	for _, p := range task.WriteSet {
		scope.Paths = append(scope.Paths, northstar.AlignmentPath{Path: p, Role: northstar.PathPlannedWrite})
	}
	for _, a := range task.Artifacts {
		if a.Path != "" {
			scope.Paths = append(scope.Paths, northstar.AlignmentPath{Path: a.Path, Role: northstar.PathArtifact})
		}
	}
}

func taskAlignmentScope(workspace string, task *Task) northstar.AlignmentScope {
	scope := northstar.AlignmentScope{Workspace: workspace, Disposition: northstar.ScopeUnresolved}
	appendTaskAlignmentPaths(&scope, task)
	if task != nil {
		for _, p := range task.observedAlignmentPaths {
			scope.Paths = append(scope.Paths, northstar.AlignmentPath{Path: p, Role: northstar.PathObservedEffect})
		}
	}
	if len(scope.Paths) > 0 {
		scope.Disposition = northstar.ScopePaths
	}
	return scope
}
