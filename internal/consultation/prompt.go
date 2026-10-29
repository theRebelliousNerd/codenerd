package consultation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// keySchemaVersion and promptRevision are part of every content key: a change
// to either the key layout or the rendered prompt makes old and new
// computations different questions.
const (
	keySchemaVersion = "consultation-key/v1"
	promptRevision   = "consultation-prompt/v2"
)

// defaultResponseContract is the answer format when the caller names none.
const defaultResponseContract = `Please provide your expert advice. Structure your response as:

ADVICE:
[Your main advice and guidance]

CONFIDENCE: [0-100]

REFERENCES:
[Any relevant references, patterns, or sources]

CAVEATS:
[Important limitations or edge cases to consider]`

// contentKey is the full SHA-256 over a length-delimited encoding of every
// input that shapes the specialist's prompt. Request IDs are administrative
// and excluded; the requester is rendered into the prompt and included.
func contentKey(req Request) string {
	h := sha256.New()
	var n [8]byte
	for _, field := range []string{
		keySchemaVersion, promptRevision,
		req.Requester, req.Specialist, req.Question, req.Context, req.ResponseContract,
	} {
		binary.BigEndian.PutUint64(n[:], uint64(len(field)))
		_, _ = h.Write(n[:])
		_, _ = h.Write([]byte(field))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// buildPrompt renders the consultation task the specialist receives.
func buildPrompt(req Request) string {
	var sb strings.Builder
	sb.WriteString("CONSULTATION REQUEST\n\n")
	sb.WriteString(fmt.Sprintf("From: %s\n", req.Requester))
	sb.WriteString(fmt.Sprintf("Question: %s\n\n", req.Question))
	if req.Context != "" {
		sb.WriteString(fmt.Sprintf("Context:\n%s\n\n", req.Context))
	}
	if req.ResponseContract != "" {
		sb.WriteString(req.ResponseContract)
	} else {
		sb.WriteString(defaultResponseContract)
	}
	return sb.String()
}

// parseArtifact reads the reply into an Artifact. The raw text is always kept,
// so a caller with its own contract parses that, not this reading of it.
func parseArtifact(key, specialist, result string, duration time.Duration) *Artifact {
	a := &Artifact{
		ContentKey:      key,
		Specialist:      specialist,
		Raw:             result,
		Confidence:      0.7, // default when the reply names none
		ParseStatus:     ParseUnstructured,
		ComputeDuration: duration,
		CreatedAt:       time.Now(),
	}

	var section string
	var content strings.Builder
	flush := func() {
		if section != "" {
			applySection(a, section, content.String())
		}
		section = ""
		content.Reset()
	}
	for _, line := range strings.Split(result, "\n") {
		trimmed := strings.TrimSpace(line)
		if rest, ok := cutField(trimmed, "ADVICE:"); ok {
			flush()
			a.ParseStatus = ParseStructured
			section = "advice"
			content.WriteString(rest)
			continue
		}
		if rest, ok := cutField(trimmed, "CONFIDENCE:"); ok {
			// CONFIDENCE is a one-line field. It used to open a section that
			// swallowed every following line, so a reply continuing with
			// REASONING never parsed its confidence at all.
			flush()
			if conf, ok := parseConfidence(rest); ok {
				a.Confidence = conf
			}
			continue
		}
		if _, ok := cutField(trimmed, "REFERENCES:"); ok {
			flush()
			section = "references"
			continue
		}
		if _, ok := cutField(trimmed, "CAVEATS:"); ok {
			flush()
			section = "caveats"
			continue
		}
		if section != "" && trimmed != "" {
			if content.Len() > 0 {
				content.WriteString("\n")
			}
			content.WriteString(trimmed)
		}
	}
	flush()
	if a.Advice == "" {
		a.Advice = result
	}
	return a
}

// cutField matches a field label case-insensitively and returns the rest of
// the line.
func cutField(line, label string) (string, bool) {
	if len(line) < len(label) || !strings.EqualFold(line[:len(label)], label) {
		return "", false
	}
	return strings.TrimSpace(line[len(label):]), true
}

func applySection(a *Artifact, section, content string) {
	content = strings.TrimSpace(content)
	switch section {
	case "advice":
		a.Advice = content
	case "references":
		if content != "" {
			a.References = strings.Split(content, "\n")
		}
	case "caveats":
		if content != "" {
			a.Caveats = strings.Split(content, "\n")
		}
	}
}

// parseConfidence reads a confidence onto the 0-1 scale. The prompt asks for
// 0-100, but models also write ratios ("0.85") and out-of-range values; the
// first must not parse as zero and the second must not escape the scale.
func parseConfidence(content string) (float64, bool) {
	content = strings.TrimSuffix(strings.TrimSpace(content), "%")
	f, err := strconv.ParseFloat(strings.TrimSpace(content), 64)
	if err != nil {
		return 0, false
	}
	if f > 1 {
		f /= 100
	}
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return f, true
}
