package observation_test

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/observation/precondition"
	"codenerd/internal/retain"
)

// DefaultReadLimits is the leaf safety net for direct API callers;
// DefaultObservationConfig is what production reads resolve through the
// caller that builds ReadLimits. The two must stay the same values, or a
// zero ReadLimits and a production read would project differently.
func TestDefaultReadLimits_MatchesConfigDefaults(t *testing.T) {
	leaf := observation.DefaultReadLimits()
	cfg := config.DefaultObservationConfig()
	if leaf.MaxRegionLines != cfg.MaxRegionLines ||
		leaf.PadLines != cfg.PadLines ||
		leaf.MaxOutline != cfg.MaxOutline ||
		leaf.MaxRegionBytes != cfg.MaxRegionBytes {
		t.Errorf("leaf defaults %+v != config defaults %+v", leaf, cfg)
	}
}

// The byte ceiling rides the limits, not a constant: a caller-supplied
// ceiling binds the projection and announces the cut.
func TestProjectRead_MaxRegionBytesFieldBinds(t *testing.T) {
	oneLine := strings.Repeat("payload,", 40000)
	r := observation.ProjectRead(
		precondition.Read{Path: "bundle.min.js", Content: oneLine},
		observation.ReadLimits{MaxRegionLines: 400, PadLines: 8, MaxOutline: 60, MaxRegionBytes: 1000},
	)
	if len(r.Region) > 1000 {
		t.Fatalf("region is %d bytes against a 1000-byte ceiling", len(r.Region))
	}
	if r.RegionCut == 0 {
		t.Error("a cut region that does not announce itself reads as the whole line")
	}
}

// The leaf hydrate constants are the safety net for a ReturnWindow that
// leaves MaxLines and DefaultLines unset. Production passes the config
// defaults in, so the two have to be the same numbers: an unbounded
// hydration and a configured one would otherwise page differently.
func TestHydrateLeafBounds_MatchConfigDefaults(t *testing.T) {
	cfg := config.DefaultObservationConfig()
	c := observation.NewSubagents(retain.DefaultConfig())
	n := cfg.SubagentHydrateMaxLines + 40
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("finding line %04d", i)
	}
	encoded := c.EncodeReturn(observation.Return{Agent: "coder", Output: strings.Join(lines, "\n")}, observation.ReturnLimits{})
	if encoded.Handle == "" {
		t.Fatal("the transcript was not retained, so the leaf page cannot be measured")
	}

	unbounded, err := c.HydrateReturn(encoded.Handle, observation.ReturnWindow{})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(unbounded.Lines) != cfg.SubagentHydrateDefaultLines {
		t.Errorf("leaf default page = %d lines, config default = %d", len(unbounded.Lines), cfg.SubagentHydrateDefaultLines)
	}

	capped, err := c.HydrateReturn(encoded.Handle, observation.ReturnWindow{Limit: n})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(capped.Lines) != cfg.SubagentHydrateMaxLines {
		t.Errorf("leaf cap = %d lines, config default = %d", len(capped.Lines), cfg.SubagentHydrateMaxLines)
	}
}
