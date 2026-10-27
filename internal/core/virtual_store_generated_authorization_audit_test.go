package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

type generatedPermissionQueryFailure struct {
	Kernel
	err error
}

func (k generatedPermissionQueryFailure) Query(pattern string) ([]Fact, error) {
	if strings.HasPrefix(pattern, "permitted(") {
		return nil, k.err
	}
	return k.Kernel.Query(pattern)
}

func TestGeneratedAuthorizationAuditRequiresExactPermittedFact(t *testing.T) {
	loggingWorkspace := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug"})
	if err := logging.Initialize(loggingWorkspace); err != nil {
		t.Fatalf("initialize audit logging: %v", err)
	}
	defer func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{DebugMode: false})
	}()

	const secret = "generated-audit-secret-canary"
	cases := []struct {
		name        string
		permission  string
		wantAllowed bool
		wantReason  string
	}{
		{
			name:        "exact permission",
			permission:  "exact",
			wantAllowed: true,
			wantReason:  "exact pending authorization and permitted/3 fact matched",
		},
		{
			name:       "mismatched permission tuple",
			permission: "mismatch",
			wantReason: "exact permitted/3 fact unavailable",
		},
		{
			name:       "missing permission tuple",
			permission: "missing",
			wantReason: "exact permitted/3 fact unavailable",
		},
		{
			name:       "permission query error",
			permission: "error",
			wantReason: "permitted/3 query failed",
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			kernel, err := NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			kernel.SetWorkspace(workspace)
			configuration := DefaultVirtualStoreConfig()
			configuration.WorkingDir = workspace
			store := NewVirtualStoreWithConfig(nil, configuration)
			store.SetKernel(kernel)

			registry := store.GetToolRegistry()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := registry.RegisterTool("generated_probe", executable, "/all"); err != nil {
				t.Fatal(err)
			}
			identity, err := registry.GeneratedToolIdentity("generated_probe")
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(workspace, fmt.Sprintf("effect-%d.txt", i))
			canonical, err := types.CanonicalGeneratedArgs(map[string]any{"path": target, "content": secret})
			if err != nil {
				t.Fatal(err)
			}
			request := types.GeneratedToolRequest{
				ScopeID: "audit-scope", CallID: fmt.Sprintf("call-%d", i), AuthorizationID: fmt.Sprintf("exec-call-%d", i),
				Action: "/generated_probe", Target: target, CanonicalArgs: canonical, Tool: identity,
			}
			now := time.Now().Unix()
			if err := kernel.Assert(Fact{Predicate: "pending_action", Args: []any{request.AuthorizationID, request.Action, request.Target, request.CanonicalArgs, now}}); err != nil {
				t.Fatal(err)
			}

			switch tc.permission {
			case "exact", "error":
				if err := assertGeneratedPermission(kernel, request.AuthorizationID, request.Action, request.Target, request.CanonicalArgs, now); err != nil {
					t.Fatal(err)
				}
			case "mismatch":
				otherCanonical, err := types.CanonicalGeneratedArgs(map[string]any{"path": target, "content": "different-payload"})
				if err != nil {
					t.Fatal(err)
				}
				if err := assertGeneratedPermission(kernel, request.AuthorizationID, request.Action, request.Target, otherCanonical, now); err != nil {
					t.Fatal(err)
				}
			}

			backend := &generatedCoreBackend{effect: true}
			store.SetToolExecutor(backend)
			if tc.permission == "error" {
				store.mu.Lock()
				store.kernel = generatedPermissionQueryFailure{Kernel: kernel, err: errors.New("fixture query failure")}
				store.mu.Unlock()
			}

			receipt, callErr := store.ExecuteGeneratedToolCall(context.Background(), request)
			if tc.wantAllowed {
				if callErr != nil || !receipt.ValidationPassed || backend.calls != 1 {
					t.Fatalf("receipt=%+v err=%v backend calls=%d", receipt, callErr, backend.calls)
				}
				if contents, err := os.ReadFile(target); err != nil || string(contents) != secret {
					t.Fatalf("allowed execution effect=%q err=%v", contents, err)
				}
			} else {
				if callErr == nil || backend.calls != 0 {
					t.Fatalf("refusal err=%v backend calls=%d", callErr, backend.calls)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatalf("refused execution had backend effect, stat err=%v", err)
				}
			}
		})
	}

	path, err := logging.LatestAuditLogPath()
	if err != nil {
		t.Fatal(err)
	}
	events, err := logging.ReadRecentAuditEvents(path, []logging.AuditEventType{logging.AuditSafetyAllow, logging.AuditSafetyBlock}, len(cases)+1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(cases) {
		t.Fatalf("safety audit events=%d, want %d: %+v", len(events), len(cases), events)
	}
	for i, tc := range cases {
		event := events[i]
		if event.Success != tc.wantAllowed || event.Action != "generated authorization /generated_probe" {
			t.Errorf("audit[%d]=%+v, want allowed=%v action=%q", i, event, tc.wantAllowed, "generated authorization /generated_probe")
		}
		if got := event.Fields["reason"]; got != tc.wantReason {
			t.Errorf("audit[%d] reason=%v, want %q", i, got, tc.wantReason)
		}
		if strings.Contains(event.Message, secret) || strings.Contains(event.MangleFact, secret) || strings.Contains(event.Message, "\"content\"") {
			t.Errorf("safety audit contains generated args: %+v", event)
		}
	}
	auditText, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(auditText), secret) || strings.Contains(string(auditText), `"content"`) {
		t.Fatalf("audit log contains canonical generated args: %s", auditText)
	}
}

func assertGeneratedPermission(kernel Kernel, authorizationID string, action MangleAtom, target, canonical string, now int64) error {
	for _, fact := range []Fact{
		{Predicate: "permitted_action", Args: []any{authorizationID, action, target, canonical, now}},
		{Predicate: "permission_check_result", Args: []any{authorizationID, MangleAtom("/permit"), "fixture permission", now}},
	} {
		if err := kernel.Assert(fact); err != nil {
			return err
		}
	}
	return nil
}
