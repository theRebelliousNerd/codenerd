package session

import "testing"

func TestSeverityAtLeast(t *testing.T) {
	cases := []struct {
		name string
		sev  string
		min  string
		want bool
	}{
		// Equal ranks satisfy the threshold.
		{name: "high at least high", sev: "high", min: "high", want: true},
		{name: "medium at least medium", sev: "medium", min: "medium", want: true},
		{name: "low at least low", sev: "low", min: "low", want: true},
		{name: "unknown at least unknown", sev: "unknown", min: "unknown", want: true},
		{name: "empty at least empty", sev: "", min: "", want: true},

		// Higher rank satisfies lower threshold.
		{name: "high at least medium", sev: "high", min: "medium", want: true},
		{name: "high at least low", sev: "high", min: "low", want: true},
		{name: "medium at least low", sev: "medium", min: "low", want: true},
		{name: "high at least unknown", sev: "high", min: "unknown", want: true},
		{name: "low at least unknown", sev: "low", min: "unknown", want: true},
		{name: "low at least empty", sev: "low", min: "", want: true},

		// Lower rank does not satisfy higher threshold.
		{name: "low at least medium", sev: "low", min: "medium", want: false},
		{name: "low at least high", sev: "low", min: "high", want: false},
		{name: "medium at least high", sev: "medium", min: "high", want: false},
		{name: "unknown at least low", sev: "unknown", min: "low", want: false},
		{name: "unknown at least high", sev: "unknown", min: "high", want: false},
		{name: "empty at least low", sev: "", min: "low", want: false},

		// Normalization: case-insensitive and whitespace-trimmed via CriticSeverityRank.
		{name: "case insensitive sev", sev: "HIGH", min: "high", want: true},
		{name: "case insensitive min", sev: "high", min: "HIGH", want: true},
		{name: "whitespace trimmed sev", sev: " high ", min: "high", want: true},
		{name: "whitespace trimmed min", sev: "high", min: " high ", want: true},
		{name: "medium lower than High case insensitive", sev: "medium", min: "HIGH", want: false},
		{name: "Mixed case Medium at least medium", sev: "Medium", min: "medium", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SeverityAtLeast(tc.sev, tc.min)
			if got != tc.want {
				t.Errorf("SeverityAtLeast(%q, %q) = %v; want %v", tc.sev, tc.min, got, tc.want)
			}
		})
	}
}
