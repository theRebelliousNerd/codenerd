package system

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/sessionlog"
	"fmt"
	"strings"
	"time"
)

// initSessionLog installs the health/transcript store before resident shards start.
func initSessionLog(bctx *bootContext) error {
	if sid := strings.TrimSpace(bctx.cfg.SessionID); sid != "" {
		bctx.sessionID = sid
	} else {
		bctx.sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	startSessionLog(bctx)
	return nil
}

// startSessionLog runs the session folder's startup sequence for the booted
// session: flat session files move into their folders, sessions past
// logging.session_ttl are pruned (never this one, never one another process is
// running), and the store the broker, the typesafe client and the session
// executor write through is installed. Exchange files follow debug_mode or
// debug_self; transcripts are always written.
func startSessionLog(bctx *bootContext) {
	ttl := config.DefaultSessionTTL
	if cfg := bctx.appCfg; cfg != nil {
		logCfg := cfg.GetLogging()
		parsed, err := logCfg.SessionRetention()
		if err != nil {
			logging.Get(logging.CategoryBoot).Warn("%v; keeping the default %s", err, ttl)
		} else {
			ttl = parsed
		}
	}
	bctx.sessionLog = sessionlog.StartSession(sessionlog.BootOptions{
		Workspace: bctx.workspace,
		Session:   bctx.sessionID,
		TTL:       ttl,
	})
}

// closeSessionLog releases the session's lease.
func (c *Cortex) closeSessionLog() {
	if c.sessionLog != nil {
		c.sessionLog.Close()
		c.sessionLog = nil
	}
}
