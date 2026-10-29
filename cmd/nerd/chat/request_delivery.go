package chat

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// Delivery rounds (policy/request_delivery.mg). A delegated or multi-step
// chat request used to end "Ran N step(s); the work is not verified" with
// nothing to say whether the steps did what was asked, and nothing to make
// them go on when they had not. When the continuation has no obligation left,
// Jev reads the request against every step's report and the files they
// changed (learnjudge.CheckDelivery); the kernel decides whether a missing
// part goes back as one more step. The rounds stop when nothing is missing,
// the reports name an obstacle codeNERD cannot clear, or a step resolves none
// of what it was handed. Jev off, or a failed call, judges nothing and says
// so: a failure is never read as "delivered".

// deliveryChain is the request one continuation is delivering, shared by
// pointer across the Model copies of that continuation. Steps run one at a
// time, so only the running step's goroutine touches it; the mutex is for the
// race detector, not for contention.
type deliveryChain struct {
	mu        sync.Mutex
	request   types.MangleAtom
	text      string
	shardType string
	fragments []string
	reports   []string
	changed   []string
	round     int
	// note is the latest round's verdict, worded for the summary.
	note string
}

// startDelivery opens a delivery chain for request, the person's own words,
// with shardType taking any step that hands a gap back. Nil when there is no
// kernel, no Jev client, or the instruction_coverage use is off. Only the
// newest request can raise a step (delivery_active).
func (m *Model) startDelivery(request, shardType string) *deliveryChain {
	if m.kernel == nil || m.jev == nil || m.Config == nil || !m.Config.TypeSafe.Uses().InstructionCoverage {
		return nil
	}
	fragments := learnjudge.RequestFragments(request)
	if len(fragments) == 0 {
		return nil
	}
	dc := &deliveryChain{
		request:   types.MangleAtom(fmt.Sprintf("/delivery_%d", time.Now().UnixNano())),
		text:      strings.TrimSpace(request),
		shardType: shardType,
		fragments: fragments,
	}
	_ = m.kernel.Retract("delivery_active")
	if err := m.kernel.Assert(types.Fact{Predicate: "delivery_active", Args: []any{dc.request}}); err != nil {
		logging.Get(logging.CategoryRouting).Warn("delivery: request not opened: %v", err)
		return nil
	}
	return dc
}

// endDelivery closes the chain: no step it offered can be raised after it.
func (m *Model) endDelivery() {
	m.delivery = nil
	if m.kernel != nil {
		_ = m.kernel.Retract("delivery_active")
	}
}

// record adds one step's report and write set to what the request has been
// delivered.
func (dc *deliveryChain) record(ret observation.Return) {
	if dc == nil {
		return
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	report := ret.Output
	if strings.TrimSpace(report) == "" && ret.Failure != "" {
		report = "The step failed: " + ret.Failure
	}
	dc.reports = append(dc.reports, report)
	for _, p := range ret.Changed {
		if !slices.Contains(dc.changed, p) {
			dc.changed = append(dc.changed, p)
		}
	}
}

// summaryNote is the latest round's verdict, for the step summary.
func (dc *deliveryChain) summaryNote() string {
	if dc == nil {
		return ""
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.note
}

// judgeDelivery asks Jev about one more round, asserts the reading and, when
// the kernel reads parts missing, offers the step that would deliver them.
// Whether that step runs is the kernel's (has_pending_subtask). It reports
// whether a reading was asserted.
func (m *Model) judgeDelivery(ctx context.Context, dc *deliveryChain) bool {
	if dc == nil || m.kernel == nil || m.jev == nil {
		return false
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()
	log := logging.Get(logging.CategoryRouting)
	reading, err := learnjudge.CheckDelivery(m.jevContext(ctx), m.jev, learnjudge.DeliveryInput{
		Request: dc.text, Reports: dc.reports, Changed: dc.changed, Fragments: dc.fragments,
	})
	if err != nil {
		log.Warn("delivery: Jev did not read the request (%v); no step is handed back", err)
		dc.note = fmt.Sprintf("Whether every part of the request was delivered was not judged (Jev: %v).", err)
		return false
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("delivery: thresholds not asserted (%v); the reading is dropped", err)
		dc.note = fmt.Sprintf("Whether every part of the request was delivered was not judged (thresholds: %v).", err)
		return false
	}
	dc.round++
	if err := m.kernel.AssertBatch(learnjudge.DeliveryFacts(dc.request, dc.round, reading)); err != nil {
		log.Warn("delivery: reading not asserted: %v", err)
		dc.note = fmt.Sprintf("Whether every part of the request was delivered was not judged (kernel: %v).", err)
		return false
	}
	missing := dc.fragmentsIn("delivery_fragment_missing", m.kernel)
	if len(missing) > 0 {
		offer := types.Fact{Predicate: "delivery_step_offer", Args: []any{
			dc.request, int64(dc.round), fmt.Sprintf("delivery_%d_%d", time.Now().UnixNano(), dc.round),
			gapStepTask(dc.text, missing), types.MangleAtom("/" + dc.shardType),
		}}
		if err := m.kernel.Assert(offer); err != nil {
			log.Warn("delivery: the step for the missing parts was not offered: %v", err)
		}
	}
	dc.note = dc.outcomeNote(m.kernel, missing)
	log.Info("delivery round %d: %d fragment(s), missing %q; %s", dc.round, len(dc.fragments), missing, dc.note)
	return true
}

// roundValues is the third column of predicate(Request, Round, Value) for
// this request's current round.
func (dc *deliveryChain) roundValues(predicate string, k types.Kernel) map[string]bool {
	rows, err := k.Query(predicate)
	if err != nil {
		logging.Get(logging.CategoryRouting).Warn("delivery: query %s: %v", predicate, err)
		return nil
	}
	held := map[string]bool{}
	for _, r := range rows {
		if len(r.Args) == 3 && types.ExtractString(r.Args[0]) == string(dc.request) && roundArg(r.Args[1]) == dc.round {
			held[types.ExtractString(r.Args[2])] = true
		}
	}
	return held
}

// fragmentsIn returns this round's fragments that the kernel derives
// predicate(Request, Round, Fragment) for, in the request's order.
func (dc *deliveryChain) fragmentsIn(predicate string, k types.Kernel) []string {
	held := dc.roundValues(predicate, k)
	var out []string
	for _, f := range dc.fragments {
		if held[f] {
			out = append(out, f)
		}
	}
	return out
}

// outcomeNote words the kernel's delivery_outcome for this round.
func (dc *deliveryChain) outcomeNote(k types.Kernel, missing []string) string {
	outcome := ""
	for o := range dc.roundValues("delivery_outcome", k) {
		outcome = o
	}
	parts := strings.Join(missing, " | ")
	switch outcome {
	case "/delivered":
		return "Jev read every part of the request as delivered."
	case "/unsettled":
		return "Jev could not tell whether these parts of the request were delivered: " + strings.Join(dc.fragmentsIn("delivery_fragment_unsettled", k), " | ") + "."
	case "/blocked":
		return "Not delivered: " + parts + ". The steps name an obstacle codeNERD cannot clear by itself."
	case "/no_progress":
		return "Not delivered: " + parts + ". The last step delivered none of them, so no further step was run."
	case "/pushback":
		return "Not delivered yet: " + parts + "."
	}
	return "Whether every part of the request was delivered was not derived."
}

func roundArg(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	}
	return -1
}

// gapStepTask is the step that hands the missing parts back. It names them in
// the person's words and keeps the whole request beside them; what earlier
// steps did reaches the shard through the prior shard context.
func gapStepTask(request string, missing []string) string {
	var b strings.Builder
	b.WriteString("Part of this request is not done yet:\n")
	for _, f := range missing {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	b.WriteString("\nThe whole request: ")
	b.WriteString(request)
	b.WriteString("\n\nDo the parts that are not done; the earlier steps' reports are in your context. " +
		"If an obstacle you cannot clear yourself stops you (a permission or policy denial, a missing credential or input, " +
		"a choice only the user can make), say exactly what it is and what you need.")
	return b.String()
}

// derivedNextStep is the next step the kernel derives, if it derives one.
func (m *Model) derivedNextStep() *continueMsg {
	if m.kernel == nil {
		return nil
	}
	shouldContinue, _ := m.kernel.Query("should_auto_continue")
	if len(shouldContinue) == 0 {
		return nil
	}
	pending, _ := m.kernel.Query("has_pending_subtask")
	for _, subtask := range pending {
		if len(subtask.Args) < 3 {
			continue
		}
		nextID, _ := subtask.Args[0].(string)
		nextDesc, _ := subtask.Args[1].(string)
		nextShard := types.StripAtomPrefix(types.ExtractString(subtask.Args[2]))
		return &continueMsg{
			subtaskID:   nextID,
			description: nextDesc,
			shardType:   nextShard,
			isMutation:  isMutationOperation(nextShard),
		}
	}
	return nil
}

// nextStep is the step after the one that just finished: an obligation the
// kernel derives first, and only when none is left, a part of the request
// the steps have not delivered (one delivery round).
func (m *Model) nextStep(ctx context.Context, dc *deliveryChain) *continueMsg {
	if next := m.derivedNextStep(); next != nil {
		return next
	}
	if !m.judgeDelivery(ctx, dc) {
		return nil
	}
	return m.derivedNextStep()
}

// gapShard is the shard a multi-step plan's missing parts go to: the last
// step that writes, since an undone part of a plan that writes is most often
// write work, else the last step that ran.
func gapShard(ran []string) string {
	for i := len(ran) - 1; i >= 0; i-- {
		if isMutationOperation(ran[i]) {
			return ran[i]
		}
	}
	if len(ran) == 0 {
		return ""
	}
	return ran[len(ran)-1]
}

// withDeliveryNote puts the delivery verdict under a reply that ended its
// request without another step.
func withDeliveryNote(surface, note string) string {
	if note == "" {
		return surface
	}
	return strings.TrimRight(surface, "\n") + "\n\n" + note
}
