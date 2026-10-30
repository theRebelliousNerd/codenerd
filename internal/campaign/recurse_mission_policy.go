package campaign

import (
	"context"
	"errors"
	"fmt"
	"time"

	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
)

func missionHas(rows []types.Fact, id string) bool {
	for _, row := range rows {
		if len(row.Args) > 0 && types.ExtractString(row.Args[0]) == id {
			return true
		}
	}
	return false
}

// admit replaces the measured clock/counters and obeys one kernel judgment.
func (m *RecurseMission) admit() (int, error) {
	spend, unknown, _ := m.meter.snapshot()
	missing := types.MangleAtom("/no")
	if unknown {
		missing = "/yes"
	}
	k := m.Deliberator.Kernel
	m.State.ElapsedSeconds = int64(time.Since(m.started) / time.Second)
	if err := k.Retract("mission_tick"); err != nil {
		return 0, err
	}
	if err := k.Assert(types.Fact{Predicate: "mission_tick", Args: []any{m.State.ID,
		int64(m.State.Cycles), m.State.ElapsedSeconds, spend, missing, int64(m.State.Stalls)}}); err != nil {
		return 0, err
	}
	rows, err := k.Query("mission_stop")
	if err != nil {
		return 0, err
	}
	var stops []types.Fact
	for _, row := range rows {
		if len(row.Args) == 2 && types.ExtractString(row.Args[0]) == m.State.ID {
			stops = append(stops, row)
		}
	}
	if len(stops) > 1 {
		return 0, fmt.Errorf("mission has conflicting stop decisions")
	}
	if len(stops) == 1 {
		m.State.Status = types.ExtractString(stops[0].Args[1])
		m.State.StopFacts = []string{stops[0].String()}
		if err := m.save(); err != nil {
			return 0, err
		}
		return 0, errMissionStopped
	}
	rows, err = k.Query("mission_continue")
	if err != nil {
		return 0, err
	}
	var allowed []int
	for _, row := range rows {
		if len(row.Args) == 2 && types.ExtractString(row.Args[0]) == m.State.ID {
			allowed = append(allowed, argInt(row.Args[1]))
		}
	}
	if len(allowed) != 1 || allowed[0] <= 0 {
		return 0, fmt.Errorf("mission has no unique positive run admission")
	}
	return allowed[0], nil
}

func (m *RecurseMission) end(ctx context.Context, runErr error) error {
	if m.State.Pending != nil && m.State.Pending.Outcome == "deliberating" {
		m.State.Pending.Outcome = "design_unresolved"
		m.State.Receipts = append(m.State.Receipts, *m.State.Pending)
		m.State.Pending = nil
	}
	if m.State.Status == "running" {
		m.State.Status = "pass_bound"
		if runErr != nil {
			m.State.Status = "interrupted"
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			_, err := m.admit()
			if !errors.Is(err, errMissionStopped) {
				runErr = errors.Join(runErr, err)
			}
		}
	}
	if runErr == errMissionStopped {
		runErr = nil
	}
	if runErr == context.DeadlineExceeded && m.State.Status == "/time" {
		runErr = nil
	}
	runErr = errors.Join(runErr, m.save())
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventFinal, Text: "Mission stopped: " + m.State.Status, Failed: runErr != nil})
	// Run state is durable; transient judgments must not leak into another run.
	for _, pred := range []string{"mission_tick", "mission_design", "mission_context_size", "mission_write_candidate", "mission_source"} {
		runErr = errors.Join(runErr, m.Deliberator.Kernel.Retract(pred))
	}
	return runErr
}
