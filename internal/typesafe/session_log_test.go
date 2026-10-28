package typesafe

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/sessionlog"
)

func installSessionLog(t *testing.T, debug bool) string {
	t.Helper()
	ws := t.TempDir()
	var on atomic.Bool
	on.Store(debug)
	sessionlog.Install(sessionlog.NewStore(sessionlog.Options{Workspace: ws, Session: "session-1", Enabled: on.Load}))
	t.Cleanup(func() { sessionlog.Install(nil) })
	return ws
}

func TestAsk_WritesAJevExchangeFileInTheJevFolder(t *testing.T) {
	ws := installSessionLog(t, true)
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answersJSON))
	}, nil)

	// Even under the chat's scope a Jev call files under jev/.
	ctx := WithOperation(sessionlog.WithChat(context.Background()), "turn_judgments")
	if _, err := c.Ask(ctx, map[string]any{"message": "charged twice\nplease refund"}, testQuestions()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ws, ".nerd", "sessions", "session-1", "jev", "0001-turn_judgments.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("jev exchange missing: %v", err)
	}
	if strings.Contains(string(raw), "sekret-key") {
		t.Fatalf("the API key reached the file:\n%s", raw)
	}
	var ex sessionlog.Exchange
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Provider != Provider || ex.Method != "Ask" || ex.Correlation.Agent != "jev" || ex.Outcome != sessionlog.OutcomeOK {
		t.Fatalf("header: %+v", ex)
	}
	if ex.Usage.InputTokens != 120 || ex.Usage.OutputTokens != 12 {
		t.Fatalf("usage: %+v", ex.Usage)
	}
	if len(ex.Wire) != 1 || ex.Wire[0].Request.Headers["Authorization"][0] != sessionlog.Redacted || ex.Wire[0].Response.Status != 200 {
		t.Fatalf("wire: %+v", ex.Wire)
	}
	if !strings.Contains(string(raw), `"charged twice",`) {
		t.Fatalf("the multi-line state must be stored as lines:\n%s", raw)
	}
}

func TestAsk_ARejectedCallIsRecordedAsAnError(t *testing.T) {
	ws := installSessionLog(t, true)
	c, _ := newTestClient(t, reply(400, `{"error":{"detail":"bad question"}}`), nil)
	if _, err := c.Ask(context.Background(), "state", testQuestions()); err == nil {
		t.Fatal("expected the API error")
	}
	var ex sessionlog.Exchange
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "sessions", "session-1", "jev", "0001-jev.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Outcome != sessionlog.OutcomeError || ex.Error == "" || len(ex.Wire) != 1 || ex.Wire[0].Response.Status != 400 {
		t.Fatalf("exchange: %+v", ex)
	}
}

func TestAsk_WritesNothingWhileDebugIsOff(t *testing.T) {
	ws := installSessionLog(t, false)
	c, _ := newTestClient(t, reply(200, answersJSON), nil)
	if _, err := c.Ask(context.Background(), "state", testQuestions()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".nerd")); !os.IsNotExist(err) {
		t.Fatalf("debug off must leave no session files: %v", err)
	}
}
