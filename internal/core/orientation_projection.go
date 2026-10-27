package core

import (
	"fmt"
	"os"
	"strings"
)

// loadOrientationProjection splits the orientation projection (.nerd/
// orientation/orientation.mg) into its declarations and its rows. The rows are
// returned as EDB facts, not program text: an incremental scan refreshes them
// with an exact retract (orient.RefreshResult.Apply), and a row compiled into
// the program is a clause no retract can remove, so every refresh would stack
// a new head, document digest and role on top of the stale ones.
func loadOrientationProjection(path string) (string, []Fact, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var decls, rows strings.Builder
	rowCount := 0
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			continue
		case strings.HasPrefix(trimmed, "Decl "):
			decls.WriteString(trimmed)
			decls.WriteString("\n")
		default:
			rows.WriteString(trimmed)
			rows.WriteString("\n")
			rowCount++
		}
	}
	if rowCount == 0 {
		return decls.String(), nil, nil
	}
	facts, err := ParseFactsFromString(rows.String())
	if err != nil {
		return "", nil, fmt.Errorf("orientation projection %s: %w", path, err)
	}
	if len(facts) != rowCount {
		return "", nil, fmt.Errorf("orientation projection %s: %d of %d rows are ground facts; the projection carries rows only", path, len(facts), rowCount)
	}
	return decls.String(), facts, nil
}
