package context

import (
	"regexp"
	"sort"
	"testing"

	"codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

// The span that makes a repeated trace a cycle is the working section's
// repeat_threshold, read by the policy as config_param(/working_repeat_threshold)
// -- not a Go constant, and not the literal it was in working_set.mg until
// 2026-09-23. The loop is the only side that can see the tool trace, so it
// does the measuring; the span it measures against is the policy's, from
// config (sweep finding F8).
func TestRepeatThreshold_IsTheWorkingSectionsKey(t *testing.T) {
	for _, want := range []int{2, 3} {
		spans := config.DefaultWorkingConfig()
		spans.RepeatThreshold = want
		w, err := NewWorkingSet(nil, t.TempDir(), "threshold", spans)
		require.NoError(t, err)
		got, err := w.RepeatThreshold(t.Context())
		_ = w.Close()
		require.NoError(t, err)
		require.Equal(t, want, got, "working.repeat_threshold")
	}
	require.Contains(t, workingSetPolicy, "working_repeat_threshold(N) :- config_param(/working_repeat_threshold, N).")
}

// Every span the policy requires is one the working section supplies, and the
// other way round: a key added to one side only is a rule that never fires or
// a knob that binds nothing.
func TestWorkingSection_SuppliesEveryRequiredSpan(t *testing.T) {
	re := regexp.MustCompile(`config_param_required\(/working, (/[a-z_]+)\)\.`)
	var required []string
	for _, m := range re.FindAllStringSubmatch(workingSetPolicy, -1) {
		required = append(required, m[1])
	}
	var supplied []string
	for _, p := range config.DefaultWorkingConfig().Params() {
		supplied = append(supplied, p.Key)
	}
	sort.Strings(required)
	sort.Strings(supplied)
	require.NotEmpty(t, required)
	require.Equal(t, required, supplied)
}

// The policy decides with the section's values: a read task is nudged to
// conclude at the section's nudge span, not the default's.
func TestWorkingSection_ThePolicyReadsTheConfiguredSpans(t *testing.T) {
	spans := config.DefaultWorkingConfig()
	spans.NudgeRounds = 3
	w, err := NewWorkingSet(nil, t.TempDir(), "spans", spans)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	got, err := w.Continue(t.Context(), WorkingProgress{Rounds: 3, SinceWrite: 3, SinceVerify: 3})
	require.NoError(t, err)
	require.Equal(t, WorkingDecision{Continue: true, Nudge: "conclude"}, got)
}
