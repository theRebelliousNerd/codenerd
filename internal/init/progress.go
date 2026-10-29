package init

import (
	"sync"
	"time"
)

// phaseClock reports where a run is: the number of the phase that is running
// among initPhases and the time since init started. Both are measured. It
// used to also report a remaining time summed from fixed per-phase guesses
// ("kb_creation": 105s whatever the agent count), which nothing could make
// true and nothing displayed.
type phaseClock struct {
	mu      sync.RWMutex
	start   time.Time
	current int
	total   int
}

func newPhaseClock(total int) *phaseClock {
	return &phaseClock{start: time.Now(), total: total}
}

func (c *phaseClock) setPhase(number int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.current = number
}

// position returns the running phase's number, the phase count and the
// elapsed time.
func (c *phaseClock) position() (current, total int, elapsed time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current, c.total, time.Since(c.start)
}

// startPhase records the phase that is starting and announces it.
func (i *Initializer) startPhase(number int, phase, message string) {
	if i.clock != nil {
		i.clock.setPhase(number)
	}
	i.sendProgress(phase, message)
}

// sendProgress sends one progress update when a progress channel is
// configured. It never blocks: a full channel drops the update.
func (i *Initializer) sendProgress(phase, message string) {
	if i.config.ProgressChan == nil {
		return
	}
	progress := InitProgress{Phase: phase, Message: message}
	if i.clock != nil {
		progress.CurrentPhaseNo, progress.TotalPhases, progress.ElapsedTime = i.clock.position()
		if progress.TotalPhases > 0 && progress.CurrentPhaseNo > 0 {
			progress.Percent = float64(progress.CurrentPhaseNo-1) / float64(progress.TotalPhases)
		}
	}
	if phase == "complete" {
		progress.Percent = 1
	}
	select {
	case i.config.ProgressChan <- progress:
	default:
	}
}
