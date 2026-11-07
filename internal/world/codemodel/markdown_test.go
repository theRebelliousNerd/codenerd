package codemodel

import (
	"strings"
	"testing"
)

// guideDoc is shaped like a real project guide: front matter, a lead, nested
// headings, a setext title, fenced code whose lines start with '#', a table
// and a repeated heading.
const guideDoc = `---
title: Guide
tags: [a, b]
# not a heading: this is YAML
---

Lead paragraph before any heading.

# Guide

Intro text.

## Install

Pick a platform.

### Linux

` + "```" + `bash
# download the release
curl -LO https://example.invalid/tool.tar.gz
tar xf tool.tar.gz
` + "```" + `

### macOS

Use the package manager:

~~~sh
# brew
brew install tool
~~~

## Usage

| Flag | Meaning |
|------|---------|
| -v   | verbose |
| -q   | quiet   |

Run it.

Notes
-----

Setext second-level heading.

Release process
===============

Setext first-level heading.

<!--
# hidden heading inside a comment
-->

## Install

A second Install section.

### Linux

Different Linux text.
`

func keysOf(f *File) []string {
	keys := make([]string, len(f.Elements))
	for i := range f.Elements {
		keys[i] = f.Elements[i].Key
	}
	return keys
}

func sectionKeys(f *File) []string {
	var keys []string
	for i := range f.Elements {
		if f.Elements[i].Kind == KindSection {
			keys = append(keys, f.Elements[i].Key)
		}
	}
	return keys
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func parseMD(t *testing.T, src string) *File {
	t.Helper()
	f, ok := Parse("docs/guide.md", src)
	if !ok || f == nil {
		t.Fatal("Parse rejected a .md path")
	}
	return f
}

func TestMarkdownSections_NestedSetextFencedFrontMatterDuplicates(t *testing.T) {
	f := parseMD(t, guideDoc)
	if f.Language != LangMarkdown || !f.Parsed {
		t.Fatalf("language %q parsed %v", f.Language, f.Parsed)
	}
	got := sectionKeys(f)
	wantKeys := []string{
		"Guide",
		"Guide > Install",
		"Guide > Install > Linux",
		"Guide > Install > macOS",
		"Guide > Usage",
		"Guide > Notes",   // setext level 2: a sibling of Usage
		"Release process", // setext level 1: a new top section
		"Release process > Install",
		"Release process > Install > Linux",
	}
	if !equalStrings(got, wantKeys) {
		t.Fatalf("section keys\n got %q\nwant %q", got, wantKeys)
	}

	for _, lvl := range []struct {
		key   string
		level int
	}{{"Guide", 1}, {"Guide > Install > Linux", 3}, {"Guide > Notes", 2}, {"Release process", 1}} {
		if e := f.Element(lvl.key); e == nil || e.Level != lvl.level {
			t.Fatalf("%s level = %+v, want %d", lvl.key, e, lvl.level)
		}
	}

	// '#' lines in front matter, fenced code and an HTML comment are not headings.
	for _, k := range got {
		for _, bad := range []string{"not a heading", "download the release", "brew", "hidden heading"} {
			if strings.Contains(k, bad) {
				t.Fatalf("%q was read as a heading (%s)", bad, k)
			}
		}
	}

	hdr := f.Header()
	if hdr == nil {
		t.Fatal("front matter and lead paragraph are the header element")
	}
	if text := f.Text(hdr); !strings.HasPrefix(text, "---\ntitle: Guide") || !strings.HasSuffix(text, "Lead paragraph before any heading.") {
		t.Fatalf("header text = %q", text)
	}

	// A section runs to the next heading of the same or a higher level, its
	// subsections included, and stops before the blank lines that separate it.
	install := f.Element("Guide > Install")
	text := f.Text(install)
	if !strings.HasPrefix(text, "## Install\n") || !strings.Contains(text, "### macOS") || !strings.HasSuffix(text, "brew install tool\n~~~") {
		t.Fatalf("Install section = %q", text)
	}
	if strings.Contains(text, "## Usage") {
		t.Fatalf("Install ran into the next section: %q", text)
	}
	if install.StartLine != f.LineOf(install.Start) || install.EndLine != f.LineOf(install.End-1) {
		t.Fatalf("Install lines %d-%d do not match its offsets", install.StartLine, install.EndLine)
	}
	if install.Signature != "## Install" || install.Doc != "Pick a platform." || install.Kind != KindSection {
		t.Fatalf("Install signature %q doc %q kind %s", install.Signature, install.Doc, install.Kind)
	}

	// Setext headings: the text line through the underline.
	notes := f.Element("Guide > Notes")
	if !strings.HasPrefix(f.Text(notes), "Notes\n-----") {
		t.Fatalf("setext section starts %q", f.Text(notes))
	}
}

func TestMarkdownSections_DuplicateHeadingsAreNumberedInFileOrder(t *testing.T) {
	f := parseMD(t, "# A\n\n## Same\n\none\n\n## Same\n\ntwo\n\n## Same\n\nthree\n\n# B\n\n## Same\n")
	got := sectionKeys(f)
	want := []string{"A", "A > Same", "A > Same#2", "A > Same#3", "B", "B > Same"}
	if !equalStrings(got, want) {
		t.Fatalf("keys %q want %q", got, want)
	}
	// A child of a duplicate carries the duplicate's key.
	g := parseMD(t, "# D\n\n## Same\n\n### Kid\n\n## Same\n\n### Kid\n")
	want = []string{"D", "D > Same", "D > Same > Kid", "D > Same#2", "D > Same#2 > Kid"}
	if got := sectionKeys(g); !equalStrings(got, want) {
		t.Fatalf("keys %q want %q", got, want)
	}
}

func TestMarkdownSections_MultiLineSetextRuleAfterListAndSkippedLevels(t *testing.T) {
	src := "Long title\ncontinued here\n==========\n\n- item\n---\n\nplain\n\n---\n\n### Deep first\n\n## Then shallower\n"
	f := parseMD(t, src)
	got := sectionKeys(f)
	// The level-1 title owns both later headings: '### Deep first' skips a level
	// and '## Then shallower' pops it.
	want := []string{"Long title continued here", "Long title continued here > Deep first", "Long title continued here > Then shallower"}
	if !equalStrings(got, want) {
		t.Fatalf("keys %q want %q", got, want)
	}
	// "- item" then "---" is a rule, not a setext underline of the list item.
	if strings.Contains(strings.Join(got, "|"), "item") {
		t.Fatalf("a list item was read as a setext heading: %q", got)
	}
}

func TestMarkdownElements_CodeBlocksAndTablesBelongToTheirSection(t *testing.T) {
	f := parseMD(t, guideDoc)
	var codes, tables []string
	for _, e := range f.Elements {
		switch e.Kind {
		case KindCodeBlock:
			codes = append(codes, e.Key)
		case KindTable:
			tables = append(tables, e.Key)
		}
	}
	if want := []string{"Guide > Install > Linux > code#1", "Guide > Install > macOS > code#1"}; !equalStrings(codes, want) {
		t.Fatalf("code blocks %q want %q", codes, want)
	}
	if want := []string{"Guide > Usage > table#1"}; !equalStrings(tables, want) {
		t.Fatalf("tables %q want %q", tables, want)
	}
	c := f.Element("Guide > Install > Linux > code#1")
	if c.Name != "bash" || c.Signature != "```bash" || !strings.HasSuffix(f.Text(c), "```") || strings.Count(f.Text(c), "\n") != 4 {
		t.Fatalf("code block %q name %q sig %q", f.Text(c), c.Name, c.Signature)
	}
	tbl := f.Element("Guide > Usage > table#1")
	if got := f.Text(tbl); !strings.HasPrefix(got, "| Flag | Meaning |") || !strings.HasSuffix(got, "| -q   | quiet   |") {
		t.Fatalf("table text %q", got)
	}
	// An unterminated fence runs to the end of the file.
	u := parseMD(t, "# T\n\n```\n# not a heading\nstill code\n")
	if got := sectionKeys(u); !equalStrings(got, []string{"T"}) {
		t.Fatalf("unterminated fence leaked headings: %q", got)
	}
	if b := u.Element("T > code#1"); b == nil || b.EndLine != u.LineCount() {
		t.Fatalf("unterminated fence element %+v", b)
	}
}

func TestMarkdownRefs_EveryKeyRoundTripsAndPathsResolve(t *testing.T) {
	f := parseMD(t, guideDoc)
	for _, e := range f.Elements {
		got := f.Lookup(e.Key)
		if len(got) != 1 || got[0].Key != e.Key {
			t.Fatalf("Lookup(%q) = %d elements", e.Key, len(got))
		}
	}
	// A reparse of the same text gives the same keys and revisions.
	g := parseMD(t, guideDoc)
	if !equalStrings(keysOf(f), keysOf(g)) {
		t.Fatal("keys are not deterministic")
	}
	for i := range f.Elements {
		if f.Elements[i].Revision != g.Elements[i].Revision {
			t.Fatalf("revision of %s is not deterministic", f.Elements[i].Key)
		}
	}
	for _, q := range []struct {
		query string
		want  string
	}{
		{"Guide>Install>Linux", "Guide > Install > Linux"},     // spacing normalised
		{"guide > install > linux", "Guide > Install > Linux"}, // case, when nothing matches exactly
		{"macOS", "Guide > Install > macOS"},                   // bare heading text
		{"Install > macOS", "Guide > Install > macOS"},         // tail of the path
		{"Usage", "Guide > Usage"},
		{"Release process > Install#2", ""}, // not a key and not a heading text: no match
	} {
		got := f.Lookup(q.query)
		switch {
		case q.want == "" && len(got) != 0:
			t.Fatalf("Lookup(%q) = %v, want none", q.query, keysOfElems(got))
		case q.want != "" && (len(got) != 1 || got[0].Key != q.want):
			t.Fatalf("Lookup(%q) = %v, want %q", q.query, keysOfElems(got), q.want)
		}
	}
	// A bare duplicate name is ambiguous, never guessed.
	if got := f.Lookup("Linux"); len(got) != 2 {
		t.Fatalf("Lookup(Linux) = %v, want both Linux sections", keysOfElems(got))
	}
	// A heading with dots, colons and a '>' in it is still addressable by its key.
	d := parseMD(t, "# v1.2: What's new > really\n\nbody\n")
	if got := d.Lookup("v1.2: What's new > really"); len(got) != 1 {
		t.Fatalf("dotted heading lookup = %v", keysOfElems(got))
	}
}

func keysOfElems(es []*Element) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Key
	}
	return out
}

func TestMarkdownRevision_BodyEditsKeepOtherKeysAndRevisions(t *testing.T) {
	f := parseMD(t, guideDoc)
	usage := f.Element("Guide > Usage")
	out, err := Apply(f, Change{Start: usage.Start, End: usage.End, Text: "## Usage\n\nRewritten."}, []string{usage.Key}, nil)
	if err != nil {
		t.Fatal(err)
	}
	nf := out.File
	for _, k := range []string{"Guide > Install", "Guide > Install > Linux", "Guide > Notes", "Release process"} {
		if nf.Element(k) == nil || nf.Element(k).Revision != f.Element(k).Revision {
			t.Fatalf("%s did not keep its key and bytes", k)
		}
	}
	if nf.Element("Guide > Usage > table#1") != nil {
		t.Fatal("the table was inside Usage and should be gone with it")
	}
	if !strings.Contains(nf.Source, "## Usage\n\nRewritten.\n\nNotes\n-----") {
		t.Fatalf("blank-line separation lost:\n%s", nf.Source)
	}
}

func TestMarkdownApply_ReplaceEditInsertDelete(t *testing.T) {
	f := parseMD(t, guideDoc)

	// Replace a leaf section whole; its siblings and the rest keep their bytes.
	mac := f.Element("Guide > Install > macOS")
	out, err := Apply(f, Change{Start: mac.Start, End: mac.End, Text: "### macOS\n\nUse Homebrew."}, []string{mac.Key}, nil)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := out.File.Text(out.File.Element("Guide > Install > macOS")); got != "### macOS\n\nUse Homebrew." {
		t.Fatalf("replaced section = %q", got)
	}
	if out.File.Element("Guide > Install > macOS > code#1") != nil {
		t.Fatal("the replaced section's code block should be gone")
	}
	if len(out.Touched) == 0 || out.Touched[0].Key != "Guide > Install > macOS" {
		t.Fatalf("touched = %v", keysOfElems(out.Touched))
	}

	// An anchored text edit inside a section: enclosing sections change, siblings do not.
	lin := f.Element("Guide > Install > Linux")
	at := strings.Index(f.Source[lin.Start:lin.End], "tar xf")
	out, err = Apply(f, Change{Start: lin.Start + at, End: lin.Start + at + len("tar xf"), Text: "tar -xzf"}, []string{lin.Key}, nil)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if out.File.Element("Guide > Install > macOS").Revision != f.Element("Guide > Install > macOS").Revision {
		t.Fatal("a sibling changed")
	}
	if !strings.Contains(out.File.Source, "tar -xzf tool.tar.gz") {
		t.Fatal("edit not applied")
	}

	// Insert a sibling after a section that has subsections: after its whole subtree.
	inst := f.Element("Guide > Install")
	out, err = Apply(f, Change{Start: inst.End, End: inst.End, Text: "\n\n## Windows\n\nUse the installer."}, nil, nil)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := sectionKeys(out.File); !equalStrings(got, []string{"Guide", "Guide > Install", "Guide > Install > Linux", "Guide > Install > macOS", "Guide > Windows", "Guide > Usage", "Guide > Notes", "Release process", "Release process > Install", "Release process > Install > Linux"}) {
		t.Fatalf("keys after insert: %q", got)
	}
	if len(out.Touched) != 1 || out.Touched[0].Key != "Guide > Windows" {
		t.Fatalf("touched = %v", keysOfElems(out.Touched))
	}

	// Delete a nested section: its parents change, its siblings do not.
	out, err = Apply(f, MarkdownDeletion(f, mac), []string{mac.Key}, nil)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if out.File.Element("Guide > Install > macOS") != nil || out.File.Element("Guide > Install > Linux") == nil {
		t.Fatalf("delete removed the wrong element: %q", sectionKeys(out.File))
	}
	if strings.Contains(out.File.Source, "brew install") || strings.Contains(out.File.Source, "\n\n\n") {
		t.Fatalf("delete left debris:\n%s", out.File.Source)
	}
}

// MarkdownDeletion removes an element's whole lines and one separating blank
// line, as the delete verb does.
func MarkdownDeletion(f *File, e *Element) Change {
	start := f.LineStart(e.StartLine)
	end := f.LineStart(e.EndLine + 1)
	if e.EndLine >= f.LineCount() {
		end = len(f.Source)
	}
	if strings.HasSuffix(f.Source[:start], "\n\n") && strings.HasPrefix(f.Source[end:], "\n") {
		end++
	}
	return Change{Start: start, End: end}
}

func TestMarkdownApply_RenamedHeadingRenamesItsSubtreeAndNothingElse(t *testing.T) {
	f := parseMD(t, guideDoc)
	inst := f.Element("Guide > Install")
	out, err := Apply(f, Change{Start: inst.Start, End: inst.Start + len("## Install"), Text: "## Setup"}, []string{inst.Key}, nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if out.File.Element("Guide > Setup > Linux") == nil || out.File.Element("Guide > Setup > Linux > code#1") == nil {
		t.Fatalf("subtree not renamed: %q", keysOf(out.File))
	}
	if out.File.Element("Guide > Usage").Revision != f.Element("Guide > Usage").Revision {
		t.Fatal("Usage changed")
	}
}

func TestMarkdownApply_RefusesAChangeThatReshapesUntargetedSections(t *testing.T) {
	f := parseMD(t, guideDoc)
	// Replacing "## Install" with a level-1 heading would swallow "## Usage"
	// and everything after it into the replaced section.
	inst := f.Element("Guide > Install")
	_, err := Apply(f, Change{Start: inst.Start, End: inst.End, Text: "# Install\n\nbody"}, []string{inst.Key}, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing the edit") {
		t.Fatalf("a heading promoted past its siblings must be refused, got %v", err)
	}
	// An unterminated fence in the replacement would swallow the rest of the file.
	usage := f.Element("Guide > Usage")
	_, err = Apply(f, Change{Start: usage.Start, End: usage.End, Text: "## Usage\n\n```\nopen fence"}, []string{usage.Key}, nil)
	if err == nil {
		t.Fatal("an unterminated fence swallowing later sections must be refused")
	}
}

func TestMarkdownApply_DuplicateHeadingsRenumberWithoutRefusal(t *testing.T) {
	f := parseMD(t, "# A\n\n## Same\n\none\n\n## Same\n\ntwo\n\n## Other\n")
	first := f.Element("A > Same")
	out, err := Apply(f, MarkdownDeletion(f, first), []string{first.Key}, nil)
	if err != nil {
		t.Fatalf("deleting the first duplicate: %v", err)
	}
	if got := sectionKeys(out.File); !equalStrings(got, []string{"A", "A > Same", "A > Other"}) {
		t.Fatalf("keys %q", got)
	}
	if !strings.Contains(out.File.Text(out.File.Element("A > Same")), "two") {
		t.Fatal("the surviving duplicate must be the second one, now unnumbered")
	}

	// Inserting a new section with an existing heading above it renumbers the old one.
	out, err = Apply(f, Change{Start: f.Element("A > Same").Start, End: f.Element("A > Same").Start, Text: "## Same\n\nnew\n\n"}, nil, nil)
	if err != nil {
		t.Fatalf("inserting a duplicate: %v", err)
	}
	if got := sectionKeys(out.File); !equalStrings(got, []string{"A", "A > Same", "A > Same#2", "A > Same#3", "A > Other"}) {
		t.Fatalf("keys %q", got)
	}
	if len(out.Touched) == 0 {
		t.Fatal("an inserted section is reported as touched even when its key was already taken")
	}
}

func TestMarkdownCRLF_ModelIsLineEndingBlind(t *testing.T) {
	crlf := strings.ReplaceAll(guideDoc, "\n", "\r\n")
	a, b := parseMD(t, guideDoc), parseMD(t, crlf)
	if strings.Contains(b.Source, "\r") {
		t.Fatal("the model works on LF text")
	}
	if !equalStrings(keysOf(a), keysOf(b)) {
		t.Fatal("CRLF changed the keys")
	}
	for i := range a.Elements {
		if a.Elements[i].Revision != b.Elements[i].Revision || a.Elements[i].StartLine != b.Elements[i].StartLine {
			t.Fatalf("CRLF changed %s", a.Elements[i].Key)
		}
	}
	// A lone-CR file (classic Mac) is read the same way.
	if c := parseMD(t, strings.ReplaceAll(guideDoc, "\n", "\r")); !equalStrings(keysOf(a), keysOf(c)) {
		t.Fatal("lone CR changed the keys")
	}
}

func TestMarkdownEdgeShapes(t *testing.T) {
	if f := parseMD(t, ""); len(f.Elements) != 0 || !f.Parsed {
		t.Fatalf("empty file: %+v", f.Elements)
	}
	if f := parseMD(t, "just prose\nno headings\n"); keysOf(f)[0] != "header" || len(f.Elements) != 1 {
		t.Fatalf("headingless file: %q", keysOf(f))
	}
	// Unclosed front matter is a thematic break, not front matter.
	f := parseMD(t, "---\n# Real heading\n")
	if got := sectionKeys(f); !equalStrings(got, []string{"Real heading"}) {
		t.Fatalf("unclosed front matter: %q", got)
	}
	// Closing hashes, empty headings, #hashtag and ATX with up to three spaces.
	g := parseMD(t, "## Closed ##\n\n#hashtag is text\n\n   ### Indented three\n\n    # four spaces is code\n\n##\n")
	if got := sectionKeys(g); !equalStrings(got, []string{"Closed", "Closed > Indented three", "(untitled)"}) {
		t.Fatalf("keys %q", got)
	}
	// A file with no trailing newline keeps its last line.
	h := parseMD(t, "# T\n\nlast line")
	if got := h.Text(h.Element("T")); got != "# T\n\nlast line" {
		t.Fatalf("section text %q", got)
	}
}

func TestRelevelMarkdown(t *testing.T) {
	src := "## Windows\n\nText.\n\n### Detail\n\n```\n# not a heading\n```\n\nTitle\n-----\n"
	out, shift, err := RelevelMarkdown(src, 3)
	if err != nil || shift != 1 {
		t.Fatalf("shift %d err %v", shift, err)
	}
	want := "### Windows\n\nText.\n\n#### Detail\n\n```\n# not a heading\n```\n\n### Title\n"
	if out != want {
		t.Fatalf("got\n%q\nwant\n%q", out, want)
	}
	if out, shift, _ := RelevelMarkdown(src, 2); shift != 0 || out != src {
		t.Fatal("no shift leaves the source alone")
	}
	if out, shift, _ := RelevelMarkdown("no headings here", 4); shift != 0 || out != "no headings here" {
		t.Fatal("headingless source is untouched")
	}
	if _, _, err := RelevelMarkdown("# A\n\n###### Deep\n", 2); err == nil {
		t.Fatal("a shift past level 6 must be refused")
	}
	if levels := MarkdownHeadingLevels(src); !equalInts(levels, []int{2, 3, 2}) {
		t.Fatalf("levels %v", levels)
	}
	if !StartsWithMarkdownHeading("\n\n## A\n") || StartsWithMarkdownHeading("text\n## A\n") {
		t.Fatal("StartsWithMarkdownHeading")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMarkdownIsNotACodeLanguage(t *testing.T) {
	for _, p := range []string{"README.md", "docs/a.MD", "x.markdown"} {
		if !IsMarkdownPath(p) {
			t.Fatalf("%s should be Markdown", p)
		}
		if LanguageOf(p) != "" {
			t.Fatalf("LanguageOf(%s) = %q: the structure index must not walk Markdown as code", p, LanguageOf(p))
		}
		if _, ok := Parse(p, "# x\n"); !ok {
			t.Fatalf("Parse(%s) refused", p)
		}
	}
	if IsMarkdownPath("notes.mdx") || IsMarkdownPath("dir.md/file.go") {
		t.Fatal("only .md and .markdown files")
	}
}
