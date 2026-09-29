// limits_cleanup_test.go pins the LIMITS CLEANUP behavior change in package
// shards: specialist matching returns every qualifying match instead of
// silently cutting the list to a per-verb count cap.
package shards

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A file matching four technology patterns must surface all four
// specialists: the old per-verb cap (2, or 3 for /review) silently withheld
// the rest from the executive, which asserts every match as a
// specialist_match fact and lets policy pick.
func TestMatchSpecialistsReturnsEveryQualifyingMatch(t *testing.T) {
	t.Parallel()

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{Name: "GoExpert", Type: "persistent", Status: "ready"},
			{Name: "SecurityAuditor", Type: "persistent", Status: "ready"},
			{Name: "DatabaseExpert", Type: "persistent", Status: "ready"},
			{Name: "APIExpert", Type: "persistent", Status: "ready"},
		},
	}
	// Matches golang (.go + func), security (auth + crypto import + hash),
	// sql (store + database/sql import + SELECT) and api (api + http.Handler).
	const body = `package authstore

import (
	"crypto/sha256"
	"database/sql"
	"net/http"
)

func HashPassword(pw string) []byte {
	sum := sha256.Sum256([]byte(pw))
	_ = sql.ErrNoRows
	_ = http.Handler(nil)
	_ = "SELECT 1"
	return sum[:]
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth_api_store.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	matches := MatchSpecialistsForTask(context.Background(), "/review", []string{path}, registry)

	want := map[string]bool{
		"GoExpert":        false,
		"SecurityAuditor": false,
		"DatabaseExpert":  false,
		"APIExpert":       false,
	}
	for _, m := range matches {
		if _, ok := want[m.AgentName]; ok {
			want[m.AgentName] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("qualifying specialist %s missing from matches %v", name, matches)
		}
	}
	if len(matches) < len(want) {
		t.Errorf("got %d matches, want at least the %d qualifying specialists (no count cap)", len(matches), len(want))
	}
	for i := 1; i < len(matches); i++ {
		if matches[i].Score > matches[i-1].Score {
			t.Errorf("matches not score-descending at %d: %v", i, matches)
			break
		}
	}
}
