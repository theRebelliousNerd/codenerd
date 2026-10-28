package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/logging"
)

// All permission results below come from a RealKernel. Faults only remove or
// corrupt evidence; none of these fixtures fabricate a successful query.
type authorityFaultKernel struct {
	KernelInterface
	queryFault   func(string) error
	emptyQuery   string
	beforeQuery  func(string)
	afterQuery   func(string, []map[string]any)
	assertFault  string
	retractFault string
	transform    func(string) string
	replacements map[string]string
}

func (kernel *authorityFaultKernel) Query(query string) ([]map[string]any, error) {
	if kernel.beforeQuery != nil {
		kernel.beforeQuery(query)
	}
	if kernel.queryFault != nil {
		if err := kernel.queryFault(query); err != nil {
			return nil, err
		}
	}
	if kernel.emptyQuery != "" && strings.HasPrefix(query, kernel.emptyQuery) {
		return nil, nil
	}
	rows, err := kernel.KernelInterface.Query(query)
	if err == nil && kernel.afterQuery != nil {
		kernel.afterQuery(query, rows)
	}
	return rows, err
}

func (kernel *authorityFaultKernel) Assert(fact string) error {
	if kernel.assertFault != "" && strings.HasPrefix(fact, kernel.assertFault) {
		return errors.New("injected assertion failure")
	}
	actual := fact
	if kernel.transform != nil {
		actual = kernel.transform(fact)
	}
	if actual != fact {
		if kernel.replacements == nil {
			kernel.replacements = make(map[string]string)
		}
		kernel.replacements[fact] = actual
	}
	return kernel.KernelInterface.Assert(actual)
}

func (kernel *authorityFaultKernel) Retract(fact string) error {
	if kernel.retractFault != "" && strings.HasPrefix(fact, kernel.retractFault) {
		return errors.New("injected retraction failure")
	}
	if replacement, replaced := kernel.replacements[fact]; replaced {
		fact = replacement
	}
	return kernel.KernelInterface.Retract(fact)
}

type authorityCountedTransport struct {
	*mockTransport
	calls         int
	resourceCalls int
	promptCalls   int
	onCall        func(map[string]any)
	resources     []MCPResource
	prompts       []MCPPrompt
}

func (transport *authorityCountedTransport) CallTool(ctx context.Context, name string, args map[string]any) (*MCPCallResult, error) {
	transport.calls++
	if transport.onCall != nil {
		transport.onCall(args)
	}
	return transport.mockTransport.CallTool(ctx, name, args)
}

func (transport *authorityCountedTransport) ListResources(context.Context) ([]MCPResource, error) {
	return transport.resources, transport.listErr
}

func (transport *authorityCountedTransport) ReadResource(_ context.Context, uri string) ([]MCPResourceContent, error) {
	transport.resourceCalls++
	return []MCPResourceContent{{URI: uri, Text: "complete resource"}}, nil
}

func (transport *authorityCountedTransport) ListPrompts(context.Context) ([]MCPPrompt, error) {
	return transport.prompts, transport.listErr
}

func (transport *authorityCountedTransport) GetPrompt(_ context.Context, name string, args map[string]string) ([]MCPPromptMessage, error) {
	transport.promptCalls++
	return []MCPPromptMessage{{Role: "user", Content: name + args["path"]}}, nil
}

type authorityFixture struct {
	manager   *MCPClientManager
	transport *authorityCountedTransport
	kernel    KernelInterface
	fault     *authorityFaultKernel
	args      map[string]any
	ctx       context.Context
	confirmed bool
}

func newAuthorityFixture(test *testing.T) *authorityFixture {
	test.Helper()
	return newAuthorityFixtureWithKernel(test, nil)
}

func newAuthorityFixtureWithKernel(test *testing.T, kernelFactory func(*testing.T) KernelInterface) *authorityFixture {
	test.Helper()
	transport := &authorityCountedTransport{mockTransport: &mockTransport{connected: true}}
	manager := NewMCPClientManager(nil, nil, nil)
	manager.servers["srv"] = &MCPServerConnection{Server: &MCPServer{ID: "srv"}, Transport: transport}
	var kernel KernelInterface
	if kernelFactory == nil {
		kernel = installRemoteAuthorityFixture(test, manager, map[string]RemoteEffect{"srv/op": RemoteRead})
	} else {
		kernel = installRemoteAuthorityFixtureWithKernel(test, manager, map[string]RemoteEffect{"srv/op": RemoteRead}, kernelFactory(test))
	}
	fault := &authorityFaultKernel{KernelInterface: kernel}
	manager.factEmitter().kernel = fault
	return &authorityFixture{manager: manager, transport: transport, kernel: kernel, fault: fault,
		args: map[string]any{"nested": map[string]any{"value": "original", "items": []any{"first"}}},
		ctx:  WithRemoteCallIdentity(context.Background(), RemoteCallIdentity{Scope: "scope", CallID: "call"})}
}

func TestRemoteAuthority_ProductionShardsAuthorizeEffectAndRevokeRemovedMetadata(t *testing.T) {
	if RemoteAuthorityShardedKernelForTest == nil {
		t.Fatal("production sharded MCP authority kernel fixture is not installed")
	}
	fixture := newAuthorityFixtureWithKernel(t, RemoteAuthorityShardedKernelForTest)
	result, err := fixture.manager.CallTool(fixture.ctx, "srv/op", fixture.args)
	if err != nil || result == nil || !result.Success {
		t.Fatalf("production sharded authorize/effect failed: result=%+v err=%v", result, err)
	}
	if fixture.transport.calls != 1 {
		t.Fatalf("authorized production sharded effect calls = %d, want 1", fixture.transport.calls)
	}
	shardInspector, ok := fixture.kernel.(interface {
		QueryShard(string, string) ([]map[string]any, error)
	})
	if !ok {
		t.Fatalf("production sharded kernel %T cannot inspect its local consumer copy", fixture.kernel)
	}
	rows, err := shardInspector.QueryShard("policy", `mcp_tool_registered("srv/op", "srv", RegisteredAt)`)
	if err != nil || len(rows) != 1 {
		t.Fatalf("MCP metadata was not copied to the production policy shard: rows=%v err=%v", rows, err)
	}

	// The in-memory connection deliberately remains stale. Remove the live
	// catalog facts so this next decision depends on production Cortex metadata.
	fixture.manager.factEmitter().RetractTool("srv/op")
	if err := fixture.manager.factEmitter().Error(); err != nil {
		t.Fatalf("retract removed MCP metadata: %v", err)
	}
	rows, err = fixture.kernel.Query(`mcp_tool_registered("srv/op", "srv", RegisteredAt)`)
	if err != nil || len(rows) != 0 {
		t.Fatalf("removed tool metadata remains in production Cortex: rows=%v err=%v", rows, err)
	}
	rows, err = shardInspector.QueryShard("policy", `mcp_tool_registered("srv/op", "srv", RegisteredAt)`)
	if err != nil || len(rows) != 0 {
		t.Fatalf("removed tool metadata remains in production policy consumer copy: rows=%v err=%v", rows, err)
	}

	fixture.ctx = WithRemoteCallIdentity(context.Background(), RemoteCallIdentity{Scope: "scope-after-removal", CallID: "call-after-removal"})
	result, err = fixture.manager.CallTool(fixture.ctx, "srv/op", fixture.args)
	if err == nil || result != nil {
		t.Fatalf("call with removed production metadata was admitted: result=%+v err=%v", result, err)
	}
	if fixture.transport.calls != 1 {
		t.Fatalf("removed production metadata caused another remote effect: calls=%d", fixture.transport.calls)
	}
	requireAuthorityClean(t, fixture.kernel)
}

func (fixture *authorityFixture) call(test *testing.T, route string) error {
	test.Helper()
	ctx := WithRiskConfirmation(fixture.ctx, fixture.confirmed)
	switch route {
	case "control-plane":
		plane := NewControlPlane(fixture.manager, nil, fixture.manager.factEmitter())
		plane.SetRiskGate(KernelRiskGate(fixture.manager.factEmitter().kernel))
		result := plane.Call(ctx, CallOptions{Tool: "srv/op", Args: fixture.args, ConfirmRisk: fixture.confirmed})
		if !result.Success {
			return errors.New(result.Error)
		}
	case "manager":
		result, err := fixture.manager.CallTool(ctx, "srv/op", fixture.args)
		if err != nil {
			return err
		}
		if result == nil || !result.Success {
			return errors.New("call refused")
		}
	case "integration":
		_, err := NewIntegrationAdapter(fixture.manager, "srv").CallTool(ctx, "op", fixture.args)
		return err
	default:
		test.Fatal("unknown route")
	}
	return nil
}

func requireAuthorityClean(test *testing.T, kernel KernelInterface) {
	test.Helper()
	for _, query := range []string{
		"pending_action(ID, Action, Target, Payload, Time)",
		"mcp_remote_request(ID, Scope, Call, Server, Tool, Schema, Effect, Action, Target, Payload, Digest, Risk, Confirmed)",
		"mcp_remote_reviewed(ID, Server, Tool, Schema, Effect)",
		"mcp_remote_operation(ID, Operation)",
		"mcp_remote_permitted(ID, Scope, Call, Server, Tool, Schema, Digest)",
		"human_approval_granted(ID, Action, Target, Payload, Digest)",
	} {
		rows, err := kernel.Query(query)
		if err != nil || len(rows) != 0 {
			test.Fatalf("transient authority remains for %s: %d rows, %v", query, len(rows), err)
		}
	}
}

func TestRemoteAuthority_AllProductionRoutesRequireExactPositivePermission(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, *authorityFixture)
		allowed bool
	}{
		{"authorized", func(*testing.T, *authorityFixture) {}, true},
		{"authorized-write", func(test *testing.T, fixture *authorityFixture) { fixture.reviewEffect(test, RemoteWrite, "") }, true},
		{"nil-kernel", func(_ *testing.T, fixture *authorityFixture) { fixture.manager.factEmitter().kernel = nil }, false},
		{"typed-nil-kernel", func(_ *testing.T, fixture *authorityFixture) {
			var kernel *authorityFaultKernel
			fixture.manager.factEmitter().kernel = kernel
		}, false},
		{"empty-admission", func(_ *testing.T, fixture *authorityFixture) { fixture.fault.emptyQuery = "mcp_remote_permitted(" }, false},
		{"missing-permitted-fact", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteDelete, RiskSafe)
			fixture.confirmed = true
		}, false},
		{"empty-risk-gate", func(_ *testing.T, fixture *authorityFixture) {
			fixture.fault.emptyQuery = "mcp_tool_risk("
			fixture.confirmed = true
		}, false},
		{"error-risk-gate", func(_ *testing.T, fixture *authorityFixture) {
			fixture.confirmed = true
			fixture.fault.queryFault = func(query string) error {
				if strings.HasPrefix(query, "mcp_tool_risk(") {
					return errors.New("classification unavailable")
				}
				return nil
			}
		}, false},
		{"admission-query-error", func(_ *testing.T, fixture *authorityFixture) {
			fixture.confirmed = true
			fixture.fault.queryFault = func(query string) error {
				if strings.HasPrefix(query, "mcp_remote_permitted(") {
					return errors.New("admission unavailable")
				}
				return nil
			}
		}, false},
		{"confirmation-query-error", func(_ *testing.T, fixture *authorityFixture) {
			fixture.confirmed = true
			fixture.fault.queryFault = func(query string) error {
				if strings.HasPrefix(query, "mcp_remote_confirmation_required(") {
					return errors.New("policy unavailable")
				}
				return nil
			}
		}, false},
		{"assertion-error", func(_ *testing.T, fixture *authorityFixture) { fixture.fault.assertFault = "mcp_remote_request(" }, false},
		{"unknown-effect", func(test *testing.T, fixture *authorityFixture) {
			tool := fixture.manager.servers["srv"].Tools[0]
			if err := fixture.manager.SetReviewedToolEffect(ReviewedToolEffect{ToolID: tool.ToolID, SchemaHash: tool.SchemaHash, Effect: RemoteEffect("unknown")}); err == nil {
				test.Fatal("unknown effect accepted")
			}
		}, false},
		{"no-reviewed-effect", func(_ *testing.T, fixture *authorityFixture) { fixture.manager.RevokeReviewedToolEffect("srv/op") }, false},
		{"missing-registration", func(_ *testing.T, fixture *authorityFixture) { fixture.manager.factEmitter().RetractTool("srv/op") }, false},
		{"schema-revoked", func(_ *testing.T, fixture *authorityFixture) {
			fixture.manager.servers["srv"].Tools[0].InputSchema = json.RawMessage(`{"type":"object"}`)
		}, false},
		{"missing-schema", func(test *testing.T, fixture *authorityFixture) {
			tool := fixture.manager.servers["srv"].Tools[0]
			tool.InputSchema = nil
			tool.SchemaHash = remoteSchemaHash(tool)
			fixture.manager.factEmitter().EmitTool(tool)
			fixture.reviewEffect(test, RemoteRead, "")
		}, false},
		{"null-schema", func(test *testing.T, fixture *authorityFixture) {
			tool := fixture.manager.servers["srv"].Tools[0]
			tool.InputSchema = json.RawMessage(`null`)
			tool.SchemaHash = remoteSchemaHash(tool)
			fixture.manager.factEmitter().EmitTool(tool)
			fixture.reviewEffect(test, RemoteRead, "")
		}, false},
		{"connection-revoked", func(_ *testing.T, fixture *authorityFixture) {
			fixture.manager.factEmitter().EmitServerStatus("srv", ServerStatusDisconnected)
		}, false},
		{"conflicting-server-status", func(test *testing.T, fixture *authorityFixture) {
			if err := fixture.kernel.Assert(`mcp_server_status("srv", /error)`); err != nil {
				test.Fatal(err)
			}
		}, false},
		{"unknown-risk", func(_ *testing.T, fixture *authorityFixture) { fixture.manager.servers["srv"].Tools[0].Risk = "" }, false},
		{"missing-provenance", func(_ *testing.T, fixture *authorityFixture) { fixture.manager.servers["srv"].Tools[0].RiskSource = "" }, false},
		{"unknown-provenance", func(_ *testing.T, fixture *authorityFixture) {
			fixture.manager.servers["srv"].Tools[0].RiskSource = ClassificationSource("unknown")
		}, false},
		{"missing-identity", func(_ *testing.T, fixture *authorityFixture) {
			fixture.ctx = WithRemoteCallIdentity(context.Background(), RemoteCallIdentity{})
		}, false},
		{"canceled", func(_ *testing.T, fixture *authorityFixture) {
			ctx, cancel := context.WithCancel(fixture.ctx)
			cancel()
			fixture.ctx = ctx
		}, false},
		{"canceled-after-admission", func(_ *testing.T, fixture *authorityFixture) {
			ctx, cancel := context.WithCancel(fixture.ctx)
			fixture.ctx = ctx
			fixture.fault.beforeQuery = func(query string) {
				if strings.HasPrefix(query, "mcp_remote_permitted(") {
					cancel()
				}
			}
		}, false},
		{"nested-map-mutated", func(_ *testing.T, fixture *authorityFixture) {
			fixture.fault.beforeQuery = func(query string) {
				if strings.HasPrefix(query, "mcp_remote_permitted(") {
					fixture.args["nested"].(map[string]any)["value"] = "changed"
				}
			}
		}, false},
		{"nested-slice-mutated", func(_ *testing.T, fixture *authorityFixture) {
			fixture.fault.beforeQuery = func(query string) {
				if strings.HasPrefix(query, "mcp_remote_permitted(") {
					fixture.args["nested"].(map[string]any)["items"].([]any)[0] = "changed"
				}
			}
		}, false},
		{"canonical-payload-mismatch", func(_ *testing.T, fixture *authorityFixture) {
			fixture.fault.transform = func(fact string) string {
				if strings.HasPrefix(fact, "mcp_remote_request(") {
					return strings.ReplaceAll(fact, "original", "different")
				}
				return fact
			}
		}, false},
		{"confirm-only", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteDelete, "")
			fixture.confirmed = true
		}, false},
		{"override-only", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteExecute, RiskSafe)
			fixture.confirmed = true
		}, false},
		{"approved-delete", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteDelete, RiskSafe)
			fixture.approve(test, approval.VerdictApprove)
			fixture.confirmed = true
		}, true},
		{"approved-execute", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteExecute, RiskSafe)
			fixture.approve(test, approval.VerdictApprove)
			fixture.confirmed = true
		}, true},
		{"approval-without-confirmation", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteDelete, RiskSafe)
			fixture.approve(test, approval.VerdictApprove)
		}, false},
		{"approval-rejected", func(test *testing.T, fixture *authorityFixture) {
			fixture.reviewEffect(test, RemoteDelete, RiskSafe)
			fixture.approve(test, approval.VerdictReject)
			fixture.confirmed = true
		}, false},
		{"approval-revoked-during-admission", func(test *testing.T, fixture *authorityFixture) {
			// The person approved, but the kernel cannot be consulted when the
			// approved admission asks: consent does not substitute for it.
			fixture.reviewEffect(test, RemoteDelete, RiskSafe)
			fixture.approve(test, approval.VerdictApprove)
			fixture.confirmed = true
			asked := 0
			fixture.fault.queryFault = func(query string) error {
				if strings.HasPrefix(query, "mcp_remote_permitted(") {
					asked++
					if asked == 2 {
						return errors.New("injected policy failure during the approved admission")
					}
				}
				return nil
			}
		}, false},
	}
	for _, mismatch := range []struct{ name, from, to string }{
		{"server-mismatch", `"srv"`, `"other"`}, {"tool-mismatch", `"srv/op"`, `"srv/other"`},
		{"scope-mismatch", `"scope"`, `"other-scope"`}, {"call-mismatch", `"call"`, `"other-call"`},
	} {
		mismatch := mismatch
		cases = append(cases, struct {
			name    string
			prepare func(*testing.T, *authorityFixture)
			allowed bool
		}{
			name: mismatch.name, prepare: func(_ *testing.T, fixture *authorityFixture) {
				fixture.fault.transform = func(fact string) string {
					if strings.HasPrefix(fact, "mcp_remote_request(") {
						return strings.Replace(fact, mismatch.from, mismatch.to, 1)
					}
					return fact
				}
			},
		})
	}
	cases = append(cases, struct {
		name    string
		prepare func(*testing.T, *authorityFixture)
		allowed bool
	}{
		name: "args-digest-mismatch", prepare: func(test *testing.T, fixture *authorityFixture) {
			canonical, _, err := canonicalRemoteArgs(fixture.args)
			if err != nil {
				test.Fatal(err)
			}
			digest := sha256.Sum256(canonical)
			fixture.fault.transform = func(fact string) string {
				if strings.HasPrefix(fact, "mcp_remote_request(") {
					return strings.Replace(fact, hex.EncodeToString(digest[:]), strings.Repeat("0", 64), 1)
				}
				return fact
			}
		},
	})
	for _, route := range []string{"control-plane", "manager", "integration"} {
		for _, scenario := range cases {
			t.Run(route+"/"+scenario.name, func(test *testing.T) {
				fixture := newAuthorityFixture(test)
				scenario.prepare(test, fixture)
				fixture.transport.onCall = func(outbound map[string]any) {
					if outbound["nested"].(map[string]any)["value"] != "original" {
						test.Fatal("transport received altered arguments")
					}
					fixture.args["nested"].(map[string]any)["value"] = "late change"
					if outbound["nested"].(map[string]any)["value"] != "original" {
						test.Fatal("transport arguments alias caller arguments")
					}
					for _, query := range []string{"permitted_action(ID, Action, Target, Payload, Time)", "next_action(Action)"} {
						rows, err := fixture.kernel.Query(query)
						if err != nil || len(rows) != 0 {
							test.Fatalf("second executable authorization for %s: %v, %d", query, err, len(rows))
						}
					}
				}
				err := fixture.call(test, route)
				wantCalls := 0
				if scenario.allowed {
					wantCalls = 1
				}
				if fixture.transport.calls != wantCalls || (err == nil) != scenario.allowed {
					test.Fatalf("calls=%d want=%d, error=%v", fixture.transport.calls, wantCalls, err)
				}
				requireAuthorityClean(test, fixture.kernel)
			})
		}
	}
}

func TestRemoteAuthority_RequiresExactManglePermitTuple(t *testing.T) {
	t.Run("missing-exact-permitted-fact", func(test *testing.T) {
		fixture := newAuthorityFixture(test)
		fixture.reviewEffect(test, RemoteDelete, RiskSafe)
		fixture.confirmed = true
		checked := false
		fixture.fault.beforeQuery = func(query string) {
			if !strings.HasPrefix(query, "mcp_remote_permitted(") {
				return
			}
			checked = true
			rows, err := fixture.kernel.Query(`permitted(/delete_file, "srv/op", Payload)`)
			if err != nil || len(rows) != 0 {
				test.Fatalf("exact permitted tuple exists after removing its rule: rows=%v err=%v", rows, err)
			}
		}
		fixture.fault.afterQuery = func(query string, rows []map[string]any) {
			if strings.HasPrefix(query, "mcp_remote_permitted(") && len(rows) != 0 {
				test.Fatalf("Mangle granted remote authority without the exact permit: %v", rows)
			}
		}
		if err := fixture.call(test, "manager"); err == nil || fixture.transport.calls != 0 {
			t.Fatalf("missing exact permitted tuple dispatched: calls=%d err=%v", fixture.transport.calls, err)
		}
		if !checked {
			t.Fatal("remote admission did not reach the real Mangle permit check")
		}
	})

	t.Run("wrong-target-permitted-fact", func(test *testing.T) {
		fixture := newAuthorityFixture(test)
		fixture.fault.transform = func(fact string) string {
			if strings.HasPrefix(fact, "pending_action(") {
				return strings.Replace(fact, `"srv/op"`, `"other-target"`, 1)
			}
			return fact
		}
		checked := false
		fixture.fault.beforeQuery = func(query string) {
			if !strings.HasPrefix(query, "mcp_remote_permitted(") {
				return
			}
			checked = true
			wrong, wrongErr := fixture.kernel.Query(`permitted(/read_file, "other-target", Payload)`)
			exact, exactErr := fixture.kernel.Query(`permitted(/read_file, "srv/op", Payload)`)
			if wrongErr != nil || len(wrong) != 1 || exactErr != nil || len(exact) != 0 {
				test.Fatalf("expected only the wrong-target permit: wrong=%v err=%v exact=%v err=%v", wrong, wrongErr, exact, exactErr)
			}
		}
		fixture.fault.afterQuery = func(query string, rows []map[string]any) {
			if strings.HasPrefix(query, "mcp_remote_permitted(") && len(rows) != 0 {
				test.Fatalf("Mangle granted remote authority for a wrong exact permit tuple: %v", rows)
			}
		}
		if err := fixture.call(test, "manager"); err == nil || fixture.transport.calls != 0 {
			t.Fatalf("wrong-target permitted tuple dispatched: calls=%d err=%v", fixture.transport.calls, err)
		}
		if !checked {
			t.Fatal("remote admission did not reach the real Mangle permit check")
		}
	})
}

func TestRemoteAuthority_AuditsFinalPolicyVerdictsWithoutArguments(t *testing.T) {
	workspace := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug", Format: "json"})
	if err := logging.Initialize(workspace); err != nil {
		t.Fatalf("initialize audit logging: %v", err)
	}
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ApplyConfig(logging.Config{})
		logging.ClearInjectedConfig()
	})

	readEvents := func(test *testing.T) []logging.AuditEvent {
		test.Helper()
		path, err := logging.LatestAuditLogPath()
		if err != nil {
			test.Fatalf("find audit log: %v", err)
		}
		events, err := logging.ReadRecentAuditEvents(path, []logging.AuditEventType{
			logging.AuditSafetyAllow, logging.AuditSafetyBlock,
		}, 16)
		if err != nil {
			test.Fatalf("read safety audit events: %v", err)
		}
		return events
	}

	const secretArgument = "remote-secret-fixture-7391"
	allowed := newAuthorityFixture(t)
	allowed.args = map[string]any{"token": secretArgument}
	allowed.transport.onCall = func(map[string]any) {
		for _, event := range readEvents(t) {
			if event.EventType == logging.AuditSafetyAllow {
				return
			}
		}
		t.Fatal("remote tool effect ran before its safety allow was audited")
	}
	if err := allowed.call(t, "manager"); err != nil || allowed.transport.calls != 1 {
		t.Fatalf("authorized remote call: calls=%d err=%v", allowed.transport.calls, err)
	}

	denied := newAuthorityFixture(t)
	denied.args = map[string]any{"token": secretArgument}
	denied.fault.emptyQuery = "mcp_remote_permitted("
	if err := denied.call(t, "manager"); err == nil || denied.transport.calls != 0 {
		t.Fatalf("remote_permitted refusal: calls=%d err=%v", denied.transport.calls, err)
	}

	events := readEvents(t)
	if len(events) != 2 {
		t.Fatalf("safety events = %+v, want one allow and one block", events)
	}
	if events[0].EventType != logging.AuditSafetyAllow || events[0].Action != "MCP tool srv/op /read_file" {
		t.Errorf("allow event = %+v", events[0])
	}
	if events[1].EventType != logging.AuditSafetyBlock || events[1].Action != "MCP tool srv/op /read_file" {
		t.Errorf("block event = %+v", events[1])
	}
	if reason, _ := events[1].Fields["reason"].(string); !strings.Contains(reason, "mcp_remote_permitted/7") {
		t.Errorf("refusal reason = %q, want exact mcp_remote_permitted/7 outcome", reason)
	}
	serialized, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal audit evidence: %v", err)
	}
	if strings.Contains(string(serialized), secretArgument) || strings.Contains(string(serialized), `"args"`) || strings.Contains(string(serialized), "CanonicalArgs") {
		t.Fatalf("safety audit exposed remote call arguments: %s", serialized)
	}
}

func (fixture *authorityFixture) reviewEffect(test *testing.T, effect RemoteEffect, override RiskClass) {
	test.Helper()
	tool := fixture.manager.servers["srv"].Tools[0]
	if err := fixture.manager.SetReviewedToolEffect(ReviewedToolEffect{ToolID: tool.ToolID, SchemaHash: tool.SchemaHash, Effect: effect, RiskOverride: override}); err != nil {
		test.Fatal(err)
	}
}

// approve attaches a host that answers every offered request with verdict,
// through the same approval.Host boundary the TUI uses.
func (fixture *authorityFixture) approve(test *testing.T, verdict approval.Verdict) {
	test.Helper()
	attachTestApprover(test, fixture.manager, time.Minute, func(int32, approval.Request) approval.Verdict { return verdict })
}

func TestRemoteAuthority_ReplayAndCleanupFailureCannotAuthorizeAnotherCall(t *testing.T) {
	for _, route := range []string{"control-plane", "manager", "integration"} {
		t.Run(route+"/replay", func(test *testing.T) {
			fixture := newAuthorityFixture(test)
			if err := fixture.call(test, route); err != nil {
				test.Fatal(err)
			}
			if err := fixture.call(test, route); err == nil || fixture.transport.calls != 1 {
				test.Fatalf("replayed call admitted: %v, %d", err, fixture.transport.calls)
			}
			requireAuthorityClean(test, fixture.kernel)
		})
		t.Run(route+"/cleanup-error", func(test *testing.T) {
			fixture := newAuthorityFixture(test)
			fixture.fault.retractFault = "pending_action("
			if err := fixture.call(test, route); err == nil || fixture.transport.calls != 1 {
				test.Fatalf("cleanup failure not visible: %v, %d", err, fixture.transport.calls)
			}
			fixture.ctx = context.Background()
			if err := fixture.call(test, route); err == nil || fixture.transport.calls != 1 {
				test.Fatalf("failed cleanup left admission healthy: %v, %d", err, fixture.transport.calls)
			}
		})
	}
}

func TestRemoteAuthority_ResourcesAndPromptsAlsoRequirePositiveAdmission(t *testing.T) {
	fixture := newAuthorityFixture(t)
	fixture.transport.resources = []MCPResource{{URI: "docs://reference", Name: "reference"}}
	fixture.transport.prompts = []MCPPrompt{{Name: "explain"}}
	ctx := context.Background()
	resources, err := fixture.manager.DiscoverResources(ctx, "srv")
	if err != nil {
		t.Fatal(err)
	}
	prompts, err := fixture.manager.DiscoverPrompts(ctx, "srv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.ReadResource(ctx, "srv", resources[0].URI); err == nil || fixture.transport.resourceCalls != 0 {
		t.Fatal("unreviewed resource dispatched")
	}
	if _, err := fixture.manager.GetPrompt(ctx, "srv", "explain", nil); err == nil || fixture.transport.promptCalls != 0 {
		t.Fatal("unreviewed prompt dispatched")
	}
	reviewRemoteFixtureSubject(t, fixture.manager, RemoteResource, ResourceAuthorityTool("srv", resources[0]))
	reviewRemoteFixtureSubject(t, fixture.manager, RemotePrompt, PromptAuthorityTool("srv", prompts[0]))
	if _, err := fixture.manager.ReadResource(ctx, "srv", resources[0].URI); err != nil || fixture.transport.resourceCalls != 1 {
		t.Fatalf("reviewed resource: %v", err)
	}
	if _, err := fixture.manager.GetPrompt(ctx, "srv", "explain", nil); err != nil || fixture.transport.promptCalls != 1 {
		t.Fatalf("reviewed prompt: %v", err)
	}
	fixture.fault.emptyQuery = "mcp_remote_permitted("
	if _, err := fixture.manager.ReadResource(ctx, "srv", resources[0].URI); err == nil || fixture.transport.resourceCalls != 1 {
		t.Fatal("resource without permission dispatched")
	}
	if _, err := fixture.manager.GetPrompt(ctx, "srv", "explain", nil); err == nil || fixture.transport.promptCalls != 1 {
		t.Fatal("prompt without permission dispatched")
	}
	requireAuthorityClean(t, fixture.kernel)
}

func TestRemoteAuthority_DiscoveryAndPublicationFailuresRevokeOldAuthority(t *testing.T) {
	for _, kind := range []string{"empty-catalog", "list-error", "risk-assert-error", "registration-assert-error", "registration-retract-error"} {
		t.Run(kind, func(test *testing.T) {
			fixture := newAuthorityFixture(test)
			switch kind {
			case "list-error":
				fixture.transport.listErr = errors.New("catalog unavailable")
			case "risk-assert-error":
				fixture.fault.assertFault = "mcp_tool_risk("
				tool := fixture.manager.servers["srv"].Tools[0]
				tool.Risk = RiskMutating
				fixture.manager.factEmitter().EmitTool(tool)
			case "registration-assert-error":
				fixture.fault.assertFault = "mcp_server_registered("
				fixture.manager.servers["srv"].Server.Endpoint = "changed"
				fixture.manager.factEmitter().EmitServer(fixture.manager.servers["srv"].Server)
			case "registration-retract-error":
				fixture.fault.retractFault = "mcp_tool_registered("
				fixture.manager.factEmitter().RetractTool("srv/op")
			}
			if kind == "empty-catalog" || kind == "list-error" {
				err := fixture.manager.DiscoverTools(context.Background(), "srv")
				if (err != nil) != (kind == "list-error") {
					test.Fatalf("discovery error: %v", err)
				}
			} else if fixture.manager.factEmitter().Error() == nil {
				test.Fatal("publication failure was hidden")
			}
			if err := fixture.call(test, "manager"); err == nil || fixture.transport.calls != 0 {
				test.Fatalf("stale authority dispatched: %v, %d", err, fixture.transport.calls)
			}
		})
	}
}

func TestRemoteAuthority_EscapingAndLargeNumbersPreserveTheAuthorizedWirePayload(t *testing.T) {
	for _, route := range []string{"control-plane", "manager", "integration"} {
		t.Run(route, func(test *testing.T) {
			fixture := newAuthorityFixture(test)
			fixture.args = map[string]any{"text": "λ\x00\"\\\n", "number": uint64(18446744073709551615), "nested": []any{map[string]any{"text": "日本語"}}}
			canonical, err := json.Marshal(fixture.args)
			if err != nil {
				test.Fatal(err)
			}
			fixture.transport.onCall = func(outbound map[string]any) {
				wire, err := json.Marshal(outbound)
				if err != nil || string(wire) != string(canonical) {
					test.Fatalf("authorized payload changed: %s, %v", wire, err)
				}
			}
			if err := fixture.call(test, route); err != nil || fixture.transport.calls != 1 {
				test.Fatalf("escaped call: %v, dispatches %d", err, fixture.transport.calls)
			}
			requireAuthorityClean(test, fixture.kernel)
		})
	}
}
