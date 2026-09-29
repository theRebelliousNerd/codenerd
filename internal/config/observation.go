package config

import (
	"fmt"
	"sync/atomic"
)

// ObservationConfig is the `observation` section of .nerd/config.json: the
// bounds of the file-read projection the model is shown
// (internal/observation.ReadLimits).
//
// Why its own section instead of execution: execution's file ceilings
// refuse reads (max_read_file_bytes) or skip files (max_search_file_bytes).
// These shape the model's view of a read that succeeds — the region, its
// padding, the outline of what was not printed — and the rest of the file
// stays retained under the read handle. Refusal and projection are
// different decisions with different readers, so they get different keys.
//
// The values reach observation through the caller that builds ReadLimits
// (VirtualStore.handleReadFile), which reads the installed policy below:
// observation is a leaf package and must not import this one.
type ObservationConfig struct {
	// MaxRegionLines bounds the shown region in lines. Above the 75th
	// percentile of Go files in this repository, so a typical whole-file
	// read still arrives whole.
	MaxRegionLines int `json:"max_region_lines,omitempty"`
	// PadLines is what a region gets on top of the code element it was
	// snapped out to. 0 through this file means the default; the codec
	// itself still honors an explicit 0 from a direct API caller.
	PadLines int `json:"pad_lines,omitempty"`
	// MaxOutline bounds the outline entries printed for what the region
	// did not show; the rest are counted, not dropped silently.
	MaxOutline int `json:"max_outline,omitempty"`
	// MaxRegionBytes bounds the shown region regardless of its line count:
	// a line ceiling alone is not a ceiling on minified bundles, base64
	// assets or generated lookup tables. Lines it sheds are counted in
	// Elided, bytes it cuts from an unbreakable line in RegionCut.
	MaxRegionBytes int `json:"max_region_bytes,omitempty"`
}

// DefaultObservationConfig is the observation section with every field
// written down. The values are the projection sizes the codec carried
// before 2026-09-29 (400 lines, 8 of padding, a 60-entry outline, 24KiB),
// mirrored by observation.DefaultReadLimits as the leaf safety net for
// direct API callers; the parity test below fails if the two drift.
func DefaultObservationConfig() ObservationConfig {
	return ObservationConfig{
		MaxRegionLines: 400,
		PadLines:       8,
		MaxOutline:     60,
		MaxRegionBytes: 24 << 10,
	}
}

// GetObservationConfig returns the observation section with every absent
// field defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetObservationConfig() ObservationConfig {
	if c == nil || c.Observation == nil {
		return DefaultObservationConfig()
	}
	return c.Observation.WithDefaults()
}

// WithDefaults fills every absent field from DefaultObservationConfig.
func (c ObservationConfig) WithDefaults() ObservationConfig {
	d := DefaultObservationConfig()
	if c.MaxRegionLines == 0 {
		c.MaxRegionLines = d.MaxRegionLines
	}
	if c.PadLines == 0 {
		c.PadLines = d.PadLines
	}
	if c.MaxOutline == 0 {
		c.MaxOutline = d.MaxOutline
	}
	if c.MaxRegionBytes == 0 {
		c.MaxRegionBytes = d.MaxRegionBytes
	}
	return c
}

// ObservationLimits is an ObservationConfig resolved for the caller that
// builds ReadLimits: defaults filled, no parsing needed. Resolve is the
// only way to make one.
type ObservationLimits struct {
	MaxRegionLines int
	PadLines       int
	MaxOutline     int
	MaxRegionBytes int
}

// Resolve defaults the section. Durations need no parsing here; Check is
// what refuses a contradictory file, and LoadUserConfig runs it first.
func (c ObservationConfig) Resolve() ObservationLimits {
	c = c.WithDefaults()
	return ObservationLimits{
		MaxRegionLines: c.MaxRegionLines,
		PadLines:       c.PadLines,
		MaxOutline:     c.MaxOutline,
		MaxRegionBytes: c.MaxRegionBytes,
	}
}

// Check reports the contradictions in an observation section, addressed
// under prefix ("observation"). Absent fields are defaulted first, so only
// what the file says can be wrong.
func (c ObservationConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	for _, f := range []struct {
		name string
		v    int
	}{
		{"max_region_lines", c.MaxRegionLines},
		{"pad_lines", c.PadLines},
		{"max_outline", c.MaxOutline},
		{"max_region_bytes", c.MaxRegionBytes},
	} {
		if f.v < 1 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + f.name,
				Message:  fmt.Sprintf("%d is below 1", f.v),
				Fix:      "a count of at least 1, or remove the key for the default",
			})
		}
	}
	return out
}

// activeObservationLimits is the process-wide resolved observation policy,
// installed by LoadUserConfig the same way the execution file limits are:
// VirtualStore reads it when it builds ReadLimits, without opening the
// config file itself.
var activeObservationLimits atomic.Pointer[ObservationLimits]

func init() {
	SetObservationLimits(DefaultObservationConfig().Resolve())
}

// SetObservationLimits installs the process-wide observation policy.
// LoadUserConfig is the production caller; tests install and restore.
func SetObservationLimits(l ObservationLimits) {
	activeObservationLimits.Store(&l)
}

// ResolvedObservationLimits is the installed observation policy. Without
// a load it is the defaults.
func ResolvedObservationLimits() ObservationLimits {
	if l := activeObservationLimits.Load(); l != nil {
		return *l
	}
	return DefaultObservationConfig().Resolve()
}
