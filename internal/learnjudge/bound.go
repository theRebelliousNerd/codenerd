package learnjudge

import "unicode/utf8"

// bounded keeps text within limit bytes for Jev's context (64k tokens a
// request, 32k a question: docs.typesafe.ai/models). It keeps the head and the
// tail, where a reply states what it did and how it ended, and returns how
// many bytes it left out so the state can say so; nothing is cut silently.
// A limit of zero or less keeps everything.
func bounded(text string, limit int) (string, int) {
	if limit <= 0 || len(text) <= limit {
		return text, 0
	}
	head := limit * 2 / 3
	tail := limit - head
	for head > 0 && !utf8.RuneStart(text[head]) {
		head--
	}
	start := len(text) - tail
	for start < len(text) && !utf8.RuneStart(text[start]) {
		start++
	}
	return text[:head] + "\n[...]\n" + text[start:], start - head
}

// putBounded sets state[key] to text within limit, and state[key+
// "_omitted_bytes"] when some was left out.
func putBounded(state map[string]any, key, text string, limit int) {
	kept, omitted := bounded(text, limit)
	state[key] = kept
	if omitted > 0 {
		state[key+"_omitted_bytes"] = omitted
	}
}
