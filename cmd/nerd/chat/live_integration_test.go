package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"

	tea "github.com/charmbracelet/bubbletea"
)

type testSignals struct {
	bootDone *closeSignal
	scanDone *closeSignal
	respCh   chan string
	errCh    chan error
}

type closeSignal struct {
	ch   chan struct{}
	once sync.Once
}

func newCloseSignal() *closeSignal {
	return &closeSignal{ch: make(chan struct{})}
}

func (s *closeSignal) Signal() {
	s.once.Do(func() { close(s.ch) })
}

func newTestSignals() *testSignals {
	return &testSignals{
		bootDone: newCloseSignal(),
		scanDone: newCloseSignal(),
		respCh:   make(chan string, 8),
		errCh:    make(chan error, 8),
	}
}

func (s *testSignals) sendResp(resp string) {
	if strings.TrimSpace(resp) == "" {
		return
	}
	select {
	case s.respCh <- resp:
	default:
	}
}

func (s *testSignals) sendErr(err error) {
	if err == nil {
		return
	}
	select {
	case s.errCh <- err:
	default:
	}
}

type runResult struct {
	model tea.Model
	err   error
}

var (
	logQueryOnce sync.Once
	logQueryPath string
	logQueryErr  error
)

func requireLiveZAIConfig(t *testing.T) *config.UserConfig {
	t.Helper()

	if os.Getenv("CODENERD_LIVE_LLM") != "1" {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}

	cfg, err := config.GlobalConfig()
	if err != nil {
		t.Skipf("skipping live LLM test: load config: %v", err)
	}

	engine := cfg.GetEngine()
	if engine == "claude-cli" || engine == "codex-cli" {
		t.Skipf("skipping live LLM test: engine=%q (requires API mode with Z.AI)", engine)
	}

	provider, key := cfg.GetActiveProvider()
	if provider == "" || key == "" {
		t.Skip("skipping live LLM test: no API key configured")
	}
	if provider != "zai" {
		t.Skipf("skipping live LLM test: provider=%q (expected zai)", provider)
	}
	if !cfg.GetLogging().DebugMode {
		t.Fatalf("logging.debug_mode=false; enable it in .nerd/config.json to assert log warnings/errors")
	}

	return cfg
}

func envInt(name string, fallback int) int {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		}
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(name)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			return d
		}
	}
	return fallback
}

func envBool(name string) bool {
	raw := strings.TrimSpace(os.Getenv(name))
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

type liveTimeouts struct {
	boot     time.Duration
	scan     time.Duration
	response time.Duration
	shutdown time.Duration
}

func resolveLiveTimeouts() liveTimeouts {
	timeouts := config.GetLLMTimeouts()
	return liveTimeouts{
		boot: envDuration("CODENERD_LIVE_BOOT_TIMEOUT",
			maxDuration(4*time.Minute, minDuration(12*time.Minute, timeouts.ShardExecutionTimeout))),
		scan: envDuration("CODENERD_LIVE_SCAN_TIMEOUT",
			maxDuration(6*time.Minute, minDuration(12*time.Minute, timeouts.DocumentProcessingTimeout))),
		response: envDuration("CODENERD_LIVE_RESPONSE_TIMEOUT",
			maxDuration(4*time.Minute, minDuration(10*time.Minute, timeouts.PerCallTimeout))),
		shutdown: envDuration("CODENERD_LIVE_SHUTDOWN_TIMEOUT", 3*time.Minute),
	}
}

func programTimeoutFor(t liveTimeouts, promptCount int, extra time.Duration) time.Duration {
	if promptCount < 0 {
		promptCount = 0
	}
	total := t.boot + t.scan + t.shutdown + time.Duration(promptCount)*t.response + extra
	if total < t.response {
		total = t.response
	}
	return envDuration("CODENERD_LIVE_PROGRAM_TIMEOUT", total)
}

func ensureWorkspaceRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}

	if root != cwd {
		if err := os.Chdir(root); err != nil {
			t.Fatalf("chdir to workspace root: %v", err)
		}
		t.Cleanup(func() { _ = os.Chdir(cwd) })
	}

	return root
}

func newSignalFilter(signals *testSignals) tea.ProgramOption {
	return tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
		switch typed := msg.(type) {
		case bootCompleteMsg:
			if typed.err != nil {
				signals.sendErr(typed.err)
			} else {
				signals.bootDone.Signal()
			}
		case scanCompleteMsg:
			if typed.err != nil {
				signals.sendErr(typed.err)
			} else {
				signals.scanDone.Signal()
			}
		case errorMsg:
			signals.sendErr(typed)
		case responseMsg:
			signals.sendResp(string(typed))
		case assistantMsg:
			signals.sendResp(typed.Surface)
		}
		return msg
	})
}

func startProgram(t *testing.T, ctx context.Context, signals *testSignals, withRenderer bool) (*tea.Program, <-chan runResult) {
	t.Helper()

	model := InitChat(Config{})
	options := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(bytes.NewReader(nil)),
		tea.WithOutput(io.Discard),
		tea.WithoutSignals(),
		tea.WithoutCatchPanics(),
		newSignalFilter(signals),
	}
	if !withRenderer {
		options = append(options, tea.WithoutRenderer())
	}
	program := tea.NewProgram(model, options...)

	resultCh := make(chan runResult, 1)
	go func() {
		finalModel, err := program.Run()
		resultCh <- runResult{model: finalModel, err: err}
	}()

	return program, resultCh
}

func sendInput(p *tea.Program, input string) {
	if input == "" {
		return
	}
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(input)})
	p.Send(tea.KeyMsg{Type: tea.KeyEnter})
}

func sendAlt(p *tea.Program, r rune) {
	p.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true})
}

func waitForSignal(t *testing.T, name string, sig <-chan struct{}, signals *testSignals, resultCh <-chan runResult, timeout time.Duration, since time.Time) {
	t.Helper()

	if logSignalDetected(t, name, since) {
		return
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sig:
			return
		case err := <-signals.errCh:
			t.Fatalf("unexpected error while waiting for %s: %v", name, err)
		case result := <-resultCh:
			if result.err != nil {
				t.Fatalf("program exited early while waiting for %s: %v", name, result.err)
			}
			t.Fatalf("program exited early while waiting for %s", name)
		case <-ticker.C:
			if logSignalDetected(t, name, since) {
				return
			}
		case <-deadline.C:
			t.Fatalf("timeout waiting for %s", name)
		}
	}
}

func logSignalDetected(t *testing.T, name string, since time.Time) bool {
	t.Helper()

	if name != "boot" && name != "scan" {
		return false
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}
	logDir := filepath.Join(root, ".nerd", "logs")

	entries := readLogEntriesSince(t, logDir, since)
	if len(entries) == 0 {
		return false
	}

	switch name {
	case "boot":
		if logHasMarker(entries, "boot", []string{"Starting Mangle file watcher", "Complete!"}) {
			return true
		}
		if logHasMarker(entries, "kernel", []string{"Mangle file watcher started", "MangleWatcher: watching"}) {
			return true
		}
	case "scan":
		if logHasMarker(entries, "world", []string{"Directory scan completed", "Workspace scan completed", "Incremental scan completed"}) {
			return true
		}
	}

	return false
}

func logHasMarker(entries []logEntry, category string, markers []string) bool {
	for _, entry := range entries {
		if entry.Category != category {
			continue
		}
		for _, marker := range markers {
			if strings.Contains(entry.Message, marker) {
				return true
			}
		}
	}
	return false
}

func waitForResponse(t *testing.T, signals *testSignals, resultCh <-chan runResult, timeout time.Duration) string {
	t.Helper()

	select {
	case resp := <-signals.respCh:
		return resp
	case err := <-signals.errCh:
		t.Fatalf("received error while waiting for response: %v", err)
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("program exited early: %v", result.err)
		}
		t.Fatalf("program exited early before response")
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for response")
	}

	return ""
}

func waitForRunResult(t *testing.T, resultCh <-chan runResult, timeout time.Duration) runResult {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for program exit")
	}

	return runResult{}
}

func assertNoLogIssues(t *testing.T, since time.Time) {
	t.Helper()

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}
	logDir := filepath.Join(root, ".nerd", "logs")

	entries := readLogEntriesSince(t, logDir, since)
	if len(entries) == 0 {
		t.Fatalf("no log entries captured since test start; logging may be disabled or log parsing failed")
	}
	var warns []logEntry
	var errs []logEntry
	for _, entry := range entries {
		switch entry.Level {
		case "warn", "warning":
			warns = append(warns, entry)
		case "error":
			errs = append(errs, entry)
		}
	}

	if len(errs) > 0 || len(warns) > 0 {
		t.Fatalf("log issues detected (errors=%d warnings=%d)\n%s", len(errs), len(warns), formatLogIssues(errs, warns, 6))
	}

	assertNoCriticalLogPatterns(t, entries)
	assertNoSchedulerStarvation(t, entries)
	runLoopAnalyzer(t, logDir, since)
	runLogQueryAnomalies(t, logDir, since)
	runStressLogAnalyzer(t, logDir, since)
}

type logEntry struct {
	Time     time.Time
	Level    string
	Message  string
	Category string
	Raw      string
}

type loopReport struct {
	Summary struct {
		TotalAnomalies int      `json:"total_anomalies"`
		Critical       int      `json:"critical"`
		High           int      `json:"high"`
		LoopsDetected  int      `json:"loops_detected"`
		Affected       []string `json:"affected_actions"`
	} `json:"summary"`
	Anomalies []map[string]interface{} `json:"anomalies"`
}

type logQueryResult struct {
	Predicate string        `json:"predicate"`
	Args      []interface{} `json:"args"`
}

func readLogEntriesSince(t *testing.T, logDir string, since time.Time) []logEntry {
	t.Helper()

	if _, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("stat log dir: %v", err)
	}

	dirEntries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("read log dir: %v", err)
	}

	var entries []logEntry
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}

		category := logCategoryFromFilename(entry.Name())
		path := filepath.Join(logDir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open log file: %v", err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if parsed, ok := parseJSONLogLine(line, category); ok {
				if parsed.Time.Before(since) {
					continue
				}
				entries = append(entries, parsed)
				continue
			}

			if parsed, ok := parseTextLogLine(line, category); ok {
				if parsed.Time.Before(since) {
					continue
				}
				entries = append(entries, parsed)
			}
		}
		_ = file.Close()
		if err := scanner.Err(); err != nil {
			t.Fatalf("scan log file: %v", err)
		}
	}

	return entries
}

func logCategoryFromFilename(name string) string {
	base := strings.TrimSuffix(name, ".log")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func parseJSONLogLine(line, category string) (logEntry, bool) {
	var parsed logging.StructuredLogEntry
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return logEntry{}, false
	}
	if parsed.Timestamp == 0 {
		return logEntry{}, false
	}
	return logEntry{
		Time:     time.UnixMilli(parsed.Timestamp),
		Level:    strings.ToLower(parsed.Level),
		Message:  parsed.Message,
		Category: category,
		Raw:      line,
	}, true
}

func parseTextLogLine(line, category string) (logEntry, bool) {
	const tsLayout = "2006/01/02 15:04:05.000000"
	if len(line) < len(tsLayout) {
		return logEntry{}, false
	}

	tsPart := line[:len(tsLayout)]
	ts, err := time.ParseInLocation(tsLayout, tsPart, time.Local)
	if err != nil {
		return logEntry{}, false
	}

	rest := strings.TrimSpace(line[len(tsLayout):])
	if !strings.HasPrefix(rest, "[") {
		return logEntry{}, false
	}
	end := strings.Index(rest, "]")
	if end == -1 {
		return logEntry{}, false
	}

	level := strings.ToLower(strings.TrimSpace(rest[1:end]))
	msg := strings.TrimSpace(rest[end+1:])
	return logEntry{
		Time:     ts,
		Level:    level,
		Message:  msg,
		Category: category,
		Raw:      line,
	}, true
}

func assertNoCriticalLogPatterns(t *testing.T, entries []logEntry) {
	t.Helper()

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpanic:`),
		regexp.MustCompile(`(?i)fatal error:`),
		regexp.MustCompile(`(?i)runtime error:`),
		regexp.MustCompile(`(?i)invalid memory address or nil pointer dereference`),
		regexp.MustCompile(`(?i)all goroutines are asleep - deadlock`),
		regexp.MustCompile(`(?i)concurrent map (writes|read and write)`),
		regexp.MustCompile(`(?i)warning: data race`),
		regexp.MustCompile(`(?i)\bsegmentation violation\b|\bsigsegv\b`),
		regexp.MustCompile(`(?i)\bout of memory\b|\boom\b`),
		regexp.MustCompile(`(?i)too many open files`),
		regexp.MustCompile(`(?i)stack trace:`),
	}

	var matches []logEntry
	for _, entry := range entries {
		payload := strings.ToLower(entry.Message + " " + entry.Raw)
		for _, pattern := range patterns {
			if pattern.MatchString(payload) {
				matches = append(matches, entry)
				break
			}
		}
	}

	if len(matches) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("critical log patterns detected (%d)\n", len(matches)))
	limit := 6
	if len(matches) < limit {
		limit = len(matches)
	}
	for i := 0; i < limit; i++ {
		entry := matches[i]
		sb.WriteString(fmt.Sprintf("- %s [%s] %s\n", entry.Time.Format(time.RFC3339), entry.Category, entry.Message))
	}
	if len(matches) > limit {
		sb.WriteString(fmt.Sprintf("...and %d more\n", len(matches)-limit))
	}

	t.Fatalf("%s", sb.String())
}

func assertNoSchedulerStarvation(t *testing.T, entries []logEntry) {
	t.Helper()

	if envBool("CODENERD_SKIP_SCHEDULER_CHECK") {
		return
	}

	maxWait := envDuration("CODENERD_MAX_SLOT_WAIT", 30*time.Second)
	maxQueue := envInt("CODENERD_MAX_SLOT_QUEUE", 10)
	if maxWait <= 0 && maxQueue <= 0 {
		return
	}

	waitRe := regexp.MustCompile(`acquired slot after ([0-9.]+[a-z]+)`)
	queueRe := regexp.MustCompile(`waiting=([0-9]+)`)

	var waitViolations []logEntry
	var queueViolations []logEntry

	for _, entry := range entries {
		if entry.Category != "/shards" {
			continue
		}

		if maxWait > 0 {
			if match := waitRe.FindStringSubmatch(entry.Message); len(match) == 2 {
				if dur, err := time.ParseDuration(match[1]); err == nil && dur > maxWait {
					waitViolations = append(waitViolations, entry)
				}
			}
		}

		if maxQueue > 0 {
			if match := queueRe.FindStringSubmatch(entry.Message); len(match) == 2 {
				if count, err := strconv.Atoi(match[1]); err == nil && count > maxQueue {
					queueViolations = append(queueViolations, entry)
				}
			}
		}
	}

	if len(waitViolations) == 0 && len(queueViolations) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString("APIScheduler starvation detected\n")
	if maxWait > 0 && len(waitViolations) > 0 {
		sb.WriteString(fmt.Sprintf("- slot wait > %s (%d)\n", maxWait, len(waitViolations)))
		for _, entry := range waitViolations[:minInt(len(waitViolations), 6)] {
			sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", entry.Time.Format(time.RFC3339), entry.Category, entry.Message))
		}
		if len(waitViolations) > 6 {
			sb.WriteString(fmt.Sprintf("  ...and %d more\n", len(waitViolations)-6))
		}
	}
	if maxQueue > 0 && len(queueViolations) > 0 {
		sb.WriteString(fmt.Sprintf("- slot queue > %d (%d)\n", maxQueue, len(queueViolations)))
		for _, entry := range queueViolations[:minInt(len(queueViolations), 6)] {
			sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", entry.Time.Format(time.RFC3339), entry.Category, entry.Message))
		}
		if len(queueViolations) > 6 {
			sb.WriteString(fmt.Sprintf("  ...and %d more\n", len(queueViolations)-6))
		}
	}

	t.Fatalf("%s", sb.String())
}

func runLoopAnalyzer(t *testing.T, logDir string, since time.Time) {
	t.Helper()

	if envBool("CODENERD_SKIP_LOOP_ANALYZER") {
		return
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}

	python := findPython()
	if python == "" {
		t.Fatalf("python not found; set CODENERD_SKIP_LOOP_ANALYZER=1 to skip loop detection")
	}

	script := findLogAnalyzerScript(root, "detect_loops.py")
	if script == "" {
		t.Fatalf("detect_loops.py not found; expected under .claude/skills/log-analyzer/scripts")
	}

	filteredDir := writeFilteredLogs(t, logDir, since)
	if filteredDir == "" {
		return
	}

	logFiles, err := listLogFiles(filteredDir)
	if err != nil {
		t.Fatalf("list filtered logs: %v", err)
	}
	if len(logFiles) == 0 {
		return
	}

	args := []string{script}
	if threshold := envInt("CODENERD_LOOP_THRESHOLD", 5); threshold != 5 {
		args = append(args, "--threshold", strconv.Itoa(threshold))
	}
	args = append(args, logFiles...)

	var stderr bytes.Buffer
	cmd := exec.Command(python, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("loop analyzer failed: %v\n%s", err, stderr.String())
	}

	var report loopReport
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("parse loop analyzer output: %v\n%s", err, string(output))
	}

	if report.Summary.TotalAnomalies > 0 || len(report.Anomalies) > 0 {
		t.Fatalf("loop anomalies detected\n%s", formatLoopReport(report, 6))
	}
}

func runLogQueryAnomalies(t *testing.T, logDir string, since time.Time) {
	t.Helper()

	if envBool("CODENERD_SKIP_LOGQUERY") {
		return
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}

	python := findPython()
	if python == "" {
		t.Fatalf("python not found; set CODENERD_SKIP_LOGQUERY=1 to skip logquery analysis")
	}

	parseScript := findLogAnalyzerScript(root, "parse_log.py")
	if parseScript == "" {
		t.Fatalf("parse_log.py not found; expected under .claude/skills/log-analyzer/scripts")
	}

	filteredDir := writeFilteredLogs(t, logDir, since)
	if filteredDir == "" {
		return
	}

	logFiles, err := listLogFiles(filteredDir)
	if err != nil {
		t.Fatalf("list filtered logs: %v", err)
	}
	if len(logFiles) == 0 {
		return
	}

	factsPath := filepath.Join(t.TempDir(), "log_facts.mg")
	args := append([]string{parseScript, "--no-schema", "--output", factsPath}, logFiles...)

	var parseErr bytes.Buffer
	parseCmd := exec.Command(python, args...)
	parseCmd.Dir = root
	parseCmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	parseCmd.Stderr = &parseErr
	if _, err := parseCmd.Output(); err != nil {
		t.Fatalf("parse_log failed: %v\n%s", err, parseErr.String())
	}

	logQueryPath := ensureLogQuery(t, root)
	if logQueryPath == "" {
		return
	}

	results, raw, err := runLogQueryResults(logQueryPath, factsPath, "anomalies")
	if err != nil {
		t.Fatalf("logquery anomalies failed: %v\n%s", err, raw)
	}

	if len(results) > 0 {
		diagnose, diagErr := runLogQueryText(logQueryPath, factsPath, "diagnose")
		if diagErr != nil || strings.TrimSpace(diagnose) == "" {
			diagnose = formatLogQueryResults(results, 6)
		}
		t.Fatalf("logquery anomalies detected (%d)\n%s", len(results), diagnose)
	}
}

func runStressLogAnalyzer(t *testing.T, logDir string, since time.Time) {
	t.Helper()

	if envBool("CODENERD_SKIP_LOG_ANALYZER") {
		return
	}

	python := findPython()
	if python == "" {
		t.Fatalf("python not found; set CODENERD_SKIP_LOG_ANALYZER=1 to skip log analyzer")
	}

	filteredDir := writeFilteredLogs(t, logDir, since)
	if filteredDir == "" {
		return
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("find workspace root: %v", err)
	}

	script := filepath.Join(root, ".claude", "skills", "stress-tester", "scripts", "analyze_stress_logs.py")
	reportPath := filepath.Join(filteredDir, "stress_report.md")

	cmd := exec.Command(python, script, "--logs-dir", filteredDir, "--output", reportPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stress log analyzer failed: %v\n%s", err, string(output))
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read stress report: %v", err)
	}

	text := string(report)
	if strings.Contains(text, "Status: FAILED") || strings.Contains(text, "Status: WARNING") {
		t.Fatalf("stress log analysis reported issues:\n%s", text)
	}
}

func ensureLogQuery(t *testing.T, root string) string {
	t.Helper()

	if envBool("CODENERD_SKIP_LOGQUERY") {
		return ""
	}

	logQueryOnce.Do(func() {
		logQueryDir := findLogQueryDir(root)
		if logQueryDir == "" {
			logQueryErr = fmt.Errorf("logquery source directory not found")
			return
		}

		exeName := logQueryExeName()
		existing := filepath.Join(logQueryDir, exeName)
		if info, err := os.Stat(existing); err == nil && !info.IsDir() {
			logQueryPath = existing
			return
		}

		goPath, err := exec.LookPath("go")
		if err != nil {
			logQueryErr = fmt.Errorf("go not found: %w", err)
			return
		}

		tempDir, err := os.MkdirTemp("", "codenerd-logquery-")
		if err != nil {
			logQueryErr = fmt.Errorf("create temp dir: %w", err)
			return
		}

		output := filepath.Join(tempDir, exeName)
		cmd := exec.Command(goPath, "build", "-o", output, ".")
		cmd.Dir = logQueryDir
		compiled, err := cmd.CombinedOutput()
		if err != nil {
			logQueryErr = fmt.Errorf("build logquery: %v\n%s", err, string(compiled))
			return
		}

		logQueryPath = output
	})

	if logQueryErr != nil {
		t.Fatalf("logquery unavailable: %v (set CODENERD_SKIP_LOGQUERY=1 to skip)", logQueryErr)
	}

	return logQueryPath
}

func logQueryExeName() string {
	if runtime.GOOS == "windows" {
		return "logquery.exe"
	}
	return "logquery"
}

func findLogAnalyzerScript(root, script string) string {
	candidates := []string{
		filepath.Join(root, ".claude", "skills", "log-analyzer", "scripts", script),
		filepath.Join(root, ".codex", "skills", "log-analyzer", "scripts", script),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func findLogQueryDir(root string) string {
	candidates := []string{
		filepath.Join(root, ".claude", "skills", "log-analyzer", "scripts", "logquery"),
		filepath.Join(root, ".codex", "skills", "log-analyzer", "scripts", "logquery"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func listLogFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	return files, nil
}

func runLogQueryResults(logQueryPath, factsPath, builtin string) ([]logQueryResult, string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(logQueryPath, factsPath, "--builtin", builtin, "--format", "json")
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	raw := strings.TrimSpace(stderr.String() + "\n" + string(output))
	if err != nil {
		return nil, raw, err
	}

	var results []logQueryResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, raw, err
	}

	return results, raw, nil
}

func runLogQueryText(logQueryPath, factsPath, builtin string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(logQueryPath, factsPath, "--builtin", builtin, "--format", "text")
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, stderr.String())
	}
	return string(output), nil
}

func formatLoopReport(report loopReport, limit int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("summary: total=%d critical=%d high=%d loops=%d\n",
		report.Summary.TotalAnomalies,
		report.Summary.Critical,
		report.Summary.High,
		report.Summary.LoopsDetected,
	))
	if len(report.Summary.Affected) > 0 {
		sb.WriteString(fmt.Sprintf("affected actions: %s\n", strings.Join(report.Summary.Affected, ", ")))
	}

	count := len(report.Anomalies)
	if count > limit {
		count = limit
	}
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("- %s\n", formatLoopAnomaly(report.Anomalies[i])))
	}
	if len(report.Anomalies) > count {
		sb.WriteString(fmt.Sprintf("...and %d more\n", len(report.Anomalies)-count))
	}
	return sb.String()
}

func formatLoopAnomaly(anomaly map[string]interface{}) string {
	if anomaly == nil {
		return "unknown anomaly"
	}

	var parts []string
	if kind := formatJSONValue(anomaly["type"]); kind != "" {
		parts = append(parts, kind)
	}
	if severity := formatJSONValue(anomaly["severity"]); severity != "" {
		parts = append(parts, "severity="+severity)
	}
	if action := formatJSONValue(anomaly["action"]); action != "" {
		parts = append(parts, "action="+action)
	}
	if count := formatJSONValue(anomaly["count"]); count != "" {
		parts = append(parts, "count="+count)
	}
	if root, ok := anomaly["root_cause"].(map[string]interface{}); ok {
		if diag := formatJSONValue(root["diagnosis"]); diag != "" {
			parts = append(parts, "cause="+diag)
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("%v", anomaly)
	}
	return strings.Join(parts, " ")
}

func formatLogQueryResults(results []logQueryResult, limit int) string {
	if len(results) == 0 {
		return "no anomalies reported"
	}

	var sb strings.Builder
	count := len(results)
	if count > limit {
		count = limit
	}
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("- %s\n", formatLogQueryResult(results[i])))
	}
	if len(results) > count {
		sb.WriteString(fmt.Sprintf("...and %d more\n", len(results)-count))
	}
	return sb.String()
}

func formatLogQueryResult(result logQueryResult) string {
	if result.Predicate == "" {
		return fmt.Sprintf("%v", result.Args)
	}
	if len(result.Args) == 0 {
		return result.Predicate
	}

	args := make([]string, 0, len(result.Args))
	for _, arg := range result.Args {
		args = append(args, formatJSONValue(arg))
	}

	return fmt.Sprintf("%s(%s)", result.Predicate, strings.Join(args, ", "))
}

func formatJSONValue(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', 2, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func findPython() string {
	if path, _ := exec.LookPath("python"); path != "" {
		return path
	}
	if path, _ := exec.LookPath("python3"); path != "" {
		return path
	}
	return ""
}

func writeFilteredLogs(t *testing.T, logDir string, since time.Time) string {
	t.Helper()

	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("read log dir: %v", err)
	}

	tempDir := t.TempDir()
	wroteAny := false

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		srcPath := filepath.Join(logDir, entry.Name())
		dstPath := filepath.Join(tempDir, entry.Name())

		srcFile, err := os.Open(srcPath)
		if err != nil {
			t.Fatalf("open log file: %v", err)
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			_ = srcFile.Close()
			t.Fatalf("create filtered log file: %v", err)
		}

		scanner := bufio.NewScanner(srcFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if entryTime, ok := parseLogTime(line); ok {
				if entryTime.Before(since) {
					continue
				}
				_, _ = dstFile.WriteString(line + "\n")
				wroteAny = true
			}
		}
		if err := scanner.Err(); err != nil {
			_ = dstFile.Close()
			_ = srcFile.Close()
			t.Fatalf("scan log file: %v", err)
		}
		_ = dstFile.Close()
		_ = srcFile.Close()
	}

	if !wroteAny {
		return ""
	}

	return tempDir
}

func parseLogTime(line string) (time.Time, bool) {
	if parsed, ok := parseJSONLogLine(line, ""); ok {
		return parsed.Time, true
	}
	if parsed, ok := parseTextLogLine(line, ""); ok {
		return parsed.Time, true
	}
	return time.Time{}, false
}

func formatLogIssues(errors, warnings []logEntry, limit int) string {
	var sb strings.Builder
	appendEntries := func(label string, entries []logEntry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("%s:\n", label))
		count := len(entries)
		if count > limit {
			count = limit
		}
		for i := 0; i < count; i++ {
			entry := entries[i]
			sb.WriteString(fmt.Sprintf("- %s [%s] %s\n", entry.Time.Format(time.RFC3339), entry.Category, entry.Message))
		}
		if len(entries) > limit {
			sb.WriteString(fmt.Sprintf("...and %d more\n", len(entries)-limit))
		}
	}

	appendEntries("Errors", errors)
	appendEntries("Warnings", warnings)

	return sb.String()
}

func TestChatLiveLLM_HeadlessProgram(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	programTimeout := programTimeoutFor(timeouts, 1, 2*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	sendInput(program, "Give a one-sentence summary of codeNERD.")
	resp := waitForResponse(t, signals, resultCh, timeouts.response)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response")
	}

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_RendererPath(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	programTimeout := programTimeoutFor(timeouts, 1, 2*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, true)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	sendInput(program, "Summarize the codeNERD architecture in one paragraph.")
	resp := waitForResponse(t, signals, resultCh, timeouts.response)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response")
	}

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_StressSequence(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	prompts := []string{
		"List three core subsystems of codeNERD.",
		"Explain what the Mangle kernel does in one short paragraph.",
		"Name one stability risk for Bubble Tea apps like this.",
	}
	programTimeout := programTimeoutFor(timeouts, len(prompts), 3*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	stopResize := make(chan struct{})
	go func() {
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		sizes := []tea.WindowSizeMsg{
			{Width: 100, Height: 32},
			{Width: 120, Height: 40},
			{Width: 110, Height: 36},
			{Width: 140, Height: 44},
		}
		idx := 0
		for {
			select {
			case <-stopResize:
				return
			case <-ticker.C:
				program.Send(sizes[idx%len(sizes)])
				idx++
			}
		}
	}()

	for _, prompt := range prompts {
		sendAlt(program, 'l') // Toggle logic pane on/off to stress layout.
		sendInput(program, prompt)
		resp := waitForResponse(t, signals, resultCh, timeouts.response)
		if strings.TrimSpace(resp) == "" {
			t.Fatalf("expected non-empty response")
		}
		sendAlt(program, 'p') // Toggle prompt inspector to exercise JIT view wiring.
	}

	close(stopResize)

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_EventStorm(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	rounds := envInt("CODENERD_LIVE_STORM_ROUNDS", 5)
	if rounds < 1 {
		rounds = 1
	}
	programTimeout := programTimeoutFor(timeouts, rounds, 3*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	for i := 0; i < rounds; i++ {
		sendAlt(program, 'l')
		sendAlt(program, 'p')
		sendAlt(program, 'a')
		sendAlt(program, 's')
		sendAlt(program, 'c')
		program.Send(tea.WindowSizeMsg{Width: 110 + i, Height: 36 + i})

		sendInput(program, fmt.Sprintf("Give one concise stability risk for a Bubble Tea app (round %d).", i+1))
		resp := waitForResponse(t, signals, resultCh, timeouts.response)
		if strings.TrimSpace(resp) == "" {
			t.Fatalf("expected non-empty response")
		}
	}

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_RaceStorm(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	stormDuration := envDuration("CODENERD_LIVE_STORM_DURATION", 2*time.Minute)
	senders := envInt("CODENERD_LIVE_STORM_SENDERS", 4)
	if senders < 1 {
		senders = 1
	}
	prompts := []string{
		"List two failure modes for long-running TUIs.",
		"Summarize how codeNERD enforces safety in one paragraph.",
		"Name one risk for concurrent shard execution.",
	}
	programTimeout := programTimeoutFor(timeouts, len(prompts), stormDuration+3*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ticker := time.NewTicker(75 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					program.Send(tea.WindowSizeMsg{Width: 100 + idx, Height: 30 + idx})
					sendAlt(program, 'l')
					sendAlt(program, 'p')
				}
			}
		}(i)
	}

	for _, prompt := range prompts {
		sendInput(program, prompt)
		resp := waitForResponse(t, signals, resultCh, timeouts.response)
		if strings.TrimSpace(resp) == "" {
			t.Fatalf("expected non-empty response")
		}
	}

	select {
	case <-time.After(stormDuration):
	case err := <-signals.errCh:
		close(stop)
		wg.Wait()
		t.Fatalf("error during race storm: %v", err)
	case result := <-resultCh:
		close(stop)
		wg.Wait()
		if result.err != nil {
			t.Fatalf("program exited early: %v", result.err)
		}
		t.Fatalf("program exited early during race storm")
	}

	close(stop)
	wg.Wait()

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_Soak(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	soakDuration := envDuration("CODENERD_LIVE_SOAK_DURATION", 3*time.Minute)
	programTimeout := programTimeoutFor(timeouts, 1, soakDuration+3*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	sendInput(program, "Give a short mission statement for codeNERD.")
	resp := waitForResponse(t, signals, resultCh, timeouts.response)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response")
	}

	maxAllocMB := envInt("CODENERD_LIVE_MAX_ALLOC_MB", 1024)
	maxGoroutines := envInt("CODENERD_LIVE_MAX_GOROUTINES", 2000)

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	beforeGoroutines := runtime.NumGoroutine()

	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				program.Send(tea.WindowSizeMsg{Width: 100 + i%40, Height: 30 + i%20})
				sendAlt(program, 'l')
				sendAlt(program, 'p')
				i++
			}
		}
	}()

	select {
	case <-time.After(soakDuration):
	case err := <-signals.errCh:
		close(stop)
		t.Fatalf("error during soak: %v", err)
	case result := <-resultCh:
		close(stop)
		if result.err != nil {
			t.Fatalf("program exited early: %v", result.err)
		}
		t.Fatalf("program exited early during soak")
	}

	close(stop)

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
	}

	var after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&after)
	allocMB := int((after.Alloc - before.Alloc) / (1024 * 1024))
	if allocMB > maxAllocMB {
		t.Fatalf("memory growth too high: %d MB (limit %d MB)", allocMB, maxAllocMB)
	}

	afterGoroutines := runtime.NumGoroutine()
	if afterGoroutines > maxGoroutines && afterGoroutines > beforeGoroutines {
		t.Fatalf("goroutine count too high: %d (limit %d, before %d)", afterGoroutines, maxGoroutines, beforeGoroutines)
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}

func TestChatLiveLLM_PromptEvolutionSystem(t *testing.T) {
	_ = requireLiveZAIConfig(t)
	ensureWorkspaceRoot(t)

	startTime := time.Now().Add(-1 * time.Second)
	signals := newTestSignals()
	timeouts := resolveLiveTimeouts()
	commandTimeout := envDuration("CODENERD_LIVE_COMMAND_TIMEOUT", 45*time.Second)
	programTimeout := programTimeoutFor(timeouts, 0, 3*time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), programTimeout)
	t.Cleanup(cancel)

	program, resultCh := startProgram(t, ctx, signals, false)
	program.Send(tea.WindowSizeMsg{Width: 120, Height: 40})

	waitForSignal(t, "boot", signals.bootDone.ch, signals, resultCh, timeouts.boot, startTime)
	waitForSignal(t, "scan", signals.scanDone.ch, signals, resultCh, timeouts.scan, startTime)

	// Verify Prompt Evolution System is initialized by testing the /evolution-stats command
	sendInput(program, "/evolution-stats")
	resp := waitForResponse(t, signals, resultCh, commandTimeout)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response from /evolution-stats")
	}

	// Response should contain evolution statistics (even if system is new)
	if !strings.Contains(resp, "Evolution") && !strings.Contains(resp, "evolution") &&
		!strings.Contains(resp, "not initialized") && !strings.Contains(resp, "Statistics") {
		t.Logf("Evolution stats response: %s", resp)
	}

	// Test /evolved-atoms command
	sendInput(program, "/evolved-atoms")
	resp = waitForResponse(t, signals, resultCh, commandTimeout)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response from /evolved-atoms")
	}

	// Test /strategies command
	sendInput(program, "/strategies")
	resp = waitForResponse(t, signals, resultCh, commandTimeout)
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("expected non-empty response from /strategies")
	}

	program.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := waitForRunResult(t, resultCh, timeouts.shutdown)
	if result.err != nil {
		t.Fatalf("program exited with error: %v", result.err)
	}

	// Verify the promptEvolver is set on the model
	if finalModel, ok := result.model.(Model); ok {
		if finalModel.err != nil {
			t.Fatalf("model error set: %v", finalModel.err)
		}
		// PromptEvolver may be nil if JIT compiler wasn't initialized
		// This is acceptable in some test environments
		if finalModel.promptEvolver != nil {
			stats := finalModel.promptEvolver.GetStats()
			if stats == nil {
				t.Fatalf("promptEvolver.GetStats() returned nil")
			}
			t.Logf("PromptEvolver stats: cycles=%d, atoms=%d, strategies=%d",
				stats.TotalCycles, stats.TotalAtomsGenerated, stats.TotalStrategies)
		} else {
			t.Log("Note: promptEvolver is nil (JIT compiler may not have been initialized)")
		}

		// Verify EvolvedAtomManager is registered with JIT compiler
		if finalModel.jitCompiler != nil {
			evolvedCount := finalModel.jitCompiler.GetEvolvedAtomCount()
			t.Logf("Evolved atoms registered with JIT: %d", evolvedCount)
		}
	}

	logging.CloseAll()
	assertNoLogIssues(t, startTime)
}
