package system

import (
	"codenerd/internal/perception"
	"errors"
)

// Close releases resources held by a Cortex instance.
//
// This is especially important in tests on Windows, where open SQLite handles
// prevent TempDir cleanup (e.g. learned_patterns.db from the Perception layer).
func (c *Cortex) Close() error {
	if c == nil {
		return nil
	}

	var errs []error

	if c.ShardManager != nil {
		c.ShardManager.StopAll()
	}

	if c.LocalDB != nil {
		if err := c.LocalDB.Close(); err != nil {
			errs = append(errs, err)
		}
		c.LocalDB = nil
	}

	if err := perception.ClosePerceptionLayer(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
