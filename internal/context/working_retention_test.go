package context

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"codenerd/internal/mangle"
)

// deadPID is a process id no host runs: past Linux's pid_max and Windows'
// practical range.
const deadPID = 1 << 30

// fakeArchive puts an archive's files under dir without opening SQLite: a
// database (and, when wal is set, the journal a crash leaves), and an owner
// record when owner is not nil.
func fakeArchive(t *testing.T, dir, scope string, owner *workingOwner, wal bool) string {
	t.Helper()
	digest := workingDigest(scope)
	if owner != nil {
		if err := recordWorkingOwner(dir, digest, *owner); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, digest+".db"), make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	if wal {
		if err := os.WriteFile(filepath.Join(dir, digest+".db-wal"), make([]byte, 1024), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return digest
}

func archiveNames(r WorkingArchiveReport) []string {
	var out []string
	for _, a := range r.Archives {
		out = append(out, a.Owner)
	}
	sort.Strings(out)
	return out
}

// .nerd/context held 320 archives on the dogfood workspace and nothing
// removed one (program of work item 15). The retention policy keeps exactly
// the archives something can still redeem a handle into.
func TestWorkingRetention_KeepsWhatCanBeRedeemedAndPrunesTheRest(t *testing.T) {
	ws := t.TempDir()

	// A scope this process is using: opened through the real store, so its
	// owner record is the one production writes.
	live, err := OpenWorkingStore(ws, "session/probe/live")
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	dir, err := workingContextDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	gone := fakeArchive(t, dir, "session/probe/crashed", &workingOwner{PID: deadPID, Host: thisHost()}, true)
	legacy := fakeArchive(t, dir, "session/probe/legacy", nil, true)
	foreign := fakeArchive(t, dir, "session/probe/elsewhere", &workingOwner{PID: deadPID, Host: thisHost() + ".another-host.invalid"}, false)
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}

	survey, err := SurveyWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if survey.Total != 4 || survey.Redeemable != 2 || survey.Prunable != 2 || survey.Removed != 0 {
		t.Fatalf("survey = %s, want 4 archives: 2 redeemable (live, foreign), 2 prunable (gone, unrecorded), none removed", survey)
	}

	pruned, err := PruneWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if pruned.Removed != 2 || pruned.Redeemable != 2 || len(pruned.Failures) != 0 {
		t.Fatalf("prune = %s, want the two prunable archives removed and the two redeemable kept", pruned)
	}
	if got := archiveNames(pruned); len(got) != 2 || got[0] != ArchiveOwnerForeign || got[1] != ArchiveOwnerLive {
		t.Fatalf("kept archives = %v, want [foreign live]", got)
	}
	for _, d := range []string{gone, legacy} {
		for _, suffix := range []string{".db", ".db-wal", ownerSuffix} {
			if _, err := os.Stat(filepath.Join(dir, d+suffix)); !os.IsNotExist(err) {
				t.Errorf("%s%s survived the prune (%v)", d[:8], suffix, err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, foreign+".db")); err != nil {
		t.Errorf("an archive owned on another host was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README")); err != nil {
		t.Errorf("a file that is not an archive was removed: %v", err)
	}
}

// A task's executor retires its scope when the task ends, and the archive
// goes with it: a campaign mints one per task, and waiting for the process to
// die would keep thousands for the length of a campaign.
func TestRetireWorkingScope_RemovesTheArchiveItsExecutorIsDoneWith(t *testing.T) {
	ws := t.TempDir()
	for _, scope := range []string{"session/task/done", "session/root/kept"} {
		s, err := OpenWorkingStore(ws, scope)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}

	report, err := RetireWorkingScope(ws, "session/task/done")
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Fatalf("retire = %s, want the retired archive removed", report)
	}
	after, err := SurveyWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if after.Total != 1 || after.Redeemable != 1 {
		t.Fatalf("after retiring one scope: %s, want only the live one left", after)
	}

	// A scope that never opened an archive has nothing to retire.
	if r, err := RetireWorkingScope(ws, "session/task/never-ran"); err != nil || r.Removed != 0 {
		t.Fatalf("retiring a scope with no archive = %s, %v", r, err)
	}
}

// An owner record is written through a temporary named for its writer. One
// whose writer still runs is a record in flight and is not an archive yet; one
// a crash left is debris of an archive nobody owns, and goes with it.
func TestWorkingRetention_AnOwnerRecordInFlightIsNotAnArchive(t *testing.T) {
	ws := t.TempDir()
	dir, err := workingContextDir(ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	inFlight := filepath.Join(dir, workingDigest("writing")+ownerSuffix+"."+strconv.Itoa(os.Getpid())+".tmp123")
	crashed := filepath.Join(dir, workingDigest("crashed")+ownerSuffix+"."+strconv.Itoa(deadPID)+".tmp456")
	for _, p := range []string{inFlight, crashed} {
		if err := os.WriteFile(p, []byte(`{"pid":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	report, err := PruneWorkingArchives(ws)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 1 || report.Removed != 1 {
		t.Fatalf("prune = %s, want only the crash's debris counted and removed", report)
	}
	if _, err := os.Stat(inFlight); err != nil {
		t.Errorf("a record being written was removed: %v", err)
	}
	if _, err := os.Stat(crashed); !os.IsNotExist(err) {
		t.Errorf("a crash's half-written record survived: %v", err)
	}
}

// The decision is working_retention.mg's: the survey only measures.
func TestWorkingRetentionPolicy_DerivesPrunableFromOwnershipFacts(t *testing.T) {
	facts := []mangle.Fact{
		{Predicate: "working_archive", Args: []any{"live"}},
		{Predicate: "working_archive_owner_live", Args: []any{"live"}},
		{Predicate: "working_archive", Args: []any{"retired-live"}},
		{Predicate: "working_archive_owner_live", Args: []any{"retired-live"}},
		{Predicate: "working_archive_retired", Args: []any{"retired-live"}},
		{Predicate: "working_archive", Args: []any{"foreign"}},
		{Predicate: "working_archive_owner_foreign", Args: []any{"foreign"}},
		{Predicate: "working_archive", Args: []any{"orphan"}},
	}
	got, err := derivePrunable(facts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"retired-live": true, "orphan": true}
	if len(got) != len(want) {
		t.Fatalf("prunable = %v, want %v", got, want)
	}
	for a := range want {
		if !got[a] {
			t.Errorf("%s not derived prunable (got %v)", a, got)
		}
	}
}
