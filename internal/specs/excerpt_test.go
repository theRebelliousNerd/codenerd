package specs

import "testing"

func TestSectionTextIsTheSectionsOwnProse(t *testing.T) {
	src := "# Guide\nIntro.\n\n## Install\nRun   the\ninstaller.\n\n```sh\nmake install\n```\n\nThen restart.\n\n### Linux\nUse apt.\n\nSetext\n------\nSetext body.\n"
	for _, c := range []struct {
		key, want string
		ok        bool
	}{
		{"Guide", "Intro.", true},
		// whitespace folds, the fenced example is not prose, nested sections are not included
		{"Guide > Install", "Run the installer. Then restart.", true},
		{"Guide > Install > Linux", "Use apt.", true},
		// a setext heading is two lines, and its underline is not prose
		{"Guide > Setext", "Setext body.", true},
		{"Guide > Missing", "", false},
	} {
		got, ok := SectionText("Docs/g.md", src, c.key)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("SectionText(%q) = %q, %v; want %q, %v", c.key, got, ok, c.want, c.ok)
		}
	}
}
