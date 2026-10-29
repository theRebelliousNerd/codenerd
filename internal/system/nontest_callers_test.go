package system

import (
	"os"
	"path/filepath"
	"strings"
)

// nonTestCallersOf reports the non-test Go files under internal/ and cmd/ that
// mention symbol as a call.
//
// Deliberately textual rather than type-aware: the property being defended is
// "somebody in production reads this", and a grep is enough to catch the
// failure mode — a switch that exists, is documented, is displayed, and is
// never consulted.
func nonTestCallersOf(symbol string) ([]string, error) {
	roots := []string{"..", "../../cmd"}
	needle := symbol + "("
	var found []string
	seen := make(map[string]struct{})

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, needle) && !strings.Contains(line, "func "+symbol) {
					abs, _ := filepath.Abs(path)
					if _, dup := seen[abs]; !dup {
						seen[abs] = struct{}{}
						found = append(found, abs)
					}
					break
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return found, nil
}
