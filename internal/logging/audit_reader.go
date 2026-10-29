package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Reading the audit log back is what makes `nerd transparency` and
// `nerd glassbox` truthful.
//
// Those commands used to query four kernel predicates — route_to,
// shard_routing, tool_invocation, file_state — that have no producer anywhere
// in the repo: the only occurrences of those strings are the queries
// themselves. Worse, the kernel facts that DO record decisions (user_intent,
// next_action, permitted) are session-scoped and die with the process, so a
// separate CLI invocation could never see a previous run's reasoning even with
// the right predicate names.
//
// The durable record already exists and is already being written: the audit
// log, one JSON object per line, each carrying a pre-formatted Mangle fact.
// These commands just never read it.

// ErrNoAuditLog reports that no audit log exists to read. Audit logging is
// gated on debug mode (see InitAudit), so its absence usually means
// logging.debug_mode is off rather than that nothing happened — a distinction
// the caller must be able to draw before telling the user "nothing recorded".
var ErrNoAuditLog = fmt.Errorf("no audit log found (audit logging requires logging.debug_mode)")

// AuditLogsDir returns the directory audit logs are written to, or "" if
// logging has not been initialized with a workspace.
func AuditLogsDir() string {
	configMu.RLock()
	defer configMu.RUnlock()
	return logsDir
}

// LatestAuditLogPath returns the most recent audit log file. run-prefixed audit
// filenames are lexically chronological because the UTC timestamp is the leading
// component, so lexical order is chronological order.
func LatestAuditLogPath() (string, error) {
	dir := AuditLogsDir()
	if dir == "" {
		return "", ErrNoAuditLog
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*_audit.log"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", ErrNoAuditLog
	}

	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

// ReadRecentAuditEvents returns the newest events whose type is in eventTypes,
// oldest first, capped at limit. An empty eventTypes matches every event.
//
// The log reaches tens of megabytes a day — mostly perf_metric and
// kernel_query — so lines are rejected by substring before being handed to the
// JSON decoder, and only `limit` events are retained at a time.
func ReadRecentAuditEvents(path string, eventTypes []AuditEventType, limit int) ([]AuditEvent, error) {
	if limit <= 0 {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoAuditLog
		}
		return nil, err
	}
	defer f.Close()

	needles := make([]string, 0, len(eventTypes))
	for _, t := range eventTypes {
		needles = append(needles, fmt.Sprintf(`"event":"%s"`, t))
	}

	matchesType := func(line string) bool {
		if len(needles) == 0 {
			return true
		}
		for _, n := range needles {
			if strings.Contains(line, n) {
				return true
			}
		}
		return false
	}

	// Ring buffer: the newest `limit` matches, so memory stays flat no matter
	// how large the log grows.
	ring := make([]AuditEvent, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		if !matchesType(line) {
			continue
		}

		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // a torn final line from a live writer is not an error
		}

		if len(ring) < limit {
			ring = append(ring, event)
			continue
		}
		copy(ring, ring[1:])
		ring[limit-1] = event
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading audit log %s: %w", path, err)
	}

	return ring, nil
}

// CountAuditEventTypes tallies every event type present in the log. Used to
// tell "this never happened" apart from "this is not instrumented" — several
// declared event families (action_route, tool_invoke, file_write) have no
// production call site, and reporting those as "none recorded" invites the
// reader to conclude the system did nothing.
func CountAuditEventTypes(path string) (map[AuditEventType]int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoAuditLog
		}
		return nil, err
	}
	defer f.Close()

	counts := make(map[AuditEventType]int)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		counts[event.EventType]++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading audit log %s: %w", path, err)
	}

	return counts, nil
}
