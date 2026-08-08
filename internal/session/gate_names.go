package session

// GateName returns the human-readable name for a verification gate index.
// Indices match the order in which the four verification gates run in runToolLoop.
func GateName(i int) string {
	switch i {
	case 0:
		return "build"
	case 1:
		return "test"
	case 2:
		return "coverage"
	case 3:
		return "critic"
	default:
		return "unknown"
	}
}
