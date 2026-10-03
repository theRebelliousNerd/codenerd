package autopoiesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The generated wrapper emits `{"output": <raw json>}`. Reading that back as a
// Go string silently worked only for tools whose return value was NOT valid
// JSON, because the wrapper marshals those into a JSON string. Every other
// tool — anything returning a count, a bool, or a JSON document — registered
// successfully and then failed on its first call.
func TestDecodeToolOutput_WhenWrapperEmitsNonStringJSON_ShouldRenderItAsText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"marshalled plain text", `"processed: hi"`, "processed: hi"},
		{"bare number", `3`, "3"},
		{"bare bool", `true`, "true"},
		{"json object passthrough", `{"lines":3}`, `{"lines":3}`},
		{"json array passthrough", `[1,2,3]`, `[1,2,3]`},
		{"null", `null`, ""},
		{"empty", ``, ""},
		{"string containing json", `"{\"a\":1}"`, `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeToolOutput(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("decodeToolOutput(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

// A copied test executable is the real subprocess fixture on every host OS.
func init() {
	if !strings.HasPrefix(filepath.Base(os.Args[0]), "generated-runtime-fixture") {
		return
	}
	var input string
	if len(os.Args) > 1 {
		input = os.Args[1]
	} else {
		var envelope struct {
			Input string `json:"input"`
		}
		if json.NewDecoder(os.Stdin).Decode(&envelope) != nil {
			os.Exit(9)
		}
		input = envelope.Input
	}
	var args struct {
		Mode      string          `json:"mode"`
		Value     json.RawMessage `json:"value"`
		Admission string          `json:"admission"`
	}
	if json.Unmarshal([]byte(input), &args) != nil {
		os.Exit(8)
	}
	switch args.Mode {
	case "error":
		fmt.Print(`{"output":"partial","error":"fixture failure"}`)
		os.Exit(3)
	case "malformed":
		fmt.Print("partial malformed output")
		fmt.Fprint(os.Stderr, "fixture diagnostic")
		os.Exit(4)
	case "block":
		connection, err := net.Dial("tcp", args.Admission)
		if err != nil {
			os.Exit(7)
		}
		_, _ = connection.Write([]byte{1})
		var release [1]byte
		_, _ = connection.Read(release[:])
		_ = connection.Close()
	case "argv":
		fmt.Print(input)
		os.Exit(0)
	}
	if args.Value == nil {
		args.Value = json.RawMessage(`"literal"`)
	}
	_, _ = os.Stdout.Write(append(append([]byte(`{"output":`), args.Value...), '}'))
	os.Exit(0)
}

func runtimeFixtureRequest(t *testing.T, args map[string]any, protocol types.GeneratedToolProtocol) types.GeneratedToolRequest {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "generated-runtime-fixture.exe")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	canonical, err := types.CanonicalGeneratedArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	return types.GeneratedToolRequest{ScopeID: "runtime", CallID: "call", AuthorizationID: "exec-call", Action: "/fixture", Target: "unknown", CanonicalArgs: canonical,
		Tool: types.GeneratedToolIdentity{Name: "fixture", BinaryPath: path, BinaryHash: hex.EncodeToString(digest[:]), Protocol: protocol}}
}

func TestGeneratedRuntimeOutputAndFailures(t *testing.T) {
	for _, scenario := range []struct {
		name, mode string
		value      any
		output     string
		failed     bool
	}{
		{"string", "", "text", "text", false}, {"number", "", 3, "3", false},
		{"object", "", map[string]any{"n": 2}, `{"n":2}`, false}, {"null", "", nil, "", false},
		{"nonzero", "error", nil, "partial", true}, {"malformed", "malformed", nil, "partial malformed output", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			request := runtimeFixtureRequest(t, map[string]any{"mode": scenario.mode, "value": scenario.value}, types.GeneratedStdinV1)
			receipt := RunGeneratedBinary(context.Background(), request)
			if !receipt.ProcessStarted || receipt.Output != scenario.output || (receipt.BackendError != nil) != scenario.failed {
				t.Fatalf("receipt: %+v", receipt)
			}
			if scenario.failed && !receipt.PartialOutput {
				t.Fatal("failure discarded partial output")
			}
		})
	}
}

func TestGeneratedRuntimeProtocolAndIdentityRefusals(t *testing.T) {
	request := runtimeFixtureRequest(t, map[string]any{"mode": "argv"}, types.LegacyArgvV1)
	receipt := RunGeneratedBinary(context.Background(), request)
	if receipt.BackendError != nil || receipt.Output != request.CanonicalArgs {
		t.Fatalf("legacy argv: %+v", receipt)
	}
	request.Tool.Protocol = "unknown"
	if receipt := RunGeneratedBinary(context.Background(), request); receipt.ProcessStarted || receipt.BackendError == nil {
		t.Fatalf("unknown protocol admitted: %+v", receipt)
	}
	request.Tool.Protocol = types.GeneratedStdinV1
	file, err := os.OpenFile(request.Tool.BinaryPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write([]byte("mutated"))
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatal(errors.Join(err, closeErr))
	}
	if receipt := RunGeneratedBinary(context.Background(), request); receipt.ProcessStarted || receipt.BackendError == nil {
		t.Fatalf("changed binary admitted: %+v", receipt)
	}
}

func TestGeneratedRuntimeCancellationJoinsProcess(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	request := runtimeFixtureRequest(t, map[string]any{"mode": "block", "admission": listener.Addr().String()}, types.GeneratedStdinV1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan types.GeneratedToolReceipt, 1)
	processDone, acceptDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(processDone); result <- RunGeneratedBinary(ctx, request) }()
	accepted := make(chan net.Conn, 1)
	go func() { defer close(acceptDone); connection, _ := listener.Accept(); accepted <- connection }()
	var connection net.Conn
	defer func() {
		cancel()
		listener.Close()
		<-processDone
		<-acceptDone
		if connection != nil {
			connection.Close()
		} else {
			select {
			case remaining := <-accepted:
				if remaining != nil {
					remaining.Close()
				}
			default:
			}
		}
	}()
	select {
	case connection = <-accepted:
	case <-time.After(15 * time.Second):
		t.Fatal("process did not enter fixture")
	}
	if connection == nil {
		t.Fatal("no child connection")
	}
	if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var marker [1]byte
	if _, err := connection.Read(marker[:]); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case receipt := <-result:
		if !receipt.ProcessStarted || !errors.Is(receipt.BackendError, context.Canceled) {
			t.Fatalf("cancel receipt: %+v", receipt)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("canceled process was not joined")
	}
	if _, err := connection.Read(marker[:]); !errors.Is(err, io.EOF) && !(runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10054))) {
		t.Fatalf("child connection was not joined: %v", err)
	}
}
