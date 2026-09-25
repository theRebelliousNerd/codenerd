package tactile

// DirectBypass records one production file that constructs a DirectExecutor
// outside this package. VirtualStore's audited composite is the governed
// route for effects: it runs after `permitted`, carries the caller's
// configuration, and emits execution facts into the kernel. A direct executor
// built anywhere else is an exception to that route and must say, here, what
// governs it instead.
//
// An entry is metadata, not authority: it grants no permission and widens no
// limit. TestDirectBypassRegistryMatchesProductionConstructors fails when a
// production file constructs a direct executor without an entry, when the
// construction count drifts, when an entry goes stale, or when the review
// date passes -- the exception must then be re-justified or removed.
type DirectBypass struct {
	// File is the constructing file, slash-separated from the module root.
	File string
	// Constructions is the number of NewDirectExecutor/NewDirectExecutorWithConfig
	// calls the file makes.
	Constructions int
	// Owner is the subsystem accountable for the exception.
	Owner string
	// Reason says why the effect cannot go through VirtualStore.
	Reason string
	// PermissionProof names what authorizes each command the executor runs.
	PermissionProof string
	// AuditSink names where the executor's execution events go.
	AuditSink string
	// Limits names the configuration that bounds its commands.
	Limits string
	// ReviewBy is the date (YYYY-MM-DD) by which the exception is re-justified
	// or removed.
	ReviewBy string
}

// DirectBypassRegistry is the complete list of production direct-executor
// constructions outside internal/tactile.
var DirectBypassRegistry = []DirectBypass{
	{
		File:            "internal/system/factory.go",
		Constructions:   1,
		Owner:           "system boot",
		Reason:          "VirtualStore's injected executor; VirtualStore builds its audited composite from this executor's configuration (tactile.ConfiguredExecutor)",
		PermissionProof: "VirtualStore.RouteAction derives permitted/3 before any command; Cortex.Executor is VirtualStore.AuditedExecutor, not this executor",
		AuditSink:       "VirtualStore composite audit logger -> kernel execution_* facts",
		Limits:          "execution.default_timeout, allowed env and project build env (executionLayerConfigs)",
		ReviewBy:        "2027-01-01",
	},
	{
		File:            "cmd/nerd/dom_cmd.go",
		Constructions:   3,
		Owner:           "CodeDOM CLI demos",
		Reason:          "VirtualStore's injected executor for the dom demo commands; commands route through the store",
		PermissionProof: "routePermittedAction: next_action must derive permitted before the store executes",
		AuditSink:       "VirtualStore composite audit logger -> the command's kernel",
		Limits:          "tactile.DefaultExecutorConfig, inherited by the composite",
		ReviewBy:        "2027-01-01",
	},
	{
		File:            "internal/campaign/assault_tasks.go",
		Constructions:   2,
		Owner:           "campaign assault",
		Reason:          "assault runs its planned test/build commands in batches with a campaign-sized timeout and output budget VirtualStore does not carry",
		PermissionProof: "user-started /campaign assault; commands come from the assault plan, not the model",
		AuditSink:       "Orchestrator.auditedExecutor -> tactile.NewFactAuditedExecutor -> campaign kernel",
		Limits:          "newAssaultExecutor: assault default timeout (<= 2h max) and log byte budget",
		ReviewBy:        "2027-01-01",
	},
	{
		File:            "internal/shards/system/campaign_runner.go",
		Constructions:   1,
		Owner:           "campaign runner shard",
		Reason:          "resumes a user-started campaign whose orchestrator needs an executor before any VirtualStore is bound to the shard",
		PermissionProof: "resumes a campaign the user started; its tasks dispatch through the task executor",
		AuditSink:       "tactile.NewFactAuditedExecutor -> shard kernel",
		Limits:          "tactile.DefaultExecutorConfig",
		ReviewBy:        "2027-01-01",
	},
}
