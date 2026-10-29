package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Section 4C (Docs/audits/mcp-capability-admission.md): ControlPlane.Call
// reported every refusal before dispatch as "transport error". The boundary
// now types each refusal, and the control plane says it reached no server and
// what to do instead.
func TestRemoteAuthority_RefusalsBeforeDispatchAreTyped(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, *authorityFixture)
		want    RemoteRefusal
	}{
		{"no host review", func(_ *testing.T, f *authorityFixture) { f.manager.RevokeReviewedToolEffect("srv/op") }, RefusedNotAdmitted},
		{"no kernel", func(_ *testing.T, f *authorityFixture) { f.manager.factEmitter().kernel = nil }, RefusedAuthorityUnavailable},
		{"no permission", func(_ *testing.T, f *authorityFixture) { f.fault.emptyQuery = "mcp_remote_permitted(" }, RefusedPolicyDenied},
		{"invalid tool name", func(_ *testing.T, f *authorityFixture) { f.tool = "srv/" }, RefusedInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAuthorityFixture(t)
			tc.prepare(t, f)
			tool := "srv/op"
			if f.tool != "" {
				tool = f.tool
			}
			_, err := f.manager.CallTool(f.ctx, tool, f.args)
			if got, ok := RefusalOf(err); !ok || got != tc.want {
				t.Fatalf("CallTool err = %v (refusal %q, %t), want %q", err, got, ok, tc.want)
			}
			if f.transport.calls != 0 {
				t.Fatalf("a refused call reached the server %d times", f.transport.calls)
			}
		})
	}
}

func TestControlPlaneCall_ARefusalIsNotATransportError(t *testing.T) {
	f := newAuthorityFixture(t)
	f.manager.RevokeReviewedToolEffect("srv/op")
	plane := NewControlPlane(f.manager, nil, f.manager.factEmitter())
	plane.SetRiskGate(KernelRiskGate(f.manager.factEmitter().kernel))
	result := plane.Call(f.ctx, CallOptions{Tool: "srv/op", Args: f.args})
	if result.Success || f.transport.calls != 0 {
		t.Fatalf("result = %+v (calls %d), want a refusal with no dispatch", result, f.transport.calls)
	}
	if result.Summary == "transport error" || !strings.Contains(result.Summary, string(RefusedNotAdmitted)) || !strings.Contains(result.NextStep, "reviews") {
		t.Fatalf("summary %q next %q, want the not-admitted refusal and how an operator admits the tool", result.Summary, result.NextStep)
	}
}

func TestCallFailure_KeepsTransportAndCancellationApart(t *testing.T) {
	if s, _ := callFailure(errors.New("connection reset")); s != "transport error" {
		t.Errorf("a transport failure = %q", s)
	}
	if s, _ := callFailure(context.Canceled); s != "canceled" {
		t.Errorf("a cancellation = %q", s)
	}
}
