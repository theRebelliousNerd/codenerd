package usage

import "testing"

func TestTokenCounts_Add(t *testing.T) {
	tc := &TokenCounts{}

	// Test case 1: Initial add
	tc.Add(10, 20)
	if tc.Input != 10 {
		t.Errorf("expected Input 10, got %d", tc.Input)
	}
	if tc.Output != 20 {
		t.Errorf("expected Output 20, got %d", tc.Output)
	}
	if tc.Total != 30 {
		t.Errorf("expected Total 30, got %d", tc.Total)
	}

	// Test case 2: Subsequent add
	tc.Add(5, 15)
	if tc.Input != 15 {
		t.Errorf("expected Input 15, got %d", tc.Input)
	}
	if tc.Output != 35 {
		t.Errorf("expected Output 35, got %d", tc.Output)
	}
	if tc.Total != 50 {
		t.Errorf("expected Total 50, got %d", tc.Total)
	}

	// Test case 3: Add zeros
	tc.Add(0, 0)
	if tc.Input != 15 {
		t.Errorf("expected Input 15, got %d", tc.Input)
	}
	if tc.Output != 35 {
		t.Errorf("expected Output 35, got %d", tc.Output)
	}
	if tc.Total != 50 {
		t.Errorf("expected Total 50, got %d", tc.Total)
	}

	// Test case 4: Add negative numbers
	tc.Add(-5, -10)
	if tc.Input != 10 {
		t.Errorf("expected Input 10, got %d", tc.Input)
	}
	if tc.Output != 25 {
		t.Errorf("expected Output 25, got %d", tc.Output)
	}
	if tc.Total != 35 {
		t.Errorf("expected Total 35, got %d", tc.Total)
	}
}
