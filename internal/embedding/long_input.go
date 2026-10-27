package embedding

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// InputTooLongError is a text longer than the model's input window. The same
// text fails the same way every time, so it is never retried as is.
type InputTooLongError struct {
	Chars int
	Model string
}

func (e *InputTooLongError) Error() string {
	return fmt.Sprintf("embedding input of %d characters exceeds the context length of model %q", e.Chars, e.Model)
}

// inputCeiling is what an engine has learned about its model's input window:
// the length of the shortest text the model refused. Texts at least that long
// are split before they are sent, so a known-too-long request is never paid
// for twice. Zero means nothing has been refused yet.
type inputCeiling struct{ refused atomic.Int64 }

func (c *inputCeiling) note(chars int) {
	for {
		cur := c.refused.Load()
		if cur != 0 && cur <= int64(chars) {
			return
		}
		if c.refused.CompareAndSwap(cur, int64(chars)) {
			return
		}
	}
}

func (c *inputCeiling) tooLong(chars int) bool {
	cur := c.refused.Load()
	return cur != 0 && int64(chars) >= cur
}

// minSplitChars stops the split when a piece this short is still refused: the
// model's window is then not the problem, and splitting further would only
// multiply the failure.
const minSplitChars = 64

// embedLong embeds text of any length with a model whose window is smaller.
// A text the model refuses is split at the most natural boundary near its
// middle (paragraph, then line, sentence, clause, word, then character) and
// each half is embedded the same way. The halves combine into one vector: the
// mean of their vectors weighted by length, normalized, which is what a single
// embedding of the whole text approximates. The learned ceiling splits a text
// up front once a shorter one has been refused.
func embedLong(ctx context.Context, text string, ceiling *inputCeiling, once func(context.Context, string) ([]float32, error)) ([]float32, error) {
	chars := utf8.RuneCountInString(text)
	small := chars < 2*minSplitChars
	if small || !ceiling.tooLong(chars) {
		vec, err := once(ctx, text)
		var tooLong *InputTooLongError
		if !errors.As(err, &tooLong) {
			return vec, err
		}
		ceiling.note(chars)
		if small {
			return nil, fmt.Errorf("%w, and so is a %d-character piece of it: splitting cannot help, configure a model with a longer context", err, chars)
		}
	}
	left, right := splitForEmbedding(text)
	lv, err := embedLong(ctx, left, ceiling, once)
	if err != nil {
		return nil, err
	}
	rv, err := embedLong(ctx, right, ceiling, once)
	if err != nil {
		return nil, err
	}
	return poolVectors(lv, utf8.RuneCountInString(left), rv, utf8.RuneCountInString(right))
}

// splitBoundaries are tried in order; the first one found inside the middle
// half of the text wins, so a piece keeps whole paragraphs, lines or
// sentences whenever the text has them.
var splitBoundaries = []string{"\n\n", "\n", ". ", "? ", "! ", "; ", ", ", " "}

// splitForEmbedding cuts text in two near its middle at the strongest
// boundary available there. Both pieces are non-empty.
func splitForEmbedding(text string) (string, string) {
	runes := []rune(text)
	n := len(runes)
	mid := n / 2
	lo, hi := n/4, n-n/4
	for _, sep := range splitBoundaries {
		sepRunes := []rune(sep)
		best := -1
		for i := lo; i+len(sepRunes) <= hi; i++ {
			if string(runes[i:i+len(sepRunes)]) != sep {
				continue
			}
			cut := i + len(sepRunes)
			if best < 0 || absInt(cut-mid) < absInt(best-mid) {
				best = cut
			}
		}
		if best > 0 && best < n {
			left, right := strings.TrimSpace(string(runes[:best])), strings.TrimSpace(string(runes[best:]))
			if left != "" && right != "" {
				return left, right
			}
		}
	}
	return string(runes[:mid]), string(runes[mid:])
}

// poolVectors is the length-weighted mean of two vectors, scaled to unit
// length, the measure cosine similarity compares.
func poolVectors(a []float32, wa int, b []float32, wb int) ([]float32, error) {
	if len(a) != len(b) {
		return nil, fmt.Errorf("pooling embeddings of different lengths %d and %d", len(a), len(b))
	}
	total := float64(wa + wb)
	out := make([]float32, len(a))
	var norm float64
	for i := range a {
		v := (float64(a[i])*float64(wa) + float64(b[i])*float64(wb)) / total
		out[i] = float32(v)
		norm += v * v
	}
	if norm == 0 {
		return out, nil
	}
	scale := 1 / math.Sqrt(norm)
	for i := range out {
		out[i] = float32(float64(out[i]) * scale)
	}
	return out, nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
