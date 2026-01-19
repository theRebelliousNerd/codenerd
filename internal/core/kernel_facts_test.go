package core

import (
	"testing"
)

func TestKernelFacts_Index(t *testing.T) {
	k := setupMockKernel(t)
	// Inject schema declarations
	k.AppendPolicy(`
	Decl user(Name, Int).
	`)
	k.Evaluate()

	// 1. Add facts with various types of arguments
	k.Assert(Fact{Predicate: "user", Args: []interface{}{"alice", 1}})
	k.Assert(Fact{Predicate: "user", Args: []interface{}{"bob", 2}})

	// 2. Add duplicate fact - should be ignored
	err := k.Assert(Fact{Predicate: "user", Args: []interface{}{"alice", 1}})
	if err != nil {
		t.Fatalf("Duplicate assert failed: %v", err)
	}

	// 3. Verify retrieval by predicate
	facts, err := k.Query("user")
	if err != nil {
		t.Fatalf("Query user failed: %v", err)
	}
	if len(facts) != 2 {
		t.Errorf("Expected 2 user facts, got %d", len(facts))
	}
}

func TestKernelFacts_Retract(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
	Decl temp(Name).
	Decl perm(Name).
	`)
	k.Evaluate()

	k.Assert(Fact{Predicate: "temp", Args: []interface{}{"a"}})
	k.Assert(Fact{Predicate: "temp", Args: []interface{}{"b"}})
	k.Assert(Fact{Predicate: "perm", Args: []interface{}{"c"}})

	// 1. Retract by predicate
	err := k.Retract("temp")
	if err != nil {
		t.Fatalf("Retract failed: %v", err)
	}

	// 2. Verify gone
	temps, _ := k.Query("temp")
	if len(temps) != 0 {
		t.Errorf("Expected 0 temp facts, got %d", len(temps))
	}

	perms, _ := k.Query("perm")
	if len(perms) != 1 {
		t.Errorf("Expected 1 perm fact, got %d", len(perms))
	}
}

func TestKernelFacts_RetractFact(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy("Decl tag(Name, Label).")
	k.Evaluate()

	// RetractFact matches Predicate + First Arg only
	k.Assert(Fact{Predicate: "tag", Args: []interface{}{"file1", "urgent"}})
	k.Assert(Fact{Predicate: "tag", Args: []interface{}{"file1", "todo"}})
	k.Assert(Fact{Predicate: "tag", Args: []interface{}{"file2", "urgent"}})

	toRetract := Fact{Predicate: "tag", Args: []interface{}{"file1"}}
	err := k.RetractFact(toRetract)
	if err != nil {
		t.Fatalf("RetractFact failed: %v", err)
	}

	remaining, _ := k.Query("tag")
	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining tag fact, got %d", len(remaining))
	} else if remaining[0].Args[0] != "file2" {
		t.Errorf("Expected file2 tag to remain, got %v", remaining[0])
	}
}
