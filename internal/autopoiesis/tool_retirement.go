package autopoiesis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/approval"
	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// =============================================================================
// TOOL VALUE ON THE ORCHESTRATOR, AND RETIRING A GENERATED TOOL
// =============================================================================
// The policy derives retirement proposals (policy/tool_value.mg). Nothing here
// acts on one by itself: a generated tool is withdrawn only when a caller asks
// for that tool by name, the kernel still derives a /retire proposal for it,
// and the person (or the standing yolo instruction) approves that exact
// request through the same human-approval broker every other work-mode
// decision uses. A built-in has no path here at all.

// ProfilesDir is where the profile store, and so the tool ledger, lives for a
// tools directory (<workspace>/.nerd/tools).
func ProfilesDir(toolsDir string) string { return filepath.Join(toolsDir, ".profiles") }

// ToolMeter is the ledger as the executor's meter. It is nil on an
// orchestrator with no profile store, so a caller never holds a typed nil.
func (o *Orchestrator) ToolMeter() types.ToolMeter {
	if o == nil || o.profiles == nil {
		return nil
	}
	return o.profiles
}

// ToolValueSnapshot is the ledger read against the built-in catalog and the
// generated tools currently registered.
func (o *Orchestrator) ToolValueSnapshot() ToolLedgerSnapshot {
	snap := ToolLedgerSnapshot{Catalog: CatalogOf(tools.Global())}
	if o == nil {
		return snap
	}
	if o.profiles != nil {
		snap.Ledgers = o.profiles.ToolLedgers()
	}
	o.mu.RLock()
	loop := o.ouroboros
	o.mu.RUnlock()
	if loop != nil {
		for _, t := range loop.ListTools() {
			snap.Generated = append(snap.Generated, t.Name)
		}
	}
	return snap
}

// ToolValueReport asserts the ledger into the orchestrator's kernel and
// returns what the policy derives about every tool.
func (o *Orchestrator) ToolValueReport() (ToolValueReport, error) {
	if o == nil {
		return ToolValueReport{}, errors.New("tool value: no orchestrator")
	}
	o.mu.RLock()
	kernel := o.kernel
	o.mu.RUnlock()
	if kernel == nil {
		return ToolValueReport{}, errors.New("tool value: no kernel attached")
	}
	return EvaluateToolValue(kernel, o.ToolValueSnapshot(), o.config.UserConfig.GetToolValueConfig())
}

// toolRevoker is the optional capability of a synthesizer that can withdraw a
// tool it registered.
type toolRevoker interface {
	RevokeTool(name string) error
}

// RevokeTool withdraws a generated tool from the Ouroboros loop: its compiled
// binary and identity record are removed, so the next restore does not
// rediscover it, and it is forgotten by the runtime registry. Its source stays,
// so the tool can be regenerated or restored by hand.
func (o *OuroborosLoop) RevokeTool(name string) error {
	return o.registry.Revoke(name)
}

// Revoke removes a registered tool's compiled artifacts and forgets it.
func (r *RuntimeRegistry) Revoke(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.tools[name]
	if !ok {
		return fmt.Errorf("generated tool %q is not registered", name)
	}
	if sanitizeToolName(name) != name || filepath.Base(name) != name {
		return fmt.Errorf("tool name %q is not a canonical generated name", name)
	}
	var artifacts []string
	for _, binary := range []string{rt.BinaryPath, r.identityBinary(name)} {
		if binary == "" {
			continue
		}
		if err := validateCompiledArtifactPath(r.compiledDir, name, binary); err != nil {
			return fmt.Errorf("refuse to revoke %q: %w", name, err)
		}
		artifacts = append(artifacts, binary, binary+".identity.json")
	}
	for _, path := range artifacts {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("revoke %s: %w", path, err)
		}
	}
	delete(r.tools, name)
	delete(r.identities, name)
	return nil
}

func (r *RuntimeRegistry) identityBinary(name string) string {
	if identity, ok := r.identities[name]; ok {
		return identity.BinaryPath
	}
	return ""
}

// retirementIntent is the execution intent a retirement approval is bound to.
func retirementIntent(tool string) string { return "retire-generated-tool:" + tool }

// RetireGeneratedTool withdraws one generated tool the policy proposes to
// retire, after the work-mode approval. It fails, changing nothing, when:
//
//   - the kernel derives no /retire proposal for the tool (a tool that earns
//     its place, a built-in, or an unknown name is never withdrawn);
//   - the person rejects the request, it expires, or nobody can be asked
//     (propose and explicit modes need an attached host; yolo approves);
//   - the approved operation changed between review and execution.
//
// withdraw is the executor-facing registry (the virtual store's), which the
// tool is also removed from so the running session stops offering it.
func (o *Orchestrator) RetireGeneratedTool(ctx context.Context, broker *approval.Broker, name string, withdraw *core.ToolRegistry) (ToolRetirementProposal, error) {
	var proposal ToolRetirementProposal
	if broker == nil {
		return proposal, errors.New("retire: no approval broker, so nobody can approve it")
	}
	name = strings.TrimSpace(name)
	report, err := o.ToolValueReport()
	if err != nil {
		return proposal, err
	}
	var reasons []string
	for _, p := range report.Proposals {
		if p.Tool == name && p.Kind == "retire" {
			proposal = p
			reasons = append(reasons, p.Reason)
		}
	}
	if len(reasons) == 0 {
		return proposal, fmt.Errorf("retire: the policy derives no retirement proposal for %q; nothing is withdrawn without one", name)
	}
	o.mu.RLock()
	loop := o.ouroboros
	o.mu.RUnlock()
	revoker, ok := loop.(toolRevoker)
	if !ok {
		return proposal, errors.New("retire: the tool synthesizer cannot withdraw a tool")
	}
	version := ""
	if rt, found := loop.GetRuntimeTool(name); found {
		version = rt.Hash
	}
	subject := approval.Subject{
		Workspace:   o.config.WorkspaceRoot,
		Agent:       "autopoiesis",
		Backend:     "autopoiesis",
		Tool:        name,
		ToolVersion: version,
		Action:      "/retire_generated_tool",
		Target:      name,
		Payload:     strings.Join(reasons, ","),
		Effect:      string(tools.EffectWrite),
	}
	preview, err := approval.BuildPreview(approval.PreviewInput{
		Summary: fmt.Sprintf("Withdraw generated tool %q (%s)", name, strings.Join(reasons, ", ")),
		Effects: []string{"removes the tool's compiled binary and identity record",
			"the tool is no longer offered to any session",
			"its source stays on disk and the tool can be regenerated"},
		Arguments: subject.Payload,
		MaxBytes:  broker.PreviewMaxBytes(),
	})
	if err != nil {
		return proposal, fmt.Errorf("retire: preview: %w", err)
	}
	intent := retirementIntent(name)
	grant, _, err := broker.Await(ctx, intent, subject, preview)
	if err != nil {
		return proposal, fmt.Errorf("retire %q: %w", name, err)
	}
	if err := grant.Reserve(intent, subject); err != nil {
		return proposal, fmt.Errorf("retire %q: %w", name, err)
	}
	if err := grant.DispatchStarted(); err != nil {
		grant.AdmissionFailed(err.Error())
		return proposal, fmt.Errorf("retire %q: %w", name, err)
	}
	err = revoker.RevokeTool(name)
	if err == nil && withdraw != nil {
		// A tool absent from the session registry is not a failure: it may
		// never have been offered this session.
		if _, registered := withdraw.GetTool(name); registered {
			err = withdraw.UnregisterTool(name)
		}
	}
	grant.Finish(err)
	if err != nil {
		return proposal, fmt.Errorf("retire %q: %w", name, err)
	}
	if o.profiles != nil {
		o.profiles.MarkToolRetired(name)
	}
	return proposal, nil
}
