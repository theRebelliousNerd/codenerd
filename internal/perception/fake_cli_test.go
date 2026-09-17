package perception

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// installFakeCLI puts a copy of this test binary, named name (name.exe on
// Windows), in a fresh temp dir and returns the dir for a test to prepend to
// PATH. Started under that name, the binary acts as the fake CLI instead of
// running tests (see init). A real executable replaces sh + .cmd script
// pairs: cmd.exe re-parses a .cmd's arguments — an empty argument ended the
// argv loop and JSON payloads lost their quotes — so on Windows the pins were
// testing cmd.exe rather than the client.
func installFakeCLI(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("fake cli executable: %v", err)
	}
	dst := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		dst += ".exe"
	}
	// On Windows use a real copy: a hard link shares the image of the
	// running test binary, which Windows refuses to unlink, so t.TempDir's
	// RemoveAll cleanup would fail with Access is denied.
	if runtime.GOOS == "windows" {
		if err := copyFakeBinary(exe, dst); err != nil {
			t.Fatalf("fake cli copy: %v", err)
		}
	} else if err := os.Link(exe, dst); err != nil {
		if err := copyFakeBinary(exe, dst); err != nil {
			t.Fatalf("fake cli copy: %v", err)
		}
	}
	return dir
}

func copyFakeBinary(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create copy: %w", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close copy: %w", err)
	}
	if err := os.Chmod(dstPath, 0o755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}
	return nil
}

func init() {
	exe, err := os.Executable()
	base := ""
	if err == nil {
		base = strings.ToLower(strings.TrimSuffix(filepath.Base(exe), ".exe"))
	} else if len(os.Args) > 0 {
		base = strings.ToLower(strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe"))
	}
	if base == "claude" {
		os.Exit(runFakeClaude(os.Args[1:]))
	}
}

// runFakeClaude mirrors the old script fake; CLAUDE_TEST_MODE selects the behaviour.
func runFakeClaude(args []string) int {
	dir, ok := fakeCLIDir()
	if !ok {
		return 2
	}
	if err := logFakeInvocation(dir, args); err != nil {
		fmt.Fprintf(os.Stderr, "fake claude log: %v\n", err)
		return 2
	}
	return emitFakeOutput(args)
}

func fakeCLIDir() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fake claude executable: %v\n", err)
		return "", false
	}
	return filepath.Dir(exe), true
}

func logFakeInvocation(dir string, args []string) error {
	if err := appendArgsLog(dir, args); err != nil {
		return err
	}
	if err := writeStdinLog(dir); err != nil {
		return err
	}
	return appendCallsLog(dir)
}

func appendArgsLog(dir string, args []string) error {
	f, err := os.OpenFile(filepath.Join(dir, "args.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("args.log: %w", err)
	}
	for _, a := range args {
		if _, err := fmt.Fprintln(f, a); err != nil {
			f.Close()
			return fmt.Errorf("args.log: %w", err)
		}
	}
	if _, err := fmt.Fprintln(f, "---"); err != nil {
		f.Close()
		return fmt.Errorf("args.log: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("args.log: %w", err)
	}
	return nil
}

func writeStdinLog(dir string) error {
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("stdin: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stdin.log"), stdin, 0o644); err != nil {
		return fmt.Errorf("stdin.log: %w", err)
	}
	return nil
}

func appendCallsLog(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, "calls.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("calls.log: %w", err)
	}
	if _, err := fmt.Fprintln(f, "called"); err != nil {
		f.Close()
		return fmt.Errorf("calls.log: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("calls.log: %w", err)
	}
	return nil
}

func emitFakeOutput(args []string) int {
	switch os.Getenv("CLAUDE_TEST_MODE") {
	case "rate_limit":
		fmt.Fprint(os.Stderr, "rate limit reached, try later\n")
		return 1
	case "fallback":
		return emitFakeFallback(args)
	case "stream_long":
		return emitFakeStreamLong()
	default:
		fmt.Fprint(os.Stdout, os.Getenv("CLAUDE_TEST_PAYLOAD"))
		return 0
	}
}

func emitFakeFallback(args []string) int {
	for _, a := range args {
		if a == "fb-model" {
			fmt.Fprint(os.Stdout, os.Getenv("CLAUDE_TEST_PAYLOAD"))
			return 0
		}
	}
	fmt.Fprint(os.Stderr, "rate limit reached\n")
	return 1
}

func emitFakeStreamLong() int {
	fmt.Fprint(os.Stdout, `{"type":"text","text":"`+strings.Repeat("A", 102400)+"\"}\n")
	fmt.Fprint(os.Stdout, "{\"type\":\"done\",\"done\":true}\n")
	return 0
}
