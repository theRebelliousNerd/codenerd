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

func diagnosticOwnerContext(t *testing.T, afterDeadline bool) (context.Context, context.CancelFunc) {
	t.Helper()
	owner, stop := context.WithCancel(t.Context())
	t.Cleanup(stop)
	parent := owner
	if afterDeadline {
		var cancel context.CancelFunc
		parent, cancel = context.WithDeadline(owner, time.Now().Add(-time.Second))
		t.Cleanup(cancel)
		if !errors.Is(parent.Err(), context.DeadlineExceeded) {
			t.Fatal("fixture did not expire its soft deadline")
		}
	}
	return withDiagnosticTimeout(WithOwnerStop(parent, owner), 5*time.Second), stop
}

func TestDiagnosticOwner_GoplsStopsDuringRequest(t *testing.T) {
	for _, afterDeadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "after_soft_deadline"}[afterDeadline], func(t *testing.T) {
			restoreGoplsSeams(t)
			lookPath = func(string) (string, error) { return "fixture-gopls", nil }
			captured := make(chan context.Context, 1)
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			runGoplsCheck = func(ctx context.Context, _, _ string, _ []string) ([]byte, error) {
				captured <- ctx
				<-release
				return []byte("a.go:2:1: partial finding\n"), ctx.Err()
			}
			ctx, stop := diagnosticOwnerContext(t, afterDeadline)
			done := make(chan string, 1)
			ws := t.TempDir()
			go func() { done <- goplsDiagnostics(ctx, ws, []string{"a.go", "b.go"}) }()
			t.Cleanup(unblock)
			runCtx := <-captured
			stop()
			select {
			case <-runCtx.Done():
			case <-time.After(time.Second):
				t.Error("gopls request detached the explicit owner's stop")
			}
			unblock()
			got := <-done
			if !strings.Contains(got, "partial finding") || !strings.Contains(got, "files not diagnosed") || !strings.Contains(got, "b.go") {
				t.Errorf("cancel lost partial findings or unchecked-file notice: %q", got)
			}
		})
	}
}

func TestDiagnosticOwner_LSPStopsAndSkipsLaterLanguage(t *testing.T) {
	for _, language := range []string{"python", "typescript"} {
		t.Run(language, func(t *testing.T) {
			ws := t.TempDir()
			for _, name := range []string{"app.py", "app.ts", "other.py", "other.ts"} {
				if err := os.WriteFile(filepath.Join(ws, name), []byte("value = 1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			previous := startLanguageServer
			t.Cleanup(func() { startLanguageServer = previous })
			var starts []string
			captured := make(chan context.Context, 1)
			opened := make(chan struct{}, 1)
			var peer net.Conn
			serverDone := make(chan struct{})
			startLanguageServer = func(ctx context.Context, lang, binary string, _ ...string) (*lsp.Client, error) {
				starts = append(starts, binary)
				if len(starts) > 1 {
					return nil, errors.New("later language dispatch spy")
				}
				captured <- ctx
				clientSide, serverSide := net.Pipe()
				peer = serverSide
				go func() {
					defer close(serverDone)
					serveLanguageServer(t, serverSide, func(int) bool { opened <- struct{}{}; return false })
				}()
				return lsp.NewClient(lang, clientSide), nil
			}
			ctx, stop := diagnosticOwnerContext(t, true)
			paths := []string{"app.py", "other.py", "app.ts"}
			if language == "typescript" {
				paths = []string{"app.ts", "other.ts"}
			}
			done := make(chan string, 1)
			go func() { done <- lspDiagnostics(ctx, ws, paths) }()
			runCtx := <-captured
			<-opened
			stop()
			select {
			case <-runCtx.Done():
			case <-time.After(time.Second):
				t.Error("LSP request detached the owner's stop after a soft deadline")
			}
			// Release the unchanged baseline deterministically too. The repaired
			// path must have closed its transport before fixture intervention.
			select {
			case <-serverDone:
			case <-time.After(time.Second):
				t.Error("owner stop did not close the LSP transport")
			}
			_ = peer.Close()
			got := <-done
			<-serverDone
			if len(starts) != 1 {
				t.Errorf("later language started after stop: %v", starts)
			}
			if !strings.Contains(got, "files not diagnosed") {
				t.Errorf("unfinished diagnostics read as clean: %q", got)
			}
		})
	}
}

func TestDiagnosticOwner_CriticDoesNotStartLaterStages(t *testing.T) {
	for _, mode := range []string{"already_stopped", "stopped_by_gopls", "normal"} {
		t.Run(mode, func(t *testing.T) {
			restoreGoplsSeams(t)
			previous := startLanguageServer
			t.Cleanup(func() { startLanguageServer = previous })
			ws := t.TempDir()
			for _, name := range []string{"a.go", "a.py", "a.ts"} {
				if err := os.WriteFile(filepath.Join(ws, name), []byte("package p\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, stop := diagnosticOwnerContext(t, false)
			// Retain only the explicit owner channel: caller cancellation may
			// already have been detached by a verification completion window.
			ctx = context.WithoutCancel(ctx)
			if mode == "already_stopped" {
				stop()
			}
			goCalls, languageCalls := 0, 0
			lookPath = func(string) (string, error) { return "fixture-gopls", nil }
			runGoplsCheck = func(context.Context, string, string, []string) ([]byte, error) {
				goCalls++
				if mode == "stopped_by_gopls" {
					stop()
				}
				return nil, nil
			}
			startLanguageServer = func(context.Context, string, string, ...string) (*lsp.Client, error) {
				languageCalls++
				return nil, errors.New("absent fixture language server")
			}
			critic := &scriptedCriticLLM{review: "NO FINDINGS"}
			cfg := DefaultExecutorConfig()
			cfg.WorkspaceRoot = ws
			e := &Executor{config: cfg, llmClient: critic, kernel: realKernel(t), rolePrompts: &fakeRolePrompts{}}
			_, err := e.verifyAndUpliftWithCritic(ctx, &recordingToolResults{}, "sys", nil, nil, nil,
				&ExecutionResult{WriteToolEffects: 1, WrittenPaths: []string{"a.go", "a.py", "a.ts"}})
			if err != nil {
				t.Fatal(err)
			}
			wantGo, wantLanguages, wantCritic := 0, 0, 0
			if mode == "stopped_by_gopls" {
				wantGo = 1
			} else if mode == "normal" {
				wantGo, wantLanguages, wantCritic = 1, 2, 1
			}
			if goCalls != wantGo || languageCalls != wantLanguages || critic.calls != wantCritic {
				t.Errorf("stage dispatch after %s: Go=%d languages=%d critic=%d, want %d/%d/%d", mode, goCalls, languageCalls, critic.calls, wantGo, wantLanguages, wantCritic)
			}
		})
	}
}
