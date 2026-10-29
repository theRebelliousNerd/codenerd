package campaign

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// The document fallback: when the shard that should have written a
// deliverable document failed, the campaign generates the document directly
// and writes it through the VirtualStore's front door.

// executeFileTaskFallback generates a deliverable document directly when the
// shard that should have written it failed; cause is that failure. It writes
// documents only. Code and configuration carry the turn's obligations -- tests,
// coverage, vet, the verdict -- which a bare generation call does not, so a
// coder stopped on them fails its task instead of landing the file another way
// (campaign 1284b6bb: "hollow success blocked: turn created Go source ...
// without a test file", then a request to "Generate the following file").
// Every failure leads with cause: the task's error is what the retry and the
// replanner read, and the fallback's own refusal is not why the work stopped.
func (o *Orchestrator) executeFileTaskFallback(ctx context.Context, task *Task, targetPath string, cause error) (any, error) {
	logging.CampaignDebug("Executing file task fallback for %s via direct LLM", task.ID)
	fail := func(err error) error {
		if cause == nil {
			return err
		}
		return fmt.Errorf("%w; then the fallback: %w", cause, err)
	}

	// If no target path, try to extract from task description.
	if targetPath == "" {
		targetPath = extractPathFromDescription(task.Description)
		if targetPath != "" {
			logging.CampaignDebug("Extracted target path from description: %s", targetPath)
		}
	}
	// F-TASK-1: the decomposer frequently emits artifact-producing tasks
	// (/document, /file_create) with no target path/artifact. Failing them
	// permanently deadlocks the phase (a failed task blocks phase completion).
	// Write to a deterministic campaign artifact path instead so the work
	// product is preserved and the phase can proceed. Tasks that mutate a
	// specific existing file (/file_modify, /refactor, /integrate) still require
	// an explicit path — defaulting one would be meaningless.
	if targetPath == "" {
		if task.Type == TaskTypeDocument || task.Type == TaskTypeFileCreate {
			targetPath = o.defaultTaskArtifactPath(task)
			logging.Campaign("No target path for %s task %s; defaulting to campaign artifact %s", task.Type, task.ID, targetPath)
		} else {
			logging.Get(logging.CategoryCampaign).Error("No target path for file task %s and could not extract from description", task.ID)
			return nil, fail(fmt.Errorf("no target path specified for file task %s", task.ID))
		}
	}

	// Path traversal guard
	cleanPath := filepath.Clean(targetPath)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
		return nil, fail(fmt.Errorf("path traversal attempt blocked for path: %s", targetPath))
	}
	targetPath = cleanPath

	// The fallback is a creator, never an editor. Refuse to overwrite an
	// existing file before generating anything: a hollow coder result must fail
	// the task (retry -> attempt-cap re-plan), never truncate real work through
	// this back door. An existing DIRECTORY is refused too (F-CAMP-1): this
	// path can only write one file, and aiming it at a directory previously
	// produced a misleading kernel write refusal instead of the real failure.
	statPath := filepath.Join(o.workspace, targetPath)
	if info, statErr := os.Stat(statPath); statErr == nil {
		if info.IsDir() {
			logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: target %s is an existing directory; the fallback writes a single file, not a directory", task.ID, targetPath)
			return nil, fail(fmt.Errorf("fallback refused for %s: target %s is an existing directory and the fallback cannot write a directory", task.ID, targetPath))
		}
		if info.Mode().IsRegular() {
			logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: target %s already exists; modify tasks need the coder path", task.ID, targetPath)
			return nil, fail(fmt.Errorf("fallback refused for %s: target %s exists (%d bytes); modify tasks need the coder path", task.ID, targetPath, info.Size()))
		}
	}

	// Documents only: see the doc comment. Checked before anything is
	// generated, so a refused target costs no model call.
	if !isDeliverableDocument(targetPath) {
		logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: %s is not a document; a coder stopped on code fails its task", task.ID, targetPath)
		return nil, fail(fmt.Errorf("fallback refused for %s: %s is not a document; direct generation writes deliverables, never code or configuration", task.ID, targetPath))
	}

	// Front door only: repository writes go through the VirtualStore so
	// permitted/3, the Dreamer gate and the FileWriteValidator apply. Never
	// fall back to a direct write around them.
	if o.virtualStore == nil {
		logging.Get(logging.CategoryCampaign).Warn("Fallback refused for task %s: no VirtualStore attached; refusing to write %s around the front door", task.ID, targetPath)
		return nil, fail(fmt.Errorf("fallback refused for %s: virtualStore is nil, cannot write %s through the VirtualStore", task.ID, targetPath))
	}

	// Holographic context: the fallback prompt carries upstream durable
	// findings after the description and before the file target, so a direct-LLM
	// document still sees what to write from. Prompt-only; the written file is
	// the shard result, never this input section.
	taskBlock := task.Description
	upstream, err := o.taskContextSection(ctx, task)
	if err != nil {
		return nil, fail(fmt.Errorf("derive the upstream evidence of %s: %w", task.ID, err))
	}
	if upstream != "" {
		taskBlock = taskBlock + "\n\n" + upstream
	}
	systemPrompt, err := rolePrompt(ctx, o.promptProvider, RoleDocumentWriter, o.campaign.ID, o.llmClient)
	if err != nil {
		return nil, fail(fmt.Errorf("generate %s: %w", targetPath, err))
	}
	// The reply is the file itself, written as it comes back -- unwrapped
	// only when the model fenced the whole of it. The VirtualStore writes
	// the bytes it is given, so this is the one place a reply becomes a file.
	reply, err := o.llmClient.CompleteWithSystem(types.WithStructuredOutputOnly(ctx), systemPrompt,
		fmt.Sprintf("Task: %s\nTarget Path: %s", taskBlock, targetPath))
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("LLM file generation failed for task %s: %v", task.ID, err)
		return nil, fail(fmt.Errorf("generate %s: %w", targetPath, err))
	}
	content := unwrapReplyFence(reply)
	if len(content) != len(reply) {
		logging.CampaignDebug("Unwrapped the fenced reply for %s: %d -> %d bytes", targetPath, len(reply), len(content))
	}

	// F-DOC-2 (observed live: "1. End. 2. Finish." x1500 written as a 19KB
	// document and counted done): Go measures the document's words and the
	// kernel decides whether it is a repetition loop
	// (generated_output_degenerate). A loop fails the attempt, and the
	// campaign's retry carries why. Until 2026-09-23 Go decided with four
	// constants of its own, re-generated with a prompt of its own, and on a
	// second loop wrote a placeholder the task counted as done (sweep finding
	// F12).
	degenerate, err := o.generationDegenerate(task.ID, content)
	if err != nil {
		return nil, fail(fmt.Errorf("judge the generated %s: %w", targetPath, err))
	}
	if degenerate {
		o.emitEvent(EventGenerationDegraded, task.PhaseID, task.ID, "the generated document is a repetition loop", nil)
		return nil, fail(fmt.Errorf("%w: the generated %s is a repetition loop", ErrNoDeliverable, targetPath))
	}

	fullPath := filepath.Join(o.workspace, targetPath)
	logging.CampaignDebug("Writing generated file via VirtualStore: %s (%d bytes)", fullPath, len(content))
	// Front door only: the write goes through the VirtualStore so permitted/3,
	// the Dreamer gate and the FileWriteValidator apply. A direct os.WriteFile
	// here bypassed every one of them (observed live: a 251-line policy file
	// truncated to a 9-line stub).
	// The router executes only what the kernel has permitted, and permission
	// derives from a pending_action fact the caller asserts first — the same
	// contract the session executor follows. Assert it so the constitution
	// decides (a critical path or an unsafe target is denied there, not here).
	actionID := fmt.Sprintf("campaign-fallback-%s", task.ID)
	// permitted/3 is matched against the exact payload the action is routed
	// with: the canonical JSON the session executor asserts (json.Marshal of
	// the args; over the cap refused, never truncated). This used to assert a
	// {"content_bytes":N} summary instead, so no fallback write was ever
	// permitted -- the kernel refused every document the model had just been
	// asked to generate.
	payload := map[string]any{"content": content}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fail(fmt.Errorf("encode the fallback write for %s: %w", targetPath, err))
	}
	if len(payloadJSON) > session.MaxActionPayloadBytes {
		return nil, fail(fmt.Errorf("fallback refused for %s: the generated %s is %d bytes encoded, over the %d-byte action payload cap",
			task.ID, targetPath, len(payloadJSON), session.MaxActionPayloadBytes))
	}
	if o.kernel != nil {
		pending := core.Fact{
			Predicate: "pending_action",
			Args:      []any{actionID, core.MangleAtom("/write_file"), fullPath, string(payloadJSON), time.Now().Unix()},
		}
		if err := o.kernel.Assert(pending); err != nil {
			logging.Get(logging.CategoryCampaign).Warn("Failed to assert pending_action for fallback write %s: %v", fullPath, err)
		} else {
			defer func() { _ = o.kernel.RetractFact(pending) }()
		}
	}
	writeResult, err := o.virtualStore.RouteActionResult(ctx, core.Fact{
		Predicate: "next_action",
		Args:      []any{actionID, "write_file", fullPath, payload},
	})
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("VirtualStore write failed for fallback file %s: %v", fullPath, err)
		return nil, fail(err)
	}
	if !writeResult.Success {
		logging.Get(logging.CategoryCampaign).Warn("VirtualStore refused fallback write for %s: %s", fullPath, writeResult.Error)
		return nil, fail(fmt.Errorf("fallback write for %s refused by VirtualStore: %s", targetPath, writeResult.Error))
	}

	logging.CampaignDebug("File fallback completed via VirtualStore: %s", fullPath)
	return map[string]any{"path": fullPath, "size": len(content)}, nil
}

// generationDegenerate measures a generated document's words, asserts them as
// generated_output_vocab and asks the kernel whether the document is a
// repetition loop. The measurement is the whole of Go's part.
func (o *Orchestrator) generationDegenerate(taskID, text string) (bool, error) {
	if o.kernel == nil {
		return false, fmt.Errorf("no kernel to judge the document generated for %s", taskID)
	}
	tokens, words, distinct := vocabulary(text)
	_ = o.kernel.RetractFact(core.Fact{Predicate: "generated_output_vocab", Args: []any{taskID}})
	if err := o.kernel.Assert(core.Fact{
		Predicate: "generated_output_vocab",
		Args:      []any{taskID, int64(tokens), int64(words), int64(distinct)},
	}); err != nil {
		return false, fmt.Errorf("assert the measurement of %s: %w", taskID, err)
	}
	return o.holdsFor("generated_output_degenerate", taskID)
}

// vocabulary measures text: its whitespace-separated tokens, the words among
// them (each token's letters, lower-cased; a counter or punctuation has none),
// and how many of those words are distinct.
func vocabulary(text string) (tokens, words, distinct int) {
	fields := strings.Fields(text)
	seen := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		word := strings.ToLower(strings.TrimFunc(f, func(r rune) bool {
			return !unicode.IsLetter(r)
		}))
		if word == "" {
			continue
		}
		words++
		seen[word] = struct{}{}
	}
	return len(fields), words, len(seen)
}

// isDeliverableDocument reports whether path names a prose document -- the
// only kind of file the direct-generation fallback may write.
func isDeliverableDocument(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".txt", ".rst":
		return true
	}
	return false
}

// unwrapReplyFence returns the document a document writer's reply carries
// (ladder V1). The role (campaign/document_writer/contract) asks for the file
// alone, so the reply is the document as it came back -- fenced examples
// inside it included -- unless the whole reply is one fenced block: a first
// line that opens a fence (three or more backticks and an optional info
// string) and a last line that is the same run of backticks alone. Then the
// model fenced the file anyway, and the document is that block's inside.
//
// This replaced an extractor that returned the inside of the first fence it
// found anywhere in the reply and trimmed it: a Markdown document with a
// fenced example was written as that example alone.
func unwrapReplyFence(reply string) string {
	lines := strings.Split(strings.TrimSpace(reply), "\n")
	if len(lines) == 1 {
		return reply
	}
	open := strings.TrimRight(lines[0], " \t\r")
	if !strings.HasPrefix(open, "```") {
		return reply
	}
	ticks := len(open) - len(strings.TrimLeft(open, "`"))
	if strings.Contains(open[ticks:], "`") {
		return reply
	}
	if strings.TrimRight(lines[len(lines)-1], " \t\r") != open[:ticks] {
		return reply
	}
	inside := lines[1 : len(lines)-1]
	if len(inside) == 0 {
		return ""
	}
	return strings.Join(inside, "\n") + "\n"
}
