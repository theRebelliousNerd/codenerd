package core

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// slowEvaluationLog is the duration above which evaluate() logs what an
// evaluation did, at INFO: a pass that holds k.mu this long stalls every
// writer and reader behind it.
const slowEvaluationLog = time.Second

// dredPhase names a DRed phase for the profile.
type dredPhase int

const (
	phaseOverDelete dredPhase = iota
	phaseRederive
	phaseInsert
	phaseAggregate
	dredPhases
)

var dredPhaseNames = [dredPhases]string{"over-delete", "re-derive", "insert", "aggregate"}

// dredProfile is where one DRed evaluation spent its time: per phase, and per
// rule head across phases. Variants of a phase may run in parallel, so it locks.
type dredProfile struct {
	mu     sync.Mutex
	phase  [dredPhases]time.Duration
	byHead map[string]time.Duration
}

func (p *dredProfile) charge(head string, d time.Duration) {
	p.mu.Lock()
	if p.byHead == nil {
		p.byHead = make(map[string]time.Duration)
	}
	p.byHead[head] += d
	p.mu.Unlock()
}

func (p *dredProfile) addPhase(ph dredPhase, d time.Duration) {
	p.mu.Lock()
	p.phase[ph] += d
	p.mu.Unlock()
}

// summary is the profile in one line: the phases, then the heads that cost
// the most.
func (p *dredProfile) summary(top int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	parts := make([]string, 0, dredPhases+1)
	for ph := dredPhase(0); ph < dredPhases; ph++ {
		parts = append(parts, fmt.Sprintf("%s %v", dredPhaseNames[ph], p.phase[ph].Round(time.Millisecond)))
	}
	heads := make([]string, 0, len(p.byHead))
	for h := range p.byHead {
		heads = append(heads, h)
	}
	sort.Slice(heads, func(i, j int) bool { return p.byHead[heads[i]] > p.byHead[heads[j]] })
	if len(heads) > top {
		heads = heads[:top]
	}
	costly := make([]string, len(heads))
	for i, h := range heads {
		costly[i] = fmt.Sprintf("%s %v", h, p.byHead[h].Round(time.Millisecond))
	}
	return strings.Join(parts, ", ") + "; costliest heads: " + strings.Join(costly, ", ")
}
