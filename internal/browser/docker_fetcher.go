package browser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Docker log fetching is bounded on every axis. Container logs are
// attacker-influenced and effectively unbounded, and they are fetched
// during a diagnosis the agent is actively waiting on. Capping total
// captured output and stderr excerpts keeps a single diagnosis from
// reading arbitrary disk, stalling on a large fetch, or flooding the
// kernel with derived facts. This mirrors the flight recorder's
// bounded-record idiom in internal/browser/flight_recorder.go.
const (
	// maxDockerLogBytes caps total captured docker logs output (stdout +
	// stderr combined) at 4 MiB. A container can log unboundedly and this
	// runs inside a diagnosis the agent is waiting on, so the read must
	// be bounded.
	maxDockerLogBytes = 4 << 20
	// maxDockerLogStderrExcerpt caps the stderr excerpt included in a
	// fetcher error at 512 bytes so a noisy daemon error cannot blow up a Note.
	maxDockerLogStderrExcerpt = 512
)

// sharedLimit tracks remaining budget across concurrent stdout/stderr writers.
type sharedLimit struct {
	mu        sync.Mutex
	remaining int
}

// boundedWriter writes to buf while respecting the shared remaining budget.
// It is safe for concurrent use via sharedLimit's mutex and discards bytes
// beyond the cap while still reporting success to the caller so exec's
// internal copy loop does not treat the truncation as an error.
type boundedWriter struct {
	buf   *bytes.Buffer
	limit *sharedLimit
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	orig := len(p)
	w.limit.mu.Lock()
	defer w.limit.mu.Unlock()
	if w.limit.remaining <= 0 {
		return orig, nil
	}
	if orig > w.limit.remaining {
		p = p[:w.limit.remaining]
	}
	n, _ := w.buf.Write(p)
	w.limit.remaining -= n
	return orig, nil
}

// LookupDockerBinary returns the resolved absolute path to the docker
// executable, or "" when it must not be used.
//
// Security ordering: the allowlist is checked FIRST and is not optional.
// PATH presence is capability, the allowlist is authorization, and
// authorization is checked first. A docker binary present on PATH but
// absent from the operator's allowlist must not be run.
func LookupDockerBinary(allowedBinaries []string) string {
	// Authorization before capability: check allowlist first.
	allowed := false
	for _, b := range allowedBinaries {
		if strings.EqualFold(b, "docker") || strings.EqualFold(b, "docker.exe") {
			allowed = true
			break
		}
	}
	if !allowed {
		return ""
	}
	path, err := exec.LookPath("docker")
	if err != nil {
		return ""
	}
	return path
}

// NewDockerLogFetcher returns a ContainerLogFetcher that shells out to the
// docker CLI, or nil when dockerPath is "".
//
// The returned fetcher:
//   - builds the argument list: logs --timestamps --since <RFC3339Nano> --until <RFC3339Nano> <container>
//   - uses exec.CommandContext with ctx so cancellation kills the process
//   - passes the container name as its own argv element — never interpolated into a shell string —
//     and rejects empty or "-" prefixed names that would be parsed as flags
//   - captures stdout and stderr separately; Docker writes container stderr into the docker logs
//     stream so BOTH streams carry log content; they are combined for parsing but the command's
//     own failure remains distinguishable via the exit error
//   - on non-zero exit returns an error that includes the container name and a bounded excerpt of stderr (512 bytes)
//   - bounds the read by capping total captured output at 4 MiB
func NewDockerLogFetcher(dockerPath string) ContainerLogFetcher {
	if dockerPath == "" {
		return nil
	}
	return func(ctx context.Context, container string, since, until time.Time) ([]ContainerLogLine, error) {
		if container == "" {
			return nil, fmt.Errorf("container %q: container name must not be empty", container)
		}
		if strings.HasPrefix(container, "-") {
			return nil, fmt.Errorf("container %q: container name must not begin with \"-\"", container)
		}

		sinceStr := since.Format(time.RFC3339Nano)
		untilStr := until.Format(time.RFC3339Nano)

		// Do NOT invoke a shell. Container is passed as its own argv element.
		cmd := exec.CommandContext(ctx, dockerPath, "logs", "--timestamps", "--since", sinceStr, "--until", untilStr, container)

		shared := &sharedLimit{remaining: maxDockerLogBytes}
		var stdoutBuf, stderrBuf bytes.Buffer
		stdoutW := &boundedWriter{buf: &stdoutBuf, limit: shared}
		stderrW := &boundedWriter{buf: &stderrBuf, limit: shared}
		cmd.Stdout = stdoutW
		cmd.Stderr = stderrW

		err := cmd.Run()
		if err != nil {
			excerpt := stderrBuf.String()
			if len(excerpt) > maxDockerLogStderrExcerpt {
				excerpt = excerpt[:maxDockerLogStderrExcerpt]
			}
			return nil, fmt.Errorf("docker logs for container %q failed: %w: %s", container, err, excerpt)
		}

		// BOTH streams carry log content; combine them for parsing.
		// Total is already bounded by boundedWriter, but enforce cap again on the combined slice.
		raw := make([]byte, 0, stdoutBuf.Len()+stderrBuf.Len())
		raw = append(raw, stdoutBuf.Bytes()...)
		raw = append(raw, stderrBuf.Bytes()...)
		if len(raw) > maxDockerLogBytes {
			raw = raw[:maxDockerLogBytes]
		}

		lines := parseDockerLogLines(container, raw)
		return lines, nil
	}
}

// parseDockerLogLines parses `docker logs --timestamps` output. Each line is
// an RFC3339Nano timestamp, a single space, then the message.
//
// Rules:
//   - Split on "\n" and tolerate a trailing "\r" (Windows).
//   - Skip empty lines.
//   - A line whose leading token does not parse as RFC3339Nano is NOT
//     dropped: keep it with a zero Timestamp and the full line as Message.
//     Losing an unparseable line would silently hide exactly the malformed
//     output most worth seeing.
//   - Set Container on every line.
//
// This function is exported-to-the-package and pure so it is directly
// testable without Docker.
func parseDockerLogLines(container string, raw []byte) []ContainerLogLine {
	if len(raw) == 0 {
		return nil
	}
	text := string(raw)
	parts := strings.Split(text, "\n")
	out := make([]ContainerLogLine, 0, len(parts))
	for _, line := range parts {
		// Tolerate Windows CRLF.
		if strings.HasSuffix(line, "\r") {
			line = strings.TrimSuffix(line, "\r")
		}
		if line == "" {
			continue
		}
		idx := strings.Index(line, " ")
		if idx == -1 {
			// No space: cannot split timestamp from message; keep whole line with zero time.
			// Documented choice: unparseable lines are preserved with zero Timestamp so malformed
			// output is not silently hidden.
			out = append(out, ContainerLogLine{
				Container: container,
				Timestamp: time.Time{},
				Message:   line,
			})
			continue
		}
		tsStr := line[:idx]
		msg := line[idx+1:]
		ts, err := time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			// Keep unparseable line with zero Timestamp and full line as Message.
			out = append(out, ContainerLogLine{
				Container: container,
				Timestamp: time.Time{},
				Message:   line,
			})
			continue
		}
		out = append(out, ContainerLogLine{
			Container: container,
			Timestamp: ts,
			Message:   msg,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
