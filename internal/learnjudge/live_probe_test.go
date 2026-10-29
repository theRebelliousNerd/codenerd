//go:build jevlive

package learnjudge

// Live probes of the learnjudge questions against the configured Jev:
//
//	go test -tags jevlive ./internal/learnjudge/ -run Live -v
//
// Each probe sends its cases through the real question builders and keeps
// every exchange (request and reply, never the key: it travels in the
// Authorization header, which the recorder does not read) under
// Docs/architecture/jev-probes/2026-10-28-learning/<use>/, so any result can
// be rerun. Probes report; they assert only that the call worked, because a
// probability is a measurement here, not a contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

const probeRoot = "../../Docs/architecture/jev-probes/2026-10-28-learning"

type probeLabelKey struct{}

// probeLabel names the case a request belongs to, for its recorded file.
func probeLabel(ctx context.Context, label string) context.Context {
	return context.WithValue(ctx, probeLabelKey{}, label)
}

// liveClient builds the client from the repository's .nerd/config.json, with
// every exchange recorded under probeRoot/use. It skips when no typesafe
// section is configured.
func liveClient(t *testing.T, use string) (*typesafe.Client, *config.TypeSafeConfig) {
	t.Helper()
	cfg, err := config.LoadUserConfig("../../.nerd/config.json")
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	r, err := cfg.TypeSafe.Resolve()
	if err != nil || r == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	dir := filepath.Join(probeRoot, use)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	hc := &http.Client{Transport: &probeRecorder{dir: dir, next: http.DefaultTransport}}
	c, err := typesafe.NewClient(typesafe.Options{Model: r.Model, BaseURL: r.BaseURL, APIKey: r.APIKey, LongContext: r.LongContext, Timeout: r.Timeout, HTTPClient: hc})
	if err != nil {
		t.Fatal(err)
	}
	return c, cfg.TypeSafe
}

type probeRecorder struct {
	dir  string
	next http.RoundTripper
	seq  atomic.Int64
}

func (r *probeRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	resp, err := r.next.RoundTrip(req)
	if err != nil || req.Method != http.MethodPost {
		return resp, err
	}
	reply, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(reply))
	label, _ := req.Context().Value(probeLabelKey{}).(string)
	label = strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			return c
		}
		return '_'
	}, label)
	name := fmt.Sprintf("%04d-%s.json", r.seq.Add(1), label)
	rec := fmt.Sprintf("{\n\"status\": %d,\n\"request\": %s,\n\"reply\": %s\n}\n", resp.StatusCode, probeIndent(body), probeIndent(reply))
	_ = os.WriteFile(filepath.Join(r.dir, name), []byte(rec), 0o644)
	return resp, nil
}

func probeIndent(b []byte) []byte {
	var out bytes.Buffer
	if json.Indent(&out, b, "", " ") != nil {
		q, _ := json.Marshal(string(b))
		return q
	}
	return out.Bytes()
}

// probeSummary appends lines to probeRoot/use.txt, the human-readable result.
func probeSummary(t *testing.T, use string, lines []string) {
	t.Helper()
	path := filepath.Join(probeRoot, use+".txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		t.Log(l)
	}
}
