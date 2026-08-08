package core

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/transparency"
	"codenerd/internal/types"
)

// RouteAction intercepts 'next_action' atoms and routes them to appropriate handlers.
func (v *VirtualStore) RouteAction(ctx context.Context, action Fact) (string, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, fmt.Sprintf("RouteAction(%s)", action.Predicate))
	defer timer.Stop()

	// Boot guard: block all action routing until first user interaction.
	// This prevents session rehydration from replaying old next_action facts.
	v.mu.RLock()
	bootGuardActive := v.bootGuardActive
	v.mu.RUnlock()
	if bootGuardActive {
		logging.VirtualStore("RouteAction BLOCKED by boot guard: %s (waiting for user interaction)", action.Predicate)
		return "", fmt.Errorf("boot guard active: action routing blocked until first user interaction")
	}

	logging.VirtualStore("Routing action: predicate=%s, args=%d", action.Predicate, len(action.Args))

	// Parse the action fact
	req, err := v.parseActionFact(action)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to parse action fact: %v", err)
		return "", fmt.Errorf("failed to parse action fact: %w", err)
	}

	logging.VirtualStoreDebug("Parsed action: type=%s, target=%s", req.Type, req.Target)

	// Dreamer safety gate: speculatively simulate destructive actions before execution.
	// The dreamer clones the kernel, projects effects, and queries panic_state rules.
	// If unsafe, the action is blocked. Fail-closed: if dreamer itself fails, action is blocked.
	if isDestructiveAction(req.Type) {
		dreamer := v.getDreamer()
		if dreamer == nil {
			reason := "dreamer unavailable for destructive action"
			logging.Get(logging.CategoryVirtualStore).Error(
				"Dreamer unavailable; BLOCKED action: %s on %s", req.Type, req.Target)
			v.injectFact(newSecurityViolationFact(req, reason))
			return "", fmt.Errorf("action %s blocked: %s", req.Type, reason)
		}

		dreamResult := dreamer.SimulateAction(ctx, req)
		if dreamResult.Unsafe {
			logging.Get(logging.CategoryVirtualStore).Warn(
				"Dreamer BLOCKED action: %s on %s (reason: %s)",
				req.Type, req.Target, dreamResult.Reason)
			v.injectFact(newSecurityViolationFact(req, "dreamer: "+dreamResult.Reason))
			v.injectFact(Fact{
				Predicate: "dream_blocked_action",
				Args:      []any{dreamResult.ActionID, string(req.Type), req.Target, dreamResult.Reason},
			})
			return "", fmt.Errorf("action %s blocked by dreamer safety gate: %s", req.Type, dreamResult.Reason)
		}
		logging.VirtualStoreDebug("Dreamer approved action: %s on %s", req.Type, req.Target)
	}

	// Constitutional logic check (defense in depth)
	if err := v.checkConstitution(req); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Constitutional violation: %s on %s - %v", req.Type, req.Target, err)
		v.injectFact(newSecurityViolationFact(req, err.Error()))
		return "", err
	}

	// Kernel-level permission gate (default deny if kernel says not permitted)
	permitted := v.CheckKernelPermitted(string(req.Type), req.Target, req.Payload)
	if !permitted {
		// Constitutional denial is the hardest class of failure to
		// debug because the user only sees "action blocked." Surface
		// the target and payload keys (not values — those can be
		// sensitive) so triage can locate the offending request
		// without reading source. Withholding values keeps the log
		// safe to ship even when payloads contain secrets.
		payloadKeys := make([]string, 0, len(req.Payload))
		for k := range req.Payload {
			payloadKeys = append(payloadKeys, k)
		}
		logging.Get(logging.CategoryVirtualStore).Warn(
			"policy DENY action=%s target=%s payload_keys=%v",
			req.Type, req.Target, payloadKeys)
		err := fmt.Errorf("action %s not permitted by kernel policy", req.Type)
		v.injectFact(newSecurityViolationFact(req, err.Error()))
		return "", err
	}

	// Route to appropriate handler
	logging.VirtualStoreDebug("Dispatching action %s to handler", req.Type)
	logging.Audit().ActionRoute(string(req.Type), req.Target)
	actionStart := time.Now()
	result, err := v.executeAction(ctx, req)
	actionDuration := time.Since(actionStart)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Action execution failed: %s - %v", req.Type, err)
		v.injectFact(Fact{
			Predicate: "execution_error",
			Args:      []any{req.ActionID, err.Error()},
		})
		return "", err
	}

	// Post-action validation: verify the action actually succeeded
	var validationErr error
	if result.Success && v.validators != nil {
		validationReq := v.requestForValidation(req)
		validationResults := v.validators.Validate(ctx, validationReq, result)
		v.processValidationResults(validationReq, result, validationResults)

		// If validation failed with high confidence, update the result
		if !ValidateAll(validationResults) {
			if failure := FirstFailure(validationResults); failure != nil && failure.Confidence >= 0.8 {
				logging.Get(logging.CategoryVirtualStore).Warn(
					"Post-action validation failed: %s - %s (confidence=%.2f)",
					req.Type, failure.Error, failure.Confidence)
				// Mark result as failed due to validation
				result.Success = false
				result.Error = "validation failed: " + failure.Error
				validationErr = fmt.Errorf("post-action validation failed: %s", failure.Error)
			}
		}
	}

	// Inject result facts into kernel (batched when possible).
	completedAt := time.Now()
	factsToInject := make([]Fact, 0, len(result.FactsToAdd)+1)
	factsToInject = append(factsToInject, result.FactsToAdd...)
	factsToInject = append(factsToInject, Fact{
		Predicate: "execution_result",
		Args:      []any{req.ActionID, string(req.Type), req.Target, result.Success, result.Output, completedAt.Unix()},
	})
	v.injectFacts(factsToInject)
	v.maybePruneActionLogs(completedAt)

	if result.Success {
		logging.VirtualStore("Action %s completed: success=%v, output_len=%d", req.Type, result.Success, len(result.Output))
	} else {
		logging.VirtualStore("Action %s completed: success=%v, error=%s", req.Type, result.Success, result.Error)
	}

	// Audit: Action completed
	logging.Audit().ActionComplete(string(req.Type), req.Target, actionDuration.Milliseconds(), result.Success, result.Error)

	// TUI visibility: tool execution always shows in chat scrollback,
	// Glass Box routing event always updates the activity line.
	v.emitToolAndRoutingEvents(req, result, actionDuration)

	return result.Output, validationErr
}

func (v *VirtualStore) requestForValidation(req ActionRequest) ActionRequest {
	switch req.Type {
	case ActionReadFile, ActionWriteFile, ActionEditFile, ActionDeleteFile,
		ActionFSRead, ActionFSWrite, ActionEditLines, ActionInsertLines, ActionDeleteLines:
		req.Target = v.resolvePath(req.Target)
	}
	return req
}

func newSecurityViolationFact(req ActionRequest, reason string) Fact {
	actionType := string(req.Type)
	if !strings.HasPrefix(actionType, "/") {
		actionType = "/" + actionType
	}
	if req.Target != "" {
		reason = fmt.Sprintf("target=%q: %s", req.Target, reason)
	}
	return Fact{
		Predicate: "security_violation",
		Args:      []any{MangleAtom(actionType), reason, time.Now().Unix()},
	}
}

// emitToolAndRoutingEvents pushes a ToolEvent and a Glass Box
// CategoryRouting event for an executed action. Cheap when both
// buses are nil. Intentionally non-blocking — both buses already
// drop on full channels.
func (v *VirtualStore) emitToolAndRoutingEvents(req ActionRequest, result ActionResult, dur time.Duration) {
	v.mu.RLock()
	tbus := v.toolEventBus
	gbus := v.glassBoxBus
	v.mu.RUnlock()

	// Pretty per-action label. Strips the leading "/" verbs use so
	// the badge reads "exec_cmd" rather than "/exec_cmd".
	verb := strings.TrimPrefix(string(req.Type), "/")
	label := verb
	if strings.TrimSpace(req.Target) != "" {
		label = verb + " " + req.Target
		if len(label) > 80 {
			label = label[:77] + "..."
		}
	}

	if tbus != nil {
		summary := result.Output
		if !result.Success {
			summary = result.Error
		}
		if len(summary) > 160 {
			summary = summary[:157] + "..."
		}
		tbus.Emit(transparency.ToolEvent{
			ToolName:  verb,
			Result:    summary,
			Success:   result.Success,
			Duration:  dur,
			Timestamp: time.Now(),
		})
	}

	if gbus != nil {
		summary := label
		if !result.Success {
			summary = label + " ❌"
		}
		// Immediate so tool/routing results stream live into chat debug mode.
		gbus.EmitImmediate(transparency.GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  transparency.CategoryRouting,
			Summary:   summary,
			Source:    string(req.Type),
			Duration:  dur,
		})
	}
}

// parseActionFact converts a Fact to an ActionRequest.
func (v *VirtualStore) parseActionFact(action Fact) (ActionRequest, error) {
	req := ActionRequest{
		Payload: make(map[string]any),
	}

	// Bug #4 fix: Add detailed logging for malformed action facts
	if len(action.Args) < 3 {
		logging.Get(logging.CategoryVirtualStore).Error(
			"Malformed action fact: predicate=%s, got %d args (need 3+), args=%v",
			action.Predicate, len(action.Args), action.Args)
		return req, fmt.Errorf("invalid action fact: requires at least 3 arguments (ActionID, Type, Target), got %d", len(action.Args))
	}

	// First arg is ActionID
	req.ActionID = types.ExtractString(action.Args[0])
	if req.ActionID == "" {
		return req, fmt.Errorf("invalid action fact: ActionID cannot be empty")
	}

	// Second arg is action type
	actionType, ok := action.Args[1].(string)
	if !ok {
		actionType = types.ExtractString(action.Args[1])
	}
	// Strip leading slash if present (Mangle name constants)
	actionType = strings.TrimPrefix(actionType, "/")
	req.Type = ActionType(actionType)
	if req.Type == "" {
		return req, fmt.Errorf("invalid action fact: Type cannot be empty")
	}

	// Third arg is target
	target, ok := action.Args[2].(string)
	if !ok {
		target = types.ExtractString(action.Args[2])
	}
	req.Target = target

	// Strict check: target must not be empty for file operations
	if req.Target == "" {
		switch req.Type {
		case ActionReadFile, ActionWriteFile, ActionEditFile, ActionDeleteFile, ActionFSRead, ActionFSWrite:
			return req, fmt.Errorf("invalid action fact: target path cannot be empty for file operations")
		}
	}

	// Remaining args go into payload
	for i := 3; i < len(action.Args); i++ {
		// If the argument is a map, merge it into the payload
		if argMap, ok := action.Args[i].(map[string]any); ok {
			maps.Copy(req.Payload, argMap)
			continue
		}

		key := fmt.Sprintf("arg%d", i-3)
		req.Payload[key] = action.Args[i]
	}

	return req, nil
}

// executeAction dispatches to the appropriate handler.
func (v *VirtualStore) executeAction(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// nerd.md write protection, at the one point every action passes through.
	// Gating the six write handlers individually would be six chances to forget
	// the seventh.
	//
	// This was a real hole, not a hypothetical: the only enforcement lived on
	// session.Executor, and shards do not route writes through it — they route
	// them here. A shard could write .nerd/config.json that the interactive path
	// refused.
	if reason, blocked := v.projectForbidsWrite(req); blocked {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("blocked by nerd.md: %s is write-protected (%s)", req.Target, reason),
		}, nil
	}

	// Mark the edit in flight for the policy layer, and clear it on every exit
	// path. pending_edit is the root fact for 26 rules across 7 policy files
	// (coder_safety, coder_quality, coder_impact, coder_workflow, coder_tdd,
	// coder_observability, projectdoc); none of them derived anything because
	// no Go code asserted it.
	//
	// Asserted here as well as in session.Executor because shards route writes
	// through the VirtualStore and never touch the executor — the same split
	// that made the nerd.md gate above necessary in both places.
	if fact, asserted := v.assertPendingEdit(req); asserted {
		defer v.retractPendingEdit(fact)
	}

	switch req.Type {
	case ActionExecCmd:
		return v.handleExecCmd(ctx, req)
	case ActionReadFile:
		return v.handleReadFile(ctx, req)
	case ActionWriteFile:
		return v.handleWriteFile(ctx, req)
	case ActionEditFile:
		return v.handleEditFile(ctx, req)
	case ActionDeleteFile:
		return v.handleDeleteFile(ctx, req)

	// Modular core filesystem tools
	case ActionListFiles, ActionGlob, ActionGrep:
		return v.handleModularTool(ctx, req)

	// Modular shell execution tools
	case ActionRunCommand, ActionBash, ActionRunBuild:
		return v.handleModularTool(ctx, req)

	case ActionSearchCode, ActionSearchFiles, ActionAnalyzeCode:
		return v.handleSearchCode(ctx, req)
	case ActionRunTests:
		return v.handleRunTests(ctx, req)
	case ActionBuildProject:
		return v.handleBuildProject(ctx, req)
	case ActionGitOperation:
		return v.handleGitOperation(ctx, req)
	case ActionShowDiff:
		return v.handleShowDiff(ctx, req)
	case ActionAnalyzeImpact:
		return v.handleAnalyzeImpact(ctx, req)
	case ActionBrowse:
		return v.handleBrowse(ctx, req)
	case ActionResearch:
		return v.handleResearch(ctx, req)
	case ActionDelegate:
		return v.handleDelegate(ctx, req)
	case ActionDelegateReviewer:
		return v.handleDelegateAlias(ctx, req, "/reviewer")
	case ActionDelegateCoder:
		return v.handleDelegateAlias(ctx, req, "/coder")
	case ActionDelegateTester:
		return v.handleDelegateAlias(ctx, req, "/tester")
	case ActionDelegateResearcher:
		return v.handleDelegateAlias(ctx, req, "/researcher")
	case ActionDelegateToolGenerator:
		return v.handleDelegateAlias(ctx, req, "/tool_generator")
	case ActionAskUser:
		return v.handleAskUser(ctx, req)
	case ActionEscalate:
		return v.handleEscalate(ctx, req)

	// Code DOM actions
	case ActionOpenFile:
		return v.handleOpenFile(ctx, req)
	case ActionGetElements:
		return v.handleGetElements(ctx, req)
	case ActionGetElement:
		return v.handleGetElement(ctx, req)
	case ActionEditElement:
		return v.handleEditElement(ctx, req)
	case ActionRefreshScope:
		return v.handleRefreshScope(ctx, req)
	case ActionCloseScope:
		return v.handleCloseScope(ctx, req)
	case ActionEditLines:
		return v.handleEditLines(ctx, req)
	case ActionInsertLines:
		return v.handleInsertLines(ctx, req)
	case ActionDeleteLines:
		return v.handleDeleteLines(ctx, req)

	// Autopoiesis actions
	case ActionExecTool:
		return v.handleExecTool(ctx, req)

	// TDD Loop actions
	case ActionReadErrorLog:
		return v.handleReadErrorLog(ctx, req)
	case ActionAnalyzeRootCause:
		return v.handleAnalyzeRootCause(ctx, req)
	case ActionGeneratePatch:
		return v.handleGeneratePatch(ctx, req)
	case ActionEscalateToUser:
		return v.handleEscalateToUser(ctx, req)
	case ActionComplete:
		return v.handleComplete(ctx, req)
	case ActionInterrogative:
		return v.handleInterrogative(ctx, req)
	case ActionResumeTask:
		return v.handleResumeTask(ctx, req)
	case ActionRefreshShardCtx:
		return v.handleRefreshShardContext(ctx, req)

	// File System semantic aliases
	case ActionFSRead:
		return v.handleReadFile(ctx, req) // Delegate to existing handler
	case ActionFSWrite:
		return v.handleWriteFile(ctx, req) // Delegate to existing handler

	// Ouroboros actions
	case ActionGenerateTool:
		return v.handleGenerateTool(ctx, req)
	case ActionOuroborosDetect:
		return v.handleOuroborosDetect(ctx, req)
	case ActionOuroborosGen:
		return v.handleOuroborosGenerate(ctx, req)
	case ActionOuroborosCompile:
		return v.handleOuroborosCompile(ctx, req)
	case ActionOuroborosReg:
		return v.handleOuroborosRegister(ctx, req)
	case ActionRefineTool:
		return v.handleRefineTool(ctx, req)

	// Campaign actions
	case ActionCampaignClarify:
		return v.handleCampaignClarify(ctx, req)
	case ActionCampaignCreateFile:
		return v.handleCampaignCreateFile(ctx, req)
	case ActionCampaignModifyFile:
		return v.handleCampaignModifyFile(ctx, req)
	case ActionCampaignWriteTest:
		return v.handleCampaignWriteTest(ctx, req)
	case ActionCampaignRunTest:
		return v.handleCampaignRunTest(ctx, req)
	case ActionCampaignResearch:
		return v.handleCampaignResearch(ctx, req)
	case ActionCampaignVerify:
		return v.handleCampaignVerify(ctx, req)
	case ActionCampaignDocument:
		return v.handleCampaignDocument(ctx, req)
	case ActionCampaignRefactor:
		return v.handleCampaignRefactor(ctx, req)
	case ActionCampaignIntegrate:
		return v.handleCampaignIntegrate(ctx, req)
	case ActionCampaignComplete:
		return v.handleCampaignComplete(ctx, req)
	case ActionCampaignFinalVerify:
		return v.handleCampaignFinalVerify(ctx, req)
	case ActionCampaignCleanup:
		return v.handleCampaignCleanup(ctx, req)
	case ActionArchiveCampaign:
		return v.handleArchiveCampaign(ctx, req)
	case ActionShowCampaignStatus:
		return v.handleShowCampaignStatus(ctx, req)
	case ActionShowCampaignProg:
		return v.handleShowCampaignProgress(ctx, req)
	case ActionAskCampaignInt:
		return v.handleAskCampaignInterrupt(ctx, req)
	case ActionRunPhaseCheckpoint:
		return v.handleRunPhaseCheckpoint(ctx, req)
	case ActionPauseAndReplan:
		return v.handlePauseAndReplan(ctx, req)

	// Context Management actions
	case ActionCompressContext:
		return v.handleCompressContext(ctx, req)
	case ActionEmergencyCompress:
		return v.handleEmergencyCompress(ctx, req)
	case ActionCreateCheckpoint:
		return v.handleCreateCheckpoint(ctx, req)

	// Investigation/Analysis actions
	case ActionInvestigateAnomaly:
		return v.handleInvestigateAnomaly(ctx, req)
	case ActionInvestigateSystemic:
		return v.handleInvestigateSystemic(ctx, req)
	case ActionUpdateWorldModel:
		return v.handleUpdateWorldModel(ctx, req)

	// Corrective actions
	case ActionCorrectiveResearch:
		return v.handleCorrectiveResearch(ctx, req)
	case ActionCorrectiveDocs:
		return v.handleCorrectiveDocs(ctx, req)
	case ActionCorrectiveDecompose:
		return v.handleCorrectiveDecompose(ctx, req)

	// Code DOM Query alias
	case ActionQueryElements:
		return v.handleGetElements(ctx, req) // Delegate to existing handler

	// Python environment actions (general-purpose)
	case ActionPythonEnvSetup:
		return v.handlePythonEnvSetup(ctx, req)
	case ActionPythonEnvExec:
		return v.handlePythonEnvExec(ctx, req)
	case ActionPythonRunPytest:
		return v.handlePythonRunPytest(ctx, req)
	case ActionPythonApplyPatch:
		return v.handlePythonApplyPatch(ctx, req)
	case ActionPythonSnapshot:
		return v.handlePythonSnapshot(ctx, req)
	case ActionPythonRestore:
		return v.handlePythonRestore(ctx, req)
	case ActionPythonTeardown:
		return v.handlePythonTeardown(ctx, req)

	// SWE-bench actions (benchmark-specific, delegates to Python handlers)
	case ActionSWEBenchSetup:
		return v.handleSWEBenchSetup(ctx, req)
	case ActionSWEBenchApplyPatch:
		return v.handleSWEBenchApplyPatch(ctx, req)
	case ActionSWEBenchRunTests:
		return v.handleSWEBenchRunTests(ctx, req)
	case ActionSWEBenchSnapshot:
		return v.handleSWEBenchSnapshot(ctx, req)
	case ActionSWEBenchRestore:
		return v.handleSWEBenchRestore(ctx, req)
	case ActionSWEBenchEvaluate:
		return v.handleSWEBenchEvaluate(ctx, req)
	case ActionSWEBenchTeardown:
		return v.handleSWEBenchTeardown(ctx, req)

	// Research tool actions (modular tools for any agent)
	case ActionContext7Fetch, ActionWebSearch, ActionWebFetch,
		ActionBrowserNavigate, ActionBrowserExtract, ActionBrowserScreenshot,
		ActionBrowserClick, ActionBrowserType, ActionBrowserClose,
		ActionResearchCacheGet, ActionResearchCacheSet:
		return v.handleModularTool(ctx, req)

	default:
		return ActionResult{}, fmt.Errorf("unknown action type: %s", req.Type)
	}
}
