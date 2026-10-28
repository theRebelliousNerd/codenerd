package approval

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"codenerd/internal/diff"
)

// Preview is what a person is shown, built by the host from the same frozen
// inputs the subject digests. A digest is an integrity identifier, not an
// explanation, so the preview carries the explanation: a summary, the effects
// the host can name, the effects it cannot (Unknowns), and an explicit record
// of everything it did not show (Incomplete). The UI renders it; it never
// recomputes it.
type Preview struct {
	Summary  string   `json:"summary"`
	Effects  []string `json:"effects,omitempty"`
	Unknowns []string `json:"unknowns,omitempty"`
	// Text is the operation's arguments as the gate saw them, verbatim up to
	// the preview limit.
	Text string `json:"text,omitempty"`
	// Diff is the content change, when the operation has one the host can
	// compute exactly.
	Diff *diff.FileDiff `json:"diff,omitempty"`
	// Complete is false whenever anything was truncated, binary or otherwise
	// withheld; Incomplete says what.
	Complete   bool     `json:"complete"`
	Incomplete []string `json:"incomplete,omitempty"`
	// Digest is sha256 over every field above, so the decision can be tied to
	// the exact preview that was on screen.
	Digest string `json:"digest"`
}

func (p Preview) clone() Preview {
	out := p
	out.Effects = append([]string(nil), p.Effects...)
	out.Unknowns = append([]string(nil), p.Unknowns...)
	out.Incomplete = append([]string(nil), p.Incomplete...)
	if p.Diff != nil {
		d := *p.Diff
		d.Hunks = make([]diff.Hunk, len(p.Diff.Hunks))
		for i, h := range p.Diff.Hunks {
			h.Lines = append([]diff.Line(nil), h.Lines...)
			d.Hunks[i] = h
		}
		out.Diff = &d
	}
	return out
}

// previewEngine computes decision previews. Its cache is off: once a diff
// decides an authorization, a cache hit that served another input's hunks
// would show a person one change and authorize another. Recomputing is the
// cost of an exact preview (TestDiffImporters_AreDisplayOnly records why).
var previewEngine = diff.NewEngineWith(diff.Options{DisableCache: true})

// PreviewInput is the operation-specific material a preview is built from.
type PreviewInput struct {
	Summary  string
	Effects  []string
	Unknowns []string
	// Arguments is the canonical payload the gate evaluates.
	Arguments string
	// File, when set, is a content change the host resolved exactly.
	File *FileChange
	// MaxBytes bounds Text and each side of the diff. Anything over it is
	// withheld and named in Incomplete, never silently cut.
	MaxBytes int
}

// FileChange is a file's bytes before and after the operation. A removal has
// After nil; a file that did not exist has Before nil.
type FileChange struct {
	Path   string
	Before []byte
	After  []byte
	// Exists reports whether the path existed when the request was frozen.
	Exists bool
	// IsDir reports a directory at Path; its contents are not previewed.
	IsDir bool
}

// BuildPreview renders a preview and seals it with its digest.
func BuildPreview(in PreviewInput) (Preview, error) {
	if in.MaxBytes <= 0 {
		return Preview{}, fmt.Errorf("approval preview limit must be positive")
	}
	p := Preview{
		Summary:  in.Summary,
		Effects:  append([]string(nil), in.Effects...),
		Unknowns: append([]string(nil), in.Unknowns...),
		Complete: true,
	}
	withhold := func(reason string) {
		p.Complete = false
		p.Incomplete = append(p.Incomplete, reason)
	}
	text := in.Arguments
	if len(text) > in.MaxBytes {
		cut := in.MaxBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		withhold(fmt.Sprintf("arguments truncated: %d of %d bytes shown", cut, len(text)))
		text = text[:cut]
	}
	p.Text = text
	if f := in.File; f != nil {
		switch {
		case f.IsDir:
			withhold(fmt.Sprintf("%s is a directory; its contents are not previewed", f.Path))
		case !f.Exists && f.After == nil:
			withhold(fmt.Sprintf("%s does not exist; there is no content to show", f.Path))
		case isBinary(f.Before) || isBinary(f.After):
			withhold(fmt.Sprintf("%s is binary; its bytes are not shown", f.Path))
		case len(f.Before) > in.MaxBytes || len(f.After) > in.MaxBytes:
			withhold(fmt.Sprintf("%s is %d bytes, over the %d-byte preview limit; the diff is not shown", f.Path, max(len(f.Before), len(f.After)), in.MaxBytes))
		default:
			before, after := string(f.Before), string(f.After)
			if !utf8.ValidString(before) || !utf8.ValidString(after) {
				withhold(fmt.Sprintf("%s is not valid UTF-8; its bytes are not shown", f.Path))
				break
			}
			newPath := f.Path
			if f.After == nil {
				newPath = "/dev/null"
			}
			p.Diff = previewEngine.ComputeDiff(f.Path, newPath, before, after)
		}
	}
	digest, err := p.digest()
	if err != nil {
		return Preview{}, err
	}
	p.Digest = digest
	return p, nil
}

func isBinary(b []byte) bool { return bytes.IndexByte(b, 0) >= 0 }

func (p Preview) digest() (string, error) {
	p.Digest = ""
	encoded, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("encode approval preview: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Verify reports whether the preview still carries the digest it was sealed
// with. The broker checks it on every request it accepts and every record it
// replays, so a preview edited in the journal cannot be shown as the one that
// was approved.
func (p Preview) Verify() bool {
	got, err := p.digest()
	return err == nil && got == p.Digest && p.Digest != ""
}

// String is a plain-text rendering for surfaces with no diff view (the
// headless refusal, the MCP status).
func (p Preview) String() string {
	var b strings.Builder
	b.WriteString(p.Summary)
	for _, e := range p.Effects {
		b.WriteString("\n  effect: " + e)
	}
	for _, u := range p.Unknowns {
		b.WriteString("\n  not modeled: " + u)
	}
	for _, w := range p.Incomplete {
		b.WriteString("\n  not shown: " + w)
	}
	return b.String()
}
