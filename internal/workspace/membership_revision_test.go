package workspace

import (
	"path/filepath"
	"testing"
)

// Revision holds while nothing that decides membership changes, and moves
// on a checkout and on an ignore edit.
func TestMembershipRevisionMovesWithTheMemberSet(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gitignore"), "*.log\n")
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "a.go", ".gitignore")
	runGit(t, root, "commit", "-m", "init")

	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	first := m.Revision()
	if first == "" || m.Revision() != first {
		t.Fatalf("revision unstable with nothing changed: %q then %q", first, m.Revision())
	}
	// A plain file edit is the kernel's event to report, not a member-set
	// change.
	writeFile(t, filepath.Join(root, "a.go"), "package a // edited\n")
	if m.Revision() != first {
		t.Fatal("a content edit moved the member-set revision")
	}

	runGit(t, root, "checkout", "-q", "-b", "other")
	afterCheckout := m.Revision()
	if afterCheckout == first {
		t.Fatal("a checkout left the revision unchanged")
	}

	writeFile(t, filepath.Join(root, ".gitignore"), "*.log\n*.tmp\n")
	if m.Revision() == afterCheckout {
		t.Fatal("an ignore edit left the revision unchanged")
	}

	other, err := Open(root, []string{"vendor"})
	if err != nil {
		t.Fatal(err)
	}
	if other.Revision() == m.Revision() {
		t.Fatal("different ignore patterns share a revision")
	}
}
