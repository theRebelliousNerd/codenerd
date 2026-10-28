package codedom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/tools"
)

// =============================================================================
// OBSERVATION ENVELOPE
// =============================================================================
// A structural answer is an observation of the workspace, not the workspace.
// It says which index generation it was computed from and what inside its
// domain could not be observed, so that "no row" is only read as "none" when
// the observation was complete, and so that pages of one answer are never
// stitched together from two source states
// (Docs/audits/structure-coverage-and-refactor-preconditions.md, S1, S2, S4).

// Observation is the envelope every structure provider answer carries.
type Observation struct {
	// Generation is the index state the answer was computed from. It moves
	// whenever an answer could change; it never moves backwards.
	Generation uint64
	// Index is the provider's one-line account of the index (files,
	// declarations, call sites, reparses).
	Index string
	// Deficits are the paths inside the answer's domain the provider could
	// not observe. Rows from them are as last observed.
	Deficits []ObservationDeficit
}

// ObservationDeficit is one path an observation could not see.
type ObservationDeficit struct {
	Path, Reason string
}

// Complete reports an observation with no deficit in its domain: the only
// kind whose empty answer establishes absence there.
func (o Observation) Complete() bool { return len(o.Deficits) == 0 }

// deficitsShown bounds how many deficits an answer names; the count is
// always given in full.
const deficitsShown = 8

// DeficitSummary names what the observation could not see.
func (o Observation) DeficitSummary() string {
	if o.Complete() {
		return ""
	}
	ds := append([]ObservationDeficit(nil), o.Deficits...)
	sort.Slice(ds, func(i, j int) bool { return ds[i].Path < ds[j].Path })
	parts := make([]string, 0, min(len(ds), deficitsShown))
	for _, d := range ds[:min(len(ds), deficitsShown)] {
		path := d.Path
		if path == "" || path == "." {
			path = "(workspace root)"
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", path, d.Reason))
	}
	summary := fmt.Sprintf("%d paths could not be observed: %s", len(ds), strings.Join(parts, "; "))
	if len(ds) > deficitsShown {
		summary += fmt.Sprintf("; and %d more", len(ds)-deficitsShown)
	}
	return summary
}

// structurePageSize bounds one answer. It is a page, never a cut: the footer
// says how many rows exist and which cursor continues them.
const structurePageSize = 150

// pageRequest is where one structural answer starts: the query it belongs
// to, and, when it continues an earlier page, the generation that page was
// printed from and the row to continue at.
type pageRequest struct {
	tool   string
	query  string
	gen    uint64
	offset int
}

// pageOf reads a call's continuation. A cursor names the query that printed
// it, so it cannot continue a different one.
func pageOf(tool string, args map[string]any) (pageRequest, error) {
	p := pageRequest{tool: tool, query: queryDigest(tool, args)}
	raw, _ := args["cursor"].(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return p, nil
	}
	gen, query, offset, ok := parseCursor(raw)
	if !ok {
		return p, fmt.Errorf("cursor %q is not one this tool printed; call again without cursor to start from the first row", raw)
	}
	if query != p.query {
		return p, fmt.Errorf("cursor %q continues a different query; a cursor continues only the %s call that printed it, with the same arguments", raw, tool)
	}
	p.gen, p.offset = gen, offset
	return p, nil
}

func formatCursor(gen uint64, query string, offset int) string {
	return fmt.Sprintf("g%d.%s.o%d", gen, query, offset)
}

func parseCursor(raw string) (gen uint64, query string, offset int, ok bool) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "g") || !strings.HasPrefix(parts[2], "o") {
		return 0, "", 0, false
	}
	gen, err := strconv.ParseUint(parts[0][1:], 10, 64)
	if err != nil {
		return 0, "", 0, false
	}
	offset, err = strconv.Atoi(parts[2][1:])
	if err != nil || offset < 0 {
		return 0, "", 0, false
	}
	return gen, parts[1], offset, true
}

// queryDigest identifies a call by its tool and every argument but the
// cursor, so a cursor is bound to the question that produced it.
func queryDigest(tool string, args map[string]any) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		if k != "cursor" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	h := sha256.New()
	h.Write([]byte(tool))
	for _, k := range keys {
		fmt.Fprintf(h, "\x00%s=%v", k, args[k])
	}
	return "q" + hex.EncodeToString(h.Sum(nil))[:12]
}

// render writes one page of an answer. A continuation printed from another
// generation is refused rather than served: rows inserted or removed ahead of
// its offset would be silently skipped or repeated across the pages.
func (p pageRequest) render(rows []string, header string, obs Observation, emptyHint string) (string, error) {
	if p.offset > 0 && p.gen != obs.Generation {
		return "", fmt.Errorf("stale cursor: the workspace changed since that page was printed (index generation %d, now %d), so its rows may have moved. Call %s again without cursor to restart; do not combine earlier pages with the new answer",
			p.gen, obs.Generation, p.tool)
	}
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n")
	if len(rows) == 0 {
		if obs.Complete() {
			sb.WriteString(emptyHint)
		} else {
			sb.WriteString("nothing found in what could be observed. The observation is incomplete, so this is NOT an absence; see the deficits below.")
		}
		sb.WriteString("\n" + tools.StructuralNoRows + "\n")
	}
	end := min(p.offset+structurePageSize, len(rows))
	if p.offset < len(rows) {
		for _, row := range rows[p.offset:end] {
			sb.WriteString(row)
			sb.WriteString("\n")
		}
	}
	switch {
	case end < len(rows):
		fmt.Fprintf(&sb, "-- rows %d-%d of %d; call again with the same arguments and cursor=%q for the rest\n",
			p.offset+1, end, len(rows), formatCursor(obs.Generation, p.query, end))
	case len(rows) > 0 && obs.Complete():
		fmt.Fprintf(&sb, "-- %d rows, complete\n", len(rows))
	case len(rows) > 0:
		fmt.Fprintf(&sb, "-- %d rows observed; the observation is incomplete\n", len(rows))
	}
	if !obs.Complete() {
		fmt.Fprintf(&sb, "-- incomplete observation: %s. Rows from those paths are as last observed, and nothing is established absent where they could hold rows; restore access or repair them, then ask again.\n", obs.DeficitSummary())
	}
	if obs.Index != "" {
		fmt.Fprintf(&sb, "-- index: %s\n", obs.Index)
	}
	return sb.String(), nil
}
