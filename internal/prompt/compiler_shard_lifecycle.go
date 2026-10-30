package prompt

import (
	"database/sql"
	"fmt"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
)

// shardDBRegistration is one generation of a persona's database. All fields
// are protected by shardMu. The owner, executions and source readers hold
// independent references; removing a name does not revoke an acquired handle.
type shardDBRegistration struct {
	db      *sql.DB
	owned   bool
	leases  int
	readers int
	closed  bool
}

// installShardDBLocked installs a registry-owned generation. Re-registering the
// same handle promotes a temporary generation without discarding its leases.
func (c *JITPromptCompiler) installShardDBLocked(key string, db *sql.DB) {
	if previous := c.shardDBs[key]; previous != nil {
		if previous.db == db {
			previous.owned = true
			return
		}
		previous.owned = false
		c.retireShardHandleLocked(previous)
	}
	registration := &shardDBRegistration{db: db, owned: true}
	c.shardDBs[key] = registration
	c.shardHandles[registration] = struct{}{}
}

// AcquireShardDB borrows the current persona registration. Only when no DB is
// registered does it open a temporary pool, shared by overlapping executions.
// An owner replacement takes effect for future compiles immediately; releasing
// an old lease cannot remove that replacement. The release is idempotent.
func (c *JITPromptCompiler) AcquireShardDB(shardID, dbPath string) (func(), error) {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return nil, fmt.Errorf("prompt compiler is closed")
	}
	c.shardMu.Lock()
	defer c.shardMu.Unlock()
	key := shardDBKey(shardID)
	registration := c.shardDBs[key]
	if registration == nil || registration.db == nil {
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			return nil, fmt.Errorf("open agent knowledge database %s: %w", dbPath, err)
		}
		sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping agent knowledge database %s: %w", dbPath, err)
		}
		if registration != nil {
			registration.owned = false
			c.retireShardHandleLocked(registration)
		}
		registration = &shardDBRegistration{db: db}
		c.shardDBs[key] = registration
		c.shardHandles[registration] = struct{}{}
		c.bumpSourceEpoch()
		c.clearPromptCache(fmt.Sprintf("temporary shard database registered: %s", shardID))
	}
	registration.leases++
	var once sync.Once
	return func() {
		once.Do(func() {
			c.shardMu.Lock()
			defer c.shardMu.Unlock()
			registration.leases--
			if !registration.owned && registration.leases == 0 && c.shardDBs[key] == registration {
				delete(c.shardDBs, key)
				c.bumpSourceEpoch()
				c.clearPromptCache(fmt.Sprintf("temporary shard database released: %s", shardID))
			}
			c.retireShardHandleLocked(registration)
		})
	}, nil
}

func (c *JITPromptCompiler) retireShardHandleLocked(registration *shardDBRegistration) {
	if !registration.owned && registration.leases == 0 && registration.readers == 0 {
		c.closeShardHandleLocked(registration)
	}
}

func (c *JITPromptCompiler) closeShardHandleLocked(registration *shardDBRegistration) {
	if registration.closed {
		return
	}
	registration.closed = true
	delete(c.shardHandles, registration)
	if registration.db != nil {
		c.catalog.forgetHandle(registration.db)
		if err := registration.db.Close(); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to close retired shard DB: %v", err)
		}
	}
}

// pinShardReadersLocked retains the exact handles a source snapshot captured.
// The caller holds lifecycleMu and shardMu, so shutdown cannot begin waiting
// before the reader is counted. Release may run after the synchronous caller
// has canceled, once its singleflight work has actually finished using sources.
func (c *JITPromptCompiler) pinShardReadersLocked(registrations []*shardDBRegistration) func() {
	if len(registrations) == 0 {
		return func() {}
	}
	c.wg.Add(1)
	for _, registration := range registrations {
		registration.readers++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			defer c.wg.Done()
			c.shardMu.Lock()
			defer c.shardMu.Unlock()
			for _, registration := range registrations {
				registration.readers--
				c.retireShardHandleLocked(registration)
			}
		})
	}
}
