package autopoiesis

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// revokingSynth is the synthesizer a retirement is asked of: it records what
// it was told to withdraw and answers the tool lookups the approval reads.
type revokingSynth struct {
	*MockToolSynthesizer
	names   []string
	revoked []string
}

func (r *revokingSynth) ListTools() []types.ToolInfo {
	out := make([]types.ToolInfo, 0, len(r.names))
	for _, n := range r.names {
		out = append(out, types.ToolInfo{Name: n})
	}
	return out
}

func (r *revokingSynth) GetRuntimeTool(name string) (*RuntimeTool, bool) {
	return &RuntimeTool{Name: name, Hash: "h-" + name}, true
}

func (r *revokingSynth) RevokeTool(name string) error {
	r.revoked = append(r.revoked, name)
	return nil
}

func retirementOrchestrator(t *testing.T) (*Orchestrator, *revokingSynth) {
	t.Helper()
	o := toolNeedOrchestrator(t)
	o.profiles = NewProfileStore(t.TempDir())
	synth := &revokingSynth{MockToolSynthesizer: &MockToolSynthesizer{}, names: []string{"gen_slow", "gen_good"}}
	o.ouroboros = synth
	g := types.ToolOriginGenerated
	// gen_slow: 30 measured calls that cost more than the work. gen_good:
	// condenses far past its cost.
	o.profiles.mutateLedgers(
		ledgerWrite{"gen_slow", func(l *ToolLedger) {
			*l = ToolLedger{Origin: g, Offers: 40, OfferSessions: 4, CatalogTokens: 4000, Calls: 30, CallSessions: 4,
				CallTokens: 900, ResultTokens: 3000, KnownCalls: 30, ConsumedBytes: 12_000, ReturnedBytes: 11_000, KnownCallTokens: 3900}
		}},
		ledgerWrite{"gen_good", func(l *ToolLedger) {
			*l = ToolLedger{Origin: g, Offers: 40, OfferSessions: 4, CatalogTokens: 100, Calls: 30, CallSessions: 4,
				CallTokens: 300, ResultTokens: 300, KnownCalls: 30, ConsumedBytes: 9_000_000, ReturnedBytes: 3_000, KnownCallTokens: 600}
		}},
	)
	return o, synth
}

func yoloBroker(t *testing.T) *approval.Broker {
	t.Helper()
	b, err := approval.NewBroker(approval.Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	b.SetAutonomous(true)
	return b
}

func TestRetireGeneratedTool_ApprovedProposalWithdrawsTheTool(t *testing.T) {
	o, synth := retirementOrchestrator(t)
	p, err := o.RetireGeneratedTool(t.Context(), yoloBroker(t), "gen_slow", core.NewToolRegistry(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "retire" || p.Reason != "cost_exceeds_value" {
		t.Errorf("proposal %+v", p)
	}
	if len(synth.revoked) != 1 || synth.revoked[0] != "gen_slow" {
		t.Errorf("revoked %v, want [gen_slow]", synth.revoked)
	}
	if l := o.profiles.ToolLedgers()["gen_slow"]; l.RetiredAt.IsZero() {
		t.Error("the ledger does not record the retirement")
	}
}

func TestRetireGeneratedTool_NeedsADerivedProposal(t *testing.T) {
	o, synth := retirementOrchestrator(t)
	for _, name := range []string{"gen_good", "no_such_tool", "bash", "read_file"} {
		if _, err := o.RetireGeneratedTool(t.Context(), yoloBroker(t), name, nil); err == nil {
			t.Errorf("%s was retired without a retirement proposal", name)
		}
	}
	if len(synth.revoked) != 0 {
		t.Errorf("withdrew %v with no proposal", synth.revoked)
	}
}

func TestRetireGeneratedTool_NobodyToAskIsARefusal(t *testing.T) {
	o, synth := retirementOrchestrator(t)
	b, err := approval.NewBroker(approval.Options{TTL: 50 * time.Millisecond, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	// Not autonomous, no host attached: the request expires undecided.
	if _, err := o.RetireGeneratedTool(t.Context(), b, "gen_slow", nil); err == nil {
		t.Fatal("a tool was withdrawn with no approval")
	}
	if len(synth.revoked) != 0 {
		t.Errorf("withdrew %v without an approval", synth.revoked)
	}
}

func TestRuntimeRegistryRevoke_RemovesArtifactsAndForgetsTheTool(t *testing.T) {
	compiled := t.TempDir()
	binary := filepath.Join(compiled, "gen_slow")
	for _, p := range []string{binary, binary + ".identity.json"} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r := NewRuntimeRegistry(t.TempDir(), compiled)
	r.tools["gen_slow"] = &RuntimeTool{Name: "gen_slow", BinaryPath: binary}
	r.identities["gen_slow"] = types.GeneratedToolIdentity{Name: "gen_slow", BinaryPath: binary}
	if err := r.Revoke("gen_slow"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{binary, binary + ".identity.json"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived the revocation", p)
		}
	}
	if _, ok := r.Get("gen_slow"); ok {
		t.Error("the registry still holds a revoked tool")
	}
	if err := r.Revoke("gen_slow"); err == nil {
		t.Error("revoking an unregistered tool succeeded")
	}
}

func TestRuntimeRegistryRevoke_RefusesAnArtifactOutsideTheCompiledDir(t *testing.T) {
	compiled, elsewhere := t.TempDir(), t.TempDir()
	victim := filepath.Join(elsewhere, "gen_slow")
	if err := os.WriteFile(victim, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewRuntimeRegistry(t.TempDir(), compiled)
	r.tools["gen_slow"] = &RuntimeTool{Name: "gen_slow", BinaryPath: victim}
	if err := r.Revoke("gen_slow"); err == nil {
		t.Fatal("revoked an artifact outside the compiled directory")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Errorf("the file outside the compiled directory was removed: %v", err)
	}
}
