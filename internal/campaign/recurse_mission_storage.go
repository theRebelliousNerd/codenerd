package campaign

import (
	"codenerd/internal/atomicfile"
	"codenerd/internal/gates"
	"codenerd/internal/northstar"
	"codenerd/internal/observation"
	"codenerd/internal/sessionlog"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// An interrupted receipt cannot donate acceptance to another invocation. The
// ordinary recurse journal/commit reconciliation remains the adoption record.
func (m *RecurseMission) checkInterrupted() error {
	paths, err := filepath.Glob(filepath.Join(sessionlog.SessionsDir(m.Deliberator.Workspace), "*", "missions", "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		f, err := atomicfile.Open(path)
		if err != nil {
			return err
		}
		b, err := io.ReadAll(f)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		var state MissionState
		if err := json.Unmarshal(b, &state); err != nil {
			return fmt.Errorf("read mission %s: %w", path, err)
		}
		if state.Session == "" || sessionlog.ValidateID(state.ID) != nil || filepath.Base(path) != state.ID+".json" {
			return fmt.Errorf("invalid mission identity in %s", path)
		}
		if state.Pending != nil {
			return fmt.Errorf("mission %s has an unsettled cycle %d; reconcile it with %s before starting another mission", state.ID, state.Pending.Cycle, RecurseJournalPath(m.Deliberator.Workspace))
		}
	}
	return nil
}

func (m *RecurseMission) save() error {
	m.State.SpendMicroUSD, m.State.UnknownSpend, m.State.UsageReports = m.meter.snapshot()
	m.State.Prices = m.meter.prices()
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m.State, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(m.path, b, 0600)
}

func (m *RecurseMission) recordWrites(writes []observation.FileWrite) error {
	if m.State.Pending == nil {
		return fmt.Errorf("mission write receipt has no intent")
	}
	path := filepath.Join(filepath.Dir(m.path), m.State.ID, fmt.Sprintf("writes-%d.json", m.State.Pending.Cycle))
	b, err := json.Marshal(writes)
	if err != nil {
		return err
	}
	if err := atomicfile.WriteFile(path, b, 0600); err != nil {
		return err
	}
	m.State.Pending.OwnedWritesRef = path
	return m.save()
}

type missionCheckObservation struct {
	Snapshot    string   `json:"snapshot"`
	Status      string   `json:"status"`
	Command     []string `json:"command"`
	OKExitCodes []int    `json:"ok_exit_codes,omitempty"`
	ExitCode    int      `json:"exit_code"`
	Output      string   `json:"output"`
	Error       string   `json:"error,omitempty"`
}

// recordChecks retains complete runner output independently of the disposable
// attempt campaign. No model output is interpreted as empirical evidence.
func (m *RecurseMission) recordChecks(before, after map[string]gateRun, source string, efBefore, efAfter *gates.Snapshot) error {
	project := func(runs map[string]gateRun) map[string]missionCheckObservation {
		out := make(map[string]missionCheckObservation, len(runs))
		for key, run := range runs {
			row := missionCheckObservation{Snapshot: run.snapshot, Status: run.verdict(), Command: run.result.Argv,
				OKExitCodes: run.result.Gate.OKExitCodes, ExitCode: run.result.ExitCode, Output: run.result.Output}
			if run.result.Err != nil {
				row.Error = run.result.Err.Error()
			}
			out[key] = row
		}
		return out
	}
	data := struct {
		SourceAfter     string                             `json:"source_after"`
		Before          map[string]missionCheckObservation `json:"before"`
		After           map[string]missionCheckObservation `json:"after"`
		ErrorFreeBefore *gates.Snapshot                    `json:"error_free_before,omitempty"`
		ErrorFreeAfter  *gates.Snapshot                    `json:"error_free_after,omitempty"`
	}{source, project(before), project(after), efBefore, efAfter}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(m.path), m.State.ID, fmt.Sprintf("checks-%d.json", m.State.Pending.Cycle))
	if err := atomicfile.WriteFile(path, b, 0600); err != nil {
		return err
	}
	m.State.Pending.ChecksRef = path
	return m.save()
}

func (m *RecurseMission) loadVision() (*northstar.Vision, error) {
	dir := filepath.Join(m.Deliberator.Workspace, ".nerd")
	store, err := northstar.NewStore(dir)
	if err != nil {
		return nil, err
	}
	res, syncErr := northstar.SyncVisionAuthority(store, dir)
	return res.Vision, errors.Join(syncErr, store.Close())
}
