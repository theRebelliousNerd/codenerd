package core

import (
	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
)

// writeMutationActions are the ActionTypes that land durable changes and are
// therefore subject to nerd.md write protection.
//
// Kept as a set rather than a switch so it reads as data next to the ActionType
// constants it mirrors (virtual_store_types.go). Reading a protected file is
// fine and often necessary — the point is to stop the agent editing it.
var writeMutationActions = map[ActionType]struct{}{
	ActionWriteFile:   {},
	ActionEditFile:    {},
	ActionDeleteFile:  {},
	ActionEditLines:   {},
	ActionInsertLines: {},
	ActionDeleteLines: {},
	ActionEditElement: {},
	ActionFSWrite:     {},
}

// projectForbidsWrite asks the kernel whether nerd.md protects this request's
// target. It mirrors session.Executor.projectForbidsWrite and shares its
// matching logic via projectdoc.ForbiddenByKernel, so the two gates cannot
// disagree about what "forbidden" means.
func (v *VirtualStore) projectForbidsWrite(req ActionRequest) (string, bool) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()
	return projectForbidsWriteWith(kernel, req)
}

// projectForbidsWriteWith is the gate itself, taking only the kernel slice it
// needs. Split out from the method so it can be exercised without standing up
// the whole Kernel interface — a gate that is awkward to test is a gate that
// gets tested loosely.
func projectForbidsWriteWith(q projectdoc.FactQuerier, req ActionRequest) (string, bool) {
	if _, isWrite := writeMutationActions[req.Type]; !isWrite {
		return "", false
	}
	if req.Target == "" {
		return "write target is missing", true
	}
	if q == nil {
		return "nerd.md write protection authority is unavailable", true
	}

	reason, forbidden, err := projectdoc.ForbiddenByKernel(q, req.Target)
	if err != nil {
		// Fail closed: an unavailable policy authority cannot prove a write is
		// allowed. Reads are not classified here and remain available.
		reason := "nerd.md write protection could not be evaluated: " + err.Error()
		logging.VirtualStoreWarn(
			"nerd.md blocked write to %s because protection could not be evaluated (%v)",
			req.Target, err)
		logging.Audit().SafetyCheck("nerd.md_write_guard", false, reason)
		return reason, true
	}
	return reason, forbidden
}
