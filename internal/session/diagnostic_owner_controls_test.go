package session

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/world/lsp"
)

func TestDiagnosticOwner_PreCanceledDispatchIsRefused(t *testing.T) {
	restoreGoplsSeams(t)
	previous := startLanguageServer
	t.Cleanup(func() { startLanguageServer = previous })
	goCalls, languageCalls := 0, 0
	lookPath = func(string) (string, error) { return "fixture-gopls", nil }
	runGoplsCheck = func(context.Context, string, string, []string) ([]byte, error) { goCalls++; return nil, nil }
	startLanguageServer = func(context.Context, string, string, ...string) (*lsp.Client, error) {
		languageCalls++
		return nil, errors.New("absent fixture")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ws := t.TempDir()
	goText := goplsDiagnostics(ctx, ws, []string{"a.go"})
	languageText := lspDiagnostics(ctx, ws, []string{"a.py", "a.ts"})
	if goCalls != 0 || languageCalls != 0 {
		t.Fatalf("pre-canceled diagnostics dispatched %d Go and %d language requests", goCalls, languageCalls)
	}
	for _, text := range []string{goText, languageText} {
		if !strings.Contains(text, "files not diagnosed") || !strings.Contains(text, "canceled") {
			t.Errorf("canceled diagnostics read as clean: %q", text)
		}
	}
}

func TestDiagnosticOwner_SoftDeadlineAlonePreservesBudgetAndValues(t *testing.T) {
	restoreGoplsSeams(t)
	lookPath = func(string) (string, error) { return "fixture-gopls", nil }
	ctx, _ := diagnosticOwnerContext(t, true)
	type marker struct{}
	ctx = context.WithValue(ctx, marker{}, "retained")
	runGoplsCheck = func(runCtx context.Context, _, _ string, _ []string) ([]byte, error) {
		if runCtx.Err() != nil || runCtx.Value(marker{}) != "retained" || diagnosticTimeout(runCtx) != 5*time.Second {
			t.Errorf("soft deadline lost independent budget or values: %v / %v", runCtx.Err(), runCtx.Value(marker{}))
		}
		return []byte("a.go:2:1: ordinary finding\n"), nil
	}
	if got := goplsDiagnostics(ctx, t.TempDir(), []string{"a.go"}); got != "a.go:2:1: ordinary finding" {
		t.Fatalf("soft deadline alone prevented ordinary completion: %q", got)
	}
}

type diagnosticWriteBarrier struct {
	net.Conn
	once    sync.Once
	started chan struct{}
}

func (p *diagnosticWriteBarrier) Write(b []byte) (int, error) {
	p.once.Do(func() { close(p.started) })
	return p.Conn.Write(b)
}

func TestDiagnosticOwner_StopsBlockedInitializationWrite(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.py"), []byte("value = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clientSide, serverSide := net.Pipe()
	t.Cleanup(func() { _ = serverSide.Close() })
	transport := &diagnosticWriteBarrier{Conn: clientSide, started: make(chan struct{})}
	previous := startLanguageServer
	t.Cleanup(func() { startLanguageServer = previous })
	startLanguageServer = func(context.Context, string, string, ...string) (*lsp.Client, error) {
		return lsp.NewClient("/python", transport), nil
	}
	ctx, stop := diagnosticOwnerContext(t, false)
	done := make(chan string, 1)
	go func() { done <- lspDiagnostics(ctx, ws, []string{"a.py"}) }()
	<-transport.started // No peer reads: Initialize is blocked in its write.
	stop()
	var text string
	select {
	case text = <-done:
	case <-time.After(time.Second):
		_ = serverSide.Close()
		text = <-done
		t.Error("owner stop did not interrupt the blocked protocol write")
	}
	if !strings.Contains(text, "files not diagnosed") || !strings.Contains(text, "canceled") {
		t.Errorf("interrupted initialization read as clean: %q", text)
	}
}
