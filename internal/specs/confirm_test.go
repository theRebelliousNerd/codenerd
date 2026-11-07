package specs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func link(spec, code string) Link {
	return Link{SpecRef: spec, CodeRef: code, Relation: RelCites, Source: SrcDocCitation}
}

func TestAFirstSeenLinkIsConfirmedAtWhatItSaysNow(t *testing.T) {
	l := link("Docs/a.md:A", "pkg.F")
	got := Reconcile(nil, []Link{l}, map[string]string{l.SpecRef: "s1"}, map[string]string{l.CodeRef: "c1"}, 100)
	want := Confirmation{SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100}
	if got[LinkKey{l.SpecRef, l.CodeRef}] != want {
		t.Fatalf("a fresh checkout has no drift to report: %+v", got)
	}
}

func TestOneSideMovingKeepsTheConfirmationAndRemembersSince(t *testing.T) {
	l := link("Docs/a.md:A", "pkg.F")
	key := LinkKey{l.SpecRef, l.CodeRef}
	prev := Confirmations{key: {SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100}}
	sections, elements := map[string]string{l.SpecRef: "s1"}, map[string]string{l.CodeRef: "c2"}

	first := Reconcile(prev, []Link{l}, sections, elements, 200)
	if got := first[key]; got.CodeFingerprint != "c1" || got.CodeChangedAt != 200 || got.SpecChangedAt != 0 {
		t.Fatalf("code moved: the pair stays, the first sighting is stamped: %+v", got)
	}
	// A later scan, with the code moved again, keeps the first sighting.
	elements[l.CodeRef] = "c3"
	second := Reconcile(first, []Link{l}, sections, elements, 300)
	if got := second[key]; got.CodeChangedAt != 200 || got.ConfirmedAt != 100 {
		t.Fatalf("since is when it first differed, not when it last did: %+v", got)
	}
	// The code returning to what was confirmed is no change at all.
	elements[l.CodeRef] = "c1"
	back := Reconcile(second, []Link{l}, sections, elements, 400)
	if got := back[key]; got.CodeChangedAt != 0 || got != (Confirmation{SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100}) {
		t.Fatalf("reverted code: %+v", got)
	}
	// The spec moving alone is stamped on its own side.
	specMoved := Reconcile(prev, []Link{l}, map[string]string{l.SpecRef: "s2"}, map[string]string{l.CodeRef: "c1"}, 500)
	if got := specMoved[key]; got.SpecChangedAt != 500 || got.CodeChangedAt != 0 {
		t.Fatalf("spec moved: %+v", got)
	}
}

func TestBothSidesMovingConfirmsThePairAgain(t *testing.T) {
	l := link("Docs/a.md:A", "pkg.F")
	key := LinkKey{l.SpecRef, l.CodeRef}
	// The code moved in an earlier scan; now the spec has followed it.
	prev := Confirmations{key: {SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100, CodeChangedAt: 200}}
	got := Reconcile(prev, []Link{l}, map[string]string{l.SpecRef: "s2"}, map[string]string{l.CodeRef: "c2"}, 300)
	if want := (Confirmation{SpecFingerprint: "s2", CodeFingerprint: "c2", ConfirmedAt: 300}); got[key] != want {
		t.Fatalf("spec followed code: %+v", got[key])
	}
}

func TestAnEndThatIsGoneKeepsTheConfirmationAndAGoneLinkLosesIt(t *testing.T) {
	kept, dropped := link("Docs/a.md:A", "pkg.Gone"), link("Docs/a.md:A", "pkg.Unlinked")
	prev := Confirmations{
		{kept.SpecRef, kept.CodeRef}:       {SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100},
		{dropped.SpecRef, dropped.CodeRef}: {SpecFingerprint: "s1", CodeFingerprint: "c1", ConfirmedAt: 100},
	}
	// pkg.Gone has no fingerprint any more; pkg.Unlinked is no longer a link.
	got := Reconcile(prev, []Link{kept}, map[string]string{"Docs/a.md:A": "s1"}, map[string]string{}, 200)
	if len(got) != 1 || got[LinkKey{kept.SpecRef, kept.CodeRef}].CodeFingerprint != "c1" {
		t.Fatalf("a link with an end gone keeps what it was confirmed at, so the end coming back as it was is no drift: %+v", got)
	}
}

func TestStoreRoundTripsAndSharesTheFileAcrossHolders(t *testing.T) {
	root := t.TempDir()
	a, b := OpenStore(root), OpenStore(root)
	if a != b {
		t.Fatal("two holders of one workspace's store must share its lock")
	}
	got, err := a.Load()
	if err != nil || len(got) != 0 {
		t.Fatalf("no file is no confirmations: %v %v", got, err)
	}
	key := LinkKey{"Docs/a.md:A > B", "pkg.F"}
	want := Confirmation{SpecFingerprint: "s", CodeFingerprint: "c", ConfirmedAt: 5, CodeChangedAt: 9}
	if _, err := a.Update(func(Confirmations) (Confirmations, error) { return Confirmations{key: want}, nil }); err != nil {
		t.Fatal(err)
	}
	read, err := b.Load()
	if err != nil || read[key] != want || len(read) != 1 {
		t.Fatalf("reload: %v %v", read, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".nerd", "specs", "links.json")); err != nil {
		t.Fatalf("the file lives beside the spec files: %v", err)
	}
}

func TestStoreRefusesAFileItCannotReadInsteadOfOverwritingIt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".nerd", "specs", "links.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := OpenStore(root)
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("a corrupt file is an error naming the file: %v", err)
	}
	called := false
	if _, err := s.Update(func(c Confirmations) (Confirmations, error) { called = true; return c, nil }); err == nil || called {
		t.Fatalf("Update must not run its change over a file it could not read (called=%v err=%v)", called, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatalf("the unreadable file was overwritten: %q", data)
	}
}

func TestSectionFingerprintIsItsOwnTextAndIgnoresLayout(t *testing.T) {
	fp := func(src, ref string) string {
		for _, s := range ScanDoc("Docs/g.md", src, CodeLexer([]string{"Docs"})).Sections {
			if s.Ref == ref {
				return s.Fingerprint
			}
		}
		t.Fatalf("no section %s in %q", ref, src)
		return ""
	}
	base := "# G\n\n## Install\nRun the installer once.\n\n```sh\nmake install\n```\n\n### Linux\nUse apt.\n\n## Other\nElsewhere.\n"
	install := "Docs/g.md:G > Install"
	want := fp(base, install)
	same := map[string]string{
		"reflowed prose":     strings.Replace(base, "Run the installer once.", "Run the\ninstaller   once.", 1),
		"a neighbour edited": strings.Replace(base, "Elsewhere.", "Somewhere else entirely.", 1),
		"a child edited":     strings.Replace(base, "Use apt.", "Use dnf.", 1),
		"a heading renumber": strings.Replace(base, "## Install", "##  Install  ", 1),
	}
	for name, src := range same {
		if got := fp(src, install); got != want {
			t.Errorf("%s moved the section's fingerprint", name)
		}
	}
	for name, src := range map[string]string{
		"reworded prose": strings.Replace(base, "once", "twice", 1),
		"edited example": strings.Replace(base, "make install", "make install PREFIX=/opt", 1),
	} {
		if fp(src, install) == want {
			t.Errorf("%s left the section's fingerprint alone", name)
		}
	}
	// A child's own text is its own fingerprint.
	if fp(base, "Docs/g.md:G > Install > Linux") == fp(strings.Replace(base, "Use apt.", "Use dnf.", 1), "Docs/g.md:G > Install > Linux") {
		t.Error("an edit to a child must move the child's fingerprint")
	}
}

func TestResolveReconcilesAgainstTheFingerprintsItWasGiven(t *testing.T) {
	doc := scanGuide(t)
	in := Input{Docs: []*DocScan{doc}, Code: fixtureIndex(), Now: 50}
	first := Resolve(in)
	if len(first.Confirmed) == 0 {
		t.Fatal("every link with both ends is confirmed on first sight")
	}
	// The same measurement again changes nothing.
	in.Confirmed = first.Confirmed
	in.Now = 60
	if again := Resolve(in); !again.Confirmed.Equal(first.Confirmed) {
		t.Fatal("an unchanged world must leave the confirmations as they were")
	}
	// Func's text changes: only the links that end in Func are reported as changed.
	changed := fixtureIndex().elems
	for i := range changed {
		if changed[i].Ref == "internal/core.Func" {
			changed[i].Fingerprint = "edited"
		}
	}
	in.Code = NewCodeIndex(changed, []string{"internal/core/foo.go", "internal/other/bar.go", "internal/core/policy.mg", "Docs/guide.md"})
	in.Now = 70
	res := Resolve(in)
	for k, c := range res.Confirmed {
		if moved := c.CodeChangedAt != 0; moved != (k.Code == "internal/core.Func") {
			t.Errorf("link %v: code changed at %d, but only Func's links should have moved", k, c.CodeChangedAt)
		}
	}
}
