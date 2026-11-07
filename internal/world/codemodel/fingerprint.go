package codemodel

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Fingerprint identifies what a piece of text says, independent of how it is
// laid out. A reindent, a reflow, trailing spaces, tabs for spaces, line
// endings, and the spaces a formatter adds around operators, commas and
// braces do not move it; any change to a token does. Revision answers a
// different question (is this the exact text an edit was planned against), so
// a whitespace-only edit moves Revision and leaves Fingerprint alone.
//
// Whitespace survives as one space only between two word characters (letters,
// digits, underscore), where dropping it would join two tokens: `return a`
// stays apart from `returna`, `x:=1` is `x := 1`. In Python the line structure
// and its indentation are the block structure, so there each line's leading
// whitespace and the line break survive, and only blank lines, trailing
// whitespace and the spaces inside a line fold.
func Fingerprint(lang, text string) string {
	h := sha256.New()
	if lang == LangPython {
		foldLines(h, Normalize(text))
	} else {
		j := joiner{h: h}
		start := -1
		for i, r := range text {
			if unicode.IsSpace(r) {
				if start >= 0 {
					j.token(text[start:i])
					start = -1
				}
			} else if start < 0 {
				start = i
			}
		}
		if start >= 0 {
			j.token(text[start:])
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// joiner writes whitespace-separated tokens, keeping a space only where the
// previous token ends and the next begins with a word character.
type joiner struct {
	h        hash.Hash
	wrote    bool
	prevWord bool
}

func (j *joiner) token(tok string) {
	first, _ := utf8.DecodeRuneInString(tok)
	last, _ := utf8.DecodeLastRuneInString(tok)
	if j.wrote && j.prevWord && isWordRune(first) {
		j.h.Write([]byte{' '})
	}
	j.h.Write([]byte(tok))
	j.wrote, j.prevWord = true, isWordRune(last)
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

func foldLines(h hash.Hash, text string) {
	for _, line := range strings.Split(text, "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}
		h.Write([]byte(line[:len(line)-len(strings.TrimLeft(line, " \t"))]))
		j := joiner{h: h}
		for _, w := range words {
			j.token(w)
		}
		h.Write([]byte{'\n'})
	}
}
