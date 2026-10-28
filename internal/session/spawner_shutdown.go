package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrSpawnerClosed is what Spawn returns once the spawner has been shut down.
var ErrSpawnerClosed = errors.New("spawner is shut down")

// Shutdown closes admission, asks every unfinished subagent to stop, and waits
// for the ones that were running to reach a terminal state, until ctx ends.
// It returns the IDs of subagents that were still running when it stopped
// waiting, with an error naming them: a stop request is not termination, and
// the caller must not release the kernel and stores such a worker still holds
// as if it had ended.
//
// A subagent registered but not yet started is not waited on. Its stop is
// latched, so the Run that follows ends it before the model is called.
// Admission closes first, under the lock Spawn registers under, so no agent
// can appear after the snapshot this takes.
func (s *Spawner) Shutdown(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	s.closed = true
	live := make([]*SubAgent, 0, len(s.subagents))
	for _, agent := range s.subagents {
		if state := agent.GetState(); state != SubAgentStateCompleted && state != SubAgentStateFailed {
			live = append(live, agent)
		}
	}
	s.mu.Unlock()

	var running []*SubAgent
	for _, agent := range live {
		_ = agent.Stop()
		if agent.GetState() == SubAgentStateRunning {
			running = append(running, agent)
		}
	}

	var unjoined []string
	for _, agent := range running {
		select {
		case <-agent.Done():
		case <-ctx.Done():
			unjoined = append(unjoined, agent.GetID())
		}
	}
	if len(unjoined) == 0 {
		return nil, nil
	}
	sort.Strings(unjoined)
	return unjoined, fmt.Errorf("%d subagent(s) still running after a stop request: %s", len(unjoined), strings.Join(unjoined, ", "))
}
