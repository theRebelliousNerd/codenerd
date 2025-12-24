package nemesis

import (
	"path/filepath"
	"testing"
	"time"
)

func TestArmoryAddAndLoad(t *testing.T) {
	dir := t.TempDir()
	armory := NewArmory(dir)

	attack := ArmoryAttack{
		Name:     "attack-one",
		Category: "logic",
	}
	armory.AddAttack(attack)

	if _, ok := armory.GetAttackByName("attack-one"); !ok {
		t.Fatalf("expected attack to be present")
	}

	reloaded := NewArmory(dir)
	if _, ok := reloaded.GetAttackByName("attack-one"); !ok {
		t.Fatalf("expected attack to be persisted")
	}

	stats := reloaded.GetStats()
	if stats.TotalAttacks != 1 {
		t.Fatalf("TotalAttacks = %d, want 1", stats.TotalAttacks)
	}
}

func TestArmoryRecordRun(t *testing.T) {
	dir := t.TempDir()
	armory := NewArmory(dir)

	armory.AddAttack(ArmoryAttack{
		Name:     "attack-run",
		Category: "concurrency",
	})

	armory.RecordRun("attack-run", false)
	attack, ok := armory.GetAttackByName("attack-run")
	if !ok {
		t.Fatalf("expected attack to be present")
	}
	if attack.RunCount < 2 {
		t.Fatalf("expected RunCount >= 2, got %d", attack.RunCount)
	}

	armory.RecordRun("attack-run", true)
	attack, _ = armory.GetAttackByName("attack-run")
	if attack.SuccessCount < 2 {
		t.Fatalf("expected SuccessCount >= 2, got %d", attack.SuccessCount)
	}
}

func TestArmoryPruneStaleAttacks(t *testing.T) {
	dir := t.TempDir()
	armory := NewArmory(dir)

	old := time.Now().Add(-40 * 24 * time.Hour)
	armory.attacks = []ArmoryAttack{
		{Name: "stale", LastSuccess: old, SuccessCount: 1},
		{Name: "kept", LastSuccess: old, SuccessCount: 3},
	}
	armory.stats.TotalAttacks = len(armory.attacks)

	pruned := armory.PruneStaleAttacks(30)
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}
	if len(armory.attacks) != 1 || armory.attacks[0].Name != "kept" {
		t.Fatalf("unexpected remaining attacks: %+v", armory.attacks)
	}
}

func TestArmoryExportForRegression(t *testing.T) {
	dir := t.TempDir()
	armory := NewArmory(dir)
	armory.AddAttack(ArmoryAttack{
		Name:          "attack-export",
		Category:      "resource",
		Vulnerability: "oom",
		BinaryPath:    filepath.Join(dir, "attack.bin"),
	})

	exports := armory.ExportForRegression()
	if len(exports) != 1 {
		t.Fatalf("exports len = %d, want 1", len(exports))
	}
	if exports[0]["name"] != "attack-export" {
		t.Fatalf("unexpected export: %+v", exports[0])
	}
}
