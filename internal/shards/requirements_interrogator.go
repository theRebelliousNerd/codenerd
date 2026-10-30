package shards

import "codenerd/internal/deliberation"

// RequirementsInterrogatorShard challenges proposed readings against explicit
// success tests and workspace evidence. Asking the person belongs exclusively
// to the conductor's Mangle-admitted values-fork continuation.
type RequirementsInterrogatorShard struct{ *DeliberationShard }

func NewRequirementsInterrogatorShard() *RequirementsInterrogatorShard {
	return &RequirementsInterrogatorShard{DeliberationShard: newDeliberationShard(deliberation.Challenger)}
}
