package codedom

import (
	"strings"
	"testing"

	"codenerd/internal/tools"
)

const installGuide = `# Tool

Overview paragraph.

## Install

Pick a platform.

### Linux

` + "```" + `bash
# download
curl -LO https://example.invalid/tool.tar.gz
` + "```" + `

### macOS

Use the package manager.

## Usage

Run ` + "`tool --help`" + `.

| Flag | Meaning |
|------|---------|
| -v   | verbose |
`

// The coder edits docs through the element verbs, the same path it edits Go.

func TestMarkdown_GetElementsListsSectionsCodeBlocksAndTables(t *testing.T) {
	reg, _, rel := editFixture(t, "GUIDE.md", installGuide)
	out, _, err := execTool(reg, "get_elements", map[string]any{"path": rel})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"(markdown,", "parses",
		"GUIDE.md:Tool", "GUIDE.md:Tool > Install", "GUIDE.md:Tool > Install > Linux",
		"GUIDE.md:Tool > Install > Linux > code#1", "GUIDE.md:Tool > Usage > table#1",
		"section", "code_block", "table", "## Install", "// Pick a platform.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("get_elements missing %q:\n%s", want, out)
		}
	}
	filtered, _, err := execTool(reg, "get_elements", map[string]any{"path": rel, "kind": "table"})
	if err != nil || strings.Count(filtered, "\n") > 4 || !strings.Contains(filtered, "table#1") || strings.Contains(filtered, "Install") {
		t.Fatalf("kind filter: %v\n%s", err, filtered)
	}
}

func TestMarkdown_GetElementByHeadingPath(t *testing.T) {
	reg, _, rel := editFixture(t, "GUIDE.md", installGuide)
	for _, args := range []map[string]any{
		{"path": rel, "ref": "Tool > Install"},
		{"path": rel, "ref": "Install"},
		{"ref": "GUIDE.md:Tool > Install"},
		{"path": rel, "ref": "tool>install"},
	} {
		out, _, err := execTool(reg, "get_element", args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out, "GUIDE.md:Tool > Install  section") || !strings.Contains(out, "### macOS") || !strings.Contains(out, "rev ") {
			t.Fatalf("%v:\n%s", args, out)
		}
	}
	out, _, err := execTool(reg, "get_element", map[string]any{"path": rel, "ref": "Tool > Install > Linux > code#1"})
	if err != nil || !strings.Contains(out, "# download") || strings.Contains(out, "macOS") {
		t.Fatalf("code block: %v\n%s", err, out)
	}
	// A bare name two sections share is a search, listed with every ref.
	dup := "# A\n\n## Notes\n\none\n\n# B\n\n## Notes\n\ntwo\n"
	reg2, _, rel2 := editFixture(t, "dup.md", dup)
	if _, _, err := execTool(reg2, "get_element", map[string]any{"path": rel2, "ref": "Notes"}); err == nil || !strings.Contains(err.Error(), "dup.md:A > Notes") || !strings.Contains(err.Error(), "dup.md:B > Notes") {
		t.Fatalf("ambiguous bare name: %v", err)
	}
}

func TestMarkdown_HeadingTextWithGoFileNameDoesNotMisparseTheRef(t *testing.T) {
	reg, _, _ := editFixture(t, "notes.md", "# Edit main.go: the entry point\n\nbody\n\n## v1.2 notes\n\nmore\n")
	for _, ref := range []string{"notes.md:Edit main.go: the entry point", "notes.md:Edit main.go: the entry point > v1.2 notes"} {
		if _, _, err := execTool(reg, "get_element", map[string]any{"ref": ref}); err != nil {
			t.Fatalf("%q: %v", ref, err)
		}
	}
}

func TestMarkdown_EditElementChangesOneSectionAndRecordsIt(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	before := fileText(t, abs)
	out, edits, err := execTool(reg, "edit_element", map[string]any{
		"path": rel, "ref": "Tool > Install > macOS", "old": "Use the package manager.", "new": "Use Homebrew: `brew install tool`.",
	})
	if err != nil {
		t.Fatal(err)
	}
	after := fileText(t, abs)
	want := strings.Replace(before, "Use the package manager.", "Use Homebrew: `brew install tool`.", 1)
	if after != want {
		t.Fatalf("only the anchored text may change:\n%s", after)
	}
	if !strings.Contains(out, "edit_element: GUIDE.md written") || !strings.Contains(out, "GUIDE.md:Tool > Install > macOS  section") || !strings.Contains(out, "Homebrew") {
		t.Fatalf("answer:\n%s", out)
	}
	if strings.Contains(out, "Tool > Usage") || strings.Contains(out, "overview") {
		t.Fatalf("the answer must show the edited section, not its enclosing ones:\n%s", out)
	}
	if len(edits) != 1 || edits[0].Language != "markdown" || edits[0].Kind != "section" || edits[0].Key != "Tool > Install > macOS" || edits[0].File != "GUIDE.md" {
		t.Fatalf("recorded edits: %+v", edits)
	}
}

func TestMarkdown_EditElementAnchorMustBeUniqueWithinTheSection(t *testing.T) {
	doc := "# A\n\n## One\n\nshared line\n\n## Two\n\nshared line\n"
	reg, abs, rel := editFixture(t, "twice.md", doc)
	// "shared line" occurs once in each section: a section ref scopes the anchor.
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "A > Two", "old": "shared line", "new": "changed"}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs); got != "# A\n\n## One\n\nshared line\n\n## Two\n\nchanged\n" {
		t.Fatalf("got %q", got)
	}
	// Anchored at the parent, a text both sections hold is refused with its lines.
	_, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "A", "old": "## ", "new": "### "})
	if err == nil || !strings.Contains(err.Error(), "occurs") {
		t.Fatalf("ambiguous anchor: %v", err)
	}
}

func TestMarkdown_StaleRefNamesTheAvailableSections(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	before := fileText(t, abs)
	for _, tool := range []struct {
		name string
		args map[string]any
	}{
		{"get_element", map[string]any{"path": rel, "ref": "Tool > Setup"}},
		{"edit_element", map[string]any{"path": rel, "ref": "Tool > Setup", "old": "x", "new": "y"}},
		{"replace_element", map[string]any{"path": rel, "ref": "Tool > Setup", "source": "## Setup\n\nx"}},
		{"delete_element", map[string]any{"path": rel, "ref": "Tool > Setup"}},
		{"insert_element", map[string]any{"path": rel, "anchor": "Tool > Setup", "source": "## New\n\nx"}},
	} {
		_, _, err := execTool(reg, tool.name, tool.args)
		if err == nil {
			t.Fatalf("%s: a stale ref must fail", tool.name)
		}
		msg := err.Error()
		for _, want := range []string{`no section "Tool > Setup"`, "Tool (lines 1-", "Tool > Install (lines 5-", "Tool > Install > Linux (lines", "Tool > Install > macOS", "Tool > Usage"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: error does not name the available sections (missing %q):\n%s", tool.name, want, msg)
			}
		}
	}
	if fileText(t, abs) != before {
		t.Fatal("a refused edit wrote the file")
	}
}

func TestMarkdown_RevisionPreconditionRefusesAStaleSection(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	out, _, err := execTool(reg, "get_element", map[string]any{"path": rel, "ref": "Tool > Usage"})
	if err != nil {
		t.Fatal(err)
	}
	rev := out[strings.Index(out, "rev ")+4:]
	rev = rev[:12]
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "Tool > Usage", "old": "Run", "new": "Execute", "revision": rev}); err != nil {
		t.Fatal(err)
	}
	// The section changed: the old rev is refused, and the answer is the section as it is now.
	_, _, err = execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "Tool > Usage", "old": "Execute", "new": "Launch", "revision": rev})
	if err == nil || !strings.Contains(err.Error(), "precondition failed") || !strings.Contains(err.Error(), "Execute") {
		t.Fatalf("stale revision: %v", err)
	}
	if !strings.Contains(fileText(t, abs), "Execute `tool --help`") {
		t.Fatal("the refused edit changed the file")
	}
}

func TestMarkdown_ReplaceElementReplacesTheWholeSectionWithItsSubsections(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	out, _, err := execTool(reg, "replace_element", map[string]any{"path": rel, "ref": "Tool > Install", "source": "## Install\n\nDownload a release and put it on PATH.\n"})
	if err != nil {
		t.Fatal(err)
	}
	got := fileText(t, abs)
	want := "# Tool\n\nOverview paragraph.\n\n## Install\n\nDownload a release and put it on PATH.\n\n## Usage\n"
	if !strings.HasPrefix(got, want) || strings.Contains(got, "macOS") || strings.Contains(got, "curl") {
		t.Fatalf("got:\n%s", got)
	}
	if !strings.Contains(out, "GUIDE.md:Tool > Install  section") || !strings.Contains(out, "removed Tool > Install > Linux") {
		t.Fatalf("answer:\n%s", out)
	}
	// A body without its heading would merge the section into the one above.
	_, _, err = execTool(reg, "replace_element", map[string]any{"path": rel, "ref": "Tool > Usage", "source": "just text"})
	if err == nil || !strings.Contains(err.Error(), "start with a heading") || !strings.Contains(err.Error(), "edit_element") {
		t.Fatalf("headless replacement: %v", err)
	}
	// A heading promoted past its siblings reshapes them, so it is refused.
	_, _, err = execTool(reg, "replace_element", map[string]any{"path": rel, "ref": "Tool > Install", "source": "# Install\n\nbody"})
	if err == nil || !strings.Contains(err.Error(), "refusing the edit") {
		t.Fatalf("promoted heading: %v", err)
	}
}

func TestMarkdown_InsertElementSiblingChildAndRelevel(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)

	// After a section: after its nested sections, at its own level, however the source spelt it.
	out, _, err := execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Install", "source": "# Windows\n\nUse the installer."})
	if err != nil {
		t.Fatal(err)
	}
	got := fileText(t, abs)
	if !strings.Contains(got, "Use the package manager.\n\n## Windows\n\nUse the installer.\n\n## Usage") {
		t.Fatalf("sibling not placed after Install's subsections, at level 2:\n%s", got)
	}
	if !strings.Contains(out, "moved +1 level(s) so its top heading is level 2, a sibling of GUIDE.md:Tool > Install") || !strings.Contains(out, "GUIDE.md:Tool > Windows  section") {
		t.Fatalf("answer:\n%s", out)
	}

	// Before a section.
	if _, _, err = execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Usage", "position": "before", "source": "## Configure\n\nEdit the file."}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs); !strings.Contains(got, "Use the installer.\n\n## Configure\n\nEdit the file.\n\n## Usage") {
		t.Fatalf("before:\n%s", got)
	}

	// Inside a section: its last subsection, one level down, nested headings kept relative.
	out, _, err = execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Install", "position": "inside", "source": "## Source build\n\n### Go\n\n```sh\ngo build\n```"})
	if err != nil {
		t.Fatal(err)
	}
	got = fileText(t, abs)
	if !strings.Contains(got, "Use the package manager.\n\n### Source build\n\n#### Go\n\n```sh\ngo build\n```\n\n## Windows") {
		t.Fatalf("inside:\n%s", got)
	}
	if !strings.Contains(out, "a child of") || !strings.Contains(out, "Tool > Install > Source build > Go") {
		t.Fatalf("answer:\n%s", out)
	}

	// Section anchors take sections: text goes in with edit_element.
	_, _, err = execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Install", "source": "plain text"})
	if err == nil || !strings.Contains(err.Error(), "must start with a heading") {
		t.Fatalf("headless source: %v", err)
	}
	// A shift past level 6 is refused, not clamped.
	_, _, err = execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Install > Source build > Go", "position": "inside", "source": "# A\n\n###### Deep"})
	if err == nil || !strings.Contains(err.Error(), "levels 1 to 6") {
		t.Fatalf("level overflow: %v", err)
	}
	// inside is for Markdown sections.
	reg2, _, _ := editFixture(t, "demo.go", twoMethods)
	if _, _, err = execTool(reg2, "insert_element", map[string]any{"path": "demo.go", "anchor": "T.M", "position": "inside", "source": "func X() {}"}); err == nil || !strings.Contains(err.Error(), "Markdown") {
		t.Fatalf("inside on Go: %v", err)
	}
}

func TestMarkdown_InsertElementTextNextToACodeBlockAndAtTheEnd(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	out, _, err := execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Install > Linux > code#1", "source": "Then verify with `tool --version`."})
	if err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs); !strings.Contains(got, "tool.tar.gz\n```\n\nThen verify with `tool --version`.\n\n### macOS") {
		t.Fatalf("text after the code block:\n%s", got)
	}
	if !strings.Contains(out, "inserted into") || !strings.Contains(out, "Then verify") {
		t.Fatalf("answer must show where the text landed:\n%s", out)
	}

	// The end of a document that has no trailing newline keeps having none.
	reg2, abs2, rel2 := editFixture(t, "bare.md", "# A\n\ntext")
	if _, _, err = execTool(reg2, "insert_element", map[string]any{"path": rel2, "anchor": "end", "source": "## B\n\nmore"}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs2); got != "# A\n\ntext\n\n## B\n\nmore" {
		t.Fatalf("end insert: %q", got)
	}
}

func TestMarkdown_DeleteElementRemovesTheSectionAndItsSubsections(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", installGuide)
	out, edits, err := execTool(reg, "delete_element", map[string]any{"path": rel, "ref": "Tool > Install"})
	if err != nil {
		t.Fatal(err)
	}
	want := "# Tool\n\nOverview paragraph.\n\n## Usage\n"
	if got := fileText(t, abs); !strings.HasPrefix(got, want) || strings.Contains(got, "macOS") || strings.Contains(got, "\n\n\n") {
		t.Fatalf("got:\n%s", got)
	}
	if !strings.Contains(out, "removed Tool > Install") || !strings.Contains(out, "removed Tool > Install > Linux > code#1") {
		t.Fatalf("answer:\n%s", out)
	}
	removed := 0
	for _, e := range edits {
		if e.Removed {
			removed++
		}
	}
	if removed < 4 {
		t.Fatalf("recorded removals: %+v", edits)
	}

	// The first section of a file leaves no blank line at its top.
	reg2, abs2, rel2 := editFixture(t, "two.md", "# One\n\nfirst\n\n# Two\n\nsecond\n")
	if _, _, err = execTool(reg2, "delete_element", map[string]any{"path": rel2, "ref": "One"}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs2); got != "# Two\n\nsecond\n" {
		t.Fatalf("got %q", got)
	}
	// A duplicate heading's later twin is renumbered, not refused.
	reg3, abs3, rel3 := editFixture(t, "dup.md", "# A\n\n## Same\n\none\n\n## Same\n\ntwo\n")
	if _, _, err = execTool(reg3, "delete_element", map[string]any{"path": rel3, "ref": "A > Same"}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs3); got != "# A\n\n## Same\n\ntwo\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMarkdown_CRLFAndTrailingNewlineArePreserved(t *testing.T) {
	crlf := strings.ReplaceAll(installGuide, "\n", "\r\n")
	reg, abs, rel := editFixture(t, "GUIDE.md", crlf)
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "Tool > Usage", "old": "Run", "new": "Execute"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execTool(reg, "insert_element", map[string]any{"path": rel, "anchor": "Tool > Usage", "position": "before", "source": "## Extra\n\nline one\nline two"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execTool(reg, "delete_element", map[string]any{"path": rel, "ref": "Tool > Install > macOS"}); err != nil {
		t.Fatal(err)
	}
	got := fileText(t, abs)
	if strings.Count(got, "\n") != strings.Count(got, "\r\n") || strings.Contains(got, "\r\r") {
		t.Fatalf("mixed line endings after edits: %d LF, %d CRLF", strings.Count(got, "\n"), strings.Count(got, "\r\n"))
	}
	if !strings.HasSuffix(got, "| -v   | verbose |\r\n") || !strings.Contains(got, "## Extra\r\n\r\nline one\r\nline two\r\n\r\n## Usage") || !strings.Contains(got, "Execute `tool --help`") {
		t.Fatalf("got:\n%q", got)
	}

	// No trailing newline stays none; a trailing newline stays one.
	for _, tail := range []string{"", "\n"} {
		reg2, abs2, rel2 := editFixture(t, "t.md", "# A\n\nbody"+tail)
		if _, _, err := execTool(reg2, "edit_element", map[string]any{"path": rel2, "ref": "A", "old": "body", "new": "changed"}); err != nil {
			t.Fatal(err)
		}
		if got := fileText(t, abs2); got != "# A\n\nchanged"+tail {
			t.Fatalf("tail %q: got %q", tail, got)
		}
	}
}

func TestMarkdown_FrontMatterIsTheHeaderAndFencedHashesAreNotHeadings(t *testing.T) {
	doc := "---\ntitle: T\n# comment in yaml\n---\n\nLead.\n\n# Real\n\n```sh\n# not a section\n```\n"
	reg, abs, rel := editFixture(t, "fm.md", doc)
	out, _, err := execTool(reg, "get_elements", map[string]any{"path": rel})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "fm.md:not a section") || strings.Contains(out, "fm.md:comment in yaml") {
		t.Fatalf("a '#' line outside Markdown structure was listed as a section:\n%s", out)
	}
	if !strings.Contains(out, "fm.md:header") || !strings.Contains(out, "fm.md:Real") {
		t.Fatalf("elements:\n%s", out)
	}
	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "header", "old": "title: T", "new": "title: Renamed"}); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, abs); !strings.HasPrefix(got, "---\ntitle: Renamed\n# comment in yaml\n---\n") {
		t.Fatalf("got %q", got)
	}
}

func TestMarkdown_ToolGuidanceNamesMarkdownSections(t *testing.T) {
	for _, tool := range []*tools.Tool{GetElementsTool(), GetElementTool(), EditElementTool(), ReplaceElementTool(), InsertElementTool(), DeleteElementTool()} {
		d := tool.Description
		if !strings.Contains(d, "Markdown") || !strings.Contains(d, "section") {
			t.Errorf("%s does not tell the coder that Markdown sections are elements:\n%s", tool.Name, d)
		}
	}
	pos := InsertElementTool().Schema.Properties["position"]
	found := false
	for _, v := range pos.Enum {
		if v == "inside" {
			found = true
		}
	}
	if !found {
		t.Error("insert_element position has no inside")
	}
}

func TestMarkdown_CreateFileSendsANewDocToWriteFile(t *testing.T) {
	reg, _, _ := editFixture(t, "keep.md", "# K\n")
	_, _, err := execTool(reg, "create_file", map[string]any{"path": "new.md", "source": "# New\n"})
	if err == nil || !strings.Contains(err.Error(), "write_file") || !strings.Contains(err.Error(), "section") {
		t.Fatalf("create_file on .md: %v", err)
	}
}
