package logging

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// Offline export of the audit trail into a loadable Mangle facts file.
//
// Every audit event already carries a pre-formatted fact string (the `mangle`
// field), but nothing could turn a log into something the kernel — or a
// standalone `mangle` run — would accept: the facts had no Decl statements, no
// deduplication, and were interleaved with JSON. This produces a .mg file that
// loads on its own.
//
// Deliberately offline (OPEN-QUESTIONS Q1): the file is written for a human or
// a forensic query, never fed back into the live executive. Telemetry that
// re-enters the kernel would let the record of what happened change what
// happens next, which is exactly the loop the north star keeps open.

// AuditFactExport reports what an export produced.
type AuditFactExport struct {
	Events     int            // audit lines parsed
	Facts      int            // unique facts written
	Duplicates int            // facts collapsed by dedup
	Predicates map[string]int // predicate name -> arity
}

// ExportAuditFacts reads an audit JSONL log and writes a Mangle facts file to
// w. eventTypes filters by event family; empty means everything.
func ExportAuditFacts(auditPath string, w io.Writer, eventTypes []AuditEventType) (AuditFactExport, error) {
	stats := AuditFactExport{Predicates: map[string]int{}}

	f, err := os.Open(auditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return stats, ErrNoAuditLog
		}
		return stats, err
	}
	defer f.Close()

	wanted := make(map[AuditEventType]bool, len(eventTypes))
	for _, t := range eventTypes {
		wanted[t] = true
	}

	seen := make(map[string]bool)
	var facts []string

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		var event AuditEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // torn final line from a live writer is not an error
		}
		if len(wanted) > 0 && !wanted[event.EventType] {
			continue
		}
		stats.Events++

		fact := strings.TrimSpace(event.MangleFact)
		if fact == "" {
			// Logs written before the fact string was populated, and any event
			// whose type had no case in generateMangleFact, still export.
			fact = generateMangleFact(event)
		}
		name, arity, ok := parseFactShape(fact)
		if !ok {
			continue
		}
		if seen[fact] {
			stats.Duplicates++
			continue
		}
		seen[fact] = true
		facts = append(facts, fact)
		if existing, found := stats.Predicates[name]; !found || arity > existing {
			stats.Predicates[name] = arity
		}
	}
	if err := scanner.Err(); err != nil {
		return stats, fmt.Errorf("reading audit log %s: %w", auditPath, err)
	}
	stats.Facts = len(facts)

	out := bufio.NewWriter(w)
	fmt.Fprintf(out, "# codeNERD audit facts\n")
	fmt.Fprintf(out, "# source: %s\n", auditPath)
	fmt.Fprintf(out, "# generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(out, "# events=%d facts=%d duplicates_collapsed=%d\n", stats.Events, stats.Facts, stats.Duplicates)
	fmt.Fprintf(out, "# Offline forensic artifact. Do not load into the live kernel.\n\n")

	names := make([]string, 0, len(stats.Predicates))
	for name := range stats.Predicates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "Decl %s(%s).\n", name, declArgs(stats.Predicates[name]))
	}
	if len(names) > 0 {
		fmt.Fprintln(out)
	}
	for _, fact := range facts {
		fmt.Fprintln(out, fact)
	}
	return stats, out.Flush()
}

// declArgs builds "Arg1, Arg2, ..." for a Decl of the given arity.
func declArgs(arity int) string {
	parts := make([]string, 0, arity)
	for i := 1; i <= arity; i++ {
		parts = append(parts, fmt.Sprintf("Arg%d", i))
	}
	return strings.Join(parts, ", ")
}

// parseFactShape extracts the predicate name and arity from a fact string like
// `perf_metric(1234, "kernel", "eval", 12).`. Commas inside quoted strings do
// not count as argument separators — several audit facts embed messages that
// contain commas, and a naive split declared the wrong arity for exactly the
// predicates carrying the most information.
func parseFactShape(fact string) (string, int, bool) {
	open := strings.IndexByte(fact, '(')
	if open <= 0 || !strings.HasSuffix(fact, ").") {
		return "", 0, false
	}
	name := strings.TrimSpace(fact[:open])
	if name == "" {
		return "", 0, false
	}
	body := fact[open+1 : len(fact)-2]
	if strings.TrimSpace(body) == "" {
		return name, 0, true
	}

	arity := 1
	inQuote := false
	escaped := false
	depth := 0
	for _, r := range body {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuote = !inQuote
		case inQuote:
			// commas inside a string are data
		case r == '(' || r == '[':
			depth++
		case r == ')' || r == ']':
			depth--
		case r == ',' && depth == 0:
			arity++
		}
	}
	if inQuote {
		return "", 0, false // malformed fact; skip rather than emit garbage
	}
	return name, arity, true
}
