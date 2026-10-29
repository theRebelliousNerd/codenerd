package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// TypeSafeConfig is the `typesafe` section of .nerd/config.json: the Jev
// (System One) API that answers typed questions as extra evidence.
//
// The section is the switch. Absent means Jev is off; present means every
// field it needs must be written. No model or endpoint is defaulted in code,
// so the file names exactly what answers.
type TypeSafeConfig struct {
	// Model is an alias (it moves when a release ships) or a versioned ID.
	// A tuned threshold belongs to one version, so pin the ID when you have
	// tuned against it.
	Model string `json:"model"`
	// BaseURL is the API origin, without the /v1 path.
	BaseURL string `json:"base_url"`
	// APIKey / APIKeyEnv: exactly one. The environment name keeps the secret
	// out of the file.
	APIKey    string `json:"api_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	// RequestTimeout bounds one call, retries included. Default "30s".
	RequestTimeout string `json:"request_timeout,omitempty"`
	// TurnJudgmentTimeout bounds a chat turn's judgments, retries included.
	// They run beside perception and routing waits for them, so this, not
	// RequestTimeout, is the longest Jev can hold a turn past perception.
	// Default "5s"; a call still out at the deadline is no evidence.
	TurnJudgmentTimeout string `json:"turn_judgment_timeout,omitempty"`
	// LongContext sends the long-context header (262k tokens instead of 64k).
	// Slower and less accurate; turn it on only for state that needs it.
	LongContext bool `json:"long_context,omitempty"`
	// JudgmentYesPercent and JudgmentNoPercent are the kernel's reading of a
	// Jev yes/no probability (policy/jev_evidence.mg): at or above yes reads
	// yes, at or below no reads no, between is unsettled and acts on nothing.
	// Defaults 70 and 30, TypeSafe's own starting thresholds.
	JudgmentYesPercent int `json:"judgment_yes_percent,omitempty"`
	JudgmentNoPercent  int `json:"judgment_no_percent,omitempty"`

	// Where Jev is used; each is on when the section exists and the key is
	// absent (Docs/architecture/jev-turn-judgments.md).
	TurnJudgments      *bool `json:"turn_judgments,omitempty"`
	RetrievalRelevance *bool `json:"retrieval_relevance,omitempty"`
	ReviewChecks       *bool `json:"review_checks,omitempty"`
	DiagnosisTriage    *bool `json:"diagnosis_triage,omitempty"`
	// The learning and self-modification loops (Docs/architecture/
	// jev-turn-judgments.md, "Learning and autopoiesis"): whether the user is
	// correcting codeNERD's last turn, the prompt-evolution task verdict and
	// problem type, ouroboros duplicate and code checks, the autopoiesis
	// campaign and persistent-agent checks, and evolved-atom promotion checks.
	CorrectionJudgments *bool `json:"correction_judgments,omitempty"`
	TaskVerdicts        *bool `json:"task_verdicts,omitempty"`
	ProblemTypes        *bool `json:"problem_types,omitempty"`
	ToolChecks          *bool `json:"tool_checks,omitempty"`
	AutopoiesisChecks   *bool `json:"autopoiesis_checks,omitempty"`
	AtomChecks          *bool `json:"atom_checks,omitempty"`
	// FinalReports: Jev reads a turn's final report for unfinished work and
	// a blocker; the /finish round pushes unfinished work back to the model
	// (session/finish_round.go). Off, the closure asks the main model.
	FinalReports *bool `json:"final_reports,omitempty"`
	// AtomRelevance: Jev judges, per turn, whether each pooled flesh prompt
	// atom applies to the request (internal/atomjudge, jit_compiler.mg).
	AtomRelevance *bool `json:"atom_relevance,omitempty"`
	// InstructionCoverage: for a compound `nerd run` instruction, Jev judges
	// which split fragments the person asked for and whether the result
	// delivers each (internal/turnjudge/coverage.go).
	InstructionCoverage *bool `json:"instruction_coverage,omitempty"`
	// AtomRelevanceTimeout bounds one prompt compile's judgment, retries
	// included. Default "4s"; past it the compile uses the vector channel
	// alone.
	AtomRelevanceTimeout string `json:"atom_relevance_timeout,omitempty"`

	// Tuning; zero means the default (TypeSafeUses).
	RecentTurns         int `json:"recent_turns,omitempty"`
	BatchSize           int `json:"batch_size,omitempty"`
	RelevancePoolFactor int `json:"relevance_pool_factor,omitempty"`
	RelevanceHeadBytes  int `json:"relevance_head_bytes,omitempty"`
	FindingContextLines int `json:"finding_context_lines,omitempty"`
	AtomRelevancePool   int `json:"atom_relevance_pool,omitempty"`
}

// TypeSafeUses is the typesafe section resolved: which uses are on, and the
// tuning each reads. The defaults are sized to Jev's context (64k tokens per
// request, docs.typesafe.ai/models).
type TypeSafeUses struct {
	TurnJudgments, RetrievalRelevance, ReviewChecks, DiagnosisTriage bool
	// The learning and autopoiesis uses (TypeSafeConfig).
	CorrectionJudgments, TaskVerdicts, ProblemTypes, ToolChecks, AutopoiesisChecks, AtomChecks bool
	// AtomRelevance is the per-turn flesh-atom judgment (atom_relevance).
	AtomRelevance bool
	// InstructionCoverage is the compound-instruction coverage judgment.
	InstructionCoverage bool
	// FinalReports is the final-report reading and the /finish round.
	FinalReports bool
	// TurnJudgmentTimeout bounds one turn's judgments (turn_judgment_timeout).
	TurnJudgmentTimeout time.Duration
	// RecentTurns bounds the prior turns a turn judgment sees.
	RecentTurns int
	// BatchSize bounds the questions in one request; batches run in parallel.
	BatchSize int
	// RelevancePoolFactor widens the keyword pool Jev judges beyond what
	// retrieval's Tier 2 keeps.
	RelevancePoolFactor int
	// RelevanceHeadBytes bounds each file's head in a relevance question.
	RelevanceHeadBytes int
	// FindingContextLines is how many lines either side of a review
	// finding's citation Jev sees.
	FindingContextLines int
	// AtomRelevancePool bounds the flesh atoms judged per compile: the
	// vector channel's slots plus the next most similar eligible atoms.
	AtomRelevancePool int
	// AtomRelevanceTimeout bounds one compile's judgment.
	AtomRelevanceTimeout time.Duration
}

// DefaultTypeSafeUses is every use on with the default tuning.
func DefaultTypeSafeUses() TypeSafeUses {
	return TypeSafeUses{
		TurnJudgments: true, RetrievalRelevance: true, ReviewChecks: true, DiagnosisTriage: true,
		CorrectionJudgments: true, TaskVerdicts: true, ProblemTypes: true, ToolChecks: true, AutopoiesisChecks: true, AtomChecks: true,
		AtomRelevance:       true,
		InstructionCoverage: true,
		FinalReports:        true,
		TurnJudgmentTimeout: defaultTurnJudgmentTimeout,
		RecentTurns:         12, BatchSize: 20, RelevancePoolFactor: 6, RelevanceHeadBytes: 3000, FindingContextLines: 12,
		AtomRelevancePool: 40, AtomRelevanceTimeout: defaultAtomRelevanceTimeout,
	}
}

// Uses resolves the section. A nil section is every use off: Jev is not
// configured.
func (c *TypeSafeConfig) Uses() TypeSafeUses {
	if c == nil {
		return TypeSafeUses{}
	}
	u := DefaultTypeSafeUses()
	on := func(b *bool, def bool) bool {
		if b == nil {
			return def
		}
		return *b
	}
	u.TurnJudgments = on(c.TurnJudgments, u.TurnJudgments)
	u.RetrievalRelevance = on(c.RetrievalRelevance, u.RetrievalRelevance)
	u.ReviewChecks = on(c.ReviewChecks, u.ReviewChecks)
	u.DiagnosisTriage = on(c.DiagnosisTriage, u.DiagnosisTriage)
	u.CorrectionJudgments = on(c.CorrectionJudgments, u.CorrectionJudgments)
	u.TaskVerdicts = on(c.TaskVerdicts, u.TaskVerdicts)
	u.ProblemTypes = on(c.ProblemTypes, u.ProblemTypes)
	u.ToolChecks = on(c.ToolChecks, u.ToolChecks)
	u.AutopoiesisChecks = on(c.AutopoiesisChecks, u.AutopoiesisChecks)
	u.AtomChecks = on(c.AtomChecks, u.AtomChecks)
	u.AtomRelevance = on(c.AtomRelevance, u.AtomRelevance)
	u.InstructionCoverage = on(c.InstructionCoverage, u.InstructionCoverage)
	u.FinalReports = on(c.FinalReports, u.FinalReports)
	set := func(v int, dst *int) {
		if v > 0 {
			*dst = v
		}
	}
	set(c.RecentTurns, &u.RecentTurns)
	set(c.BatchSize, &u.BatchSize)
	set(c.RelevancePoolFactor, &u.RelevancePoolFactor)
	set(c.RelevanceHeadBytes, &u.RelevanceHeadBytes)
	set(c.FindingContextLines, &u.FindingContextLines)
	set(c.AtomRelevancePool, &u.AtomRelevancePool)
	// Check refuses a timeout that does not parse, and Resolve with it, so a
	// running client never reaches here with one.
	if d, err := parseTypeSafeDuration(c.TurnJudgmentTimeout, defaultTurnJudgmentTimeout); err == nil {
		u.TurnJudgmentTimeout = d
	}
	if d, err := parseTypeSafeDuration(c.AtomRelevanceTimeout, defaultAtomRelevanceTimeout); err == nil {
		u.AtomRelevanceTimeout = d
	}
	return u
}

// Default Jev thresholds, as percents.
const (
	DefaultJudgmentYesPercent = 70
	DefaultJudgmentNoPercent  = 30
)

// thresholds are the yes and no percents, defaults applied. Nil-safe.
func (c *TypeSafeConfig) thresholds() (yes, no int) {
	yes, no = DefaultJudgmentYesPercent, DefaultJudgmentNoPercent
	if c != nil && c.JudgmentYesPercent > 0 {
		yes = c.JudgmentYesPercent
	}
	if c != nil && c.JudgmentNoPercent > 0 {
		no = c.JudgmentNoPercent
	}
	return yes, no
}

// Thresholds are the yes and no percents with defaults applied, for a Jev
// consumer that asserts its own band rows beside its judgments (atom
// relevance: atom_relevance_threshold). Nil-safe.
func (c *TypeSafeConfig) Thresholds() (yes, no int) {
	return c.thresholds()
}

// NoProbability is judgment_no_percent as a probability, for the one Jev
// consumer that decides before the kernel sees its evidence: retrieval's
// Tier 2 drop, which runs inside the tier build. Every other reading of a
// judgment is a rule over config_param (Params).
func (c *TypeSafeConfig) NoProbability() float64 {
	_, no := c.thresholds()
	return float64(no) / 100
}

// Params are the thresholds the kernel's Jev rules read, as config_param rows
// (config_param_required(/jev, Key), policy/jev_evidence.mg). Nil-safe: the
// rules only ever see evidence when a section exists, but the thresholds are
// asserted whole either way.
func (c *TypeSafeConfig) Params() []Param {
	yes, no := c.thresholds()
	return []Param{
		{Key: "/jev_yes_percent", Value: int64(yes)},
		{Key: "/jev_no_percent", Value: int64(no)},
	}
}

// OffWhenAbsent marks the section optional as a whole (see OptionalSection).
func (TypeSafeConfig) OffWhenAbsent() {}

const (
	defaultTypeSafeTimeout     = "30s"
	defaultTurnJudgmentTimeout = 5 * time.Second
	// defaultAtomRelevanceTimeout bounds one compile's atom judgment. A
	// 40-atom pool is about 30k input tokens in two parallel requests.
	defaultAtomRelevanceTimeout = 4 * time.Second
)

// ResolvedTypeSafe is runtime-only; never marshal it into diagnostics, it
// carries the key.
type ResolvedTypeSafe struct {
	Model       string
	BaseURL     string
	APIKey      string
	Timeout     time.Duration
	LongContext bool
}

// Check reports what a present typesafe section cannot run with, addressed
// under prefix ("typesafe"). It never reads the environment: whether the
// named variable is set is Resolve's question, so a config can be checked on
// a machine that lacks the key.
func (c TypeSafeConfig) Check(prefix string) []Problem {
	var out []Problem
	add := func(field, msg, fix string) {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + "." + field, Message: msg, Fix: fix})
	}
	if strings.TrimSpace(c.Model) == "" {
		add("model", "is required; no model is defaulted", `a model alias or versioned ID from GET /v1/models, such as "jev-latest"`)
	}
	switch u, err := url.Parse(strings.TrimSpace(c.BaseURL)); {
	case strings.TrimSpace(c.BaseURL) == "":
		add("base_url", "is required; no endpoint is defaulted", "the API origin, such as https://api.typesafe.ai")
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "":
		add("base_url", fmt.Sprintf("%q is not an http(s) URL with a host", c.BaseURL), "the API origin, such as https://api.typesafe.ai")
	case u.RawQuery != "" || u.Fragment != "" || u.User != nil:
		add("base_url", fmt.Sprintf("%q carries a query, fragment or credentials", c.BaseURL), "the origin only; the key goes in api_key or api_key_env")
	}
	if (c.APIKey == "") == (c.APIKeyEnv == "") {
		add("api_key", "exactly one of api_key and api_key_env is required", "api_key_env names the variable that holds the key")
	}
	if c.APIKeyEnv != "" && !credentialEnvPattern.MatchString(c.APIKeyEnv) {
		add("api_key_env", fmt.Sprintf("%q is not an environment variable name", c.APIKeyEnv), "letters, digits and underscores, not starting with a digit")
	}
	if c.APIKey != "" && (strings.TrimSpace(c.APIKey) != c.APIKey || strings.ContainsAny(c.APIKey, "\r\n\x00")) {
		add("api_key", "has surrounding whitespace or control characters", "the key alone")
	}
	yes, no := c.JudgmentYesPercent, c.JudgmentNoPercent
	if yes == 0 {
		yes = DefaultJudgmentYesPercent
	}
	if no == 0 {
		no = DefaultJudgmentNoPercent
	}
	if yes < 0 || yes > 100 || no < 0 || no > 100 || no >= yes {
		add("judgment_yes_percent", fmt.Sprintf("yes %d and no %d must be percents with no below yes", yes, no), "for example 70 and 30, or remove both for the defaults")
	}
	for field, v := range map[string]int{
		"recent_turns": c.RecentTurns, "batch_size": c.BatchSize, "relevance_pool_factor": c.RelevancePoolFactor,
		"relevance_head_bytes": c.RelevanceHeadBytes, "finding_context_lines": c.FindingContextLines,
		"atom_relevance_pool": c.AtomRelevancePool,
	} {
		if v < 0 {
			add(field, fmt.Sprintf("%d is negative", v), "a positive number, or remove the key for the default")
		}
	}
	if _, err := c.timeout(); err != nil {
		add("request_timeout", err.Error(), `a positive Go duration such as "30s", or remove the key for the default`)
	}
	if _, err := parseTypeSafeDuration(c.TurnJudgmentTimeout, defaultTurnJudgmentTimeout); err != nil {
		add("turn_judgment_timeout", err.Error(), `a positive Go duration such as "5s", or remove the key for the default`)
	}
	if _, err := parseTypeSafeDuration(c.AtomRelevanceTimeout, defaultAtomRelevanceTimeout); err != nil {
		add("atom_relevance_timeout", err.Error(), `a positive Go duration such as "4s", or remove the key for the default`)
	}
	return out
}

func (c TypeSafeConfig) timeout() (time.Duration, error) {
	raw := c.RequestTimeout
	if raw == "" {
		raw = defaultTypeSafeTimeout
	}
	return parseTypeSafeDuration(raw, 0)
}

// parseTypeSafeDuration reads a positive Go duration; "" is def.
func parseTypeSafeDuration(raw string, def time.Duration) (time.Duration, error) {
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration: %v", raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%q is not positive", raw)
	}
	return d, nil
}

// Resolve checks the section and reads the key. A nil section is Jev off:
// (nil, nil). A present section that cannot run is an error, and a named
// environment variable that is empty is one too; nothing falls back to
// another key.
func (c *TypeSafeConfig) Resolve() (*ResolvedTypeSafe, error) {
	if c == nil {
		return nil, nil
	}
	if problems := c.Check("typesafe"); len(problems) > 0 {
		msg := fmt.Sprintf("the typesafe config has %d error(s):", len(problems))
		for _, p := range problems {
			msg += "\n  " + p.String()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	key := c.APIKey
	if c.APIKeyEnv != "" {
		key = os.Getenv(c.APIKeyEnv)
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("typesafe.api_key_env: %s is not set; set it or use api_key", c.APIKeyEnv)
		}
		if strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n\x00") {
			return nil, fmt.Errorf("typesafe.api_key_env: the key in %s has invalid whitespace or control characters", c.APIKeyEnv)
		}
	}
	timeout, _ := c.timeout() // Check parsed it
	return &ResolvedTypeSafe{
		Model:       strings.TrimSpace(c.Model),
		BaseURL:     strings.TrimRight(strings.TrimSpace(c.BaseURL), "/"),
		APIKey:      key,
		Timeout:     timeout,
		LongContext: c.LongContext,
	}, nil
}
