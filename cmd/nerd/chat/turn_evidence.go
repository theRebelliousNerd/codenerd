package chat

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
)

// evidenceCoverage says how much of a source an observation holds. Acquiring a
// source and delivering all of it are different facts: a read can succeed and
// still be a prefix.
type evidenceCoverage string

const (
	// coverageWhole: the body is the whole file.
	coverageWhole evidenceCoverage = "whole"
	// coverageSpan: the body is a prefix; the rest was not read.
	coverageSpan evidenceCoverage = "span"
	// coverageUnavailable: nothing was obtained (denied, missing, no file
	// named). The Diagnostic says why.
	coverageUnavailable evidenceCoverage = "unavailable"
)

// evidenceObservation is one workspace source the answering call is given, or
// the typed reason it was not. The body is what the model sees; a span states
// exactly which bytes, so nothing is presented as more than it is.
type evidenceObservation struct {
	Path          string
	Coverage      evidenceCoverage
	Body          string
	ReturnedBytes int64
	SourceBytes   int64
	ReturnedLines int
	Diagnostic    string
}

// evidenceReadBudget is the bytes of one source the answering call is handed:
// the installed observation policy's shown-region bound
// (observation.max_region_bytes), not a number of this file's.
func evidenceReadBudget() int {
	return config.ResolvedObservationLimits().MaxRegionBytes
}

// readEvidenceFile reads a workspace file for the answering call. IO is
// bounded at the read, not after it: it opens the file, learns the size from
// the handle and reads at most budget bytes, so a large file is never slurped
// to be sliced. Workspace containment is the shared resolver's. A prefix says
// it is one, with the source size.
func readEvidenceFile(workspace, target string, budget int) evidenceObservation {
	obs := evidenceObservation{Path: target, Coverage: coverageUnavailable}
	root := workspace
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	full, err := tools.ResolveWorkspacePath(context.Background(), root, target)
	if err != nil {
		obs.Diagnostic = err.Error()
		return obs
	}
	f, err := os.Open(full)
	if err != nil {
		obs.Diagnostic = err.Error()
		return obs
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		obs.Diagnostic = err.Error()
		return obs
	}
	if info.IsDir() {
		obs.Diagnostic = "is a directory, not a file"
		return obs
	}
	if budget < 1 {
		obs.Diagnostic = "the evidence budget is zero"
		return obs
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(budget)))
	if err != nil {
		obs.Diagnostic = err.Error()
		return obs
	}
	obs.SourceBytes = info.Size()
	if int64(len(data)) < obs.SourceBytes {
		// A cut inside a multi-byte rune is not content; trim it back to a
		// rune boundary rather than hand the model a broken character.
		for i := 0; i < utf8.UTFMax && len(data) > 0 && !utf8.Valid(data); i++ {
			data = data[:len(data)-1]
		}
		obs.Coverage = coverageSpan
	} else {
		obs.Coverage = coverageWhole
	}
	obs.Body = string(data)
	obs.ReturnedBytes = int64(len(data))
	obs.ReturnedLines = strings.Count(obs.Body, "\n")
	if obs.Body != "" && !strings.HasSuffix(obs.Body, "\n") {
		obs.ReturnedLines++
	}
	return obs
}

// workspaceEvidenceTargets names the files an evidence turn should read:
// perception's file scope, else a target that is a path. Perception's
// suggested tools alone name no file.
func workspaceEvidenceTargets(intent perception.Intent) []string {
	if f := strings.TrimSpace(intent.ScopeFile); f != "" {
		return []string{f}
	}
	if t := strings.TrimSpace(intent.Target); t != "" && !strings.EqualFold(t, "none") && looksLikePath(t) {
		return []string{t}
	}
	return nil
}

// gatherWorkspaceEvidence is the /gather_evidence lane
// (policy/routing_arbitration.mg): the turn needs the workspace to be answered
// and no shard was available to hand it to. The read is the same read-only,
// workspace-contained one the residual path does, under no new authority. Every
// outcome is typed: an observation that holds the file, holds a prefix, or says
// why nothing was obtained. A turn that names no file gets that last one, so
// the answering call is told it has no evidence rather than left to answer from
// prose as though it had looked. Files already observed this turn are not read
// twice.
func gatherWorkspaceEvidence(workspace string, intent perception.Intent, have []evidenceObservation) []evidenceObservation {
	seen := map[string]struct{}{}
	for _, o := range have {
		seen[o.Path] = struct{}{}
	}
	targets := workspaceEvidenceTargets(intent)
	if len(targets) == 0 {
		suggested := strings.Join(intent.ToolsNeeded, ", ")
		diag := "the request needs the workspace, but names no file to read"
		if suggested != "" {
			diag += " (perception suggested: " + suggested + ")"
		}
		return []evidenceObservation{{Coverage: coverageUnavailable, Diagnostic: diag}}
	}
	var out []evidenceObservation
	for _, t := range targets {
		if _, dup := seen[t]; dup {
			continue
		}
		out = append(out, readEvidenceFile(workspace, t, evidenceReadBudget()))
	}
	return out
}

// renderEvidence is the answering call's evidence section: each observation's
// body or its typed absence, with the coverage stated so a prefix is never
// taken for the file. Empty input renders nothing.
func renderEvidence(obs []evidenceObservation) string {
	if len(obs) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Workspace Evidence (read by the host for this turn)\n")
	sb.WriteString("(Answer only from what is shown. Where coverage is partial or unavailable, say so and do not describe unseen content.)\n\n")
	for _, o := range obs {
		switch o.Coverage {
		case coverageWhole:
			sb.WriteString(fmt.Sprintf("### %s - whole file, %d bytes, %d lines\n", o.Path, o.SourceBytes, o.ReturnedLines))
			sb.WriteString(fencedBody(o.Body))
		case coverageSpan:
			sb.WriteString(fmt.Sprintf("### %s - PARTIAL: the first %d of %d bytes (%d lines) are shown; the rest was NOT read\n", o.Path, o.ReturnedBytes, o.SourceBytes, o.ReturnedLines))
			sb.WriteString(fencedBody(o.Body))
			sb.WriteString("The user can read the rest with `/read` and a line range.\n")
		default:
			name := o.Path
			if name == "" {
				name = "(no file)"
			}
			sb.WriteString(fmt.Sprintf("### %s - UNAVAILABLE: %s. No contents were obtained; do not claim to have read it.\n", name, o.Diagnostic))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// fencedBody wraps a body in a Markdown fence longer than any backtick run
// inside it, so the file cannot close its own fence.
func fencedBody(body string) string {
	longest, run := 0, 0
	for _, r := range body {
		if r == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return fence + "\n" + body + fence + "\n"
}
