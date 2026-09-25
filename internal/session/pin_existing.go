package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	internalbuild "codenerd/internal/build"
)

// pinnedByExistingTests reports, for one change the turn made, whether any test
// already in the repository notices when that change alone is taken back out.
//
// The pinning gate measures the tests the TURN wrote, which is right when a
// turn wrote some. When a turn wrote none, the gate used to say
//
//	This turn changed the functions below and wrote no test, so nothing
//	would notice if a change were lost
//
// and that sentence was asserted, never checked. Ladder run R1-20 (2026-09-20)
// is what it costs: the turn deleted three asserts of an undeclared predicate
// and updated the repository's undeclared-assert baseline in the same change,
// which TestUndeclaredAssertBudget pins exactly -- put the file back and it
// fails, naming the predicate and the file. That guard lives three packages
// away from the code it guards, so no per-package or importer scope would have
// reached it either. The gate demanded a test anyway, and the model spent three
// repair attempts and 940,761 input tokens writing one that should not exist
// before the round gave up and a correct change ended /unverified.
//
// One unit at a time, against the whole repository. Reverting the entire turn
// at once is cheaper and wrong: a single unit whose removal stops the tree
// compiling would report every other change as pinned too, which is how a
// genuinely unpinned function hides behind a well-covered neighbour.
func pinnedByExistingTests(ctx context.Context, workspace string, u pinUnit) (bool, string) {
	tmpDir, err := os.MkdirTemp("", "pin-existing-*")
	if err != nil {
		return false, err.Error()
	}
	defer os.RemoveAll(tmpDir)

	workspace = goWorkspace(workspace)
	abs := goOverlayKey(workspace, u.path)
	replace := map[string]string{abs: ""}
	if !u.absent {
		stand := filepath.Join(tmpDir, "unit.go")
		if err := os.WriteFile(stand, []byte(u.content), 0o644); err != nil {
			return false, err.Error()
		}
		replace[abs] = stand
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return false, err.Error()
	}
	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		return false, err.Error()
	}

	// -failfast: the first test that notices answers the question, and most
	// answer early. Only a change nothing pins pays for a whole run, which is
	// exactly when the gate is about to ask the model for a test and had
	// better be sure.
	args := []string{"test", "-overlay", overlayPath, "-count=1", "-failfast", "./..."}
	out, outcome, _ := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	switch outcome {
	case VerifyFailed:
		if failed := topLevelFailedTests(string(out)); len(failed) > 0 {
			return true, strings.Join(failed, ", ")
		}
		// It stopped compiling without the change. That is something noticing,
		// but not a behaviour anyone pinned: a caller that no longer resolves
		// says the symbol is used, not that what it does is checked.
		return false, "the tree stops compiling without it, which no test asserts"
	case VerifyPassed:
		return false, "the whole suite passes with it taken out"
	default:
		// No verdict is not evidence either way, and the gate must not read it
		// as one: a change that cannot be measured is asked for its test.
		return false, "could not be measured"
	}
}
