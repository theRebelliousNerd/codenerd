package sqlpragmas

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	"codenerd/internal/logging"
)

// ApplyDefaultPragmas has a structural hole: PRAGMAs are per-connection, but
// it runs against a *sql.DB and therefore tunes exactly one pooled connection
// — whichever the pool happened to hand it. Every connection the pool opens
// afterwards (under concurrency, or after an idle connection is closed and
// replaced) starts with SQLite's defaults: no WAL, 2 MiB cache, no mmap.
//
// Most call sites in this codebase get away with it because their pools are
// effectively single-connection. The general fix is a connector hook: wrap the
// driver's connector so the pragmas are applied inside Connect, at the birth
// of every connection, where database/sql cannot route around them.

// NewConnector wraps drv's connector for dsn so that every connection it
// creates has the profile's pragmas applied before the pool sees it.
//
// Pass the result to sql.OpenDB. Drivers implementing driver.DriverContext
// (both mattn/go-sqlite3 and modernc.org/sqlite do) get their native
// connector; others are wrapped in a minimal DSN connector.
func NewConnector(drv driver.Driver, dsn string, profile PragmaProfile) (driver.Connector, error) {
	if drv == nil {
		return nil, errors.New("sqlpragmas: nil driver")
	}
	var base driver.Connector
	if dc, ok := drv.(driver.DriverContext); ok {
		c, err := dc.OpenConnector(dsn)
		if err != nil {
			return nil, fmt.Errorf("sqlpragmas: open connector: %w", err)
		}
		base = c
	} else {
		base = dsnConnector{dsn: dsn, drv: drv}
	}
	return &pragmaConnector{base: base, profile: profile}, nil
}

// OpenWithPragmas is sql.Open plus a connector hook: the returned *sql.DB
// applies the profile's pragmas to every connection it opens, not just the
// first. Prefer it over sql.Open + ApplyDefaultPragmas for any handle whose
// pool is allowed to grow past one connection.
//
// Like sql.Open it does not contact the database, so a bad path surfaces on
// first use rather than here.
func OpenWithPragmas(driverName, dsn string, profile PragmaProfile) (*sql.DB, error) {
	// database/sql exposes no way to look a registered driver up by name, but
	// sql.Open is lazy — it resolves the driver and connects to nothing — so a
	// throwaway handle is a cheap and supported way to obtain the instance.
	probe, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	drv := probe.Driver()
	if cerr := probe.Close(); cerr != nil {
		logging.Get(logging.CategoryStore).Debug("closing driver probe handle: %v", cerr)
	}

	connector, err := NewConnector(drv, dsn, profile)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

// pragmaConnector applies a profile to each newly created connection.
type pragmaConnector struct {
	base    driver.Connector
	profile PragmaProfile
}

func (c *pragmaConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	applyToDriverConn(ctx, conn, c.profile)
	return conn, nil
}

func (c *pragmaConnector) Driver() driver.Driver { return c.base.Driver() }

// dsnConnector adapts a plain driver.Driver (no DriverContext) to the
// connector interface.
type dsnConnector struct {
	dsn string
	drv driver.Driver
}

func (c dsnConnector) Connect(context.Context) (driver.Conn, error) { return c.drv.Open(c.dsn) }
func (c dsnConnector) Driver() driver.Driver                        { return c.drv }

// applyToDriverConn runs the profile's pragmas on a raw driver connection.
// Failure handling matches ApplyDefaultPragmas exactly — Debug log, count,
// carry on — because a pragma the driver rejects must not turn into a failed
// connection and thus a failed query.
func applyToDriverConn(ctx context.Context, conn driver.Conn, profile PragmaProfile) {
	logger := logging.Get(logging.CategoryStore)
	for _, p := range pragmasFor(profile) {
		if err := execOnDriverConn(ctx, conn, p); err != nil {
			recordPragmaFailure(profile, p)
			logger.Debug("pragma %q failed on new connection (profile %s): %v", p, profile, err)
		}
	}
}

func execOnDriverConn(ctx context.Context, conn driver.Conn, query string) error {
	if ec, ok := conn.(driver.ExecerContext); ok {
		_, err := ec.ExecContext(ctx, query, nil)
		return err
	}

	var (
		stmt driver.Stmt
		err  error
	)
	if pc, ok := conn.(driver.ConnPrepareContext); ok {
		stmt, err = pc.PrepareContext(ctx, query)
	} else {
		stmt, err = conn.Prepare(query)
	}
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	if sec, ok := stmt.(driver.StmtExecContext); ok {
		_, err = sec.ExecContext(ctx, nil)
		return err
	}
	_, err = stmt.Exec(nil) //nolint:staticcheck // pre-context driver fallback
	return err
}
