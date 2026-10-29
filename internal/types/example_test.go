package types_test

import (
	"fmt"

	"codenerd/internal/types"
	"codenerd/internal/types/typestest"
)

// ExampleFact_ToAtom shows how a Go value's TYPE, not its spelling, decides the
// Mangle constant that reaches the kernel. Getting this wrong is silent: a
// name-shaped value passed as a plain string becomes a name, and a name passed
// as a MangleString stays a string, and neither errors.
func ExampleFact_ToAtom() {
	fact := types.Fact{
		Predicate: "tool_registered",
		Args: []any{
			"ripgrep",                    // plain text -> string constant
			types.MangleAtom("/active"),  // explicit name constant
			types.MangleString("/1.2.3"), // name-SHAPED, forced to string
			int64(3),                     // -> number
			true,                         // -> the /true name constant
		},
	}

	atom, err := fact.ToAtom()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(atom.String())
	// Output: tool_registered("ripgrep",/active,"/1.2.3",3,/true)
}

// ExampleFact_ToAtom_container shows the container encoding: Mangle has no
// compound constant, so maps and slices are stored as JSON string constants.
func ExampleFact_ToAtom_container() {
	fact := types.Fact{
		Predicate: "pending_action",
		Args: []any{
			"act-1",
			map[string]any{"path": "main.go", "confirmed": true},
		},
	}
	atom, _ := fact.ToAtom()
	fmt.Println(atom.String())
	// Output: pending_action("act-1","{\"confirmed\":true,\"path\":\"main.go\"}")
}

// ExampleFact_ToAtom_unsupported shows the loud failure. A struct pointer used
// to be coerced with %v into something like "0x7ff63be770e0", which then killed
// the whole evaluation the first time a numeric builtin touched it.
func ExampleFact_ToAtom_unsupported() {
	type payload struct{ n int }
	_, err := types.Fact{Predicate: "bad", Args: []any{&payload{}}}.ToAtom()
	fmt.Println(err != nil)
	// Output: true
}

// ExampleNewKernelTx shows the multi-operation update pattern: buffer every
// retract and assert, then commit once. The alternative — calling Retract and
// Assert directly — rebuilds and re-evaluates the whole program per call, and
// leaves the kernel observable in the half-updated state between them.
//
// NewKernelTx panics if the kernel does not implement KernelTransactor; use
// types.TransactorOf when the kernel's provenance is unknown.
func ExampleNewKernelTx() {
	kernel := typestest.NewMockKernel() // any types.Kernel + KernelTransactor

	tx := types.NewKernelTx(kernel)
	tx.Retract("shard_state")
	tx.Assert(types.Fact{Predicate: "shard_state", Args: []any{"coder", types.MangleAtom("/running")}})
	tx.Assert(types.Fact{Predicate: "shard_state", Args: []any{"tester", types.MangleAtom("/idle")}})

	fmt.Println("visible before commit:", kernel.FactCount("shard_state"))
	if err := tx.Commit(); err != nil {
		fmt.Println("commit failed:", err)
		return
	}
	fmt.Println("visible after commit:", kernel.FactCount("shard_state"))
	// Output:
	// visible before commit: 0
	// visible after commit: 2
}

// ExampleTransactorOf shows the non-panicking capability check, for code holding
// a Kernel it did not construct — a forwarding adapter may have dropped the
// capability on the way in.
func ExampleTransactorOf() {
	var k types.Kernel = typestest.NewMockKernel()
	if _, ok := types.TransactorOf(k); ok {
		fmt.Println("atomic updates available")
	} else {
		fmt.Println("fall back to individual asserts")
	}
	// Output: atomic updates available
}
