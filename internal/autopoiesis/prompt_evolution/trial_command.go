package prompt_evolution

import (
	"fmt"
	"strconv"
	"time"
)

// TrialCommandUsage is the operator syntax of the trial surface.
const TrialCommandUsage = "`<atom-id> [hours] [max-exposures]` approves a trial, `revoke <atom-id>` ends one, `list` shows all"

// TrialAction is what an operator trial command asks for.
type TrialAction string

const (
	TrialApprove TrialAction = "approve"
	TrialRevoke  TrialAction = "revoke"
	TrialList    TrialAction = "list"
)

// TrialCommand is a parsed operator trial command. Zero TTL or MaxExposures
// mean "the configured default" (ApproveTrial applies them).
type TrialCommand struct {
	Action       TrialAction
	AtomID       string
	TTL          time.Duration
	MaxExposures int
}

// ParseTrialCommand parses the arguments after the command word.
func ParseTrialCommand(args []string) (TrialCommand, error) {
	if len(args) == 0 {
		return TrialCommand{}, fmt.Errorf("usage: %s", TrialCommandUsage)
	}
	head, rest := args[0], args[1:]
	switch TrialAction(head) {
	case TrialList:
		return TrialCommand{Action: TrialList}, nil
	case TrialRevoke:
		if len(rest) == 0 {
			return TrialCommand{}, fmt.Errorf("usage: %s", TrialCommandUsage)
		}
		return TrialCommand{Action: TrialRevoke, AtomID: rest[0]}, nil
	}
	cmd := TrialCommand{Action: TrialApprove, AtomID: head}
	positive := func(name, raw string) (int, error) {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return 0, fmt.Errorf("%s must be a positive integer (usage: %s)", name, TrialCommandUsage)
		}
		return n, nil
	}
	if len(rest) > 0 {
		hours, err := positive("hours", rest[0])
		if err != nil {
			return TrialCommand{}, err
		}
		cmd.TTL = time.Duration(hours) * time.Hour
		rest = rest[1:]
	}
	if len(rest) > 0 {
		n, err := positive("max-exposures", rest[0])
		if err != nil {
			return TrialCommand{}, err
		}
		cmd.MaxExposures = n
	}
	return cmd, nil
}

// shortVersionLen is how much of a version hash an operator display shows.
const shortVersionLen = 12

// ShortVersion is the displayed prefix of the pinned version.
func (e TrialEnvelope) ShortVersion() string {
	if len(e.Version) > shortVersionLen {
		return e.Version[:shortVersionLen]
	}
	return e.Version
}

// State names an envelope's standing at now: live, expired or exhausted.
func (e TrialEnvelope) State(now time.Time) string {
	switch {
	case !now.Before(e.ExpiresAt):
		return "expired"
	case e.Exposures >= e.MaxExposures:
		return "exhausted"
	}
	return "live"
}
