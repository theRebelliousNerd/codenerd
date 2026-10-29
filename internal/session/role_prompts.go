package session

import (
	"context"
	"fmt"

	"codenerd/internal/types"
)

// The model calls a turn makes that are not the persona's own -- the build
// critic, the step planner, the final-report audit -- get their system prompt from prompt atoms through a
// types.RolePromptCompiler, never from Go text. A missing compiler or a failed
// compile is an error the caller reports; there is no embedded prompt to send
// instead.

// The roles a turn's own model calls compile under. Each role's atoms
// (atoms/system/<role>.yaml) declare shard_types ["/<role>"].
const (
	// buildCriticRole reviews the change a turn made (build_verify.go).
	buildCriticRole = "build_critic"
	// workStepPlannerRole divides a multi-site task into steps (work_steps.go).
	workStepPlannerRole = "work_step_planner"
	// finalReportAuditorRole reads a turn's report for an admission that the
	// work is unfinished (admission_audit.go).
	finalReportAuditorRole = "final_report_auditor"
)

// roleJudgeIntent keeps the worker intent atoms out of a role compile whose
// reply Go parses.
const roleJudgeIntent = "judge"

// SetRolePrompts installs the compiler of this executor's role prompts. Nil
// leaves every role call reporting types.ErrRolePromptUnavailable.
func (e *Executor) SetRolePrompts(c types.RolePromptCompiler) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rolePrompts = c
}

func (e *Executor) currentRolePrompts() types.RolePromptCompiler {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rolePrompts
}

// rolePrompt compiles role's system prompt: its instructions and the reply
// contract Go parses.
func (e *Executor) rolePrompt(ctx context.Context, role string) (string, error) {
	rp := e.currentRolePrompts()
	if rp == nil {
		return "", fmt.Errorf("%w: no role prompt compiler for the %s", types.ErrRolePromptUnavailable, role)
	}
	return rp.CompileRolePrompt(ctx, types.RolePromptRequest{Role: role, Intent: roleJudgeIntent})
}

// SetRolePrompts installs the role prompt compiler handed to every subagent
// spawned from here on. Already-spawned subagents keep the one they were
// built with.
func (s *Spawner) SetRolePrompts(c types.RolePromptCompiler) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rolePrompts = c
}

func (s *Spawner) currentRolePrompts() types.RolePromptCompiler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rolePrompts
}
