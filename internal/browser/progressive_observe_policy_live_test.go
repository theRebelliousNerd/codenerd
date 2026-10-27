//go:build integration

package browser_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

type progressivePolicyKernelSink struct{ kernel types.Kernel }

func (s progressivePolicyKernelSink) AddFacts(facts []mangle.Fact) error {
	converted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, types.Fact{Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...)})
	}
	return s.kernel.AssertBatch(converted)
}

func TestProgressiveObservePolicyRoundTrip(t *testing.T) {
	chromeBinary := os.Getenv("NERD_TEST_CHROME_BIN")
	if chromeBinary == "" {
		t.Skip("set NERD_TEST_CHROME_BIN to run progressive observation with headless Chrome")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body>
<button id="submit" type="submit" style="position:absolute;left:17.25px;top:21.5px;width:51.5px;height:23.25px">Continue</button>
<button id="native-disabled" disabled>Native disabled</button>
<button id="aria-disabled" aria-disabled="true">ARIA disabled</button>
<button id="display-trap" style="display:none">Display trap</button>
<button id="opacity-trap" style="opacity:0">Opacity trap</button>
<button id="zero-trap" style="position:absolute;left:0;top:0;width:0;height:0;padding:0;border:0">Zero geometry trap</button>
<button id="subpixel" style="position:absolute;left:400px;top:40px;box-sizing:border-box;width:0.25px;height:0.25px;padding:0;border:0">Subpixel control</button>
</body></html>`)
	}))
	defer server.Close()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	policy := config.DefaultBrowserPolicyConfig()
	configuredPrimaryRank := 137
	policy.PriorityPrimary = &configuredPrimaryRank
	if err := kernel.AssertBatch(config.ParamFacts(policy.Params())); err != nil {
		t.Fatalf("assert configured browser policy: %v", err)
	}

	cfg := browser.DefaultConfig()
	cfg.Headless = true
	cfg.Launch = []string{chromeBinary}
	cfg.NavigationTimeoutMs = 10000
	cfg.EventThrottleMs = 10
	manager := browser.NewSessionManagerWithSink(cfg, progressivePolicyKernelSink{kernel: kernel})
	manager.SetFactQuerier(browser.NewKernelFactQuerier(kernel))
	manager.SetFactRetractor(browser.NewKernelFactRetractor(kernel))
	t.Cleanup(func() {
		_ = manager.Shutdown(context.Background())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	session, err := manager.CreateSession(ctx, server.URL)
	if err != nil {
		t.Skipf("headless Chrome unavailable: %v", err)
	}

	first, err := manager.Observe(ctx, session.ID, browser.ObserveOptions{
		Mode: "interactive", View: "compact", MaxItems: 20, VisibleOnly: false,
	})
	if err != nil {
		t.Fatalf("first Observe: %v", err)
	}
	candidates := progressiveCandidates(t, first)
	if len(candidates) < 2 {
		t.Fatalf("fixture must provide multiple kernel candidates for the MaxItems bound check, got %+v", candidates)
	}
	refs := make(map[string]string)
	for _, element := range interactiveFrom(first) {
		refs[element.Label] = element.Ref
	}
	submitRef := refs["Continue"]
	if submitRef == "" {
		t.Fatalf("interactive observation omitted submit control: %+v", interactiveFrom(first))
	}
	assertProgressiveCandidate(t, candidates, submitRef, "Continue", "click", int64(configuredPrimaryRank), "primary_action")
	if subpixelRef := refs["Subpixel control"]; subpixelRef == "" {
		t.Fatalf("interactive observation omitted subpixel control: %+v", interactiveFrom(first))
	} else {
		assertProgressiveCandidate(t, candidates, subpixelRef, "Subpixel control", "click", 80, "enabled_button")
		assertProgressiveGeometry(t, kernel, session.ID, subpixelRef, "400", "40", "0.25", "0.25")
	}
	for _, label := range []string{"Native disabled", "ARIA disabled", "Display trap", "Opacity trap", "Zero geometry trap"} {
		if ref := refs[label]; ref == "" {
			t.Errorf("interactive observation omitted %q", label)
		} else if hasCandidateForRef(candidates, ref) {
			t.Errorf("policy returned an action candidate for %q (%s): %+v", label, ref, candidates)
		}
	}
	if first.Data["action_candidates_stale_facts_possible"] != false {
		t.Fatalf("fresh retractable snapshot marked stale: %v", first.Data["action_candidates_stale_facts_possible"])
	}
	assertProgressiveMeasuredFacts(t, kernel, session.ID, submitRef)
	assertProgressiveGeometry(t, kernel, session.ID, submitRef, "17.25", "21.5", "51.5", "23.25")
	assertProgressiveGeometry(t, kernel, session.ID, refs["Zero geometry trap"], "0", "0", "0", "0")

	page, ok := manager.Page(session.ID)
	if !ok || page == nil {
		t.Fatal("session page disappeared before replacement observation")
	}
	if _, err := page.Context(ctx).Eval(`() => {
  document.getElementById('submit').disabled = true;
  document.getElementById('native-disabled').remove();
}`); err != nil {
		t.Fatalf("mutate controls before replacement observation: %v", err)
	}
	second, err := manager.Observe(ctx, session.ID, browser.ObserveOptions{
		Mode: "interactive", View: "compact", MaxItems: 20, VisibleOnly: false,
	})
	if err != nil {
		t.Fatalf("second Observe: %v", err)
	}
	if second.Data["action_candidates_stale_facts_possible"] != false {
		t.Fatalf("retractable replacement marked stale: %v", second.Data["action_candidates_stale_facts_possible"])
	}
	if hasCandidateForRef(progressiveCandidates(t, second), submitRef) {
		t.Fatalf("disabled submit retained an action candidate: %+v", progressiveCandidates(t, second))
	}
	if got := queryLiveFacts(t, kernel, "element_enabled", session.ID, submitRef); len(got) != 1 || types.ExtractString(got[0].Args[2]) != "false" {
		t.Fatalf("replacement did not retract old enabled evidence: %+v", got)
	}
	if got := queryLiveFacts(t, kernel, "interactive", session.ID, refs["Native disabled"]); len(got) != 0 {
		t.Fatalf("removed control facts survived replacement: %+v", got)
	}
	if _, err := page.Context(ctx).Eval(`() => { document.getElementById('submit').disabled = false; }`); err != nil {
		t.Fatalf("enable one control before bounded observation: %v", err)
	}
	bounded, err := manager.Observe(ctx, session.ID, browser.ObserveOptions{
		Mode: "interactive", View: "compact", MaxItems: 1, VisibleOnly: false,
	})
	if err != nil {
		t.Fatalf("bounded Observe: %v", err)
	}
	boundedInteractive := interactiveFrom(bounded)
	if len(boundedInteractive) != 1 {
		t.Fatalf("MaxItems=1 returned %d interactive observations: %+v", len(boundedInteractive), boundedInteractive)
	}
	boundedCandidates := progressiveCandidates(t, bounded)
	if len(boundedCandidates) > 1 {
		t.Fatalf("MaxItems=1 returned %d action candidates: %+v", len(boundedCandidates), boundedCandidates)
	}
	assertProgressiveCandidate(t, boundedCandidates, boundedInteractive[0].Ref, "Continue", "click", int64(configuredPrimaryRank), "primary_action")
	if !bounded.Truncated {
		t.Fatal("MaxItems=1 did not report truncation for the multi-control page")
	}
}

func interactiveFrom(observation browser.ProgressiveObservation) []browser.InteractiveElement {
	items, _ := observation.Data["interactive"].([]browser.InteractiveElement)
	return items
}

func progressiveCandidates(t *testing.T, observation browser.ProgressiveObservation) []browser.ActionCandidate {
	t.Helper()
	candidates, ok := observation.Data["action_candidates"].([]browser.ActionCandidate)
	if !ok {
		t.Fatalf("action_candidates has type %T", observation.Data["action_candidates"])
	}
	return candidates
}

func assertProgressiveCandidate(t *testing.T, candidates []browser.ActionCandidate, ref, label, action string, priority int64, reason string) {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Ref == ref && candidate.Label == label && candidate.Action == action && candidate.Priority == priority && candidate.Reason == reason {
			return
		}
	}
	t.Fatalf("Mangle candidate (%q, %q, %q, %d, %q) missing from %+v", ref, label, action, priority, reason, candidates)
}

func hasCandidateForRef(candidates []browser.ActionCandidate, ref string) bool {
	for _, candidate := range candidates {
		if candidate.Ref == ref {
			return true
		}
	}
	return false
}

func assertProgressiveMeasuredFacts(t *testing.T, kernel types.Kernel, sessionID, ref string) {
	t.Helper()
	interactive := queryLiveFacts(t, kernel, "interactive", sessionID, ref)
	if len(interactive) != 1 || types.ExtractString(interactive[0].Args[2]) != "button" || types.ExtractString(interactive[0].Args[4]) != "click" {
		t.Fatalf("interactive source fact missing or malformed: %+v", interactive)
	}
	enabled := queryLiveFacts(t, kernel, "element_enabled", sessionID, ref)
	if len(enabled) != 1 || types.ExtractString(enabled[0].Args[2]) != "true" {
		t.Fatalf("enabled source fact missing: %+v", enabled)
	}
	attribute := queryLiveFacts(t, kernel, "browser_control_attribute", sessionID, ref)
	foundSubmit := false
	for _, fact := range attribute {
		if types.ExtractString(fact.Args[2]) == "type" && types.ExtractString(fact.Args[3]) == "submit" {
			foundSubmit = true
		}
	}
	if !foundSubmit {
		t.Fatalf("explicit submit control attribute missing: %+v", attribute)
	}
	styles := queryLiveFacts(t, kernel, "computed_style", sessionID+":"+ref)
	styleValues := map[string]string{}
	for _, fact := range styles {
		styleValues[types.ExtractString(fact.Args[1])] = types.ExtractString(fact.Args[2])
	}
	if styleValues["display"] == "" || styleValues["visibility"] != "visible" || styleValues["opacity"] != "1" {
		t.Fatalf("measured computed styles missing or malformed: %+v", styles)
	}
	geometry := queryLiveFacts(t, kernel, "geometry", sessionID+":"+ref)
	if len(geometry) != 1 {
		t.Fatalf("policy geometry missing: %+v", geometry)
	}
	if x, ok := types.ExtractInt64(geometry[0].Args[1]); !ok || x != 17 {
		t.Fatalf("policy geometry x projection=%v, want integer 17: %+v", geometry[0].Args[1], geometry[0].Args)
	}
	if width, ok := types.ExtractInt64(geometry[0].Args[3]); !ok || width != 52 {
		t.Fatalf("policy geometry width projection=%v, want integer 52: %+v", geometry[0].Args[3], geometry[0].Args)
	}
	observedAt := queryLiveFacts(t, kernel, "browser_observed_at", sessionID)
	if len(observedAt) == 0 {
		t.Fatal("browser_observed_at was not asserted")
	}
	if _, ok := types.ExtractInt64(observedAt[len(observedAt)-1].Args[1]); !ok {
		t.Fatalf("browser_observed_at is not a millisecond integer: %+v", observedAt[len(observedAt)-1])
	}
}

func assertProgressiveGeometry(t *testing.T, kernel types.Kernel, sessionID, ref, x, y, width, height string) {
	t.Helper()
	exactGeometry := queryLiveFacts(t, kernel, "browser_geometry_css_px", sessionID, ref)
	if len(exactGeometry) != 1 || types.ExtractString(exactGeometry[0].Args[2]) != x || types.ExtractString(exactGeometry[0].Args[3]) != y || types.ExtractString(exactGeometry[0].Args[4]) != width || types.ExtractString(exactGeometry[0].Args[5]) != height {
		t.Fatalf("lossless CSS-pixel geometry for %s is missing or altered: %+v", ref, exactGeometry)
	}
	geometry := queryLiveFacts(t, kernel, "geometry", sessionID+":"+ref)
	if len(geometry) != 1 {
		t.Fatalf("policy geometry for %s missing: %+v", ref, geometry)
	}
	projectedWidth, widthOK := types.ExtractInt64(geometry[0].Args[3])
	projectedHeight, heightOK := types.ExtractInt64(geometry[0].Args[4])
	if width == "0" || height == "0" {
		if !widthOK || !heightOK || projectedWidth != 0 || projectedHeight != 0 {
			t.Fatalf("exact-zero geometry projected nonzero for %s: %+v", ref, geometry[0].Args)
		}
	} else if !widthOK || !heightOK || projectedWidth < 1 || projectedHeight < 1 {
		t.Fatalf("positive subpixel geometry projected to zero for %s: %+v", ref, geometry[0].Args)
	}
}

func queryLiveFacts(t *testing.T, kernel types.Kernel, predicate string, args ...string) []types.Fact {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	matched := make([]types.Fact, 0)
	for _, fact := range facts {
		match := true
		for index, want := range args {
			if want == "" {
				continue
			}
			if index >= len(fact.Args) {
				match = false
				break
			}
			got := types.ExtractString(fact.Args[index])
			if got != want && got != "/"+want {
				match = false
				break
			}
		}
		if match {
			matched = append(matched, fact)
		}
	}
	return matched
}
