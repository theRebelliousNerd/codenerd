// Package broker is the single admission and accounting boundary for every
// inference call codeNERD makes.
//
// # Why this exists
//
// Before this package there were 145 LLM call sites across 76 files and at
// least three independent implementations of "how many tokens is this". The
// context compressor budgeted the full configured window; the JIT prompt
// compiler budgeted the full configured window; neither subtracted the other.
// The session executor ignored both and used a character-capped ring buffer.
// Every one of those numbers was measured with charsPerToken = 4.0.
//
// Meanwhile every provider client already reported the provider's exact token
// usage through internal/usage, and no budgeting decision anywhere consumed it.
// The ground truth was in the process, correct, free, and disconnected from
// every component that needed it.
//
// # The two-clock model
//
// Token counting has two jobs with different accuracy requirements, and
// conflating them is why the heuristic survived:
//
//   - Accounting ("what did this cost?") happens after the response and is
//     answered exactly by the provider's own usage report.
//   - Admission ("will this fit?") happens before the request. Anthropic
//     exposes a count endpoint that answers it exactly. Most providers do not,
//     so the estimator predicts, then corrects itself against the actual once
//     the response returns.
//
// Every count carries a Confidence saying which regime produced it. Nothing in
// this package reports an estimate as though it were a measurement.
//
// # How it is installed
//
// Client decorates types.LLMClient and is installed inside the three
// constructors in internal/perception/client_factory.go. Callers did not
// change and cannot opt out: obtaining an LLMClient in this codebase means
// going through the factory, and the factory wraps.
//
// Because several call sites probe optional interfaces to choose a control
// flow — types.ToolResultsProvider selects the native tool loop over a
// synthesized envelope, core.SchemaCapableLLMClient selects structured output,
// core.LLMStreamingWithThoughts selects thought streaming — the wrapper must
// expose exactly the capability set the underlying client has. Wrap() composes
// one of eight shapes to preserve that. See wrap.go.
//
// # What it does not do
//
// The broker is a meter, not an executive. It does not select prompt atoms,
// compress context, evaluate policy, or authorize effects. permitted/3 in the
// Mangle kernel remains the sole authority over what the agent may do; the
// broker only decides whether a request fits and records what it spent.
package broker
