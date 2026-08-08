package session

import (
	"math"
	"testing"
)

func TestGateName_ValidIndices_ReturnsExpectedName(t *testing.T) {
	tests := map[string]struct {
		i    int
		want string
	}{
		"GateBuild via literal 0":    {i: 0, want: "build"},
		"GateTest via literal 1":     {i: 1, want: "test"},
		"GateCoverage via literal 2": {i: 2, want: "coverage"},
		"GateCritic via literal 3":   {i: 3, want: "critic"},
		"GateBuild constant":         {i: GateBuild, want: "build"},
		"GateTest constant":          {i: GateTest, want: "test"},
		"GateCoverage constant":      {i: GateCoverage, want: "coverage"},
		"GateCritic constant":        {i: GateCritic, want: "critic"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := GateName(tt.i)
			if got != tt.want {
				t.Errorf("GateName(%d) = %q; want %q", tt.i, got, tt.want)
			}
		})
	}
}

func TestGateName_InvalidIndices_ReturnsUnknown(t *testing.T) {
	tests := map[string]struct {
		i int
	}{
		"negative one":       {i: -1},
		"negative two":       {i: -2},
		"large negative":     {i: -100},
		"min int":            {i: math.MinInt},
		"exactly GateCount":  {i: GateCount},
		"GateCount plus one": {i: GateCount + 1},
		"large positive 100": {i: 100},
		"max int":            {i: math.MaxInt},
		"just beyond critic": {i: 4},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := GateName(tt.i)
			if got != "unknown" {
				t.Errorf("GateName(%d) = %q; want %q", tt.i, got, "unknown")
			}
		})
	}
}

func TestGateName_Boundaries_CorrectBehavior(t *testing.T) {
	tests := []struct {
		name string
		i    int
		want string
	}{
		{"lower bound valid 0", 0, "build"},
		{"upper bound valid 3", 3, "critic"},
		{"just below lower bound -1", -1, "unknown"},
		{"just above upper bound 4", 4, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GateName(tt.i)
			if got != tt.want {
				t.Errorf("GateName(%d) = %q; want %q", tt.i, got, tt.want)
			}
		})
	}
}

func TestGateConstants_Values_AreContiguousAndCorrect(t *testing.T) {
	if GateBuild != 0 {
		t.Errorf("GateBuild = %d; want 0", GateBuild)
	}
	if GateTest != 1 {
		t.Errorf("GateTest = %d; want 1", GateTest)
	}
	if GateCoverage != 2 {
		t.Errorf("GateCoverage = %d; want 2", GateCoverage)
	}
	if GateCritic != 3 {
		t.Errorf("GateCritic = %d; want 3", GateCritic)
	}
	if GateCount != 4 {
		t.Errorf("GateCount = %d; want 4", GateCount)
	}
	// Verify contiguity: each successive gate is +1.
	if GateTest != GateBuild+1 {
		t.Errorf("GateTest (%d) should be GateBuild+1 (%d)", GateTest, GateBuild+1)
	}
	if GateCoverage != GateTest+1 {
		t.Errorf("GateCoverage (%d) should be GateTest+1 (%d)", GateCoverage, GateTest+1)
	}
	if GateCritic != GateCoverage+1 {
		t.Errorf("GateCritic (%d) should be GateCoverage+1 (%d)", GateCritic, GateCoverage+1)
	}
	if GateCount != GateCritic+1 {
		t.Errorf("GateCount (%d) should be GateCritic+1 (%d)", GateCount, GateCritic+1)
	}
}

func TestGateConstants_Count_MatchesGateNamesLength(t *testing.T) {
	if GateCount != len(gateNames) {
		t.Errorf("GateCount (%d) != len(gateNames) (%d)", GateCount, len(gateNames))
	}
	if len(gateNames) != 4 {
		t.Errorf("len(gateNames) = %d; want 4", len(gateNames))
	}
}

func TestGateNames_ArrayContents_AreCorrectAndUnique(t *testing.T) {
	expected := map[int]string{
		GateBuild:    "build",
		GateTest:     "test",
		GateCoverage: "coverage",
		GateCritic:   "critic",
	}
	seen := make(map[string]int)
	for idx, want := range expected {
		got := gateNames[idx]
		if got != want {
			t.Errorf("gateNames[%d] = %q; want %q", idx, got, want)
		}
		if got == "" {
			t.Errorf("gateNames[%d] is empty; want non-empty", idx)
		}
		if got == "unknown" {
			t.Errorf("gateNames[%d] = %q collides with out-of-range sentinel", idx, got)
		}
		if prevIdx, ok := seen[got]; ok {
			t.Errorf("duplicate gate name %q at indices %d and %d", got, prevIdx, idx)
		}
		seen[got] = idx
	}
	if len(seen) != GateCount {
		t.Errorf("unique gate names = %d; want %d", len(seen), GateCount)
	}
}

func TestGateName_UnknownSentinel_DoesNotCollideWithValidNames(t *testing.T) {
	validNames := make(map[string]bool)
	for i := 0; i < GateCount; i++ {
		validNames[GateName(i)] = true
	}
	if validNames["unknown"] {
		t.Error("GateName valid range returned \"unknown\", which is reserved for out-of-range")
	}
	// Invalid indices must always return exactly "unknown", never empty.
	invalids := []int{-1, GateCount, math.MinInt, math.MaxInt}
	for _, idx := range invalids {
		got := GateName(idx)
		if got == "" {
			t.Errorf("GateName(%d) returned empty string; want \"unknown\"", idx)
		}
		if got != "unknown" {
			t.Errorf("GateName(%d) = %q; want \"unknown\"", idx, got)
		}
	}
}

func TestGateName_AllValidIndices_NeverReturnsUnknownOrEmpty(t *testing.T) {
	for i := 0; i < GateCount; i++ {
		t.Run(GateName(i), func(t *testing.T) {
			got := GateName(i)
			if got == "unknown" {
				t.Errorf("GateName(%d) returned \"unknown\" but %d is a valid gate index", i, i)
			}
			if got == "" {
				t.Errorf("GateName(%d) returned empty string", i)
			}
		})
	}
}
