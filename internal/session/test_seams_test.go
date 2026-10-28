package session

import "io"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// parseCoverProfile parses a Go coverage profile from r and returns only the
// blocks whose count is 0 and whose file path has a suffix matching one of
// writtenFiles. parseCoverBlocks is the same parse without the count filter:
// the element mapping needs the blocks this run did execute.
func parseCoverProfile(r io.Reader, writtenFiles []string) ([]UncoveredBlock, error) {
	blocks, err := parseCoverBlocks(r, writtenFiles)
	if err != nil {
		return nil, err
	}
	return uncoveredStatementBlocks(blocks), nil
}

func (e *Executor) checkSafetyWithGate(call ToolCall, safetyGateEnabled bool) (bool, string) {
	v := e.checkSafetyWithLease(call, safetyGateEnabled, "", nil, nil)
	return v.allowed, v.reason
}

// RepairWriteGuard confines a repair episode to the turn's write set: the
// files the turn wrote before the first gate ran. Dogfood run 5 (2026-09-29):
// a repair loop added //go:build ignore to another agent's files to make a
// workspace build pass. A write to an existing file outside the set is
// refused with the set named, so the model reads where it may edit; files
// the episode itself creates are allowed, because the coverage and pinning
// rounds write their tests inside the episode (change_gates.go,
// pin_gate.go) and a static set would refuse the round's own work. Creating
// cannot destroy another agent's bytes -- the file did not exist -- while an
// overwrite outside the set is exactly run 5. parent, when non-nil, is asked
// first and its refusal wins.
func RepairWriteGuard(workspace string, writeSet []string, parent WriteGuard) WriteGuard {
	return repairWriteGuard(workspace, writeSet, nil, false, parent)
}
