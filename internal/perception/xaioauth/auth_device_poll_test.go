package xaioauth

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Without expires_in the server never named a deadline, so the poll runs
// until the context ends or the token endpoint reports expired_token
// (RFC 8628). A zero and a negative expires_in both mean "absent".
func TestDevicePoll_NoExpiresIn_StopsOnExpiredToken(t *testing.T) {
	for _, expiresIn := range []time.Duration{0, -time.Second} {
		var calls int32
		rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if atomic.AddInt32(&calls, 1) == 1 {
				return jsonPollResponse(`{"error":"authorization_pending"}`), nil
			}
			// A real RFC 8628 token endpoint answers a dead code with 400.
			return pollStatusResponse(http.StatusBadRequest,
				`{"error":"expired_token","error_description":"code expired"}`), nil
		})
		hc := &http.Client{Transport: rt, Timeout: 10 * time.Second}
		_, err := PollDeviceToken(context.Background(), hc, "http://issuer/oauth2/token",
			"cid", "dev", time.Millisecond, expiresIn)
		if !IsAuthRequired(err) || !strings.Contains(err.Error(), "expired_token") {
			t.Errorf("expiresIn=%v: err = %v, want AuthRequiredError naming expired_token", expiresIn, err)
		}
		if got := atomic.LoadInt32(&calls); got != 2 {
			t.Errorf("expiresIn=%v: poll attempts = %d, want 2 (pending, expired)", expiresIn, got)
		}
	}
}

// Without expires_in nothing client-side may stop the poll: it ends when the
// context does, after as many polls as that takes.
func TestDevicePoll_NoExpiresIn_PollsUntilContextEnds(t *testing.T) {
	var calls int32
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return jsonPollResponse(`{"error":"authorization_pending"}`), nil
	})
	hc := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := PollDeviceToken(ctx, hc, "http://issuer/oauth2/token",
		"cid", "dev", time.Millisecond, 0)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if got := atomic.LoadInt32(&calls); got < 2 {
		t.Errorf("poll attempts = %d, want at least 2 before the context ended", got)
	}
}

// A positive expires_in is the authorization server's protocol deadline and
// still stops the poll. (The 10s context is a backstop so a regression fails
// instead of polling into the void.)
func TestDevicePoll_PositiveExpiresIn_IsProtocolDeadline(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return jsonPollResponse(`{"error":"authorization_pending"}`), nil
	})
	hc := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := PollDeviceToken(ctx, hc, "http://issuer/oauth2/token",
		"cid", "dev", 10*time.Millisecond, 100*time.Millisecond)
	if !IsAuthRequired(err) || !strings.Contains(err.Error(), "device authorization timed out") {
		t.Fatalf("err = %v, want the protocol-deadline timeout", err)
	}
}

// PollDeviceToken's deleted 15-minute fallback only fired at 15 minutes, so no
// fast behavioral test can tell it is gone. This pins its absence instead: the
// poll interval and slow_down backoff are seconds-scale, so any minute- or
// hour-scale clock inside the poll is a client fallback that came back.
func TestPollDeviceToken_HasNoClientFallbackClock(t *testing.T) {
	src, err := os.ReadFile("auth_device.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "auth_device.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var offenders []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "PollDeviceToken" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "time" {
				return true
			}
			if sel.Sel.Name == "Minute" || sel.Sel.Name == "Hour" {
				offenders = append(offenders, fset.Position(sel.Pos()).String())
			}
			return true
		})
	}
	if len(offenders) > 0 {
		t.Errorf("client fallback clock in PollDeviceToken:\n  %s", strings.Join(offenders, "\n  "))
	}
}

func pollStatusResponse(status int, body string) *http.Response {
	resp := jsonPollResponse(body)
	resp.StatusCode = status
	return resp
}
