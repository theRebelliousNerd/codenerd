package northstar

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// page is a lossless UTF-8 slice. End is an exclusive byte offset.
type page struct {
	Start int
	End   int
	Body  string
}

// nextFrame reserves space for the entire carry-forward summary and framing
// before taking document bytes. An oversized summary is an explicit failure,
// not an excuse to discard context or silently exceed the request bound.
func nextFrame(kind, path, body string, start int, summary string, budget int) (page, string, error) {
	if budget <= 0 || start < 0 || start > len(body) {
		return page{}, "", fmt.Errorf("invalid page offset %d or request budget %d", start, budget)
	}
	if !utf8.ValidString(summary) || (start < len(body) && !utf8.RuneStart(body[start])) {
		return page{}, "", fmt.Errorf("%s at byte %d is not UTF-8; refusing lossy model transport", path, start)
	}
	var header strings.Builder
	// Using the whole remaining length reserves at least as many header
	// digits as the actual page; false also reserves the longer final flag.
	frameDocument(&header, kind, path, start, len(body), false, summary, "")
	allowance := budget - header.Len() - (len(fmt.Sprint(len(body)-start)) - 1)
	if allowance < 0 || (allowance == 0 && start < len(body)) {
		return page{}, "", fmt.Errorf("%s at byte %d: framing and %d summary bytes leave no document space in the %d-byte request; increase orient.derive_request_bytes", path, start, len(summary), budget)
	}
	end := len(body)
	if allowance < len(body)-start {
		end = start + allowance
		for end > start && !utf8.RuneStart(body[end]) {
			end--
		}
	}
	if end == start && start < len(body) {
		return page{}, "", fmt.Errorf("%s at byte %d: the next UTF-8 rune cannot fit in the %d-byte request; increase orient.derive_request_bytes", path, start, budget)
	}
	pg := page{Start: start, End: end, Body: body[start:end]}
	var frame strings.Builder
	frameDocument(&frame, kind, path, pg.Start, pg.End, pg.End == len(body), summary, pg.Body)
	if frame.Len() > budget {
		return page{}, "", fmt.Errorf("%s frame is %d bytes, above the %d-byte request bound", path, frame.Len(), budget)
	}
	return pg, frame.String(), nil
}

// frameDocument writes one length-prefixed frame. The summary is exactly
// len(summary) bytes and the body is exactly len(body) bytes; the trailing
// newline is a separator and is not part of either.
//
// kind is "doc", "page", or "brief". A paged document uses "page" so a
// reader can tell a prefix from a whole document. Paths are written as
// given: the model is asked to echo them byte for byte.
func frameDocument(buf *strings.Builder, kind, path string, start, end int, final bool, summary, body string) {
	switch kind {
	case "page":
		fmt.Fprintf(buf, "@@PAGE %s\n", path)
	case "brief":
		buf.WriteString("@@BRIEF\n")
	default:
		fmt.Fprintf(buf, "@@DOC %s\n", path)
	}
	fmt.Fprintf(buf, "byte_start: %d\n", start)
	fmt.Fprintf(buf, "byte_end: %d\n", end)
	fmt.Fprintf(buf, "byte_length: %d\n", len(body))
	if final {
		buf.WriteString("final_page: true\n")
	} else {
		buf.WriteString("final_page: false\n")
	}
	fmt.Fprintf(buf, "running_summary_length: %d\n", len(summary))
	buf.WriteString(summary)
	buf.WriteString(body)
	buf.WriteByte('\n')
}
