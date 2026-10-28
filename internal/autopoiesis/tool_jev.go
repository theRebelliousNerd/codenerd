package autopoiesis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/typesafe"
)

// Jev's tool checks for the Ouroboros loop (switch typesafe.tool_checks;
// questions in internal/learnjudge/toolcheck.go;
// Docs/architecture/jev-turn-judgments.md, "Tool checks").
//
// Before a tool is generated, Jev is asked whether a tool already registered
// does what the request needs. After the code passes the safety check, Jev is
// asked whether it does what was asked and whether it also does something
// that was not. Each answer is a probability; the decision is the truth table
// below, read against the typesafe section's yes and no thresholds:
//
//	duplicate check   any tool does this >= yes   reuse the chosen tool, generate nothing
//	                  <= no, or between           generate
//	code check        does what was asked <= no   regenerate with Jev's reason as feedback
//	                  does unrequested >= yes     regenerate with Jev's reason as feedback
//	                  otherwise (yes, or between) the code goes on to the Thunderdome
//
// The decision lives here, not in the kernel: the loop runs on its own
// transient engine, and its outcome is the LoopResult its callers already read.
// A Jev that is unconfigured, switched off, failed or timed out is no answer,
// never a "no": the loop generates and accepts exactly as it did without Jev,
// and the log says so.

// toolJev is the loop's Jev: the client and the section that holds the
// tool_checks switch and the thresholds. A zero toolJev is Jev off.
type toolJev struct {
	asker learnjudge.Asker
	cfg   *internalconfig.TypeSafeConfig
}

// SetJev installs Jev for the tool checks; a nil asker turns them off.
func (o *OuroborosLoop) SetJev(asker learnjudge.Asker, cfg *internalconfig.TypeSafeConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.jev = toolJev{asker: asker, cfg: cfg}
}

// initJev builds Jev from the operator's typesafe section. A section that
// cannot resolve leaves the checks off and says why.
func (o *OuroborosLoop) initJev() {
	uc := o.config.UserConfig
	if uc == nil || uc.TypeSafe == nil {
		return
	}
	client, err := typesafe.FromConfig(uc.TypeSafe)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("jev: tool checks off, the typesafe section does not resolve: %v", err)
		return
	}
	if client != nil {
		o.jev = toolJev{asker: client, cfg: uc.TypeSafe}
	}
}

// toolChecks is the installed Jev when the tool_checks use is on; ok is false
// otherwise, and why names the reason for the log.
func (o *OuroborosLoop) toolChecks() (j toolJev, ok bool, why string) {
	o.mu.RLock()
	j = o.jev
	o.mu.RUnlock()
	switch {
	case j.asker == nil:
		return j, false, "Jev is not configured"
	case !j.cfg.Uses().ToolChecks:
		return j, false, "typesafe.tool_checks is off"
	}
	return j, true, ""
}

// thresholds are the yes and no thresholds as probabilities.
func (j toolJev) thresholds() (yes, no float64) {
	for _, p := range j.cfg.Params() {
		switch p.Key {
		case "/jev_yes_percent":
			yes = float64(p.Value) / 100
		case "/jev_no_percent":
			no = float64(p.Value) / 100
		}
	}
	return yes, no
}

// toolRequestOf is the need as the request Jev reads.
func toolRequestOf(need *ToolNeed) learnjudge.ToolRequest {
	return learnjudge.ToolRequest{Name: need.Name, Purpose: need.Purpose, Input: need.InputType, Output: need.OutputType}
}

// duplicateCandidates are the registered tools other than the one being
// (re)generated, newest first: regenerating a tool under its own name is a
// deliberate new version, not a duplicate. Past learnjudge.MaxToolOptions the
// oldest are left out, and the count says so.
func (o *OuroborosLoop) duplicateCandidates(name string) (byName map[string]*RuntimeTool, tools []learnjudge.ExistingTool, left int) {
	all := o.registry.List()
	sort.Slice(all, func(i, k int) bool { return all[i].RegisteredAt.After(all[k].RegisteredAt) })
	byName = map[string]*RuntimeTool{}
	for _, t := range all {
		if t == nil || t.Name == "" || t.Name == name {
			continue
		}
		if len(tools) == learnjudge.MaxToolOptions {
			left++
			continue
		}
		byName[t.Name] = t
		tools = append(tools, learnjudge.ExistingTool{Name: t.Name, Description: t.Description})
	}
	return byName, tools, left
}

// decideDuplicate is the duplicate check's truth table: the tool to reuse,
// or "" to generate, and the row that decided.
func decideDuplicate(c learnjudge.DuplicateCheck, yes, no float64) (reuse, row string) {
	switch {
	case c.AnyExists >= yes:
		return c.Tool, "an existing tool does this"
	case c.AnyExists <= no:
		return "", "no existing tool does this"
	default:
		return "", "unsettled"
	}
}

// existingToolFor asks Jev whether a registered tool already does what need
// asks. It returns that tool, or nil when the loop should generate.
func (o *OuroborosLoop) existingToolFor(ctx context.Context, need *ToolNeed) *RuntimeTool {
	log := logging.Get(logging.CategoryAutopoiesis)
	j, ok, why := o.toolChecks()
	if !ok {
		log.Info("jev: no duplicate check for tool %s (%s); generating as before", need.Name, why)
		return nil
	}
	byName, tools, left := o.duplicateCandidates(need.Name)
	if len(tools) == 0 {
		logging.AutopoiesisDebug("jev: no registered tool to compare %s with; generating", need.Name)
		return nil
	}
	if left > 0 {
		log.Warn("jev: duplicate check for %s compares the %d newest tools; %d older ones are not compared", need.Name, len(tools), left)
	}
	check, err := learnjudge.CheckDuplicate(ctx, j.asker, toolRequestOf(need), tools)
	if err != nil {
		log.Warn("jev: duplicate check for tool %s failed, generating as before: %v", need.Name, err)
		return nil
	}
	yes, no := j.thresholds()
	reuse, row := decideDuplicate(check, yes, no)
	log.Info("jev: duplicate check for %s: any=%.2f closest=%s (%.2f): %s", need.Name, check.AnyExists, check.Tool, check.ToolProbability, row)
	if reuse == "" {
		return nil
	}
	tool, found := byName[reuse]
	if !found {
		log.Warn("jev: duplicate check chose %q, which is not a registered tool; generating", reuse)
		return nil
	}
	return tool
}

// decideCode is the code check's truth table: the reason the code does not
// match its request, or "" when it goes on.
func decideCode(c learnjudge.CodeCheck, yes, no float64) string {
	var why []string
	if c.DoesAsked <= no {
		why = append(why, fmt.Sprintf("the code does not do what the request asks (Jev %.0f%% that it does)", c.DoesAsked*100))
	}
	if c.DoesExtra >= yes {
		why = append(why, fmt.Sprintf("the code also does something the request did not ask for, such as writing, deleting or sending data or running another program (Jev %.0f%%)", c.DoesExtra*100))
	}
	return strings.Join(why, "; ")
}

// codeMismatch asks Jev whether code matches need. It returns why it does
// not, or "" when the code goes on: a match, an unsettled reading, or no
// answer (which is logged).
func (o *OuroborosLoop) codeMismatch(ctx context.Context, need *ToolNeed, code string) string {
	log := logging.Get(logging.CategoryAutopoiesis)
	j, ok, why := o.toolChecks()
	if !ok {
		log.Info("jev: no code check for tool %s (%s); accepting as before", need.Name, why)
		return ""
	}
	check, err := learnjudge.CheckCode(ctx, j.asker, toolRequestOf(need), code)
	if err != nil {
		log.Warn("jev: code check for tool %s failed, accepting as before: %v", need.Name, err)
		return ""
	}
	yes, no := j.thresholds()
	reason := decideCode(check, yes, no)
	verdict := reason
	if verdict == "" {
		verdict = "matches its request"
		if check.DoesAsked < yes || check.DoesExtra > no {
			verdict = "unsettled, accepted"
		}
	}
	log.Info("jev: code check for %s: asked=%.2f extra=%.2f: %s", need.Name, check.DoesAsked, check.DoesExtra, verdict)
	return reason
}
