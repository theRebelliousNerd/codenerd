package main

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

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

type labelKey struct{}

// withLabel names the case a request belongs to, for its recorded file.
func withLabel(ctx context.Context, label string) context.Context {
	return context.WithValue(ctx, labelKey{}, label)
}

// newClient builds the client from the typesafe section; with a directory it
// keeps every exchange there. The key stays in the Authorization header, which
// the recorder never reads.
func newClient(sec *config.TypeSafeConfig, dir string) (*typesafe.Client, error) {
	r, err := sec.Resolve()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("no typesafe section")
	}
	hc := &http.Client{}
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		hc.Transport = &recorder{dir: dir, next: http.DefaultTransport}
	}
	return typesafe.NewClient(typesafe.Options{Model: r.Model, BaseURL: r.BaseURL, APIKey: r.APIKey, LongContext: r.LongContext, Timeout: r.Timeout, HTTPClient: hc})
}

type recorder struct {
	dir  string
	next http.RoundTripper
	seq  atomic.Int64
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
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
	label, _ := req.Context().Value(labelKey{}).(string)
	label = strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			return c
		}
		return '_'
	}, label)
	name := fmt.Sprintf("%04d-%s.json", r.seq.Add(1), label)
	rec := fmt.Sprintf("{\n\"status\": %d,\n\"request\": %s,\n\"reply\": %s\n}\n", resp.StatusCode, indent(body), indent(reply))
	_ = os.WriteFile(filepath.Join(r.dir, name), []byte(rec), 0o644)
	return resp, nil
}

func indent(b []byte) []byte {
	var out bytes.Buffer
	if err := jsonIndent(&out, b); err != nil {
		return []byte(fmt.Sprintf("%q", b))
	}
	return out.Bytes()
}

func jsonIndent(out *bytes.Buffer, b []byte) error { return json.Indent(out, b, "", " ") }
