package regression

import (
	"codenerd/internal/types"
)

// Constitutional predicate names this package projects into the kernel.
// They are declared in internal/core/defaults/schemas_safety.mg (SECTION 24)
// and consumed by internal/core/defaults/policy/regression_battery.mg.
//
// Every argument of both predicates is declared /string, so plain Go strings
// are the correct constant type here — a MangleAtom in one of these slots
// would land as a NameType constant and never unify with the rules.
const (
	// PredBatteryDeclared is asserted once per battery submitted for a decision.
	PredBatteryDeclared = "regression_battery_declared"
	// PredBatteryTask is asserted once per shell task, carrying the exact
	// command text the shell would receive.
	PredBatteryTask = "regression_battery_task"
	// PredBatteryPermitted is the derived gate a host must query before running
	// a battery on an agent's behalf.
	PredBatteryPermitted = "regression_battery_permitted"
	// PredBatteryRefused carries the reason a declared battery may not run.
	PredBatteryRefused = "regression_battery_refused"
)

// ActionRunBattery is the action name a future agent-facing host would submit.
//
// It is deliberately NOT a safe_action: policy/regression_battery.mg registers
// it via requires_permission, which the constitution turns into a
// dangerous_action. A battery is a file of shell commands, and the
// constitution's content gate only inspects an action's target and payload —
// so an allowlisted battery action would let anything that can write a file
// launder a blocked command past the entire blocked_pattern list. Nothing in
// the tree routes this action today; the constant exists so a host that adds
// one starts from the gated name rather than inventing an ungated one.
const ActionRunBattery = "/run_regression_battery"

// PolicyFacts projects a battery into the EDB facts the kernel needs to decide
// whether it may run. The decision itself lives in Mangle, not here: this
// function only makes the battery's contents visible to the rules, because a
// path alone tells the constitution nothing about the commands behind it.
//
// batteryPath identifies the battery in the derived facts and must match the
// path the host queries regression_battery_permitted/1 with.
//
// A nil or empty battery still yields the declaration fact. That is deliberate:
// declaring an empty battery lets regression_battery_refused derive
// "battery declares no tasks" instead of the host silently deriving nothing and
// being unable to say why.
//
// Order is load-bearing. The task facts come first and the declaration LAST,
// because the kernel evaluates incrementally and never retracts a derived fact:
// asserting the declaration first derives
// regression_battery_refused(Path, "battery declares no tasks") in that instant,
// and the later task facts cannot take it back. Asserting the whole slice in one
// batch is also correct — a batch is evaluated once — but a host that asserts
// fact by fact only gets the right answer with this ordering, so the ordering is
// the contract rather than the batching.
func PolicyFacts(batteryPath string, b *Battery) []types.Fact {
	tasks := taskSlice(b)
	facts := make([]types.Fact, 0, 1+len(tasks))
	for _, task := range tasks {
		facts = append(facts, types.Fact{
			Predicate: PredBatteryTask,
			Args:      []any{batteryPath, task.ID, task.Command},
		})
	}
	// The declaration is the commit point: everything the gate needs to see is
	// already in the store by the time it lands.
	facts = append(facts, types.Fact{
		Predicate: PredBatteryDeclared,
		Args:      []any{batteryPath},
	})
	return facts
}

func taskSlice(b *Battery) []Task {
	if b == nil {
		return nil
	}
	return b.Tasks
}
