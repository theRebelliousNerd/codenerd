package system

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

func TestProductionCompilerConsumesSelectedExpertKnowledge(t *testing.T) {
	cortex := bootKernelForShardModeTest(t, true)
	defer cortex.Close()
	compiler := cortex.JITCompiler
	if compiler == nil {
		t.Fatal("production factory omitted JIT compiler")
	}
	cc := prompt.NewCompilationContext().WithShard("bridgeexpert", "bridgeexpert", "").
		WithSemanticQuery("zephyrquark calibration", 5).WithTokenBudget(20000, 1000)
	before, err := compiler.Compile(t.Context(), cc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(before.Prompt, "expert-only-bounded-witness") {
		t.Fatal("fabricated source before registration")
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "expert.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE knowledge_atoms(concept TEXT, content TEXT, confidence REAL);
INSERT INTO knowledge_atoms VALUES('guide/Z12','zephyrquark calibration expert-only-bounded-witness',0.4)`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	compiler.RegisterShardDB("bridgeexpert", db)
	after, err := compiler.Compile(t.Context(), cc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after.Prompt, "expert-only-bounded-witness") || !strings.Contains(after.Prompt, "confidence=0.4") {
		t.Fatalf("production selection/assembly dropped retrieved source: included %d atoms", len(after.IncludedAtoms))
	}
	sibling, err := compiler.Compile(t.Context(), prompt.NewCompilationContext().WithShard("sibling", "sibling", "").
		WithSemanticQuery("zephyrquark calibration", 5).WithTokenBudget(20000, 1000))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sibling.Prompt, "expert-only-bounded-witness") {
		t.Fatal("source leaked into sibling compiled prompt")
	}
}
