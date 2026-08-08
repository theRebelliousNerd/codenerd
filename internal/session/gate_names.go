package session

// GateName returns the human-readable name for a verification gate index.
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
