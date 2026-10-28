package core

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"codenerd/internal/mangle/feedback"
)

// =============================================================================
// MANGLE CAPABILITY MANIFEST
// =============================================================================
//
// "Mangle supports X" is not the question a rule author needs answered. The
// question is whether THIS host path -- RealKernel, at this dependency version,
// with this program -- preserves X's semantics end to end: typed admission,
// lifecycle, validation. Upstream documents temporal evaluation; RealKernel
// installs no temporal store, so a generated rule using it compiles in the
// sandbox and then means nothing.
//
// The manifest states, per feature, what the host path does, and only the
// features marked Teach reach the guidance generated-rule authors receive
// (GeneratedRuleGuidance, consumed by feedback.FeedbackLoop for the
// Legislator, the constitution gate, executive autopoiesis and
// GenerateValidatedRule). A feature qualified against one dependency version
// is not taught on another: the state drops to unverified until it is
// requalified here.

// mangleModulePath is the dependency the manifest describes.
const mangleModulePath = "codeberg.org/TauCeti/mangle-go"

// qualifiedMangleVersion is the dependency version these feature states were
// established against (Docs/audits/mangle-feature-adoption-contracts.md).
const qualifiedMangleVersion = "v0.7.0"

// MangleFeatureState is what a host path does with a feature.
type MangleFeatureState string

const (
	FeatureProduction        MangleFeatureState = "production"
	FeatureImplementedOptIn  MangleFeatureState = "implemented-opt-in"
	FeatureHostBatchOnly     MangleFeatureState = "host-batch-only"
	FeatureNotWired          MangleFeatureState = "not-wired"
	FeatureUnverified        MangleFeatureState = "unverified"
	FeatureVersionUnverified MangleFeatureState = "unverified-on-this-version"
)

// MangleFeature is one row of the manifest.
type MangleFeature struct {
	Name     string
	State    MangleFeatureState
	Contract string // the host contract that makes the state true
	Teach    bool   // generated rules may rely on it
	Guidance string // what a rule author is told
}

// MangleCapabilityManifest describes one host path.
type MangleCapabilityManifest struct {
	Dependency        string
	DependencyVersion string // from build info; "unknown" when unavailable
	HostPath          string
	ProgramDigest     string
	SourceRevision    string
	Features          []MangleFeature
}

// CapabilityManifest describes RealKernel's host path as currently loaded.
func (k *RealKernel) CapabilityManifest() MangleCapabilityManifest {
	k.mu.Lock()
	digest := k.programDigestLocked()
	recording := k.proofRecorder != nil
	k.mu.Unlock()
	return realKernelManifest(mangleDependencyVersion(), digest, recording)
}

// realKernelManifest is the RealKernel manifest at a given dependency version.
func realKernelManifest(version, digest string, recording bool) MangleCapabilityManifest {
	m := MangleCapabilityManifest{
		Dependency:        mangleModulePath,
		DependencyVersion: version,
		HostPath:          "RealKernel",
		ProgramDigest:     digest,
		SourceRevision:    buildRevision(),
		Features:          realKernelFeatures(recording),
	}
	if version != qualifiedMangleVersion {
		for i := range m.Features {
			if m.Features[i].Teach {
				m.Features[i].State = FeatureVersionUnverified
				m.Features[i].Teach = false
			}
		}
	}
	return m
}

func realKernelFeatures(recording bool) []MangleFeature {
	recordingState := FeatureImplementedOptIn
	recordingContract := "EnableProvenance before the evaluation; off by default"
	if recording {
		recordingContract = "recording is on for this kernel"
	}
	return []MangleFeature{
		{
			Name: "stratified_rules_negation_aggregation", State: FeatureProduction, Teach: true,
			Contract: "analysis.AnalyzeOneUnit + EvalStratifiedProgramWithStats over the whole program",
			Guidance: "Stratified rules, negation of atoms whose variables are bound by positive atoms, and `|> do fn:group_by(...), let N = fn:count()` aggregation are evaluated as written.",
		},
		{
			Name: "runtime_rich_bounds", State: FeatureProduction, Teach: true,
			Contract: "typed admission: every fact is checked against its Decl's bound alternatives (kernel_fact_decl.go)",
			Guidance: "Decl bounds are enforced on every fact: `bound [...] bound [...]` alternatives are whole tuples, /name and /string are distinct, and /number is an int64.",
		},
		{
			Name: "static_bounds_for_generated_rules", State: FeatureProduction, Teach: true,
			Contract: "candidate rules are analyzed with ErrorForBoundsMismatch against the program's Decls (kernel_rule_bounds.go)",
			Guidance: "Your rule is type-checked against the declared bounds before it is accepted: join /name columns with /name columns and /string with /string, and derive into a head only values its Decl admits.",
		},
		{
			Name: "inclusion_constraints", State: FeatureHostBatchOnly, Teach: false,
			Contract: "checked by the host over complete closed batches (CheckBatchInclusion), never during evaluation",
			Guidance: "Do not declare `inclusion [...]` constraints: the host checks them only for batches it owns.",
		},
		{
			Name: "derivation_recording", State: recordingState, Teach: false,
			Contract: recordingContract,
		},
		{
			Name: "proof_fact_consumer", State: FeatureImplementedOptIn, Teach: false,
			Contract: "ExplanationEnvelope: epoch-stamped, partial-preserving, bounded proof facts in a separate read-only store",
			Guidance: "Rules cannot read proves/premise/edb_leaf facts: proof facts never enter the execution kernel.",
		},
		{
			Name: "temporal_evaluation", State: FeatureNotWired, Teach: false,
			Contract: "RealKernel installs no temporal store or clock",
			Guidance: "Do not use temporal syntax (`temporal` Decls, `@[...]` intervals, temporal operators): this host does not evaluate it.",
		},
		{
			Name: "deferred_and_merge_evaluation", State: FeatureUnverified, Teach: false,
			Contract: "binding, convergence and replacement semantics not qualified on this host",
			Guidance: "Do not rely on deferred evaluation or merge-based state replacement.",
		},
	}
}

// GeneratedRuleGuidance is the manifest as a prompt section for a
// generated-rule author: what the host enforces, and what not to use.
// It carries no digest, so it is stable across programs and cacheable.
func (m MangleCapabilityManifest) GeneratedRuleGuidance() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "## Host capabilities (%s on %s@%s)\n\n", m.HostPath, m.Dependency, m.DependencyVersion)
	sb.WriteString("Rely on:\n")
	for _, f := range m.Features {
		if f.Teach && f.Guidance != "" {
			fmt.Fprintf(&sb, "- %s\n", f.Guidance)
		}
	}
	sb.WriteString("\nDo not use:\n")
	for _, f := range m.Features {
		if !f.Teach && f.Guidance != "" {
			fmt.Fprintf(&sb, "- %s\n", f.Guidance)
		}
	}
	for _, f := range m.Features {
		if f.State == FeatureVersionUnverified {
			fmt.Fprintf(&sb, "\nFeatures qualified on %s are unverified on %s: use only plain rules over declared predicates.\n",
				qualifiedMangleVersion, m.DependencyVersion)
			break
		}
	}
	return sb.String()
}

var _ feedback.RuleCapabilityGuide = (*RealKernel)(nil)

// GeneratedRuleGuidance implements feedback.RuleCapabilityGuide: the
// feedback loop appends it to every generated-rule system prompt.
func (k *RealKernel) GeneratedRuleGuidance() string {
	return k.CapabilityManifest().GeneratedRuleGuidance()
}

var buildInfoOnce = sync.OnceValue(func() *debug.BuildInfo {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return info
})

// mangleDependencyVersion is the linked mangle-go version, honouring a
// replace directive; "unknown" when the binary carries no build info.
func mangleDependencyVersion() string {
	info := buildInfoOnce()
	if info == nil {
		return "unknown"
	}
	for _, dep := range info.Deps {
		if dep.Path != mangleModulePath {
			continue
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version
		}
		return dep.Version
	}
	return "unknown"
}

// buildRevision is the VCS revision this binary was built from, "" when the
// build did not stamp one (go test, go run).
func buildRevision() string {
	info := buildInfoOnce()
	if info == nil {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return ""
}
