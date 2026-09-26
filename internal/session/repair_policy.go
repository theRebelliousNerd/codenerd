package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"sync/atomic"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

var repairEpisodeSeq atomic.Uint64

// newRepairEpisodeAtom mints a repair episode's key in the kernel. Its facts
// are the turn's (assertTurnFact), so they leave with the turn; the process id
// keeps two processes' episodes apart, as newTurnAtom does for turns.
func newRepairEpisodeAtom() types.MangleAtom {
	return types.MangleAtom(fmt.Sprintf("/repair_%d_%d", os.Getpid(), repairEpisodeSeq.Add(1)))
}

// repairDurations are the parts of a build or test run's output that change
// from one run of the same failure to the next: "--- FAIL: TestX (0.01s)" and
// "FAIL\tpkg\t0.412s".
var repairDurations = regexp.MustCompile(`(?m)\(\d+(?:\.\d+)?s\)|[ \t]\d+(?:\.\d+)?s[ \t]*$`)

// repairFailureDigest identifies a failure: the same failing output, with its
// durations removed, digests the same. The policy compares digests; it never
// reads the output.
func repairFailureDigest(output string) string {
	sum := sha256.Sum256([]byte(repairDurations.ReplaceAllString(output, " ")))
	return hex.EncodeToString(sum[:8])
}

// repairNext is the policy's answer after a failed attempt.
type repairNext struct {
	// gaveUp is why the episode gives up; "" to go on.
	gaveUp string
	// closed runs the next attempt with reading closed.
	closed bool
	// restart undoes the episode's edits before the next attempt, which is
	// told to name a different cause.
	restart bool
}

// nextRepairMove records a failed attempt and asks the policy what the
// episode does next (repair_episode.mg): give up, retry (with reading closed
// or not), or restart.
//
// No policy -- no kernel, a failed query, or no answer -- is giving up: an
// episode nothing bounds is the failure the count used to paper over, so the
// executor never continues on its own say.
func (e *Executor) nextRepairMove(episode types.MangleAtom, attempt int, wrote bool, failureOutput string) repairNext {
	if e.kernel == nil {
		return repairNext{gaveUp: "no repair policy (no kernel)"}
	}
	e.ensureSessionParams()
	wroteAtom := types.MangleAtom("/false")
	if wrote {
		wroteAtom = types.MangleAtom("/true")
	}
	e.assertTurnFact(types.Fact{Predicate: "repair_attempt", Args: []any{
		episode, int64(attempt), wroteAtom, types.MangleString(repairFailureDigest(failureOutput)),
	}})
	episodeRows := func(predicate string) ([]types.Fact, error) {
		rows, err := e.kernel.Query(predicate)
		if err != nil {
			return nil, err
		}
		var out []types.Fact
		for _, f := range rows {
			if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == string(episode) {
				out = append(out, f)
			}
		}
		return out, nil
	}
	moves, err := episodeRows("repair_move")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("repair_move query failed: %v; the episode gives up", err)
		return repairNext{gaveUp: fmt.Sprintf("the repair policy could not be asked (%v)", err)}
	}
	retry, restart := false, false
	for _, f := range moves {
		if len(f.Args) != 2 {
			continue
		}
		switch types.ExtractString(f.Args[1]) {
		case "/give_up":
			return repairNext{gaveUp: e.repairGiveUpReason(episodeRows)}
		case "/retry":
			retry = true
		case "/restart":
			restart = true
		}
	}
	if restart {
		// A restart reopens reading: the different cause may be in code no
		// attempt has read yet.
		if !e.assertTurnFact(types.Fact{Predicate: "repair_restart", Args: []any{episode, int64(attempt)}}) {
			return repairNext{gaveUp: "the repair policy's restart could not be recorded"}
		}
		return repairNext{restart: true}
	}
	if !retry {
		return repairNext{gaveUp: "the repair policy derived no next move"}
	}
	closed, err := episodeRows("repair_closed")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("repair_closed query failed: %v; the next attempt runs closed", err)
		return repairNext{closed: true}
	}
	return repairNext{closed: len(closed) > 0}
}

// repairGiveUpReason names the rule that ended the episode, for its error.
func (e *Executor) repairGiveUpReason(episodeRows func(string) ([]types.Fact, error)) string {
	if rows, err := episodeRows("repair_not_converging_since_restart"); err == nil && len(rows) > 0 {
		return "the same failure survived two edits, before and after a restart from a different cause"
	}
	return fmt.Sprintf("the attempt cap (session.repair_max_attempts=%d) is reached", e.configSnapshot().sessionRepairMaxAttempts())
}
