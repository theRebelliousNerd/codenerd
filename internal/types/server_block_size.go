package types

import (
	"encoding/json"
	"unicode/utf8"
)

// opaqueEncodedKeys are the fields of an Anthropic server-tool block whose
// string value is an opaque encoded payload, not prose: a web search result's
// encrypted_content and a citation's encrypted_index.
var opaqueEncodedKeys = map[string]bool{
	"encrypted_content": true,
	"encrypted_index":   true,
}

// ServerBlockChars is the size, in characters, that replaying one server-tool
// block (BlockServerTool) puts in a request. It is the one definition both the
// broker's request measure (characters) and the session's request estimate
// (tokens, over characters) use, so the two cannot disagree about a block they
// both have to count.
//
// The block's JSON is counted as the text it is -- URLs, titles, fetched page
// text, framing. The exception is an opaque encoded payload: an encrypted
// search result and a base64 document body (a source of type "base64"). Their
// encoded length is no measure of what the provider decodes them to, and
// charging base64 or ciphertext characters as prose overstates a block by the
// encoding's expansion. Each is charged at the length of what it can decode to
// (three bytes for every four encoded characters): an upper bound on the
// plaintext, so a block is never read as smaller than the content it stands
// for. It is a bound, not a provider count, and the callers that use it say so.
//
// Raw that is not valid JSON cannot be classified; it is charged whole, at its
// full length, rather than as nothing.
func ServerBlockChars(raw []byte) int {
	total := utf8.RuneCount(raw)
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return total
	}
	encoded := opaqueEncodedChars(doc)
	if encoded > total {
		return total
	}
	return total - encoded + encoded*3/4
}

// opaqueEncodedChars is the combined length of the opaque encoded payloads in
// one decoded JSON value.
func opaqueEncodedChars(v any) int {
	switch node := v.(type) {
	case []any:
		n := 0
		for _, item := range node {
			n += opaqueEncodedChars(item)
		}
		return n
	case map[string]any:
		n := 0
		base64Source := node["type"] == "base64"
		for key, item := range node {
			if text, ok := item.(string); ok {
				if opaqueEncodedKeys[key] || (base64Source && key == "data") {
					n += utf8.RuneCountInString(text)
				}
				continue
			}
			n += opaqueEncodedChars(item)
		}
		return n
	default:
		return 0
	}
}
