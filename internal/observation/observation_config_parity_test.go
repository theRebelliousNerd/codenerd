package observation_test

import (
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/observation"
	"codenerd/internal/observation/precondition"
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
