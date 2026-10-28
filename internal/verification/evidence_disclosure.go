package verification

import (
	"fmt"
	"strings"
)

// Progressive disclosure (Docs/audits/verification-applicability-and-
// progressive-evidence.md, V2). An expansion used to hand the judge the first
// delegation.judge_evidence_bytes of a requested file, every time: a decisive
// function past that prefix could not be reached, and a second request
// re-sent the same prefix as though it were new evidence. Each item now keeps
// how much of its retained content the judge has been shown, and a request
// discloses the next range from there. An item shown in full is no longer
// expandable, and a request for it gets a reason, not a repeat.

// unexpandable is why the host cannot supply more of a requested item, or ""
// when it can.
func (p *EvidencePacket) unexpandable(id string) string {
	it, ok := p.item(id)
	switch {
	case !ok:
		return "not in the packet"
	case !it.current():
		return "its file no longer holds what the attempt left"
	case it.full == "":
		return "the packet already shows all of it"
	case it.disclosed >= len(it.full):
		return "every byte of it has been shown"
	}
	return ""
}

// expandable reports whether a requested item has more the host can supply,
// and still describes the file as it is.
func (p *EvidencePacket) expandable(id string) bool {
	return p.unexpandable(id) == ""
}

// rangeHead introduces one disclosed range.
func (it *EvidenceItem) rangeHead(from, to int) string {
	what := "text"
	if it.path != "" {
		what = "post-change content"
	}
	return fmt.Sprintf("\n[%s] %s of %s, bytes %d-%d of %d\n```\n", it.ID, what, it.Label, from, to, len(it.full))
}

const rangeTail = "\n```\n"

func (it *EvidenceItem) remainMarker(left int) string {
	return fmt.Sprintf("(%d bytes remain; request %s again for the next range)\n", left, it.ID)
}

// frameReserve is the most a range's framing can cost: its heading at the
// widest offsets, its fence and its remaining-bytes marker.
func (it *EvidenceItem) frameReserve() int {
	const wide = 1 << 62
	return len(it.rangeHead(wide, wide)) + len(rangeTail) + len(it.remainMarker(wide))
}

const disclosureHeading = "## Requested evidence\n"

func noRoomNote(ids []string) string {
	return fmt.Sprintf("(no room this round for %s; request them again)\n", strings.Join(ids, ", "))
}

// expand discloses, for each requested item the host can supply, the next
// range of its retained content, from where the last disclosure ended. The
// whole disclosure -- heading, each range's framing, markers and content -- is
// held to the packet's budget, and the requested items share it fairly. An
// item that gets no room this round is named, so the judge can ask again.
func (p *EvidencePacket) expand(ids []string) string {
	var items []*EvidenceItem
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !p.expandable(id) {
			continue
		}
		seen[id] = true
		it, _ := p.item(id)
		items = append(items, it)
	}
	allIDs := make([]string, len(items))
	for i, it := range items {
		allIDs[i] = it.ID
	}
	room := p.Budget - len(disclosureHeading)
	if len(items) > 0 {
		room -= len(noRoomNote(allIDs))
	}
	demand := make([]int, len(items))
	for k, it := range items {
		demand[k] = it.frameReserve() + len(it.full) - it.disclosed
	}
	alloc := shareFairly(demand, max(room, 0))

	var b strings.Builder
	b.WriteString(disclosureHeading)
	var skipped []string
	for k, it := range items {
		chunk := cutAtLine(it.full[it.disclosed:], alloc[k]-it.frameReserve())
		if chunk == "" {
			skipped = append(skipped, it.ID)
			continue
		}
		from := it.disclosed
		it.disclosed += len(chunk)
		b.WriteString(it.rangeHead(from, it.disclosed))
		b.WriteString(chunk)
		b.WriteString(rangeTail)
		if left := len(it.full) - it.disclosed; left > 0 {
			b.WriteString(it.remainMarker(left))
		}
	}
	if len(skipped) > 0 {
		b.WriteString(noRoomNote(skipped))
	}
	return b.String()
}

// cutAtLine is s cut to at most n bytes, at the last line end inside the
// cut when there is one, else on a rune boundary.
func cutAtLine(s string, n int) string {
	c := cutUTF8(s, n)
	if len(c) < len(s) {
		if i := strings.LastIndexByte(c, '\n'); i >= 0 {
			c = c[:i+1]
		}
	}
	return c
}
