package campaign

import (
	"fmt"
	"strings"
	"time"
)

// What `nerd campaign recurse status` says about the run's base and its own
// binary, read from the ledger (stepBase, stepAbsorb, stepAbsorbConflict,
// stepRebuild, stepRebuildFailed).

// RecurseConflict is a base merge that conflicted and was aborted.
type RecurseConflict struct {
	Time time.Time
	// BaseCommit is the base commit whose merge conflicted.
	BaseCommit string
	Detail     string
}

// RecurseGeneration is one build of the run's own binary: built from Commit at
// Time. A failed build carries its failure and no binary.
type RecurseGeneration struct {
	Time    time.Time
	Commit  string
	Binary  string
	Failure string
}

// RecurseBaseStatus is the base branch and the binary generations.
type RecurseBaseStatus struct {
	// Branch is the base the run last recorded; TipAtStart the base commit it
	// had then.
	Branch, TipAtStart string
	// LastAbsorbed is the newest base commit merged in, and when.
	LastAbsorbed   string
	LastAbsorbedAt time.Time
	Absorbs        int
	Conflicts      []RecurseConflict
	Generations    []RecurseGeneration
}

// note reads one ledger record.
func (b *RecurseBaseStatus) note(r recurseRecord) {
	switch r.Step {
	case stepBase:
		b.Branch, b.TipAtStart = r.Base, r.Commit
	case stepAbsorb:
		b.LastAbsorbed, b.LastAbsorbedAt = r.Commit, r.Time
		b.Absorbs++
	case stepAbsorbConflict:
		b.Conflicts = append(b.Conflicts, RecurseConflict{Time: r.Time, BaseCommit: r.Commit, Detail: r.Detail})
	case stepRebuild:
		b.Generations = append(b.Generations, RecurseGeneration{Time: r.Time, Commit: r.Commit, Binary: r.Detail})
	case stepRebuildFailed:
		b.Generations = append(b.Generations, RecurseGeneration{Time: r.Time, Commit: r.Commit, Failure: r.Detail})
	}
}

func shortCommit(h string) string { return h[:min(len(h), 9)] }

// render is the base and binary lines of the status; "" when the ledger has
// none of them.
func (b RecurseBaseStatus) render() string {
	var out strings.Builder
	stamp := func(t time.Time) string { return t.Local().Format("2006-01-02 15:04:05") }
	if b.Branch != "" {
		fmt.Fprintf(&out, "Base: %s (at %s when recorded).", b.Branch, shortCommit(b.TipAtStart))
		if b.LastAbsorbed != "" {
			fmt.Fprintf(&out, " Last absorbed base commit %s at %s (%d absorb(s)).", shortCommit(b.LastAbsorbed), stamp(b.LastAbsorbedAt), b.Absorbs)
		} else {
			out.WriteString(" Nothing absorbed yet.")
		}
		out.WriteString("\n")
	}
	if n := len(b.Conflicts); n > 0 {
		last := b.Conflicts[n-1]
		fmt.Fprintf(&out, "Merge conflicts: %d. Latest: base commit %s at %s; %s\n", n, shortCommit(last.BaseCommit), stamp(last.Time), last.Detail)
	}
	if len(b.Generations) > 0 {
		out.WriteString("Binary generations:\n")
		gen := 0
		for _, g := range b.Generations {
			if g.Failure != "" {
				fmt.Fprintf(&out, "  build FAILED from %s at %s: %s\n", shortCommit(g.Commit), stamp(g.Time), oneLine(g.Failure))
				continue
			}
			gen++
			fmt.Fprintf(&out, "  %d: built from %s at %s  %s\n", gen, shortCommit(g.Commit), stamp(g.Time), g.Binary)
		}
	}
	return out.String()
}
