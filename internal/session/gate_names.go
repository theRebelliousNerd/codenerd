package session

// Gate indices — must match the order of the verification signals a turn
// passes through in runToolLoop. GateName is the only reader today; the
// constants exist so a future caller uses a name rather than a magic number.
const (
	GateBuild    = 0
	GateTest     = 1
	GateCoverage = 2
	GateCritic   = 3

	// GateCount is the number of defined verification gates.
	GateCount = 4
)

// gateNames is the canonical ordered list of gate names. Index positions
// correspond to the Gate* constants above.
var gateNames = [GateCount]string{
	GateBuild:    "build",
	GateTest:     "test",
	GateCoverage: "coverage",
	GateCritic:   "critic",
}

// Compile-time assertion that GateCount matches the length of gateNames.
// If they diverge the array length becomes negative and the build fails.
var _ [len(gateNames) - GateCount]struct{}
var _ [GateCount - len(gateNames)]struct{}

// GateName returns the human-readable name for a verification gate index.
// Out-of-range indices, including negatives, return "unknown".
//
// The indices name the four verification signals a turn passes through. They
// are not four separate calls in runToolLoop: that function invokes build
// (verifyAndRepairBuild), test (verifyAndRepairTests) and critic
// (verifyAndUpliftWithCritic), and coverage is produced inside the test gate by
// verifyTestsWithCoverage, which runs the suite once and returns both the
// verdict and the uncovered blocks. The order build → test+coverage → critic
// holds; the count of four matches SummarizeTurnSignals.
func GateName(i int) string {
	if i < 0 || i >= len(gateNames) {
		return "unknown"
	}
	return gateNames[i]
}
