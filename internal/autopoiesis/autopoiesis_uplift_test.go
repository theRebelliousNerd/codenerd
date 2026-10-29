package autopoiesis

import (
	"strings"
	"testing"
	"time"
)

// FormatBattleResultForFeedback is the live Thunderdome-to-generator channel:
// its text is what the ToolGenerator reads when a tool dies in the arena.
// Pin both branches plus the degenerate defeated shape (no fatal attack
// recorded), which must format instead of panicking the turn.

func TestFormatBattleResultForFeedback_Survived(t *testing.T) {
	td := NewThunderdomeWithConfig(DefaultThunderdomeConfig())
	got := td.FormatBattleResultForFeedback(&BattleResult{
		ToolName:     "json_tool",
		Survived:     true,
		TotalAttacks: 7,
		Duration:     3 * time.Second,
	})
	for _, want := range []string{"SURVIVED", "json_tool", "7 attacks defended"} {
		if !strings.Contains(got, want) {
			t.Errorf("survived feedback missing %q:\n%s", want, got)
		}
	}
}

func TestFormatBattleResultForFeedback_Defeated_CarriesKillDetail(t *testing.T) {
	td := NewThunderdomeWithConfig(DefaultThunderdomeConfig())
	got := td.FormatBattleResultForFeedback(&BattleResult{
		ToolName: "json_tool",
		Survived: false,
		Results: []AttackResult{{
			Vector:    AttackVector{Name: "nil_deref", Category: "nil_pointer", Input: "{}"},
			Survived:  false,
			Failure:   "panic: runtime error",
			StackDump: "goroutine 1 [running]: ...",
		}},
		FatalAttack: &AttackVector{Name: "nil_deref", Category: "nil_pointer", Input: "{}"},
	})
	for _, want := range []string{
		"DEFEATED", "json_tool", "nil_deref", "nil_pointer",
		"panic: runtime error", "goroutine 1", "must be regenerated",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("defeated feedback missing %q:\n%s", want, got)
		}
	}
}

func TestFormatBattleResultForFeedback_DefeatedDegenerate_DoesNotPanic(t *testing.T) {
	td := NewThunderdomeWithConfig(DefaultThunderdomeConfig())
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("degenerate defeated result panicked: %v", r)
		}
	}()
	got := td.FormatBattleResultForFeedback(&BattleResult{
		ToolName: "json_tool",
		Survived: false,
	})
	for _, want := range []string{"DEFEATED", "json_tool", "must be regenerated"} {
		if !strings.Contains(got, want) {
			t.Errorf("degenerate feedback missing %q:\n%s", want, got)
		}
	}
}
