package gates

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/types"
)

// Snapshot is every error-free gate measured once over a workspace.
type Snapshot struct {
	// ID names the snapshot in the kernel and on disk.
	ID    string    `json:"id"`
	Taken time.Time `json:"taken"`
	Root  string    `json:"root"`
	// Head is the commit the tree was at, "" outside git. A dirty tree is
	// measured as it is; Head says where it started.
	Head  string        `json:"head,omitempty"`
	Gates []Measurement `json:"gates"`
	// ComposedOf, when set, says the snapshot was composed (Overlay), not
	// measured: the IDs of the measured snapshots its rows came from.
	ComposedOf []string `json:"composed_of,omitempty"`
}

// SnapshotDir is where snapshots are written, under the workspace.
const SnapshotDir = ".nerd/gates"

func newSnapshot(ctx context.Context, root string) Snapshot {
	now := time.Now().UTC()
	s := Snapshot{ID: "gates_" + now.Format("20060102T150405.000000000Z"), Taken: now, Root: root}
	if out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output(); err == nil {
		s.Head = strings.TrimSpace(string(out))
	}
	return s
}

// Gate returns the measurement of gate (name or label), if measured.
func (s Snapshot) Gate(gate string) (Measurement, bool) {
	for _, g := range s.Gates {
		if g.Gate == gate || strings.EqualFold(g.Label, gate) {
			return g, true
		}
	}
	return Measurement{}, false
}

// Save writes the snapshot to <root>/.nerd/gates/<id>.json and returns the
// path.
func (s Snapshot) Save(root string) (string, error) {
	dir := filepath.Join(root, filepath.FromSlash(SnapshotDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, s.ID+".json")
	if err := atomicfile.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadSnapshot reads a snapshot file.
func LoadSnapshot(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("%s is not a gate snapshot: %w", path, err)
	}
	if s.ID == "" {
		return Snapshot{}, fmt.Errorf("%s is not a gate snapshot: no id", path)
	}
	return s, nil
}

// SnapshotPaths lists the workspace's saved snapshots, oldest first.
func SnapshotPaths(root string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(SnapshotDir), "gates_*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches) // IDs are UTC timestamps
	return matches, nil
}

// Facts projects the snapshot for policy/gates.mg: gate_snapshot,
// gate_measured and gate_domain per gate, and gate_finding per distinct
// finding.
func (s Snapshot) Facts() []types.Fact {
	facts := []types.Fact{{Predicate: "gate_snapshot", Args: []any{s.ID}}}
	for _, g := range s.Gates {
		gate := types.MangleAtom("/" + g.Gate)
		facts = append(facts,
			types.Fact{Predicate: "gate_measured", Args: []any{s.ID, gate, types.MangleAtom("/" + string(g.Status)), int64(g.Count)}},
			types.Fact{Predicate: "gate_domain", Args: []any{s.ID, gate, s.domain(g)}})
		if g.Unenumerated {
			facts = append(facts, types.Fact{Predicate: "gate_unenumerated", Args: []any{s.ID, gate}})
		}
		seen := map[string]bool{}
		for _, f := range g.Findings {
			key := FindingKey(f)
			if seen[key] {
				continue
			}
			seen[key] = true
			facts = append(facts, types.Fact{Predicate: "gate_finding", Args: []any{s.ID, gate, key}})
		}
	}
	return facts
}

// domain names what a measurement measured: the workspace root and the
// package patterns, sorted. A narrower test selection, or another
// workspace, is another domain.
func (s Snapshot) domain(m Measurement) string {
	pkgs := append([]string(nil), m.Packages...)
	sort.Strings(pkgs)
	pkgs = slices.Compact(pkgs)
	return s.Root + " | " + strings.Join(pkgs, " ")
}

// GateUnadmitted is a gate the comparison could not judge, and why:
// missing_after, missing_before (measured on one side only) or
// domain_changed (measured over another workspace or package selection).
type GateUnadmitted struct {
	Gate   string `json:"gate"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

// GateChange is one gate compared across two snapshots.
type GateChange struct {
	Gate   string `json:"gate"`
	Label  string `json:"label"`
	Change string `json:"change"` // better, worse, same
	Before MeasureStatus
	After  MeasureStatus
	// BeforeCount and AfterCount are the gate's finding counts.
	BeforeCount, AfterCount int
	// New and Gone are finding keys (FindingKey) present in one snapshot only.
	New  []string `json:"new,omitempty"`
	Gone []string `json:"gone,omitempty"`
}

// Comparison is the kernel's judgment of two snapshots.
type Comparison struct {
	Before, After string
	// Keepable: every gate was compared and none got worse (cycle_keepable).
	Keepable bool
	Gates    []GateChange
	// Unadmitted are the gates the pair could not compare; any one makes
	// the pair not keepable (gate_comparison_unadmitted).
	Unadmitted []GateUnadmitted
	// Empty: the pair compared no gate at all, so it vouches for nothing.
	Empty bool
}

// NotKeepableBecause names why a pair is not keepable: the gates that got
// worse and the gates it could not compare.
func (c Comparison) NotKeepableBecause() []string {
	out := []string{}
	for _, r := range c.Regressed() {
		out = append(out, r+" got worse")
	}
	for _, u := range c.Unadmitted {
		out = append(out, u.Label+" "+u.Gate+" not compared ("+strings.ReplaceAll(u.Reason, "_", " ")+")")
	}
	if c.Empty {
		out = append(out, "no gate was compared")
	}
	return out
}

// Regressed lists the gates that got worse.
func (c Comparison) Regressed() []string {
	var out []string
	for _, g := range c.Gates {
		if g.Change == "worse" {
			out = append(out, g.Label+" "+g.Gate)
		}
	}
	return out
}

// Compare asks the kernel (policy/gates.mg) how after compares with before:
// per gate better, worse or same, the findings new and gone, the gates it
// could not compare and why, and whether the pair is keepable. Go reads the
// judgment; it does not make it.
func Compare(k types.Kernel, before, after Snapshot) (Comparison, error) {
	if before.ID == after.ID {
		return Comparison{}, fmt.Errorf("compare: both snapshots are %s", before.ID)
	}
	facts := append(before.Facts(), after.Facts()...)
	facts = append(facts, types.Fact{Predicate: "gate_comparison", Args: []any{before.ID, after.ID}})
	if err := k.AssertBatch(facts); err != nil {
		return Comparison{}, fmt.Errorf("compare: assert snapshots: %w", err)
	}
	defer func() { _ = k.RetractExactFactsBatch(facts) }()

	c := Comparison{Before: before.ID, After: after.ID}
	pair := func(f types.Fact) bool {
		return len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == before.ID && types.ExtractString(f.Args[1]) == after.ID
	}
	rows, err := k.Query("cycle_keepable")
	if err != nil {
		return c, fmt.Errorf("compare: query cycle_keepable: %w", err)
	}
	for _, f := range rows {
		c.Keepable = c.Keepable || pair(f)
	}
	if rows, err = k.Query("gate_comparison_empty"); err != nil {
		return c, fmt.Errorf("compare: query gate_comparison_empty: %w", err)
	}
	for _, f := range rows {
		c.Empty = c.Empty || pair(f)
	}
	labels := map[string]string{}
	for _, g := range append(append([]Measurement(nil), before.Gates...), after.Gates...) {
		labels[g.Gate] = g.Label
	}
	unadmitted := map[string]bool{}
	if rows, err = k.Query("gate_unadmitted"); err != nil {
		return c, fmt.Errorf("compare: query gate_unadmitted: %w", err)
	}
	for _, f := range rows {
		if pair(f) && len(f.Args) == 4 {
			gate := strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")
			unadmitted[gate] = true
			c.Unadmitted = append(c.Unadmitted, GateUnadmitted{Gate: gate, Label: labels[gate], Reason: strings.TrimPrefix(types.ExtractString(f.Args[3]), "/")})
		}
	}
	sort.Slice(c.Unadmitted, func(i, j int) bool { return c.Unadmitted[i].Gate < c.Unadmitted[j].Gate })
	changes := map[string]*GateChange{}
	for _, g := range after.Gates {
		if unadmitted[g.Gate] {
			continue
		}
		b, _ := before.Gate(g.Gate)
		changes[g.Gate] = &GateChange{Gate: g.Gate, Label: g.Label, Before: b.Status, After: g.Status, BeforeCount: b.Count, AfterCount: g.Count}
	}
	if rows, err = k.Query("gate_change"); err != nil {
		return c, fmt.Errorf("compare: query gate_change: %w", err)
	}
	for _, f := range rows {
		if pair(f) && len(f.Args) == 4 {
			if gc := changes[strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")]; gc != nil {
				gc.Change = strings.TrimPrefix(types.ExtractString(f.Args[3]), "/")
			}
		}
	}
	keys := func(rows []types.Fact, add func(*GateChange, string)) {
		for _, f := range rows {
			if pair(f) && len(f.Args) == 4 {
				if gc := changes[strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")]; gc != nil {
					add(gc, types.ExtractString(f.Args[3]))
				}
			}
		}
	}
	if rows, err = k.Query("gate_finding_new"); err != nil {
		return c, fmt.Errorf("compare: query gate_finding_new: %w", err)
	}
	keys(rows, func(gc *GateChange, key string) { gc.New = append(gc.New, key) })
	if rows, err = k.Query("gate_finding_gone"); err != nil {
		return c, fmt.Errorf("compare: query gate_finding_gone: %w", err)
	}
	keys(rows, func(gc *GateChange, key string) { gc.Gone = append(gc.Gone, key) })
	for _, g := range after.Gates {
		gc := changes[g.Gate]
		if gc == nil {
			continue // unadmitted: reported with its reason, never as a change
		}
		if gc.Change == "" {
			return c, fmt.Errorf("compare: the kernel derived no gate_change for %s %s; the policy must decide one", g.Label, g.Gate)
		}
		sort.Strings(gc.New)
		sort.Strings(gc.Gone)
		c.Gates = append(c.Gates, *gc)
	}
	return c, nil
}
