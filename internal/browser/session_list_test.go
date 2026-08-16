package browser

import (
	"testing"
	"time"
)

func TestListSessions_WhenEmpty_ShouldReturnNoSessions(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	sessions := sm.ListSessions()
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_ShouldReturnNewestFirstAndBeDeterministic(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	base := time.Unix(1700000000, 0).UTC()

	sm.sessions["id-old"] = &sessionRecord{
		meta: Session{ID: "id-old", CreatedAt: base},
	}
	sm.sessions["id-mid"] = &sessionRecord{
		meta: Session{ID: "id-mid", CreatedAt: base.Add(1 * time.Hour)},
	}
	sm.sessions["id-new"] = &sessionRecord{
		meta: Session{ID: "id-new", CreatedAt: base.Add(2 * time.Hour)},
	}
	// Two records sharing identical CreatedAt to exercise tie-break by ID.
	tieTime := base.Add(3 * time.Hour)
	sm.sessions["b-tie"] = &sessionRecord{
		meta: Session{ID: "b-tie", CreatedAt: tieTime},
	}
	sm.sessions["a-tie"] = &sessionRecord{
		meta: Session{ID: "a-tie", CreatedAt: tieTime},
	}

	got := sm.ListSessions()
	wantOrder := []string{"a-tie", "b-tie", "id-new", "id-mid", "id-old"}
	if len(got) != len(wantOrder) {
		t.Fatalf("expected %d sessions, got %d: %+v", len(wantOrder), len(got), got)
	}
	for i, wantID := range wantOrder {
		if got[i].ID != wantID {
			t.Errorf("position %d: got ID %q, want %q (full order: %v)", i, got[i].ID, wantID, gotOrder(got))
		}
	}
}

func gotOrder(sessions []Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.ID
	}
	return out
}

func TestListSessions_ShouldReturnCopiesNotAliases(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	now := time.Now()
	sm.sessions["s-1"] = &sessionRecord{
		meta: Session{ID: "s-1", URL: "https://example.com", CreatedAt: now, LastActive: now},
	}

	sessions := sm.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	// Mutate the returned copy.
	sessions[0].URL = "https://mutated.example.com"
	sessions[0].Title = "mutated"

	// Stored meta must be unchanged.
	stored, ok := sm.sessions["s-1"]
	if !ok {
		t.Fatal("session disappeared from manager")
	}
	if stored.meta.URL != "https://example.com" {
		t.Errorf("stored URL was mutated: got %q, want %q", stored.meta.URL, "https://example.com")
	}
	if stored.meta.Title != "" {
		t.Errorf("stored Title was mutated: got %q, want empty", stored.meta.Title)
	}
	// Also verify via GetSession.
	got, _ := sm.GetSession("s-1")
	if got.URL != "https://example.com" {
		t.Errorf("GetSession URL was mutated: got %q, want %q", got.URL, "https://example.com")
	}
}

func TestDefaultSessionID_ShouldReportTheManagersDefault(t *testing.T) {
	sm := NewSessionManagerWithSink(DefaultConfig(), nil)
	if got := sm.DefaultSessionID(); got != "" {
		t.Fatalf("expected empty default before set, got %q", got)
	}
	sm.defaultID = "session-abc"
	if got := sm.DefaultSessionID(); got != "session-abc" {
		t.Fatalf("expected %q, got %q", "session-abc", got)
	}
	// Clearing should return empty again.
	sm.defaultID = ""
	if got := sm.DefaultSessionID(); got != "" {
		t.Fatalf("expected empty after clear, got %q", got)
	}
}
