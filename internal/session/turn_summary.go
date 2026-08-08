package session

import "fmt"

// SummarizeTurnSignals returns a one-line human-readable summary of the turn
// signals. It reports build and test status and counts of uncovered blocks and
// findings, pluralizing correctly.
func SummarizeTurnSignals(built bool, tested bool, uncovered int, findings int) string {
	buildPart := "build ok"
	if !built {
		buildPart = "build FAILED"
	}
	testPart := "tests ok"
	if !tested {
		testPart = "tests FAILED"
	}
	uncoveredPart := fmt.Sprintf("%d uncovered", uncovered)
	findingsPart := fmt.Sprintf("%d findings", findings)
	if findings == 1 {
		findingsPart = "1 finding"
	}
	return fmt.Sprintf("%s | %s | %s | %s", buildPart, testPart, uncoveredPart, findingsPart)
}
