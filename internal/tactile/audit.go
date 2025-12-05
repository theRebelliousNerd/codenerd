package tactile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Fact represents a Mangle fact for kernel injection.
// This mirrors core.Fact but is defined here to avoid import cycles.
type Fact struct {
	Predicate string        `json:"predicate"`
	Args      []interface{} `json:"args"`
}

// String returns the Datalog string representation of the fact.
func (f Fact) String() string {
	var args []string
	for _, arg := range f.Args {
		switch v := arg.(type) {
		case string:
			if strings.HasPrefix(v, "/") {
				args = append(args, v)
			} else {
				args = append(args, fmt.Sprintf("%q", v))
			}
		case int:
			args = append(args, fmt.Sprintf("%d", v))
		case int64:
			args = append(args, fmt.Sprintf("%d", v))
		case float64:
			args = append(args, fmt.Sprintf("%f", v))
		case bool:
			if v {
				args = append(args, "/true")
			} else {
				args = append(args, "/false")
			}
		default:
			args = append(args, fmt.Sprintf("%v", v))
		}
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
}

// ToFacts converts an AuditEvent to Mangle facts for kernel injection.
// These facts allow the kernel to reason about execution history.
func (e AuditEvent) ToFacts() []Fact {
	facts := make([]Fact, 0)

	timestamp := e.Timestamp.Unix()
	cmdString := e.Command.CommandString()

	switch e.Type {
	case AuditEventStart:
		// execution_started(SessionID, RequestID, Binary, Timestamp)
		facts = append(facts, Fact{
			Predicate: "execution_started",
			Args: []interface{}{
				e.SessionID,
				e.Command.RequestID,
				e.Command.Binary,
				timestamp,
			},
		})

		// execution_command(RequestID, CommandString)
		facts = append(facts, Fact{
			Predicate: "execution_command",
			Args: []interface{}{
				e.Command.RequestID,
				cmdString,
			},
		})

		// execution_working_dir(RequestID, WorkingDir)
		if e.Command.WorkingDirectory != "" {
			facts = append(facts, Fact{
				Predicate: "execution_working_dir",
				Args: []interface{}{
					e.Command.RequestID,
					e.Command.WorkingDirectory,
				},
			})
		}

	case AuditEventComplete:
		if e.Result == nil {
			break
		}

		// execution_completed(RequestID, ExitCode, DurationMs, Timestamp)
		facts = append(facts, Fact{
			Predicate: "execution_completed",
			Args: []interface{}{
				e.Command.RequestID,
				int64(e.Result.ExitCode),
				e.Result.Duration.Milliseconds(),
				timestamp,
			},
		})

		// execution_output(RequestID, StdoutLen, StderrLen)
		facts = append(facts, Fact{
			Predicate: "execution_output",
			Args: []interface{}{
				e.Command.RequestID,
				int64(len(e.Result.Stdout)),
				int64(len(e.Result.Stderr)),
			},
		})

		// execution_success(RequestID) or execution_failure(RequestID, Error)
		if e.Result.Success && e.Result.ExitCode == 0 {
			facts = append(facts, Fact{
				Predicate: "execution_success",
				Args:      []interface{}{e.Command.RequestID},
			})
		} else if e.Result.Success && e.Result.ExitCode != 0 {
			facts = append(facts, Fact{
				Predicate: "execution_nonzero",
				Args:      []interface{}{e.Command.RequestID, int64(e.Result.ExitCode)},
			})
		} else {
			facts = append(facts, Fact{
				Predicate: "execution_failure",
				Args:      []interface{}{e.Command.RequestID, e.Result.Error},
			})
		}

		// Resource usage facts
		if e.Result.ResourceUsage != nil {
			ru := e.Result.ResourceUsage
			// execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes)
			facts = append(facts, Fact{
				Predicate: "execution_resource_usage",
				Args: []interface{}{
					e.Command.RequestID,
					ru.TotalCPUTimeMs(),
					ru.MaxRSSBytes,
				},
			})

			// execution_io(RequestID, ReadBytes, WriteBytes)
			if ru.DiskReadBytes > 0 || ru.DiskWriteBytes > 0 {
				facts = append(facts, Fact{
					Predicate: "execution_io",
					Args: []interface{}{
						e.Command.RequestID,
						ru.DiskReadBytes,
						ru.DiskWriteBytes,
					},
				})
			}
		}

		// Sandbox mode fact
		// execution_sandbox(RequestID, SandboxMode)
		facts = append(facts, Fact{
			Predicate: "execution_sandbox",
			Args: []interface{}{
				e.Command.RequestID,
				"/" + string(e.Result.SandboxUsed),
			},
		})

	case AuditEventKilled:
		if e.Result == nil {
			break
		}

		// execution_killed(RequestID, Reason, DurationMs)
		facts = append(facts, Fact{
			Predicate: "execution_killed",
			Args: []interface{}{
				e.Command.RequestID,
				e.Result.KillReason,
				e.Result.Duration.Milliseconds(),
			},
		})

	case AuditEventError:
		errorMsg := ""
		if e.Result != nil {
			errorMsg = e.Result.Error
		}

		// execution_error(RequestID, ErrorMessage)
		facts = append(facts, Fact{
			Predicate: "execution_error",
			Args: []interface{}{
				e.Command.RequestID,
				errorMsg,
			},
		})

	case AuditEventBlocked:
		// execution_blocked(RequestID, Reason)
		facts = append(facts, Fact{
			Predicate: "execution_blocked",
			Args: []interface{}{
				e.Command.RequestID,
				e.BlockReason,
			},
		})

	case AuditEventSandboxed:
		// execution_sandboxed(RequestID, SandboxMode)
		sandboxMode := "none"
		if e.Command.Sandbox != nil {
			sandboxMode = string(e.Command.Sandbox.Mode)
		}
		facts = append(facts, Fact{
			Predicate: "execution_sandboxed",
			Args: []interface{}{
				e.Command.RequestID,
				"/" + sandboxMode,
			},
		})
	}

	// Add tags as facts
	for key, value := range e.Command.Tags {
		facts = append(facts, Fact{
			Predicate: "execution_tag",
			Args: []interface{}{
				e.Command.RequestID,
				key,
				value,
			},
		})
	}

	return facts
}

// AuditLogger provides structured audit logging for command execution.
type AuditLogger struct {
	mu sync.RWMutex

	// callbacks are functions to call for each event
	callbacks []func(AuditEvent)

	// factCallback is called for each generated fact
	factCallback func(Fact)

	// fileLogger writes events to a file
	fileLogger *AuditFileLogger

	// metrics tracks execution statistics
	metrics *ExecutionMetrics
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger() *AuditLogger {
	return &AuditLogger{
		callbacks: make([]func(AuditEvent), 0),
		metrics:   NewExecutionMetrics(),
	}
}

// AddCallback adds a callback function for audit events.
func (l *AuditLogger) AddCallback(callback func(AuditEvent)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.callbacks = append(l.callbacks, callback)
}

// SetFactCallback sets the callback for generated facts.
func (l *AuditLogger) SetFactCallback(callback func(Fact)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.factCallback = callback
}

// EnableFileLogging enables logging to a file.
func (l *AuditLogger) EnableFileLogging(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	fl, err := NewAuditFileLogger(path)
	if err != nil {
		return err
	}
	l.fileLogger = fl
	return nil
}

// Close closes the audit logger and any file handles.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fileLogger != nil {
		return l.fileLogger.Close()
	}
	return nil
}

// Log logs an audit event.
func (l *AuditLogger) Log(event AuditEvent) {
	l.mu.RLock()
	callbacks := l.callbacks
	factCallback := l.factCallback
	fileLogger := l.fileLogger
	metrics := l.metrics
	l.mu.RUnlock()

	// Update metrics
	if metrics != nil {
		metrics.RecordEvent(event)
	}

	// Call registered callbacks
	for _, cb := range callbacks {
		cb(event)
	}

	// Generate and emit facts
	if factCallback != nil {
		for _, fact := range event.ToFacts() {
			factCallback(fact)
		}
	}

	// Write to file if enabled
	if fileLogger != nil {
		fileLogger.Write(event)
	}
}

// GetMetrics returns the current execution metrics.
func (l *AuditLogger) GetMetrics() ExecutionMetricsSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.metrics == nil {
		return ExecutionMetricsSnapshot{}
	}
	return l.metrics.Snapshot()
}

// AuditFileLogger writes audit events to a file in JSON Lines format.
type AuditFileLogger struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// NewAuditFileLogger creates a new file logger.
func NewAuditFileLogger(path string) (*AuditFileLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file for append
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &AuditFileLogger{
		file: file,
		path: path,
	}, nil
}

// Write writes an event to the log file.
func (l *AuditFileLogger) Write(event AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("log file not open")
	}

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = l.file.Write(append(data, '\n'))
	return err
}

// Close closes the log file.
func (l *AuditFileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// Rotate rotates the log file (renames current and opens new).
func (l *AuditFileLogger) Rotate() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("log file not open")
	}

	// Close current file
	if err := l.file.Close(); err != nil {
		return err
	}

	// Rename to timestamped backup
	backupPath := fmt.Sprintf("%s.%s", l.path, time.Now().Format("20060102-150405"))
	if err := os.Rename(l.path, backupPath); err != nil {
		return err
	}

	// Open new file
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = file
	return nil
}

// ExecutionMetrics tracks aggregate execution statistics.
type ExecutionMetrics struct {
	mu sync.RWMutex

	totalExecutions     int64
	successfulExecutions int64
	failedExecutions    int64
	killedExecutions    int64
	blockedExecutions   int64

	totalDurationMs     int64
	totalCPUTimeMs      int64
	totalMemoryBytes    int64

	executionsByBinary  map[string]int64
	executionsBySession map[string]int64

	lastEventTime time.Time
}

// NewExecutionMetrics creates a new metrics tracker.
func NewExecutionMetrics() *ExecutionMetrics {
	return &ExecutionMetrics{
		executionsByBinary:  make(map[string]int64),
		executionsBySession: make(map[string]int64),
	}
}

// RecordEvent updates metrics based on an audit event.
func (m *ExecutionMetrics) RecordEvent(event AuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastEventTime = event.Timestamp

	switch event.Type {
	case AuditEventStart:
		m.totalExecutions++
		m.executionsByBinary[event.Command.Binary]++
		if event.SessionID != "" {
			m.executionsBySession[event.SessionID]++
		}

	case AuditEventComplete:
		if event.Result != nil {
			if event.Result.Success && event.Result.ExitCode == 0 {
				m.successfulExecutions++
			} else if !event.Result.Success {
				m.failedExecutions++
			}
			m.totalDurationMs += event.Result.Duration.Milliseconds()

			if event.Result.ResourceUsage != nil {
				m.totalCPUTimeMs += event.Result.ResourceUsage.TotalCPUTimeMs()
				m.totalMemoryBytes += event.Result.ResourceUsage.MaxRSSBytes
			}
		}

	case AuditEventKilled:
		m.killedExecutions++
		if event.Result != nil {
			m.totalDurationMs += event.Result.Duration.Milliseconds()
		}

	case AuditEventError:
		m.failedExecutions++

	case AuditEventBlocked:
		m.blockedExecutions++
	}
}

// ExecutionMetricsSnapshot is a point-in-time snapshot of metrics.
type ExecutionMetricsSnapshot struct {
	TotalExecutions      int64            `json:"total_executions"`
	SuccessfulExecutions int64            `json:"successful_executions"`
	FailedExecutions     int64            `json:"failed_executions"`
	KilledExecutions     int64            `json:"killed_executions"`
	BlockedExecutions    int64            `json:"blocked_executions"`
	TotalDurationMs      int64            `json:"total_duration_ms"`
	TotalCPUTimeMs       int64            `json:"total_cpu_time_ms"`
	TotalMemoryBytes     int64            `json:"total_memory_bytes"`
	ExecutionsByBinary   map[string]int64 `json:"executions_by_binary"`
	ExecutionsBySession  map[string]int64 `json:"executions_by_session"`
	LastEventTime        time.Time        `json:"last_event_time"`
	SuccessRate          float64          `json:"success_rate"`
	AvgDurationMs        float64          `json:"avg_duration_ms"`
}

// Snapshot returns a point-in-time copy of the metrics.
func (m *ExecutionMetrics) Snapshot() ExecutionMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy maps
	byBinary := make(map[string]int64)
	for k, v := range m.executionsByBinary {
		byBinary[k] = v
	}
	bySession := make(map[string]int64)
	for k, v := range m.executionsBySession {
		bySession[k] = v
	}

	// Calculate derived metrics
	successRate := float64(0)
	avgDuration := float64(0)
	completed := m.successfulExecutions + m.failedExecutions + m.killedExecutions
	if completed > 0 {
		successRate = float64(m.successfulExecutions) / float64(completed)
		avgDuration = float64(m.totalDurationMs) / float64(completed)
	}

	return ExecutionMetricsSnapshot{
		TotalExecutions:      m.totalExecutions,
		SuccessfulExecutions: m.successfulExecutions,
		FailedExecutions:     m.failedExecutions,
		KilledExecutions:     m.killedExecutions,
		BlockedExecutions:    m.blockedExecutions,
		TotalDurationMs:      m.totalDurationMs,
		TotalCPUTimeMs:       m.totalCPUTimeMs,
		TotalMemoryBytes:     m.totalMemoryBytes,
		ExecutionsByBinary:   byBinary,
		ExecutionsBySession:  bySession,
		LastEventTime:        m.lastEventTime,
		SuccessRate:          successRate,
		AvgDurationMs:        avgDuration,
	}
}

// Reset clears all metrics.
func (m *ExecutionMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalExecutions = 0
	m.successfulExecutions = 0
	m.failedExecutions = 0
	m.killedExecutions = 0
	m.blockedExecutions = 0
	m.totalDurationMs = 0
	m.totalCPUTimeMs = 0
	m.totalMemoryBytes = 0
	m.executionsByBinary = make(map[string]int64)
	m.executionsBySession = make(map[string]int64)
	m.lastEventTime = time.Time{}
}

// AuditedExecutorWrapper wraps any Executor to add audit logging.
type AuditedExecutorWrapper struct {
	executor Executor
	logger   *AuditLogger
}

// NewAuditedExecutor wraps an executor with audit logging.
func NewAuditedExecutor(executor Executor, logger *AuditLogger) *AuditedExecutorWrapper {
	// If the executor already supports audit callbacks, use that
	if audited, ok := executor.(interface{ SetAuditCallback(func(AuditEvent)) }); ok {
		audited.SetAuditCallback(logger.Log)
	}

	return &AuditedExecutorWrapper{
		executor: executor,
		logger:   logger,
	}
}

// Execute runs a command and logs the execution.
func (w *AuditedExecutorWrapper) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	return w.executor.Execute(ctx, cmd)
}

// Capabilities returns the wrapped executor's capabilities.
func (w *AuditedExecutorWrapper) Capabilities() ExecutorCapabilities {
	return w.executor.Capabilities()
}

// Validate validates a command.
func (w *AuditedExecutorWrapper) Validate(cmd Command) error {
	return w.executor.Validate(cmd)
}

// GetLogger returns the audit logger.
func (w *AuditedExecutorWrapper) GetLogger() *AuditLogger {
	return w.logger
}

// OutputAnalyzer extracts structured information from command output.
type OutputAnalyzer struct{}

// NewOutputAnalyzer creates a new output analyzer.
func NewOutputAnalyzer() *OutputAnalyzer {
	return &OutputAnalyzer{}
}

// AnalyzeTestOutput extracts test results from typical test framework output.
func (a *OutputAnalyzer) AnalyzeTestOutput(output string) TestAnalysis {
	analysis := TestAnalysis{
		RawOutput: output,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Go test patterns
		if strings.HasPrefix(line, "--- PASS:") {
			analysis.Passed++
		} else if strings.HasPrefix(line, "--- FAIL:") {
			analysis.Failed++
			// Extract test name
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				analysis.FailedTests = append(analysis.FailedTests, parts[2])
			}
		} else if strings.HasPrefix(line, "--- SKIP:") {
			analysis.Skipped++
		} else if strings.HasPrefix(line, "PASS") {
			analysis.OverallPass = true
		} else if strings.HasPrefix(line, "FAIL") {
			analysis.OverallPass = false
		}

		// Extract timing
		if strings.Contains(line, "coverage:") {
			// Parse coverage percentage
			for _, part := range strings.Fields(line) {
				if strings.HasSuffix(part, "%") {
					fmt.Sscanf(part, "%f%%", &analysis.Coverage)
				}
			}
		}
	}

	analysis.Total = analysis.Passed + analysis.Failed + analysis.Skipped
	return analysis
}

// TestAnalysis contains extracted test information.
type TestAnalysis struct {
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Skipped     int      `json:"skipped"`
	Total       int      `json:"total"`
	OverallPass bool     `json:"overall_pass"`
	FailedTests []string `json:"failed_tests,omitempty"`
	Coverage    float64  `json:"coverage,omitempty"`
	RawOutput   string   `json:"-"`
}

// ToFacts converts test analysis to Mangle facts.
func (t TestAnalysis) ToFacts(requestID string) []Fact {
	facts := []Fact{
		{
			Predicate: "test_result",
			Args:      []interface{}{requestID, int64(t.Passed), int64(t.Failed), int64(t.Skipped)},
		},
	}

	if t.OverallPass {
		facts = append(facts, Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/passing"},
		})
	} else {
		facts = append(facts, Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/failing"},
		})
	}

	for _, name := range t.FailedTests {
		facts = append(facts, Fact{
			Predicate: "failed_test",
			Args:      []interface{}{requestID, name},
		})
	}

	if t.Coverage > 0 {
		facts = append(facts, Fact{
			Predicate: "test_coverage",
			Args:      []interface{}{requestID, t.Coverage},
		})
	}

	return facts
}

// AnalyzeBuildOutput extracts build errors from compiler output.
func (a *OutputAnalyzer) AnalyzeBuildOutput(output string) BuildAnalysis {
	analysis := BuildAnalysis{
		RawOutput:   output,
		Diagnostics: make([]Diagnostic, 0),
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Go compiler error pattern: file.go:line:col: message
		if strings.Contains(line, ".go:") && (strings.Contains(line, "error") || strings.Contains(line, "warning") || strings.Contains(line, "undefined")) {
			parts := strings.SplitN(line, ":", 4)
			if len(parts) >= 4 {
				lineNum := int64(0)
				colNum := int64(0)
				fmt.Sscanf(parts[1], "%d", &lineNum)
				fmt.Sscanf(parts[2], "%d", &colNum)

				severity := "error"
				if strings.Contains(parts[3], "warning") {
					severity = "warning"
				}

				analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{
					File:     parts[0],
					Line:     int(lineNum),
					Column:   int(colNum),
					Message:  strings.TrimSpace(parts[3]),
					Severity: severity,
				})

				if severity == "error" {
					analysis.Errors++
				} else {
					analysis.Warnings++
				}
			}
		}
	}

	analysis.Success = analysis.Errors == 0
	return analysis
}

// BuildAnalysis contains extracted build information.
type BuildAnalysis struct {
	Success     bool         `json:"success"`
	Errors      int          `json:"errors"`
	Warnings    int          `json:"warnings"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	RawOutput   string       `json:"-"`
}

// Diagnostic represents a single build error or warning.
type Diagnostic struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// ToFacts converts build analysis to Mangle facts.
func (b BuildAnalysis) ToFacts(requestID string) []Fact {
	facts := []Fact{
		{
			Predicate: "build_result",
			Args:      []interface{}{requestID, b.Success, int64(b.Errors), int64(b.Warnings)},
		},
	}

	for _, d := range b.Diagnostics {
		severityName := "/" + d.Severity
		facts = append(facts, Fact{
			Predicate: "diagnostic",
			Args:      []interface{}{severityName, d.File, int64(d.Line), d.Message},
		})
	}

	return facts
}
