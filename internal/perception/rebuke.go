package perception

import (
	"regexp"
	"strings"
)

// maxComplaintWords is the longest message a bare complaint word can make a
// rebuke. A request that merely mentions a failure ("Mean in stats.go is wrong
// - go test says TestMean fails ...") is a task description, not a rebuke of
// the assistant.
const maxComplaintWords = 6

var (
	// rebukePhrases are addressed to the assistant: a rebuke at any length.
	rebukePhrases = []string{"bad bot", "no that's not right", "you didn't"}
	// complaintWords are a rebuke only in a short message.
	complaintWords = regexp.MustCompile(`(?i)\b(wrong|incorrect|mistake)\b`)
)

// IsRebuke reports a rebuke of the assistant's last answer, which starts the
// learning loop in place of the turn. "stop" and "fail" are not rebukes: a
// bare "stop" is the user stopping running work (the kernel reads it from
// Jev's wants_to_stop judgment), and "the tests fail" is a report.
func IsRebuke(input string) bool {
	lower := strings.ToLower(input)
	for _, p := range rebukePhrases {
		if strings.Contains(lower, p) {
			return true
		}
	}
	if len(strings.Fields(input)) > maxComplaintWords {
		return false
	}
	return complaintWords.MatchString(input)
}
