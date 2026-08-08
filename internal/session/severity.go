package session

// SeverityAtLeast reports whether sev ranks at or above min.
//
// Ranking is defined by CriticSeverityRank: high=3, medium=2, low=1,
// anything else=0. Comparison is rank(sev) >= rank(min) so that unknown
// severities rank lowest and equal severities satisfy the threshold.
func SeverityAtLeast(sev string, min string) bool {
	return CriticSeverityRank(sev) >= CriticSeverityRank(min)
}
