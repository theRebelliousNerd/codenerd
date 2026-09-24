package config

import (
	"fmt"
	"time"
)

// CampaignConfig is the `campaign` section of .nerd/config.json: every knob the
// campaign executive reads. Steve's rule for it: "everything is configurable,
// not hard coded.. all from config.json".
//
// Before this section existed the campaign had no configuration at all. Its
// retry cap, backoffs, checkpoint cap, repro escalation, acceptance rounds and
// command limits were Go literals and Mangle constants, and the three doors a
// campaign starts through built three different orchestrators: chat /assault
// and /recurse ran one task at a time with auto-replan and checkpoint-on-fail,
// while `nerd campaign start` and chat /campaign ran three at a time with
// neither. The same campaign behaved differently depending on who started it.
// Now every entry point passes this section, and nothing else, as the policy.
//
// Counts left at 0 and durations left empty take the default below (every
// count here must be at least 1, so 0 cannot mean anything else). The two
// switches and the confidence floor are pointers because false and 0 are
// meaningful values for them.
//
// The knobs the kernel's own rules read reach it as config_param facts
// (Params, ParamFacts): the policy and the Go drivers read one struct.
type CampaignConfig struct {
	// --- a task's attempts ---

	// MaxTaskAttempts is how many failed attempts a task may have before it
	// fails (policy: task_exhausted). The default 4 is the old MaxRetries of 3
	// counted as the orchestrator counted it: three retries after the first.
	MaxTaskAttempts int `json:"max_task_attempts,omitempty"`
	// ReproAfterFailures is how many attempts of a task that writes code must
	// end on a red test suite before a reproduction test run is scheduled in
	// front of its next attempt (policy: task_owes_repro).
	ReproAfterFailures int `json:"repro_after_failures,omitempty"`
	// ReplanAtAttemptCap asks the replanner, once per task, to drop or retype a
	// task that exhausted its attempts before the phase blocks on it.
	ReplanAtAttemptCap *bool `json:"replan_at_attempt_cap,omitempty"`
	// RetryBackoffBase is the first wait before a retry; it doubles per attempt.
	RetryBackoffBase string `json:"retry_backoff_base,omitempty"`
	// RetryBackoffMax caps the wait before a retry that has to arrive later
	// (a refused request, a deadline): only time changes those answers.
	RetryBackoffMax string `json:"retry_backoff_max,omitempty"`
	// RetryWithReasonBackoffMax caps the wait before a retry that carries the
	// failure's reason to the next attempt: waiting does not fix it, the next
	// attempt does.
	RetryWithReasonBackoffMax string `json:"retry_with_reason_backoff_max,omitempty"`

	// --- a phase's verification ---

	// MaxCheckpointAttempts is how many failed checkpoints close a phase
	// unverified (policy: phase_ckpt_exhausted).
	MaxCheckpointAttempts int `json:"max_checkpoint_attempts,omitempty"`
	// ReplanOnCheckpointFailure gives a phase whose checkpoint failed, below
	// the cap, one task briefed with the checkpoint's findings (policy:
	// phase_ckpt_move /replan). Off, the checkpoint re-runs over the same work.
	ReplanOnCheckpointFailure *bool `json:"replan_on_checkpoint_failure,omitempty"`
	// CheckpointOnTaskFailure runs the phase's checkpoint as soon as one of
	// its tasks fails for good.
	CheckpointOnTaskFailure *bool `json:"checkpoint_on_task_failure,omitempty"`
	// CheckpointMinConfidence is the reviewer confidence (0-100) a /pass needs
	// to pass a checkpoint; below it the verdict is inconclusive and the
	// checkpoint fails (policy: checkpoint_verdict_outcome).
	CheckpointMinConfidence *int `json:"checkpoint_min_confidence,omitempty"`
	// AcceptanceRounds is how many failed runs of a campaign's acceptance
	// command block the campaign (policy: campaign_acceptance_exhausted).
	AcceptanceRounds int `json:"acceptance_rounds,omitempty"`

	// --- the commands the campaign runs itself ---

	// VerifyBuildTimeout bounds the build a /verify task runs when its phase
	// wrote code.
	VerifyBuildTimeout string `json:"verify_build_timeout,omitempty"`
	// CheckpointCommandTimeout bounds a checkpoint's build or test command.
	CheckpointCommandTimeout string `json:"checkpoint_command_timeout,omitempty"`
	// TestRunTimeout bounds a /test_run task's test command.
	TestRunTimeout string `json:"test_run_timeout,omitempty"`

	// --- scheduling ---

	// MaxParallelTasks is how many of a phase's tasks run at once.
	MaxParallelTasks int `json:"max_parallel_tasks,omitempty"`
	// HeartbeatEvery is how often progress and the kernel heartbeat refresh.
	HeartbeatEvery string `json:"heartbeat_every,omitempty"`
	// AutosaveEvery is how often the campaign is persisted while it runs.
	AutosaveEvery string `json:"autosave_every,omitempty"`
	// TaskResultCacheLimit is how many task results are kept for the tasks
	// that read them (results a pending task still names are never dropped).
	TaskResultCacheLimit int `json:"task_result_cache_limit,omitempty"`
	// WriteSetLockTimeout bounds the wait for a task's write-set lease before
	// the task goes back to wait its turn.
	WriteSetLockTimeout string `json:"write_set_lock_timeout,omitempty"`
	// WriteSetLockRetry is when a task that timed out on its lease is next
	// eligible.
	WriteSetLockRetry string `json:"write_set_lock_retry,omitempty"`
	// WriteSetLockPoll is how often a waiting lease re-checks.
	WriteSetLockPoll string `json:"write_set_lock_poll,omitempty"`

	// --- what a task's brief and the replanner are given ---

	// UpstreamInlineMaxBytes is the size above which an upstream artifact the
	// policy would inline is sent as its digest instead (its outline and its
	// path): never cut (policy: task_evidence).
	UpstreamInlineMaxBytes int `json:"upstream_inline_max_bytes,omitempty"`
	// VerifyReportMinBytes is the size under which a report a /verify task
	// checks is hollow when its producer was owed upstream evidence (policy:
	// verify_report_hollow).
	VerifyReportMinBytes int `json:"verify_report_min_bytes,omitempty"`
	// PreloadOutlineMaxRows is how many declarations a file or package the
	// brief names may have for its outline to be preloaded into the brief;
	// a larger one is named with its count (policy: task_preload).
	PreloadOutlineMaxRows int `json:"preload_outline_max_rows,omitempty"`
	// PreloadElementMaxLines is how many lines an element the brief names may
	// span for its source to be preloaded; a longer one is preloaded as its
	// outline row (policy: task_preload).
	PreloadElementMaxLines int `json:"preload_element_max_lines,omitempty"`
	// ReplanContextTasks is how many failed or blocked tasks, and triggers,
	// the replanner's context lists before it says how many it left out.
	ReplanContextTasks int `json:"replan_context_tasks,omitempty"`
	// ReplanContextAttempts is how many of a failed task's latest attempts the
	// replanner's context shows.
	ReplanContextAttempts int `json:"replan_context_attempts,omitempty"`
	// ReplanContextTextBytes bounds each description or error the replanner's
	// context quotes.
	ReplanContextTextBytes int `json:"replan_context_text_bytes,omitempty"`
	// ReplanContextBytes bounds the replanner's whole context section.
	ReplanContextBytes int `json:"replan_context_bytes,omitempty"`

	// --- a generated document that is a repetition loop ---
	//
	// Observed live: "1. End. 2. Finish. 3. Complete." about 1500 times, a
	// 19KB document counted done. Go measures a generated document's words;
	// the policy decides with these (generated_output_degenerate).

	// DegenerateMinTokens is how many whitespace-separated tokens a document
	// has before it can be judged a loop at all; a shorter one never is.
	DegenerateMinTokens int `json:"degenerate_min_tokens,omitempty"`
	// DegenerateDistinctPermille: a document with fewer distinct words than
	// this per thousand words is a loop.
	DegenerateDistinctPermille int `json:"degenerate_distinct_permille,omitempty"`
	// DegenerateLongWords and DegenerateLongMinDistinct: a document of more
	// words than DegenerateLongWords with fewer distinct words than
	// DegenerateLongMinDistinct is a loop, even when a prefix that does not
	// repeat lifts its ratio.
	DegenerateLongWords       int `json:"degenerate_long_words,omitempty"`
	DegenerateLongMinDistinct int `json:"degenerate_long_min_distinct,omitempty"`
}

// DefaultCampaignConfig is the campaign section with every field written down.
func DefaultCampaignConfig() CampaignConfig {
	yes, no := true, false
	confidence := 50
	return CampaignConfig{
		MaxTaskAttempts:            4,
		ReproAfterFailures:         2,
		ReplanAtAttemptCap:         &yes,
		RetryBackoffBase:           "5s",
		RetryBackoffMax:            "5m",
		RetryWithReasonBackoffMax:  "30s",
		MaxCheckpointAttempts:      3,
		ReplanOnCheckpointFailure:  &yes,
		CheckpointOnTaskFailure:    &no,
		CheckpointMinConfidence:    &confidence,
		AcceptanceRounds:           3,
		VerifyBuildTimeout:         "5m",
		CheckpointCommandTimeout:   "10m",
		TestRunTimeout:             "15m",
		MaxParallelTasks:           3,
		HeartbeatEvery:             "15s",
		AutosaveEvery:              "1m",
		TaskResultCacheLimit:       100,
		WriteSetLockTimeout:        "15s",
		WriteSetLockRetry:          "500ms",
		WriteSetLockPoll:           "10ms",
		UpstreamInlineMaxBytes:     48 * 1024,
		VerifyReportMinBytes:       1024,
		PreloadOutlineMaxRows:      150,
		PreloadElementMaxLines:     400,
		ReplanContextTasks:         25,
		ReplanContextAttempts:      3,
		ReplanContextTextBytes:     400,
		ReplanContextBytes:         16000,
		DegenerateMinTokens:        200,
		DegenerateDistinctPermille: 30,
		DegenerateLongWords:        400,
		DegenerateLongMinDistinct:  25,
	}
}

// GetCampaignConfig returns the campaign section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetCampaignConfig() CampaignConfig {
	if c == nil || c.Campaign == nil {
		return DefaultCampaignConfig()
	}
	return c.Campaign.WithDefaults()
}

// WithDefaults fills every absent field from DefaultCampaignConfig.
func (c CampaignConfig) WithDefaults() CampaignConfig {
	d := DefaultCampaignConfig()
	intOr := func(v *int, def int) {
		if *v == 0 {
			*v = def
		}
	}
	strOr := func(v *string, def string) {
		if *v == "" {
			*v = def
		}
	}
	intOr(&c.MaxTaskAttempts, d.MaxTaskAttempts)
	intOr(&c.ReproAfterFailures, d.ReproAfterFailures)
	if c.ReplanAtAttemptCap == nil {
		c.ReplanAtAttemptCap = d.ReplanAtAttemptCap
	}
	strOr(&c.RetryBackoffBase, d.RetryBackoffBase)
	strOr(&c.RetryBackoffMax, d.RetryBackoffMax)
	strOr(&c.RetryWithReasonBackoffMax, d.RetryWithReasonBackoffMax)
	intOr(&c.MaxCheckpointAttempts, d.MaxCheckpointAttempts)
	if c.ReplanOnCheckpointFailure == nil {
		c.ReplanOnCheckpointFailure = d.ReplanOnCheckpointFailure
	}
	if c.CheckpointOnTaskFailure == nil {
		c.CheckpointOnTaskFailure = d.CheckpointOnTaskFailure
	}
	if c.CheckpointMinConfidence == nil {
		c.CheckpointMinConfidence = d.CheckpointMinConfidence
	}
	intOr(&c.AcceptanceRounds, d.AcceptanceRounds)
	strOr(&c.VerifyBuildTimeout, d.VerifyBuildTimeout)
	strOr(&c.CheckpointCommandTimeout, d.CheckpointCommandTimeout)
	strOr(&c.TestRunTimeout, d.TestRunTimeout)
	intOr(&c.MaxParallelTasks, d.MaxParallelTasks)
	strOr(&c.HeartbeatEvery, d.HeartbeatEvery)
	strOr(&c.AutosaveEvery, d.AutosaveEvery)
	intOr(&c.TaskResultCacheLimit, d.TaskResultCacheLimit)
	strOr(&c.WriteSetLockTimeout, d.WriteSetLockTimeout)
	strOr(&c.WriteSetLockRetry, d.WriteSetLockRetry)
	strOr(&c.WriteSetLockPoll, d.WriteSetLockPoll)
	intOr(&c.UpstreamInlineMaxBytes, d.UpstreamInlineMaxBytes)
	intOr(&c.VerifyReportMinBytes, d.VerifyReportMinBytes)
	intOr(&c.PreloadOutlineMaxRows, d.PreloadOutlineMaxRows)
	intOr(&c.PreloadElementMaxLines, d.PreloadElementMaxLines)
	intOr(&c.ReplanContextTasks, d.ReplanContextTasks)
	intOr(&c.ReplanContextAttempts, d.ReplanContextAttempts)
	intOr(&c.ReplanContextTextBytes, d.ReplanContextTextBytes)
	intOr(&c.ReplanContextBytes, d.ReplanContextBytes)
	intOr(&c.DegenerateMinTokens, d.DegenerateMinTokens)
	intOr(&c.DegenerateDistinctPermille, d.DegenerateDistinctPermille)
	intOr(&c.DegenerateLongWords, d.DegenerateLongWords)
	intOr(&c.DegenerateLongMinDistinct, d.DegenerateLongMinDistinct)
	return c
}

// CampaignPolicy is a CampaignConfig resolved for the orchestrator: defaults
// filled, durations parsed, switches read. Resolve is the only way to make one,
// so a policy in hand has passed Check.
type CampaignPolicy struct {
	MaxTaskAttempts            int
	ReproAfterFailures         int
	ReplanAtAttemptCap         bool
	RetryBackoffBase           time.Duration
	RetryBackoffMax            time.Duration
	RetryWithReasonBackoffMax  time.Duration
	MaxCheckpointAttempts      int
	ReplanOnCheckpointFailure  bool
	CheckpointOnTaskFailure    bool
	CheckpointMinConfidence    int
	AcceptanceRounds           int
	VerifyBuildTimeout         time.Duration
	CheckpointCommandTimeout   time.Duration
	TestRunTimeout             time.Duration
	MaxParallelTasks           int
	HeartbeatEvery             time.Duration
	AutosaveEvery              time.Duration
	TaskResultCacheLimit       int
	WriteSetLockTimeout        time.Duration
	WriteSetLockRetry          time.Duration
	WriteSetLockPoll           time.Duration
	UpstreamInlineMaxBytes     int
	VerifyReportMinBytes       int
	PreloadOutlineMaxRows      int
	PreloadElementMaxLines     int
	ReplanContextTasks         int
	ReplanContextAttempts      int
	ReplanContextTextBytes     int
	ReplanContextBytes         int
	DegenerateMinTokens        int
	DegenerateDistinctPermille int
	DegenerateLongWords        int
	DegenerateLongMinDistinct  int

	// Section is the resolved config the policy came from, for Params.
	Section CampaignConfig
}

// Resolve defaults, checks and parses the section. Every problem Check finds is
// an error here: a campaign does not start on a policy with a contradiction in
// it.
func (c CampaignConfig) Resolve() (CampaignPolicy, error) {
	c = c.WithDefaults()
	if problems := c.Check("campaign"); len(problems) > 0 {
		msg := fmt.Sprintf("the campaign config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return CampaignPolicy{}, fmt.Errorf("%s", msg)
	}
	d := func(s string) time.Duration {
		v, _ := time.ParseDuration(s) // Check parsed every one of these
		return v
	}
	return CampaignPolicy{
		MaxTaskAttempts:            c.MaxTaskAttempts,
		ReproAfterFailures:         c.ReproAfterFailures,
		ReplanAtAttemptCap:         *c.ReplanAtAttemptCap,
		RetryBackoffBase:           d(c.RetryBackoffBase),
		RetryBackoffMax:            d(c.RetryBackoffMax),
		RetryWithReasonBackoffMax:  d(c.RetryWithReasonBackoffMax),
		MaxCheckpointAttempts:      c.MaxCheckpointAttempts,
		ReplanOnCheckpointFailure:  *c.ReplanOnCheckpointFailure,
		CheckpointOnTaskFailure:    *c.CheckpointOnTaskFailure,
		CheckpointMinConfidence:    *c.CheckpointMinConfidence,
		AcceptanceRounds:           c.AcceptanceRounds,
		VerifyBuildTimeout:         d(c.VerifyBuildTimeout),
		CheckpointCommandTimeout:   d(c.CheckpointCommandTimeout),
		TestRunTimeout:             d(c.TestRunTimeout),
		MaxParallelTasks:           c.MaxParallelTasks,
		HeartbeatEvery:             d(c.HeartbeatEvery),
		AutosaveEvery:              d(c.AutosaveEvery),
		TaskResultCacheLimit:       c.TaskResultCacheLimit,
		WriteSetLockTimeout:        d(c.WriteSetLockTimeout),
		WriteSetLockRetry:          d(c.WriteSetLockRetry),
		WriteSetLockPoll:           d(c.WriteSetLockPoll),
		UpstreamInlineMaxBytes:     c.UpstreamInlineMaxBytes,
		VerifyReportMinBytes:       c.VerifyReportMinBytes,
		PreloadOutlineMaxRows:      c.PreloadOutlineMaxRows,
		PreloadElementMaxLines:     c.PreloadElementMaxLines,
		ReplanContextTasks:         c.ReplanContextTasks,
		ReplanContextAttempts:      c.ReplanContextAttempts,
		ReplanContextTextBytes:     c.ReplanContextTextBytes,
		ReplanContextBytes:         c.ReplanContextBytes,
		DegenerateMinTokens:        c.DegenerateMinTokens,
		DegenerateDistinctPermille: c.DegenerateDistinctPermille,
		DegenerateLongWords:        c.DegenerateLongWords,
		DegenerateLongMinDistinct:  c.DegenerateLongMinDistinct,
		Section:                    c,
	}, nil
}

// Check reports the contradictions in a campaign section, addressed under
// prefix ("campaign"). Absent fields are defaulted first, so only what the
// file says can be wrong. Every finding is an error: none of these has a
// reading a campaign could run on.
func (c CampaignConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	for _, f := range []struct {
		name string
		v    int
	}{
		{"max_task_attempts", c.MaxTaskAttempts},
		{"repro_after_failures", c.ReproAfterFailures},
		{"max_checkpoint_attempts", c.MaxCheckpointAttempts},
		{"acceptance_rounds", c.AcceptanceRounds},
		{"max_parallel_tasks", c.MaxParallelTasks},
		{"task_result_cache_limit", c.TaskResultCacheLimit},
		{"upstream_inline_max_bytes", c.UpstreamInlineMaxBytes},
		{"verify_report_min_bytes", c.VerifyReportMinBytes},
		{"preload_outline_max_rows", c.PreloadOutlineMaxRows},
		{"preload_element_max_lines", c.PreloadElementMaxLines},
		{"replan_context_tasks", c.ReplanContextTasks},
		{"replan_context_attempts", c.ReplanContextAttempts},
		{"replan_context_text_bytes", c.ReplanContextTextBytes},
		{"replan_context_bytes", c.ReplanContextBytes},
		{"degenerate_min_tokens", c.DegenerateMinTokens},
		{"degenerate_distinct_permille", c.DegenerateDistinctPermille},
		{"degenerate_long_words", c.DegenerateLongWords},
		{"degenerate_long_min_distinct", c.DegenerateLongMinDistinct},
	} {
		if f.v < 1 {
			add(f.name, fmt.Sprintf("%d is below 1", f.v), "a count of at least 1, or remove the key for the default")
		}
	}
	if v := c.DegenerateDistinctPermille; v > 1000 {
		add("degenerate_distinct_permille", fmt.Sprintf("%d is above 1000: every document would be a loop", v), "distinct words per thousand, 1-1000")
	}
	if v := *c.CheckpointMinConfidence; v < 0 || v > 100 {
		add("checkpoint_min_confidence", fmt.Sprintf("%d is outside 0-100", v), "an integer percent")
	}
	durations := map[string]time.Duration{}
	for _, f := range []struct {
		name string
		v    string
	}{
		{"retry_backoff_base", c.RetryBackoffBase},
		{"retry_backoff_max", c.RetryBackoffMax},
		{"retry_with_reason_backoff_max", c.RetryWithReasonBackoffMax},
		{"verify_build_timeout", c.VerifyBuildTimeout},
		{"checkpoint_command_timeout", c.CheckpointCommandTimeout},
		{"test_run_timeout", c.TestRunTimeout},
		{"heartbeat_every", c.HeartbeatEvery},
		{"autosave_every", c.AutosaveEvery},
		{"write_set_lock_timeout", c.WriteSetLockTimeout},
		{"write_set_lock_retry", c.WriteSetLockRetry},
		{"write_set_lock_poll", c.WriteSetLockPoll},
	} {
		d, err := time.ParseDuration(f.v)
		switch {
		case err != nil:
			add(f.name, fmt.Sprintf("%q is not a duration: %v", f.v, err), `a Go duration such as "30s", "5m" or "1500ms"`)
		case d <= 0:
			add(f.name, fmt.Sprintf("%q is not positive", f.v), "a positive duration, or remove the key for the default")
		default:
			durations[f.name] = d
		}
	}
	base, okBase := durations["retry_backoff_base"]
	for _, ceiling := range []string{"retry_backoff_max", "retry_with_reason_backoff_max"} {
		if c, ok := durations[ceiling]; ok && okBase && c < base {
			add(ceiling, fmt.Sprintf("%s is below retry_backoff_base (%s): no retry could wait the base", c, base), "a ceiling at least retry_backoff_base")
		}
	}
	return out
}

// Params are the campaign knobs the kernel's rules read, as config_param
// rows. Each key is declared config_param_required(/campaign, Key) next to
// the rules that read it; the orchestrator refuses to run while any of them
// is missing (config_param_missing), because a rule over an absent threshold
// derives nothing and fails open.
func (p CampaignPolicy) Params() []Param {
	flag := func(b bool) int64 {
		if b {
			return 1
		}
		return 0
	}
	return []Param{
		{Key: "/campaign_max_task_attempts", Value: int64(p.MaxTaskAttempts)},
		{Key: "/campaign_repro_after_failures", Value: int64(p.ReproAfterFailures)},
		{Key: "/campaign_replan_at_attempt_cap", Value: flag(p.ReplanAtAttemptCap)},
		{Key: "/campaign_max_checkpoint_attempts", Value: int64(p.MaxCheckpointAttempts)},
		{Key: "/campaign_replan_on_checkpoint_failure", Value: flag(p.ReplanOnCheckpointFailure)},
		{Key: "/campaign_checkpoint_min_confidence", Value: int64(p.CheckpointMinConfidence)},
		{Key: "/campaign_acceptance_rounds", Value: int64(p.AcceptanceRounds)},
		{Key: "/campaign_upstream_inline_max_bytes", Value: int64(p.UpstreamInlineMaxBytes)},
		{Key: "/campaign_verify_report_min_bytes", Value: int64(p.VerifyReportMinBytes)},
		{Key: "/campaign_preload_outline_max_rows", Value: int64(p.PreloadOutlineMaxRows)},
		{Key: "/campaign_preload_element_max_lines", Value: int64(p.PreloadElementMaxLines)},
		{Key: "/campaign_degenerate_min_tokens", Value: int64(p.DegenerateMinTokens)},
		{Key: "/campaign_degenerate_distinct_permille", Value: int64(p.DegenerateDistinctPermille)},
		{Key: "/campaign_degenerate_long_words", Value: int64(p.DegenerateLongWords)},
		{Key: "/campaign_degenerate_long_min_distinct", Value: int64(p.DegenerateLongMinDistinct)},
	}
}
