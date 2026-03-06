package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// CodexCLIProbeFailure classifies health probe failures for auth/status UX.
type CodexCLIProbeFailure string

const (
	CodexCLIProbeFailureNone                 CodexCLIProbeFailure = ""
	CodexCLIProbeFailureAuthUnavailable      CodexCLIProbeFailure = "auth_unavailable"
	CodexCLIProbeFailureSkillMissing         CodexCLIProbeFailure = "skill_missing"
	CodexCLIProbeFailureSchemaRejected       CodexCLIProbeFailure = "schema_rejected"
	CodexCLIProbeFailureRateLimited          CodexCLIProbeFailure = "rate_limited"
	CodexCLIProbeFailureFallbackModelMissing CodexCLIProbeFailure = "fallback_model_exhausted"
	CodexCLIProbeFailureExecFailed           CodexCLIProbeFailure = "exec_failed"
)

// CodexExecProbeClassification is the exported auth/status-facing probe taxonomy.
type CodexExecProbeClassification string

const (
	CodexExecProbeReady             CodexExecProbeClassification = "ready"
	CodexExecProbeLoginRequired     CodexExecProbeClassification = "login_required"
	CodexExecProbeSkillMissing      CodexExecProbeClassification = "skill_missing"
	CodexExecProbeSchemaRejected    CodexExecProbeClassification = "schema_rejected"
	CodexExecProbeRateLimited       CodexExecProbeClassification = "rate_limited"
	CodexExecProbeFallbackExhausted CodexExecProbeClassification = "fallback_model_exhausted"
	CodexExecProbeExecFailed        CodexExecProbeClassification = "exec_failed"
)

// CodexCLIProbeResult captures the state of a codex exec readiness probe.
type CodexCLIProbeResult struct {
	SkillEnabled    bool
	SkillName       string
	SkillPath       string
	SkillAvailable  bool
	SchemaValidated bool
	AuthAvailable   bool
	Failure         CodexCLIProbeFailure
	Detail          string
	RawError        string
}

// CodexExecProbeResult is the exported shape used by auth/status UX.
type CodexExecProbeResult struct {
	Classification  CodexExecProbeClassification
	Message         string
	RawError        string
	SkillPath       string
	SkillAvailable  bool
	SchemaSupported bool
}

const codexCLIProbeSchema = `{
  "type": "object",
  "required": ["status", "mode", "skill", "schema_valid"],
  "additionalProperties": false,
  "properties": {
    "status": {"type": "string", "enum": ["ok"]},
    "mode": {"type": "string", "enum": ["codex-exec-health"]},
    "skill": {"type": "string", "minLength": 1},
    "schema_valid": {"type": "boolean", "const": true}
  }
}`

// RunHealthProbe verifies auth, skill availability, and schema-constrained codex exec behavior.
func (c *CodexCLIClient) RunHealthProbe(ctx context.Context) (*CodexCLIProbeResult, error) {
	result := &CodexCLIProbeResult{
		SkillEnabled:   c.skillEnabled,
		SkillName:      c.skillName,
		SkillPath:      c.skillPath,
		SkillAvailable: c.skillAvailable,
	}

	expectedSkill := c.skillName
	if !(c.skillEnabled && c.skillAvailable) {
		expectedSkill = "disabled"
	}

	systemPrompt := "You are verifying codeNERD codex exec readiness. Return only schema-valid JSON."
	userPrompt := fmt.Sprintf(
		"Return JSON with status=\"ok\", mode=\"codex-exec-health\", skill=%q, schema_valid=true.",
		expectedSkill,
	)

	raw, err := c.CompleteWithSchema(ctx, systemPrompt, userPrompt, codexCLIProbeSchema)
	if err != nil {
		result.Failure, result.Detail = classifyCodexCLIProbeError(err)
		result.RawError = err.Error()
		if result.Failure == CodexCLIProbeFailureRateLimited {
			result.AuthAvailable = true
		}
		logging.PerceptionWarn("Codex CLI health probe failed: failure=%s detail=%s", result.Failure, result.Detail)
		return result, err
	}

	var payload struct {
		Status      string `json:"status"`
		Mode        string `json:"mode"`
		Skill       string `json:"skill"`
		SchemaValid bool   `json:"schema_valid"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &payload); err != nil {
		result.Failure = CodexCLIProbeFailureSchemaRejected
		result.Detail = "codex exec returned non-JSON or schema-invalid health payload"
		result.RawError = err.Error()
		logging.PerceptionWarn("Codex CLI health probe JSON parse failed: %v", err)
		return result, fmt.Errorf("%s: %w", result.Detail, err)
	}

	if payload.Status != "ok" || payload.Mode != "codex-exec-health" || !payload.SchemaValid || payload.Skill != expectedSkill {
		result.Failure = CodexCLIProbeFailureSchemaRejected
		result.Detail = fmt.Sprintf("unexpected health payload: status=%q mode=%q skill=%q schema_valid=%t", payload.Status, payload.Mode, payload.Skill, payload.SchemaValid)
		result.RawError = result.Detail
		logging.PerceptionWarn("Codex CLI health probe payload mismatch: %s", result.Detail)
		return result, fmt.Errorf("%s", result.Detail)
	}

	result.AuthAvailable = true
	result.SchemaValidated = true
	if c.skillEnabled && !c.skillAvailable {
		result.Failure = CodexCLIProbeFailureSkillMissing
		result.Detail = fmt.Sprintf("repo skill %q not found at %s", c.skillName, c.skillPath)
		result.RawError = result.Detail
		logging.PerceptionWarn("Codex CLI health probe found working exec but missing repo skill: %s", result.Detail)
		return result, fmt.Errorf("%s", result.Detail)
	}

	logging.Perception("Codex CLI health probe succeeded: skill=%s schema_validated=true", expectedSkill)
	return result, nil
}

func classifyCodexCLIProbeError(err error) (CodexCLIProbeFailure, string) {
	if err == nil {
		return CodexCLIProbeFailureNone, ""
	}

	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return CodexCLIProbeFailureRateLimited, rateLimitErr.Error()
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "fallback model also failed"):
		return CodexCLIProbeFailureFallbackModelMissing, "fallback model also failed after codex exec error"
	case strings.Contains(msg, "invalid json schema"),
		strings.Contains(msg, "schema-invalid"),
		strings.Contains(msg, "non-json"):
		return CodexCLIProbeFailureSchemaRejected, "codex exec rejected or failed schema-constrained output"
	case strings.Contains(msg, "login"),
		strings.Contains(msg, "authenticate"),
		strings.Contains(msg, "subscription"),
		strings.Contains(msg, "unauthorized"),
		strings.Contains(msg, "forbidden"):
		return CodexCLIProbeFailureAuthUnavailable, "codex exec could not use the current ChatGPT login/subscription"
	default:
		return CodexCLIProbeFailureExecFailed, truncateString(err.Error(), 240)
	}
}

// ProbeCodexExec runs the exported codex exec readiness probe for auth/status flows.
func ProbeCodexExec(ctx context.Context, cfg *config.CodexCLIConfig) (*CodexExecProbeResult, error) {
	client := NewCodexCLIClient(cfg)
	result, err := client.RunHealthProbe(ctx)

	exported := &CodexExecProbeResult{
		Classification:  mapCodexCLIProbeClassification(result),
		Message:         result.Detail,
		RawError:        result.RawError,
		SkillPath:       result.SkillPath,
		SkillAvailable:  result.SkillAvailable,
		SchemaSupported: result.SchemaValidated,
	}
	if err == nil {
		exported.Classification = CodexExecProbeReady
		exported.Message = "codex exec noninteractive probe succeeded"
	}
	return exported, err
}

func mapCodexCLIProbeClassification(result *CodexCLIProbeResult) CodexExecProbeClassification {
	if result == nil {
		return CodexExecProbeExecFailed
	}

	switch result.Failure {
	case CodexCLIProbeFailureNone:
		return CodexExecProbeReady
	case CodexCLIProbeFailureAuthUnavailable:
		return CodexExecProbeLoginRequired
	case CodexCLIProbeFailureSkillMissing:
		return CodexExecProbeSkillMissing
	case CodexCLIProbeFailureSchemaRejected:
		return CodexExecProbeSchemaRejected
	case CodexCLIProbeFailureRateLimited:
		return CodexExecProbeRateLimited
	case CodexCLIProbeFailureFallbackModelMissing:
		return CodexExecProbeFallbackExhausted
	default:
		return CodexExecProbeExecFailed
	}
}
