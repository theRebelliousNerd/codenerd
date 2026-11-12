package turnjudge

// verdictCases are real-shaped diagnosis reports with the answer a person
// would give, per target: does the report name a fix in codeNERD's own
// source? The live probe (verdict_live_test.go, -tags jevlive) sends them to
// Jev; the offline tests check the requests they become.
//
// Measured against jev-latest on 2026-11-12 (probability the report names a
// fix; the kernel reads >= 0.70 yes, <= 0.30 no), 20 judgments, 0 wrong, 0
// unsettled:
//
//	live-no-defect 0.01; live-no-defect-targets 0.01 0.01; provider-outage
//	0.01; build-config 0.02; user-config-and-network 0.03 0.02; nil-deref
//	0.99; missing-decl 0.98; mixed 0.04 0.85; inconclusive 0.02;
//	signature-only 0.98; speed-outside 0.01; speed-model 0.01; speed-fix 0.93
//	held out: held-long-correct 0.02; held-user-project-bug 0.01;
//	held-swallowed-error 0.98; held-speed-alloc 0.96
//
// The live sentence that started this (a correct "nothing to change" verdict
// that still ran a fix turn) scores 0.01, alone and per target.

type verdictCase struct {
	name    string
	lane    DiagnosisLane
	report  string
	targets []DiagnosisTarget
	fix     []bool // per target; one entry for a job-level question
}

var verdictCases = []verdictCase{
	{"live-no-defect", DiagnosisDefect,
		"Both findings name no defect in codeNERD's own source — handling was correct — so I changed nothing.",
		nil, []bool{false}},
	{"live-no-defect-targets", DiagnosisDefect,
		"Both findings name no defect in codeNERD's own source — handling was correct — so I changed nothing.",
		[]DiagnosisTarget{
			{Key: "jev triage failed, the log's order stands: <s>", Level: "WARN", Category: "kernel", Message: "jev triage failed, the log's order stands: Post https://api.typesafe.ai/v1/ask: context deadline exceeded"},
			{Key: "perception error: <s>", Level: "ERROR", Category: "perception", Message: "perception error: classification model returned 503 Service Unavailable"},
		},
		[]bool{false, false}},
	{"provider-outage", DiagnosisDefect,
		"1. ERROR [api] x3: provider returned 529 overloaded. Raised at internal/perception/client.go:212 when the provider answers 5xx; the retry loop backs off and gives up after 3 attempts, which is the intended behavior. Root cause: the provider was overloaded. Nothing in codeNERD to change.",
		[]DiagnosisTarget{{Key: "provider returned <n> overloaded", Level: "ERROR", Category: "api", Message: "anthropic: provider returned 529 overloaded after 3 attempts"}},
		[]bool{false}},
	{"build-config", DiagnosisDefect,
		"The warning `sqlite-vec not available; falling back from ANN to lexical search` is logged by internal/store/vec_support_disabled.go:14 because this binary was built without `-tags sqlite_vec`. That is how the binary was built, not a defect: rebuild with the tag. The fallback itself works as designed.",
		[]DiagnosisTarget{{Key: "sqlite-vec not available; falling back from ANN to lexical search", Level: "WARN", Category: "store", Message: "sqlite-vec not available; falling back from ANN to lexical search"}},
		[]bool{false}},
	{"user-config-and-network", DiagnosisDefect,
		"Problem 1 came from a malformed .nerd/config.json the user edited (a missing comma at line 41); codeNERD reported it correctly at internal/config/load.go:88 and kept the previous config. Problem 2 is timeouts reaching api.typesafe.ai: a network failure, and the judgment fell back as designed. Neither needs a change.",
		[]DiagnosisTarget{
			{Key: "config parse error: invalid character", Level: "ERROR", Category: "config", Message: "config parse error: invalid character '\"' after object key:value pair at line 41"},
			{Key: "jev call failed: context deadline exceeded", Level: "WARN", Category: "kernel", Message: "jev triage failed, the log's order stands: Post https://api.typesafe.ai/v1/ask: context deadline exceeded"},
		},
		[]bool{false, false}},
	{"nil-deref", DiagnosisDefect,
		"Problem 1 is a nil pointer dereference in internal/session/executor_tools.go:2291: `res.Meta` is read before `res` is checked when a tool returns an error, so every failing tool call panics the turn. Fix: return on err before reading res, and add a test with a tool that errors.",
		[]DiagnosisTarget{{Key: "PANIC in processInput (recovered): runtime error: invalid memory address or nil pointer dereference", Level: "ERROR", Category: "api", Message: "PANIC in processInput (recovered): runtime error: invalid memory address or nil pointer dereference"}},
		[]bool{true}},
	{"missing-decl", DiagnosisDefect,
		"The kernel logs `undeclared predicate self_problem_triage` at every boot because policy/self_repair.mg reads self_problem_around before jev_evidence.mg is loaded in the stratum order at internal/core/kernel_init.go:301. Fix: load jev_evidence.mg with the schemas, or move the Decl into schemas_shards.mg.",
		[]DiagnosisTarget{{Key: "undeclared predicate self_problem_triage", Level: "ERROR", Category: "kernel", Message: "undeclared predicate self_problem_triage in rule self_repair_held"}},
		[]bool{true}},
	{"mixed", DiagnosisDefect,
		"Problem 1 is a provider rate limit (429) — a failure around codeNERD, handled correctly by the backoff. Problem 2 is a real bug: the retry counter in internal/perception/retry.go:57 is never reset after a success, so after one failure every later call skips its retries. Fix: reset the counter on success, with a test.",
		[]DiagnosisTarget{
			{Key: "rate limited (<n>)", Level: "WARN", Category: "api", Message: "rate limited (429): retrying in 4s"},
			{Key: "retry budget exhausted before first attempt", Level: "ERROR", Category: "perception", Message: "retry budget exhausted before first attempt"},
		},
		[]bool{false, true}},
	{"inconclusive", DiagnosisDefect,
		"I could not find where `stale lease dropped` is raised in codeNERD's source; it may come from a dependency. I would need more of the log to say what causes it.",
		[]DiagnosisTarget{{Key: "stale lease dropped", Level: "WARN", Category: "store", Message: "stale lease dropped"}},
		[]bool{false}},
	{"signature-only", DiagnosisDefect,
		"The error is raised at internal/world/scan.go:412: the scanner compares mtimes with == after a round-trip through seconds, so every file counts as changed and the whole tree is re-parsed. Fix: compare at the stored precision.",
		[]DiagnosisTarget{{Key: "world scan re-parsed <n> unchanged files"}},
		[]bool{true}},
	{"speed-outside", DiagnosisSpeed,
		"world.scan spends 94% of its time in os.ReadDir and file reads on a cold disk (internal/world/scan.go:80-140); the parsing after the read is under 3%. The time is waiting on the disk; no change in codeNERD's code would cut it meaningfully.",
		[]DiagnosisTarget{{Key: "timing:world.scan"}}, []bool{false}},
	{"speed-model", DiagnosisSpeed,
		"perception.classify waits on the classification model for 1.8s of its 1.9s mean; codeNERD's own work around the call is under 100ms. Nothing in codeNERD's code would make it faster.",
		[]DiagnosisTarget{{Key: "timing:perception.classify"}}, []bool{false}},
	{"speed-fix", DiagnosisSpeed,
		"kernel.evaluate is called 412 times in the run; each call rebuilds the whole program from source at internal/core/kernel_eval.go:140 though only facts changed between calls. Caching the compiled program across calls and recompiling only when a rule changes would cut most of the 380ms mean; behavior is unchanged because the rules are the same.",
		[]DiagnosisTarget{{Key: "timing:kernel.evaluate"}}, []bool{true}},
	// Held out: written after the wording was fixed, never tuned on.
	{"held-long-correct", DiagnosisDefect,
		"I traced `tool run_build failed: exit status 1` to internal/tools/shell/build.go:144, which runs `go build` and reports a non-zero exit as a failure, then to internal/session/executor_tools.go:1880, which hands the compiler output back to the model. The build failed because the user's package had a syntax error in main.go:12 that the user introduced; codeNERD reported it faithfully and the model fixed it on the next call. The code path is correct.",
		[]DiagnosisTarget{{Key: "tool run_build failed: exit status <n>", Level: "ERROR", Category: "tools", Message: "tool run_build failed: exit status 1"}},
		[]bool{false}},
	{"held-user-project-bug", DiagnosisDefect,
		"The failing test is TestParseDate in the user's own project (pkg/dates/parse_test.go:30); codeNERD's tester ran it and reported the failure correctly. The defect is in the user's code, not codeNERD's.",
		[]DiagnosisTarget{{Key: "test failed: TestParseDate", Level: "WARN", Category: "tester", Message: "test failed: TestParseDate (pkg/dates/parse_test.go:30)"}},
		[]bool{false}},
	{"held-swallowed-error", DiagnosisDefect,
		"The warning repeats because internal/store/session_store.go:233 ignores the error from tx.Commit and logs `saved session` anyway; when the disk is full the session silently fails to save and the next save warns that the row is missing. Fix: return the Commit error to the caller and stop logging success before it.",
		[]DiagnosisTarget{{Key: "session row missing on save: <s>", Level: "WARN", Category: "store", Message: "session row missing on save: session 42 was never committed"}},
		[]bool{true}},
	{"held-speed-alloc", DiagnosisSpeed,
		"prompt.compile re-reads and re-parses every atom YAML file from disk on each call (internal/prompt/loader.go:61), though the files do not change during a run; 70% of its 220ms mean is that parsing. Load the atoms once and reuse them; behavior is the same because the files are the same.",
		[]DiagnosisTarget{{Key: "timing:prompt.compile"}}, []bool{true}},
}
