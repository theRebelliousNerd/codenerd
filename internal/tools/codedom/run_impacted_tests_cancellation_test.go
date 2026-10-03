package codedom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"codenerd/internal/processutil"
	"codenerd/internal/tools"
)

type impactedNativeFixture struct {
	Mode        string
	Address     string
	Token       string
	ReceiptPath string
	Workspace   string
	Executable  string
}

type impactedNativeReceipt struct {
	Role        string
	Token       string
	PID         int
	Arguments   []string
	Directory   string
	Environment map[string]string
}

func TestMain(runner *testing.M) {
	executableName := filepath.Base(os.Args[0])
	if executableName == "go" || strings.EqualFold(executableName, "go.exe") {
		executable, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		fixtureBytes, err := os.ReadFile(filepath.Join(filepath.Dir(executable), "impacted-native-fixture.json"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		var fixture impactedNativeFixture
		if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		if err := runImpactedNativeHelper(fixture); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(98)
		}
		if fixture.Mode == "nonzero" {
			os.Exit(23)
		}
		os.Exit(0)
	}
	os.Exit(runner.Run())
}

func runImpactedNativeHelper(fixture impactedNativeFixture) error {
	descendant := len(os.Args) > 1 && os.Args[1] == "--impacted-descendant"
	role := "runner"
	if descendant {
		role = "descendant"
	}
	directory, err := os.Getwd()
	if err != nil {
		return err
	}
	receipt := impactedNativeReceipt{
		Role: role, Token: fixture.Token, PID: os.Getpid(),
		Arguments: os.Args[1:], Directory: directory,
		Environment: make(map[string]string),
	}
	for _, key := range []string{"PATH", "GOPATH", "GOFLAGS", "CGO_ENABLED", "CGO_CFLAGS", "CGO_LDFLAGS", "GOTRACEBACK", "CODENERD_IMPACTED_PRIVATE"} {
		receipt.Environment[key] = os.Getenv(key)
	}
	if !descendant {
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		if err := os.WriteFile(fixture.ReceiptPath, encoded, 0o600); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stdout, "impacted "+role+" stdout preserved")
	fmt.Fprintln(os.Stderr, "impacted "+role+" stderr preserved")
	if fixture.Address == "" {
		return nil
	}
	connection, err := net.DialTimeout("tcp", fixture.Address, 10*time.Second)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.SetDeadline(time.Now().Add(45 * time.Second)); err != nil {
		return err
	}
	if err := json.NewEncoder(connection).Encode(receipt); err != nil {
		return err
	}
	var instruction string
	decoder := json.NewDecoder(connection)
	if err := decoder.Decode(&instruction); err != nil {
		return err
	}
	if descendant {
		if instruction != "release" {
			return fmt.Errorf("unexpected descendant instruction %q", instruction)
		}
		return nil
	}
	if instruction != "spawn" {
		return fmt.Errorf("unexpected runner instruction %q", instruction)
	}
	child := exec.Command(fixture.Executable, "--impacted-descendant")
	child.Dir = directory
	child.Env = os.Environ()
	child.Stdin = strings.NewReader("")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		return err
	}
	if fixture.Mode == "collector-error" {
		if err := decoder.Decode(&instruction); err != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			return err
		}
		if instruction != "exit" {
			_ = child.Process.Kill()
			_ = child.Wait()
			return fmt.Errorf("unexpected exit instruction %q", instruction)
		}
		return nil
	}
	return child.Wait()
}

func newImpactedNativeFixture(test *testing.T, mode string) impactedNativeFixture {
	test.Helper()
	fixtureRoot := test.TempDir()
	workspace := filepath.Join(fixtureRoot, "workspace with spaces")
	binDirectory := filepath.Join(fixtureRoot, "native bin with spaces")
	for _, directory := range []string{workspace, binDirectory, filepath.Join(workspace, ".nerd"), filepath.Join(workspace, "pkg with spaces;$(literal)")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			test.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "go.work"), []byte("go 1.26\n"), 0o600); err != nil {
		test.Fatal(err)
	}
	canonicalWorkspace, err := tools.CanonicalWorkspaceRoot(workspace)
	if err != nil {
		test.Fatal(err)
	}
	buildConfiguration := `{"build":{"go_flags":["-tags=impacted_fixture"],"env_vars":{"CGO_ENABLED":"1","CGO_CFLAGS":"-Ifixture include with spaces -DIMPACTED=1","CGO_LDFLAGS":"-Lfixture lib with spaces"}}}`
	if err := os.WriteFile(filepath.Join(workspace, ".nerd", "config.json"), []byte(buildConfiguration), 0o600); err != nil {
		test.Fatal(err)
	}
	executableName := "go"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	currentExecutable, err := os.Executable()
	if err != nil {
		test.Fatal(err)
	}
	executableBytes, err := os.ReadFile(currentExecutable)
	if err != nil {
		test.Fatal(err)
	}
	executable := filepath.Join(binDirectory, executableName)
	if err := os.WriteFile(executable, executableBytes, 0o700); err != nil {
		test.Fatal(err)
	}
	test.Setenv("PATH", binDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	test.Setenv("GOPATH", filepath.Join(fixtureRoot, "gopath with spaces"))
	test.Setenv("GOFLAGS", "-mod=mod")
	test.Setenv("GOTRACEBACK", "all")
	test.Setenv("CODENERD_IMPACTED_PRIVATE", "must-not-reach-project-code")
	fixture := impactedNativeFixture{
		Mode: mode, Token: fixtureRoot, ReceiptPath: filepath.Join(fixtureRoot, "receipt.json"),
		Workspace: canonicalWorkspace, Executable: executable,
	}
	writeImpactedNativeFixture(test, fixture)
	return fixture
}

func writeImpactedNativeFixture(test *testing.T, fixture impactedNativeFixture) {
	test.Helper()
	encoded, err := json.Marshal(fixture)
	if err != nil {
		test.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(fixture.Executable), "impacted-native-fixture.json"), encoded, 0o600); err != nil {
		test.Fatal(err)
	}
}

func assertImpactedInvocation(test *testing.T, fixture impactedNativeFixture, runs []tools.TestRun, verbose bool) {
	test.Helper()
	encoded, err := os.ReadFile(fixture.ReceiptPath)
	if err != nil {
		test.Fatal(err)
	}
	var receipt impactedNativeReceipt
	if err := json.Unmarshal(encoded, &receipt); err != nil {
		test.Fatal(err)
	}
	expectedArguments := []string{"test", "-tags=impacted_fixture", "-count=1"}
	if verbose {
		expectedArguments = append(expectedArguments, "-v")
	}
	expectedArguments = append(expectedArguments, "-timeout", "1m0s", "./pkg with spaces;$(literal)")
	if !reflect.DeepEqual(receipt.Arguments, expectedArguments) {
		test.Errorf("literal native argv = %#v, want %#v", receipt.Arguments, expectedArguments)
	}
	if receipt.Directory != fixture.Workspace {
		test.Errorf("cwd = %q, want canonical workspace %q", receipt.Directory, fixture.Workspace)
	}
	for key, expected := range map[string]string{
		"PATH": os.Getenv("PATH"), "GOPATH": os.Getenv("GOPATH"), "GOFLAGS": "-mod=mod -count=1",
		"CGO_ENABLED": "1", "CGO_CFLAGS": "-Ifixture include with spaces -DIMPACTED=1",
		"CGO_LDFLAGS": "-Lfixture lib with spaces", "GOTRACEBACK": "all", "CODENERD_IMPACTED_PRIVATE": "",
	} {
		if receipt.Environment[key] != expected {
			test.Errorf("native environment %s = %q, want %q", key, receipt.Environment[key], expected)
		}
	}
	if len(runs) != 1 {
		test.Fatalf("execution receipts = %+v, want exactly one", runs)
	}
	if len(runs[0].Argv) == 0 || runs[0].Argv[0] != "go" || !reflect.DeepEqual(runs[0].Argv[1:], expectedArguments) {
		test.Errorf("receipt argv lost actual invocation: %#v", runs[0].Argv)
	}
}

func TestRunGoTestsNativeResults(test *testing.T) {
	for _, mode := range []string{"success", "nonzero", "startup-error"} {
		test.Run(mode, func(test *testing.T) {
			fixture := newImpactedNativeFixture(test, mode)
			if mode == "startup-error" {
				if err := os.WriteFile(fixture.Executable, []byte("not a native executable"), 0o700); err != nil {
					test.Fatal(err)
				}
			}
			ctx, runs := tools.WithTestRunLog(test.Context())
			output, err := runGoTests(ctx, fixture.Workspace, []string{filepath.Join(fixture.Workspace, "pkg with spaces;$(literal)")}, "60s", true)
			if mode == "startup-error" {
				if err == nil || len(runs()) != 0 || !strings.Contains(output, "Error:") {
					test.Fatalf("startup failure became verification: error=%v output=%q receipts=%+v", err, output, runs())
				}
				return
			}
			if mode == "success" && err != nil {
				test.Fatalf("native success: %v\n%s", err, output)
			}
			if mode == "nonzero" && err == nil {
				test.Fatal("native nonzero exit returned success")
			}
			assertImpactedInvocation(test, fixture, runs(), true)
			expectedExitCode := 0
			if mode == "nonzero" {
				expectedExitCode = 23
			}
			if runs()[0].ExitCode != expectedExitCode {
				test.Errorf("receipt exit code = %d, want %d", runs()[0].ExitCode, expectedExitCode)
			}
			for _, expectedOutput := range []string{"impacted runner stdout preserved", "impacted runner stderr preserved", "Directory: " + fixture.Workspace} {
				if !strings.Contains(output, expectedOutput) {
					test.Errorf("runner output lost %q: %s", expectedOutput, output)
				}
			}
		})
	}
}

func acceptImpactedHelper(test *testing.T, listener *net.TCPListener, token, role string) (net.Conn, impactedNativeReceipt) {
	test.Helper()
	if err := listener.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		test.Fatal(err)
	}
	connection, err := listener.Accept()
	if err != nil {
		test.Fatalf("admit %s: %v", role, err)
	}
	test.Cleanup(func() { _ = connection.Close() })
	if err := connection.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		test.Fatal(err)
	}
	var receipt impactedNativeReceipt
	if err := json.NewDecoder(connection).Decode(&receipt); err != nil {
		test.Fatalf("read %s admission: %v", role, err)
	}
	if receipt.Token != token || receipt.Role != role || receipt.PID <= 0 {
		test.Fatalf("unexpected %s admission: %+v", role, receipt)
	}
	return connection, receipt
}

func TestRunGoTestsNativeTreeCancellation(test *testing.T) {
	for _, mode := range []string{"cancel", "collector-error"} {
		test.Run(mode, func(test *testing.T) {
			test.Logf("native impacted-runner lifecycle on %s", runtime.GOOS)
			fixture := newImpactedNativeFixture(test, mode)
			listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("127.0.0.1")})
			if err != nil {
				test.Fatal(err)
			}
			defer listener.Close()
			fixture.Address = listener.Addr().String()
			writeImpactedNativeFixture(test, fixture)
			callerContext, cancel := context.WithCancel(test.Context())
			defer cancel()
			ctx, runs := tools.WithTestRunLog(callerContext)
			type runnerResult struct {
				output string
				err    error
			}
			results := make(chan runnerResult, 1)
			finished := make(chan struct{})
			go func() {
				output, err := runGoTests(ctx, fixture.Workspace, []string{filepath.Join(fixture.Workspace, "pkg with spaces;$(literal)")}, "60s", false)
				results <- runnerResult{output: output, err: err}
				close(finished)
			}()
			var descendantConnection net.Conn
			test.Cleanup(func() {
				cancel()
				if descendantConnection != nil {
					_ = descendantConnection.SetWriteDeadline(time.Now().Add(time.Second))
					_ = json.NewEncoder(descendantConnection).Encode("release")
				}
				select {
				case <-finished:
				case <-time.After(12 * time.Second):
					test.Error("owned runner did not join after fixture cleanup")
				}
			})
			parentConnection, _ := acceptImpactedHelper(test, listener, fixture.Token, "runner")
			if err := json.NewEncoder(parentConnection).Encode("spawn"); err != nil {
				test.Fatal(err)
			}
			var descendantReceipt impactedNativeReceipt
			descendantConnection, descendantReceipt = acceptImpactedHelper(test, listener, fixture.Token, "descendant")
			var descendantExit <-chan error
			if runtime.GOOS == "windows" {
				process, err := os.FindProcess(descendantReceipt.PID)
				if err != nil {
					test.Fatalf("open admitted descendant process: %v", err)
				}
				processExited := make(chan error, 1)
				go func() {
					_, err := process.Wait()
					processExited <- err
					close(processExited)
				}()
				descendantExit = processExited
				test.Cleanup(func() {
					cancel()
					_ = descendantConnection.SetWriteDeadline(time.Now().Add(time.Second))
					_ = json.NewEncoder(descendantConnection).Encode("release")
					select {
					case <-processExited:
					case <-time.After(12 * time.Second):
						test.Error("admitted descendant process did not exit")
					}
				})
			}
			startedCollection := time.Now()
			if mode == "cancel" {
				cancel()
			} else if err := json.NewEncoder(parentConnection).Encode("exit"); err != nil {
				test.Fatal(err)
			}
			var result runnerResult
			select {
			case result = <-results:
			case <-time.After(processutil.PipeWaitDelay + 5*time.Second):
				test.Fatal("descendant-held output pipes blocked the actual runner")
			}
			if result.err == nil {
				test.Fatalf("%s published success: %s", mode, result.output)
			}
			if mode == "collector-error" && !errors.Is(result.err, exec.ErrWaitDelay) {
				test.Errorf("successful parent with retained pipes returned %v, want ErrWaitDelay", result.err)
			}
			if elapsed := time.Since(startedCollection); elapsed > processutil.PipeWaitDelay+5*time.Second {
				test.Errorf("pipe collection exceeded bound: %v", elapsed)
			}
			assertImpactedInvocation(test, fixture, runs(), false)
			if runs()[0].ExitCode == 0 {
				test.Fatalf("%s left a passing verification receipt: %+v", mode, runs())
			}
			for _, expectedOutput := range []string{"impacted runner stdout preserved", "impacted runner stderr preserved", "impacted descendant stdout preserved", "impacted descendant stderr preserved"} {
				if !strings.Contains(result.output, expectedOutput) {
					test.Errorf("retained-pipe output lost %q: %s", expectedOutput, result.output)
				}
			}
			if mode == "collector-error" {
				if err := json.NewEncoder(descendantConnection).Encode("release"); err != nil {
					test.Fatal(err)
				}
			}
			if err := descendantConnection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				test.Fatal(err)
			}
			var unexpectedByte [1]byte
			if _, err := descendantConnection.Read(unexpectedByte[:]); !errors.Is(err, io.EOF) && !(runtime.GOOS == "windows" && descendantExit != nil && errors.Is(err, syscall.Errno(10054))) {
				test.Fatalf("admitted descendant remained alive after cancellation/release: %v", err)
			}
			if descendantExit != nil {
				select {
				case err := <-descendantExit:
					if err != nil {
						test.Errorf("wait admitted descendant exit: %v", err)
					}
				case <-time.After(5 * time.Second):
					test.Fatal("descendant socket closed but admitted Windows process remained alive")
				}
			}
		})
	}
}
