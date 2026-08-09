// Package logging — LLM I/O tracing subsystem.
//
// When trace_llm_io is explicitly enabled in the logging config, this logger captures
// the FULL prompt package (system prompt, conversation history, user prompt)
// and raw LLM responses to a dedicated log file:
//
//	.nerd/logs/<runPrefix>_llm_io.log  (falls back to .nerd/logs/YYYY-MM-DD_llm_io.log when run prefix is empty)
//
// This is invaluable for debugging perception/articulation prompt quality,
// token usage, and understanding what the LLM actually sees vs what we think
// it sees.
package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// llmIOLogger handles dedicated LLM I/O trace logging.
type llmIOLogger struct {
	mu      sync.Mutex
	logger  *log.Logger
	file    *os.File
	enabled bool
}

var (
	llmIO     *llmIOLogger
	llmIOOnce sync.Once
)

// LLMMessage represents a single message in the conversation history.
type LLMMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// initLLMIOLogger initializes the LLM I/O logger if trace_llm_io is enabled.
// Called lazily on first use.
func initLLMIOLogger() {
	llmIOOnce.Do(func() {
		llmIO = &llmIOLogger{}

		configMu.RLock()
		enabled := config.TraceLLMIO
		configMu.RUnlock()

		if !enabled || logsDir == "" {
			llmIO.enabled = false
			return
		}

		prefix := currentRunPrefix()
		if prefix == "" {
			prefix = time.Now().Format("2006-01-02")
		}
		logPath := filepath.Join(logsDir, prefix+"_llm_io.log")

		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[logging] Failed to open LLM I/O log: %v\n", err)
			llmIO.enabled = false
			return
		}

		llmIO.file = f
		llmIO.logger = log.New(f, "", 0) // No prefix — we format our own headers
		llmIO.enabled = true

		Get(CategoryBoot).Info("LLM I/O tracing enabled: %s", logPath)
	})
}

// IsLLMIOTracingEnabled returns whether LLM I/O tracing is active.
func IsLLMIOTracingEnabled() bool {
	initLLMIOLogger()
	if llmIO == nil {
		return false
	}
	return llmIO.enabled
}

// LogLLMRequest logs a full LLM request (system prompt + conversation history + user prompt).
//
// callsite identifies where the call originated (e.g., "perception-transducer",
// "articulation-emitter", "coder-shard").
func LogLLMRequest(callsite string, systemPrompt string, userPrompt string, history []LLMMessage, model string, temperature float64) {
	initLLMIOLogger()
	if llmIO == nil || !llmIO.enabled {
		return
	}

	llmIO.mu.Lock()
	defer llmIO.mu.Unlock()

	now := time.Now().Format("15:04:05.000")
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("\n═══ LLM REQUEST [%s] @ %s ═══\n", callsite, now))
	sb.WriteString(fmt.Sprintf("MODEL: %s\n", model))
	sb.WriteString(fmt.Sprintf("TEMPERATURE: %.2f\n", temperature))

	// System prompt
	sb.WriteString(fmt.Sprintf("SYSTEM PROMPT (%d chars, ~%d tokens):\n", len(systemPrompt), len(systemPrompt)/4))
	sb.WriteString("─── BEGIN SYSTEM PROMPT ───\n")
	sb.WriteString(systemPrompt)
	if !strings.HasSuffix(systemPrompt, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("─── END SYSTEM PROMPT ───\n")

	// Conversation history
	if len(history) > 0 {
		sb.WriteString(fmt.Sprintf("CONVERSATION HISTORY (%d turns):\n", len(history)))
		sb.WriteString("─── BEGIN HISTORY ───\n")
		for i, msg := range history {
			role := strings.ToUpper(msg.Role)
			content := msg.Content
			if len(content) > 2000 {
				content = content[:2000] + fmt.Sprintf("... [TRUNCATED, full=%d chars]", len(msg.Content))
			}
			sb.WriteString(fmt.Sprintf("[%d][%s] %s\n", i+1, role, content))
		}
		sb.WriteString("─── END HISTORY ───\n")
	} else {
		sb.WriteString("CONVERSATION HISTORY: (none)\n")
	}

	// User prompt
	sb.WriteString(fmt.Sprintf("USER PROMPT (%d chars, ~%d tokens):\n", len(userPrompt), len(userPrompt)/4))
	sb.WriteString("─── BEGIN USER PROMPT ───\n")
	sb.WriteString(userPrompt)
	if !strings.HasSuffix(userPrompt, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("─── END USER PROMPT ───\n")

	// Total token estimate
	totalChars := len(systemPrompt) + len(userPrompt)
	for _, m := range history {
		totalChars += len(m.Content)
	}
	sb.WriteString(fmt.Sprintf("TOTAL ESTIMATED TOKENS: ~%d (%d chars)\n", totalChars/4, totalChars))
	sb.WriteString(fmt.Sprintf("═══ END REQUEST [%s] ═══\n", callsite))

	llmIO.logger.Print(sb.String())
}

// LogLLMResponse logs a full LLM response.
//
// callsite identifies where the call originated. duration is how long the API
// call took. The full raw response is logged without truncation.
func LogLLMResponse(callsite string, response string, duration time.Duration, tokenEstimate int) {
	initLLMIOLogger()
	if llmIO == nil || !llmIO.enabled {
		return
	}

	llmIO.mu.Lock()
	defer llmIO.mu.Unlock()

	now := time.Now().Format("15:04:05.000")
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\n═══ LLM RESPONSE [%s] @ %s (%dms) ═══\n", callsite, now, duration.Milliseconds()))
	sb.WriteString(fmt.Sprintf("RESPONSE (%d chars, ~%d tokens):\n", len(response), tokenEstimate))
	sb.WriteString("─── BEGIN RESPONSE ───\n")
	sb.WriteString(response)
	if !strings.HasSuffix(response, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("─── END RESPONSE ───\n")
	sb.WriteString(fmt.Sprintf("═══ END RESPONSE [%s] ═══\n", callsite))

	llmIO.logger.Print(sb.String())
}

// LogLLMError logs a failed LLM call with the error details.
func LogLLMError(callsite string, err error, duration time.Duration) {
	initLLMIOLogger()
	if llmIO == nil || !llmIO.enabled {
		return
	}

	llmIO.mu.Lock()
	defer llmIO.mu.Unlock()

	now := time.Now().Format("15:04:05.000")
	llmIO.logger.Printf("\n═══ LLM ERROR [%s] @ %s (%dms) ═══\nERROR: %v\n═══ END ERROR [%s] ═══\n",
		callsite, now, duration.Milliseconds(), err, callsite)
}

// CloseLLMIOLogger closes the LLM I/O log file.
// Call during shutdown cleanup.
func CloseLLMIOLogger() {
	if llmIO != nil && llmIO.file != nil {
		llmIO.mu.Lock()
		defer llmIO.mu.Unlock()
		llmIO.file.Close()
		llmIO.file = nil
		llmIO.enabled = false
	}
}
