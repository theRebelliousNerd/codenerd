package context_harness

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileLogger manages log file output for context testing.
type FileLogger struct {
	baseDir string
	session string

	// Output files
	promptLog      *os.File
	jitLog         *os.File
	activationLog  *os.File
	compressionLog *os.File
	piggybacking   *os.File
	feedbackLog    *os.File
	summaryLog     *os.File

	// Multi-writers (console + file)
	promptWriter      io.Writer
	jitWriter         io.Writer
	activationWriter  io.Writer
	compressionWriter io.Writer
	piggybackWriter   io.Writer
	feedbackWriter    io.Writer
	summaryWriter     io.Writer
}

// NewFileLogger creates a new file logger with output directory.
func NewFileLogger(baseDir string, console io.Writer) (*FileLogger, error) {
	// Create session directory
	timestamp := time.Now().Format("20060102-150405")
	sessionDir := filepath.Join(baseDir, fmt.Sprintf("session-%s", timestamp))

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	fl := &FileLogger{
		baseDir: baseDir,
		session: sessionDir,
	}

	// Open log files
	var err error

	fl.promptLog, err = os.Create(filepath.Join(sessionDir, "prompts.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create prompts.log: %w", err)
	}

	fl.jitLog, err = os.Create(filepath.Join(sessionDir, "jit-compilation.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create jit-compilation.log: %w", err)
	}

	fl.activationLog, err = os.Create(filepath.Join(sessionDir, "spreading-activation.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create spreading-activation.log: %w", err)
	}

	fl.compressionLog, err = os.Create(filepath.Join(sessionDir, "compression.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create compression.log: %w", err)
	}

	fl.piggybacking, err = os.Create(filepath.Join(sessionDir, "piggyback-protocol.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create piggyback-protocol.log: %w", err)
	}

	fl.feedbackLog, err = os.Create(filepath.Join(sessionDir, "context-feedback.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create context-feedback.log: %w", err)
	}

	fl.summaryLog, err = os.Create(filepath.Join(sessionDir, "summary.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to create summary.log: %w", err)
	}

	// Create multi-writers (console + file)
	if console != nil {
		fl.promptWriter = io.MultiWriter(console, fl.promptLog)
		fl.jitWriter = io.MultiWriter(console, fl.jitLog)
		fl.activationWriter = io.MultiWriter(console, fl.activationLog)
		fl.compressionWriter = io.MultiWriter(console, fl.compressionLog)
		fl.piggybackWriter = io.MultiWriter(console, fl.piggybacking)
		fl.feedbackWriter = io.MultiWriter(console, fl.feedbackLog)
		fl.summaryWriter = io.MultiWriter(console, fl.summaryLog)
	} else {
		// File-only
		fl.promptWriter = fl.promptLog
		fl.jitWriter = fl.jitLog
		fl.activationWriter = fl.activationLog
		fl.compressionWriter = fl.compressionLog
		fl.piggybackWriter = fl.piggybacking
		fl.feedbackWriter = fl.feedbackLog
		fl.summaryWriter = fl.summaryLog
	}

	// Write headers
	fl.writeHeaders()

	return fl, nil
}

// writeHeaders writes file headers with metadata.
func (fl *FileLogger) writeHeaders() {
	header := fmt.Sprintf(`═══════════════════════════════════════════════════════════════
codeNERD Context Harness - Test Session
═══════════════════════════════════════════════════════════════
Session Start: %s
Session Directory: %s
═══════════════════════════════════════════════════════════════

`, time.Now().Format("2006-01-02 15:04:05"), fl.session)

	fl.promptLog.WriteString(header)
	fl.jitLog.WriteString(header)
	fl.activationLog.WriteString(header)
	fl.compressionLog.WriteString(header)
	fl.piggybacking.WriteString(header)
	fl.feedbackLog.WriteString(header)
	fl.summaryLog.WriteString(header)
}

// GetPromptWriter returns the prompt log writer.
func (fl *FileLogger) GetPromptWriter() io.Writer {
	return fl.promptWriter
}

// GetJITWriter returns the JIT compilation log writer.
func (fl *FileLogger) GetJITWriter() io.Writer {
	return fl.jitWriter
}

// GetActivationWriter returns the spreading activation log writer.
func (fl *FileLogger) GetActivationWriter() io.Writer {
	return fl.activationWriter
}

// GetCompressionWriter returns the compression log writer.
func (fl *FileLogger) GetCompressionWriter() io.Writer {
	return fl.compressionWriter
}

// GetPiggybackWriter returns the piggyback protocol log writer.
func (fl *FileLogger) GetPiggybackWriter() io.Writer {
	return fl.piggybackWriter
}

// GetSummaryWriter returns the summary log writer.
func (fl *FileLogger) GetSummaryWriter() io.Writer {
	return fl.summaryWriter
}

// GetFeedbackWriter returns the context feedback log writer.
func (fl *FileLogger) GetFeedbackWriter() io.Writer {
	return fl.feedbackWriter
}

// GetSessionDir returns the session directory path.
func (fl *FileLogger) GetSessionDir() string {
	return fl.session
}

// Close closes all log files.
func (fl *FileLogger) Close() error {
	// Write footer to all logs
	footer := fmt.Sprintf("\n═══════════════════════════════════════════════════════════════\nSession End: %s\n═══════════════════════════════════════════════════════════════\n",
		time.Now().Format("2006-01-02 15:04:05"))

	fl.promptLog.WriteString(footer)
	fl.jitLog.WriteString(footer)
	fl.activationLog.WriteString(footer)
	fl.compressionLog.WriteString(footer)
	fl.piggybacking.WriteString(footer)
	fl.feedbackLog.WriteString(footer)
	fl.summaryLog.WriteString(footer)

	// Close all files
	files := []*os.File{
		fl.promptLog,
		fl.jitLog,
		fl.activationLog,
		fl.compressionLog,
		fl.piggybacking,
		fl.feedbackLog,
		fl.summaryLog,
	}

	var firstErr error
	for _, f := range files {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Write manifest
	if err := fl.writeManifest(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// writeManifest writes a manifest file with session metadata.
func (fl *FileLogger) writeManifest() error {
	manifestPath := filepath.Join(fl.session, "MANIFEST.txt")
	f, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer f.Close()

	manifest := fmt.Sprintf(`codeNERD Context Harness - Test Session Manifest
═══════════════════════════════════════════════════════════════

Session Directory: %s
Generated: %s

FILES:
───────────────────────────────────────────────────────────────

prompts.log
  - Full prompts sent to LLM
  - Token counts and budget utilization
  - JIT atom selection details
  - Spreading activation results

jit-compilation.log
  - JIT prompt compiler traces
  - Atom selection with priorities
  - Context dimension matching
  - Budget allocation breakdown

spreading-activation.log
  - Activation score calculations
  - Dependency graph traversal
  - Campaign/Issue/Session context
  - Fact selection reasoning

compression.log
  - Before/after compression comparisons
  - Metadata extraction
  - Compression ratios per turn
  - Lossy element identification

piggyback-protocol.log
  - Surface vs. control packet parsing
  - Intent classification
  - Kernel state changes
  - Tool call tracking

context-feedback.log
  - LLM context usefulness ratings
  - Helpful vs noise predicate tracking
  - Learned predicate scores over time
  - Score impact on activation

summary.log
  - Overall session statistics
  - Aggregate metrics
  - Performance summaries
  - Final reports

═══════════════════════════════════════════════════════════════

VIEWING INSTRUCTIONS:

1. Start with summary.log for high-level overview
2. Check prompts.log to see what the LLM received
3. Use jit-compilation.log to understand atom selection
4. Review spreading-activation.log for retrieval decisions
5. Examine compression.log to validate compression quality
6. Inspect piggyback-protocol.log for kernel state changes
7. Check context-feedback.log for usefulness learning patterns

All logs are plain text and can be viewed with any text editor.

═══════════════════════════════════════════════════════════════
`, fl.session, time.Now().Format("2006-01-02 15:04:05"))

	_, err = f.WriteString(manifest)
	return err
}
