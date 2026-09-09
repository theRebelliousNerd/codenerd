package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// injectable_context and specialist_knowledge become ONE synthetic atom each,
// both IsMandatory, so Fit skips category allocation for them and only rejects
// them when the merged atom exceeds the entire budget. That turns growth into
// a cliff: crossing the line drops the whole northstar with no marker. These
// bounds convert the cliff into a graduated, visible loss.
func TestRenderKernelContextBlock(t *testing.T) {
	tests := []struct {
		name       string
		rows       []string
		wantMarker bool
		wantFirst  bool
	}{
		{
			name:      "a small set is emitted whole",
			rows:      []string{"mission: ship it", "constraint: no floats in mangle"},
			wantFirst: true,
		},
		{
			name:       "too many rows are capped with a marker",
			rows:       repeatRows("risk", maxKernelContextRows*4),
			wantMarker: true,
			wantFirst:  true,
		},
		{
			name:       "one enormous row is clamped, not dropped",
			rows:       []string{"FIRST" + strings.Repeat("y", maxKernelContextRowChars*8)},
			wantMarker: true,
			wantFirst:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderKernelContextBlock(tt.rows)

			if len(got) > maxKernelInjectedAtomChars+512 {
				t.Errorf("block is %d chars, cap is %d", len(got), maxKernelInjectedAtomChars)
			}
			if IsClamped(got) != tt.wantMarker {
				t.Errorf("IsClamped = %v, want %v", IsClamped(got), tt.wantMarker)
			}
			if !strings.Contains(got, "KERNEL-INJECTED CONTEXT") {
				t.Error("block header was lost")
			}
			if tt.wantFirst && !strings.Contains(got, strings.SplitN(tt.rows[0], "y", 2)[0]) {
				t.Error("the first row must survive: it is the highest-activation fact")
			}
		})
	}
}

// A merged block that exceeds the whole budget is rejected outright by Fit
// ("Mandatory atom %s rejected"), taking every row with it. The construction
// cap must keep the block small enough that this never happens at a realistic
// shard budget.
func TestRenderKernelContextBlock_SurvivesFitAtShardBudget(t *testing.T) {
	const shardBudget = 8192

	content := renderKernelContextBlock(repeatRows("constraint", 5000))
	atom := NewPromptAtom("kernel/context/test", CategoryContext, content)
	atom.IsMandatory = true

	fitted, err := NewTokenBudgetManager().Fit(
		[]*OrderedAtom{{Atom: atom, Score: 95, RenderMode: "standard"}}, shardBudget)
	if err != nil {
		t.Fatalf("Fit: %v", err)
	}
	if len(fitted) != 1 {
		t.Fatalf("kernel context block was rejected wholesale at a %d-token budget; "+
			"the construction cap exists so this cannot happen", shardBudget)
	}
}

// The specialist roster is interpolated into {{available_specialists}} after
// Fit has charged tokens for the placeholder, so it is unaccounted by
// construction. .nerd/agents.json grows monotonically and nothing prunes it.
func TestFormatSpecialists_IsBounded(t *testing.T) {
	tests := []struct {
		name       string
		agents     int
		descChars  int
		wantMarker bool
	}{
		{name: "empty registry falls back to core shards", agents: 0},
		{name: "a few agents are listed whole", agents: 3, descChars: 40},
		{name: "a large registry is capped", agents: 500, descChars: 40, wantMarker: true},
		{name: "one verbose description is clamped", agents: 2, descChars: 50000, wantMarker: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Built through the registry's own JSON contract so the test
			// exercises the same decode path .nerd/agents.json takes.
			entries := make([]string, 0, tt.agents)
			for i := 0; i < tt.agents; i++ {
				entries = append(entries, fmt.Sprintf(
					`{"name":"agent%d","type":"domain","status":"ready","description":%q}`,
					i, strings.Repeat("d", tt.descChars)))
			}
			var reg agentRegistry
			if err := json.Unmarshal([]byte(`{"agents":[`+strings.Join(entries, ",")+`]}`), &reg); err != nil {
				t.Fatalf("registry fixture: %v", err)
			}

			got := formatSpecialists(reg)

			lines := strings.Count(got, "\n") + 1
			if lines > maxSpecialistEntries+2 {
				t.Errorf("roster has %d lines, cap is %d", lines, maxSpecialistEntries)
			}
			if tt.wantMarker && !IsClamped(got) {
				t.Error("a capped roster must say so")
			}
			if got == "" {
				t.Error("roster must never be empty; the model needs to know who it can consult")
			}
		})
	}
}

func repeatRows(prefix string, n int) []string {
	rows := make([]string, n)
	for i := range rows {
		rows[i] = prefix + ": row content"
	}
	rows[0] = "FIRST: " + prefix
	return rows
}
