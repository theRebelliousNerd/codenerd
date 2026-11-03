package main

import (
	"bytes"
	"codeberg.org/TauCeti/mangle-go/ast"
	"errors"
	"github.com/google/go-cmp/cmp"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMainPrintsFields(t *testing.T) {
	output := captureStdout(t, func() {
		main()
	})
	if !strings.Contains(output, "Field:") {
		t.Fatalf("expected output to include field names")
	}
}

func TestMainPrintsCompleteClauseLayout(t *testing.T) {
	clauseType := reflect.TypeFor[ast.Clause]()
	var listing string
	for i := 0; i < clauseType.NumField(); i++ {
		listing += "Field: " + clauseType.Field(i).Name + "\n"
	}

	tests := []struct {
		name        string
		invocations int
	}{
		{name: "single invocation", invocations: 1},
		{name: "repeated invocations", invocations: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// main writes process-global stdout; these cases must stay serial.
			got := captureStdout(t, func() {
				for range tt.invocations {
					main()
				}
			})
			want := strings.Repeat(listing, tt.invocations)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("main() output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	t.Cleanup(func() {
		os.Stdout = orig
		if err := w.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close stdout writer: %v", err)
		}
		if err := r.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			t.Errorf("close stdout reader: %v", err)
		}
	})
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close captured stdout: %v", err)
	}
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return buf.String()
}
