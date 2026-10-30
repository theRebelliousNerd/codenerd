package session

import "testing"

// A reply in neither form read as a clean review, and a stock NO FINDINGS
// line erased the findings beside it. The parser now says which reply it was.
func TestParseCriticFindings_TellsSoundFromUnparseable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		want     criticVerdict
		findings int
	}{
		{"an explicit clean review", "NO FINDINGS", verdictSound, 0},
		{"a reply in neither form", "The change looks reasonable overall.", verdictUnparseable, 0},
		{"an empty reply", "", verdictUnparseable, 0},
		{"findings", "FINDING a.go:3 high: off by one", verdictFindings, 1},
		{"findings beside NO FINDINGS", "FINDING a.go:3 high: off by one\nNO FINDINGS", verdictContradictory, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, verdict := parseCriticFindings(tc.response)
			if verdict != tc.want || len(got) != tc.findings {
				t.Fatalf("parseCriticFindings(%q) = %d finding(s), %q; want %d, %q", tc.response, len(got), verdict, tc.findings, tc.want)
			}
		})
	}
}
