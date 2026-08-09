package tactile

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

const (
	auditFileMode        = 0600
	auditDirectoryMode   = 0700
	auditOutputMaxBytes  = 64 * 1024
	auditDetailFactLimit = 100
)

var goDiagnosticPattern = regexp.MustCompile(`^(.+\.go):([0-9]+):([0-9]+):\s*(.*)$`)

var secretAssignmentPattern = regexp.MustCompile(`(?i)(token|password|api[-_]?key|access_token)=([^&\s]+)`)

// Fact represents a Mangle fact for kernel injection.
// This mirrors core.Fact but is defined here to avoid import cycles.
type Fact struct {
	Predicate string `json:"predicate"`
	Args      []any  `json:"args"`
}

// String returns the Datalog string representation of the fact.
func (f Fact) String() string {
	return types.Fact{Predicate: f.Predicate, Args: f.Args}.String()
}

// ToFacts converts an AuditEvent to Mangle facts for kernel injection.
// These facts allow the kernel to reason about execution history.
func (e AuditEvent) ToFacts() []Fact {
	facts := make([]Fact, 0)

	timestamp := e.Timestamp.Unix()
	commandForAudit := e.Command
	commandForAudit.Arguments = redactArguments(e.Command.Arguments)
	cmdString := commandForAudit.CommandString()

	switch e.Type {
	case AuditEventStart:
		// execution_started(SessionID, RequestID, Binary, Timestamp)
		facts = append(facts, Fact{
			Predicate: "execution_started",
			Args: []any{
				e.SessionID,
				e.Command.RequestID,
				e.Command.Binary,
				timestamp,
			},
		})

		// execution_command(RequestID, CommandString)
		facts = append(facts, Fact{
			Predicate: "execution_command",
			Args: []any{
				e.Command.RequestID,
				cmdString,
			},
		})

		// execution_working_dir(RequestID, WorkingDir)
		if e.Command.WorkingDirectory != "" {
			facts = append(facts, Fact{
				Predicate: "execution_working_dir",
				Args: []any{
					e.Command.RequestID,
					e.Command.WorkingDirectory,
				},
			})
		}

	case AuditEventComplete:
		if e.Result == nil {
			return facts
		}

		// execution_completed(RequestID, ExitCode, DurationMs, Timestamp)
		facts = append(facts, Fact{
			Predicate: "execution_completed",
			Args: []any{
				e.Command.RequestID,
				int64(e.Result.ExitCode),
				e.Result.Duration.Milliseconds(),
				timestamp,
			},
		})

		facts = append(facts, analyzeExecutionOutputFacts(e.Command, e.Result)...)

		// execution_output(RequestID, StdoutLen, StderrLen)
		facts = append(facts, Fact{
			Predicate: "execution_output",
			Args: []any{
				e.Command.RequestID,
				int64(len(e.Result.Stdout)),
				int64(len(e.Result.Stderr)),
			},
		})

		// execution_success(RequestID) or execution_failure(RequestID, Error)
		if e.Result.Success && e.Result.ExitCode == 0 {
			facts = append(facts, Fact{
				Predicate: "execution_success",
				Args:      []any{e.Command.RequestID},
			})
		} else if e.Result.Success && e.Result.ExitCode != 0 {
			facts = append(facts, Fact{
				Predicate: "execution_nonzero",
				Args:      []any{e.Command.RequestID, int64(e.Result.ExitCode)},
			})
		} else {
			facts = append(facts, Fact{
				Predicate: "execution_failure",
				Args:      []any{e.Command.RequestID, e.Result.Error},
			})
		}

		// Resource usage facts
		if e.Result.ResourceUsage != nil {
			ru := e.Result.ResourceUsage
			// execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes)
			facts = append(facts, Fact{
				Predicate: "execution_resource_usage",
				Args: []any{
					e.Command.RequestID,
					ru.TotalCPUTimeMs(),
					ru.MaxRSSBytes,
				},
			})

			// execution_io(RequestID, ReadBytes, WriteBytes)
			if ru.DiskReadBytes > 0 || ru.DiskWriteBytes > 0 {
				facts = append(facts, Fact{
					Predicate: "execution_io",
					Args: []any{
						e.Command.RequestID,
						ru.DiskReadBytes,
						ru.DiskWriteBytes,
					},
				})
			}
		}

		// Sandbox mode fact
		// execution_sandbox(RequestID, SandboxMode)
		sandboxMode := string(e.Result.SandboxUsed)
		if sandboxMode == "" {
			sandboxMode = string(SandboxNone)
		}
		facts = append(facts, Fact{
			Predicate: "execution_sandbox",
			Args: []any{
				e.Command.RequestID,
				"/" + sandboxMode,
			},
		})

	case AuditEventKilled:
		if e.Result == nil {
			return facts
		}

		// execution_killed(RequestID, Reason, DurationMs)
		facts = append(facts, Fact{
			Predicate: "execution_killed",
			Args: []any{
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
			Args: []any{
				e.Command.RequestID,
				errorMsg,
			},
		})

	case AuditEventBlocked:
		// execution_blocked(RequestID, Reason, Timestamp)
		facts = append(facts, Fact{
			Predicate: "execution_blocked",
			Args: []any{
				e.Command.RequestID,
				e.BlockReason,
				timestamp,
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
			Args: []any{
				e.Command.RequestID,
				"/" + sandboxMode,
			},
		})
	}

	// Add tags as facts
	for key, value := range e.Command.Tags {
		facts = append(facts, Fact{
			Predicate: "execution_tag",
			Args: []any{
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

	// File-write failures must be observable even though Log cannot return an error.
	fileWriteErrors int64
	lastFileError   string
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
	fl, err := NewAuditFileLogger(path)
	if err != nil {
		return err
	}

	l.mu.Lock()
	previous := l.fileLogger
	l.fileLogger = fl
	l.mu.Unlock()

	if previous != nil {
		if err := previous.Close(); err != nil {
			logging.TactileWarn("New audit log is active, but closing the previous log failed: %v", err)
		}
	}
	return nil
}

// Close closes the audit logger and any file handles.
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	fileLogger := l.fileLogger
	l.fileLogger = nil
	l.mu.Unlock()

	if fileLogger != nil {
		return fileLogger.Close()
	}
	return nil
}

// Log logs an audit event.
func (l *AuditLogger) Log(event AuditEvent) {
	l.mu.RLock()
	callbacks := slices.Clone(l.callbacks)
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
		if err := fileLogger.Write(event); err != nil {
			l.mu.Lock()
			l.fileWriteErrors++
			l.lastFileError = err.Error()
			l.mu.Unlock()
			logging.TactileWarn("Audit file write failed: %v", err)
		}
	}
}

// GetMetrics returns the current execution metrics.
func (l *AuditLogger) GetMetrics() ExecutionMetricsSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.metrics == nil {
		return ExecutionMetricsSnapshot{
			AuditFileWriteErrors: l.fileWriteErrors,
			LastAuditFileError:   l.lastFileError,
		}
	}
	snapshot := l.metrics.Snapshot()
	snapshot.AuditFileWriteErrors = l.fileWriteErrors
	snapshot.LastAuditFileError = l.lastFileError
	return snapshot
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
	if err := os.MkdirAll(dir, auditDirectoryMode); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file for append
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
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

	data, err := json.Marshal(sanitizeAuditEvent(event))
	if err != nil {
		return err
	}

	_, err = l.file.Write(append(data, '\n'))
	return err
}

// sanitizeAuditEvent keeps the on-disk audit useful without persisting caller
// environment values, stdin, or unbounded process output. In-memory callbacks
// still receive the original event.
func sanitizeAuditEvent(event AuditEvent) AuditEvent {
	sanitized := event
	sanitized.Command = event.Command
	sanitized.Command.Arguments = redactArguments(event.Command.Arguments)
	sanitized.Command.Environment = redactEnvironment(event.Command.Environment)
	sanitized.Command.Tags = maps.Clone(event.Command.Tags)
	if sanitized.Command.Stdin != "" {
		sanitized.Command.Stdin = "[REDACTED]"
	}

	if event.Result != nil {
		result := *event.Result
		result.Stdout = boundedAuditOutput(result.Stdout)
		result.Stderr = boundedAuditOutput(result.Stderr)
		result.Combined = boundedAuditOutput(result.Combined)
		if result.Command != nil {
			command := *result.Command
			command.Arguments = redactArguments(result.Command.Arguments)
			command.Environment = redactEnvironment(result.Command.Environment)
			command.Tags = maps.Clone(result.Command.Tags)
			if command.Stdin != "" {
				command.Stdin = "[REDACTED]"
			}
			result.Command = &command
		}
		sanitized.Result = &result
	}

	return sanitized
}

func redactEnvironment(environment []string) []string {
	if len(environment) == 0 {
		return nil
	}
	redacted := make([]string, len(environment))
	for i, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			name = entry
		}
		redacted[i] = name + "=[REDACTED]"
	}
	return redacted
}

func redactArguments(arguments []string) []string {
	if len(arguments) == 0 {
		return nil
	}
	redacted := slices.Clone(arguments)
	redactNext := false
	for i, argument := range redacted {
		if redactNext {
			redacted[i] = "[REDACTED]"
			redactNext = false
			continue
		}

		lower := strings.ToLower(argument)
		switch lower {
		case "--token", "--password", "--api-key", "--api_key", "--apikey", "--authorization", "--access-token", "--access_token":
			redactNext = true
			continue
		}
		if index := strings.Index(lower, "authorization:"); index >= 0 {
			redacted[i] = argument[:index+len("authorization:")] + " [REDACTED]"
			continue
		}
		redacted[i] = secretAssignmentPattern.ReplaceAllString(argument, "$1=[REDACTED]")
	}
	return redacted
}

func boundedAuditOutput(output string) string {
	if len(output) <= auditOutputMaxBytes {
		return output
	}
	cut := auditOutputMaxBytes
	for cut > 0 && !utf8.RuneStart(output[cut]) {
		cut--
	}
	return output[:cut] + "\n[TRUNCATED IN AUDIT LOG]"
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
	l.file = nil

	// Rename to timestamped backup
	backupPath := fmt.Sprintf("%s.%s", l.path, time.Now().Format("20060102-150405.000000000"))
	if err := os.Rename(l.path, backupPath); err != nil {
		// Recover the active path when possible. Regardless of recovery, l.file
		// never retains a closed handle.
		file, reopenErr := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
		if reopenErr == nil {
			l.file = file
			return fmt.Errorf("rotate audit log: %w", err)
		}
		return fmt.Errorf("rotate audit log: %v; reopen active log: %w", err, reopenErr)
	}

	// Open new file
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
	if err != nil {
		if rollbackErr := os.Rename(backupPath, l.path); rollbackErr == nil {
			recovered, reopenErr := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, auditFileMode)
			if reopenErr == nil {
				l.file = recovered
				return fmt.Errorf("open new audit log after rotate: %w; restored previous log", err)
			}
			return fmt.Errorf("open new audit log after rotate: %v; reopen restored log: %w", err, reopenErr)
		}
		return fmt.Errorf("open new audit log after rotate: %w", err)
	}

	l.file = file
	return nil
}

// ExecutionMetrics tracks aggregate execution statistics.
type ExecutionMetrics struct {
	mu sync.RWMutex

	totalExecutions      int64
	successfulExecutions int64
	failedExecutions     int64
	killedExecutions     int64
	blockedExecutions    int64
	durationSamples      int64

	totalDurationMs  int64
	totalCPUTimeMs   int64
	totalMemoryBytes int64

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
			m.durationSamples++
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
			m.durationSamples++
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
	AuditFileWriteErrors int64            `json:"audit_file_write_errors"`
	LastAuditFileError   string           `json:"last_audit_file_error,omitempty"`
}

// Snapshot returns a point-in-time copy of the metrics.
func (m *ExecutionMetrics) Snapshot() ExecutionMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Copy maps
	byBinary := make(map[string]int64)
	maps.Copy(byBinary, m.executionsByBinary)
	bySession := make(map[string]int64)
	maps.Copy(bySession, m.executionsBySession)

	// Calculate derived metrics
	successRate := float64(0)
	avgDuration := float64(0)
	classified := m.successfulExecutions + m.failedExecutions + m.killedExecutions
	if classified > 0 {
		successRate = float64(m.successfulExecutions) / float64(classified)
	}
	if m.durationSamples > 0 {
		avgDuration = float64(m.totalDurationMs) / float64(m.durationSamples)
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
	m.durationSamples = 0
	m.totalDurationMs = 0
	m.totalCPUTimeMs = 0
	m.totalMemoryBytes = 0
	m.executionsByBinary = make(map[string]int64)
	m.executionsBySession = make(map[string]int64)
	m.lastEventTime = time.Time{}
}

// AuditedExecutorWrapper wraps any Executor to add audit logging.
type AuditedExecutorWrapper struct {
	executor      Executor
	logger        *AuditLogger
	callbackWired bool
}

// NewAuditedExecutor wraps an executor with audit logging.
func NewAuditedExecutor(executor Executor, logger *AuditLogger) *AuditedExecutorWrapper {
	// If the executor already supports audit callbacks, use that
	callbackWired := false
	if audited, ok := executor.(AuditedExecutorInterface); ok && logger != nil {
		audited.SetAuditCallback(logger.Log)
		callbackWired = true
	}

	return &AuditedExecutorWrapper{
		executor:      executor,
		logger:        logger,
		callbackWired: callbackWired,
	}
}

// Execute runs a command and logs the execution.
func (w *AuditedExecutorWrapper) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	if w.callbackWired || w.logger == nil {
		return w.executor.Execute(ctx, cmd)
	}

	startedAt := time.Now()
	executorName := w.executor.Capabilities().Name
	w.logger.Log(AuditEvent{
		Type:         AuditEventStart,
		Timestamp:    startedAt,
		Command:      cmd,
		SessionID:    cmd.SessionID,
		ExecutorName: executorName,
	})

	result, err := w.executor.Execute(ctx, cmd)
	finishedAt := time.Now()
	auditResult := result
	if result != nil {
		resultCopy := *result
		if resultCopy.StartedAt.IsZero() {
			resultCopy.StartedAt = startedAt
		}
		if resultCopy.FinishedAt.IsZero() {
			resultCopy.FinishedAt = finishedAt
		}
		if resultCopy.Duration == 0 {
			resultCopy.Duration = finishedAt.Sub(startedAt)
		}
		if err != nil && resultCopy.Error == "" {
			resultCopy.Error = err.Error()
		}
		auditResult = &resultCopy
	} else if err != nil {
		auditResult = &ExecutionResult{
			Success:    false,
			ExitCode:   -1,
			Error:      err.Error(),
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			Duration:   finishedAt.Sub(startedAt),
		}
	} else if result == nil {
		auditResult = &ExecutionResult{
			Success:    false,
			ExitCode:   -1,
			Error:      "executor returned no result",
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			Duration:   finishedAt.Sub(startedAt),
		}
	}

	eventType := AuditEventComplete
	if result != nil && result.Killed {
		eventType = AuditEventKilled
	} else if err != nil || result == nil {
		eventType = AuditEventError
	}
	w.logger.Log(AuditEvent{
		Type:         eventType,
		Timestamp:    finishedAt,
		Command:      cmd,
		Result:       auditResult,
		SessionID:    cmd.SessionID,
		ExecutorName: executorName,
	})

	return result, err
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
	sawFailure := false

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)

		// Go test patterns
		if strings.HasPrefix(line, "--- PASS:") {
			analysis.Passed++
		} else if strings.HasPrefix(line, "--- FAIL:") {
			analysis.Failed++
			sawFailure = true
			// Extract test name
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				analysis.FailedTests = append(analysis.FailedTests, parts[2])
			}
		} else if strings.HasPrefix(line, "--- SKIP:") {
			analysis.Skipped++
		} else if line == "PASS" && !sawFailure {
			analysis.OverallPass = true
		} else if line == "FAIL" || strings.HasPrefix(line, "FAIL\t") {
			analysis.OverallPass = false
			sawFailure = true
		}

		// Extract timing
		if strings.Contains(line, "coverage:") {
			// Parse coverage percentage
			for part := range strings.FieldsSeq(line) {
				if strings.HasSuffix(part, "%") {
					var coverage float64
					if n, err := fmt.Sscanf(part, "%f%%", &coverage); err == nil && n == 1 {
						analysis.Coverage = coverage
					}
				}
			}
		}
	}

	analysis.Total = analysis.Passed + analysis.Failed + analysis.Skipped
	if sawFailure || analysis.Failed > 0 {
		analysis.OverallPass = false
	}
	return analysis
}

// analyzeExecutionOutputFacts deterministically connects completed Go test/build
// commands to the structured analyzer facts. Other binaries and Go subcommands
// retain only the generic execution lifecycle facts.
func analyzeExecutionOutputFacts(command Command, result *ExecutionResult) []Fact {
	if result == nil || goCommandSubcommand(command) == "" {
		return nil
	}
	output := result.Combined
	if output == "" {
		output = result.Stdout
		if result.Stderr != "" {
			if output != "" {
				output += "\n"
			}
			output += result.Stderr
		}
	}

	analyzer := NewOutputAnalyzer()
	switch goCommandSubcommand(command) {
	case "test":
		analysis := analyzer.AnalyzeTestOutput(output)
		analysis.OverallPass = result.Success && result.ExitCode == 0
		return analysis.ToFacts(command.RequestID)
	case "build":
		analysis := analyzer.AnalyzeBuildOutput(output)
		analysis.Success = result.Success && result.ExitCode == 0
		return analysis.ToFacts(command.RequestID)
	default:
		return nil
	}
}

func goCommandSubcommand(command Command) string {
	binaryPath := strings.ReplaceAll(command.Binary, `\`, "/")
	binary := strings.TrimSuffix(strings.ToLower(filepath.Base(binaryPath)), ".exe")
	if binary != "go" {
		return ""
	}
	for i := 0; i < len(command.Arguments); i++ {
		argument := command.Arguments[i]
		if argument == "-C" {
			i++
			continue
		}
		if strings.HasPrefix(argument, "-C=") || strings.HasPrefix(argument, "-") {
			continue
		}
		if argument == "test" || argument == "build" {
			return argument
		}
		return ""
	}
	return ""
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
			Predicate: "execution_test_summary",
			Args:      []any{requestID, int64(t.Passed), int64(t.Failed), int64(t.Skipped)},
		},
	}

	if t.OverallPass {
		facts = append(facts, Fact{
			Predicate: "execution_test_state",
			Args:      []any{requestID, "/passing"},
		})
	} else {
		facts = append(facts, Fact{
			Predicate: "execution_test_state",
			Args:      []any{requestID, "/failing"},
		})
	}

	for i, name := range t.FailedTests {
		if i >= auditDetailFactLimit {
			break
		}
		facts = append(facts, Fact{
			Predicate: "execution_failed_test",
			Args:      []any{requestID, name},
		})
	}

	if t.Coverage > 0 {
		facts = append(facts, Fact{
			Predicate: "execution_test_coverage",
			Args:      []any{requestID, types.PercentClamp(t.Coverage)},
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

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Go compiler error pattern: file.go:line:col: message. The greedy
		// file group preserves Windows drive-letter paths.
		parts := goDiagnosticPattern.FindStringSubmatch(line)
		if len(parts) == 5 {
			lineNum, err1 := strconv.ParseInt(parts[2], 10, 64)
			colNum, err2 := strconv.ParseInt(parts[3], 10, 64)

			// Only process if we successfully parsed line and column numbers
			if err1 == nil && err2 == nil && lineNum > 0 {
				severity := "error"
				if strings.Contains(parts[4], "warning") {
					severity = "warning"
				}

				analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{
					File:     parts[1],
					Line:     int(lineNum),
					Column:   int(colNum),
					Message:  strings.TrimSpace(parts[4]),
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
	success := "/false"
	if b.Success {
		success = "/true"
	}
	facts := []Fact{
		{
			Predicate: "execution_build_summary",
			Args:      []any{requestID, success, int64(b.Errors), int64(b.Warnings)},
		},
	}

	for i, d := range b.Diagnostics {
		if i >= auditDetailFactLimit {
			break
		}
		severity := strings.TrimSpace(d.Severity)
		if severity == "" {
			severity = "error"
		}
		severityName := "/" + severity
		facts = append(facts, Fact{
			Predicate: "execution_diagnostic",
			Args: []any{
				requestID,
				severityName,
				d.File,
				int64(d.Line),
				int64(d.Column),
				d.Message,
			},
		})
	}

	return facts
}
