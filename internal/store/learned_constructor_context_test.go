package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
)

type learnedConstructorBarrier struct {
	phase        string
	entered      chan context.Context
	canceled     chan struct{}
	release      chan struct{}
	drained      chan struct{}
	closeEntered chan struct{}
	closeRelease chan struct{}
	closeDrained chan struct{}
	blocked      atomic.Bool
	closes       atomic.Int64
	cleanupErr   error
	rollbackErr  error
	operationErr error
	connectErr   error
	releaseOnce  sync.Once
	closeOnce    sync.Once
}

func (barrier *learnedConstructorBarrier) unblock() {
	barrier.releaseOnce.Do(func() { close(barrier.release) })
	barrier.closeOnce.Do(func() { close(barrier.closeRelease) })
}

func (barrier *learnedConstructorBarrier) block(ctx context.Context, phase string) error {
	if barrier.phase != phase || !barrier.blocked.CompareAndSwap(false, true) {
		return ctx.Err()
	}
	barrier.entered <- ctx
	<-ctx.Done()
	close(barrier.canceled)
	<-barrier.release
	close(barrier.drained)
	return ctx.Err()
}

type learnedConstructorDriver struct {
	barrier *learnedConstructorBarrier
}

func (backend learnedConstructorDriver) Open(path string) (driver.Conn, error) {
	connection, err := (&sqlite3.SQLiteDriver{}).Open(path)
	if err != nil {
		return nil, err
	}
	return &learnedConstructorConnection{Conn: connection, barrier: backend.barrier}, nil
}

func (backend learnedConstructorDriver) OpenConnector(path string) (driver.Connector, error) {
	return learnedConstructorConnector{backend: backend, path: path}, nil
}

type learnedConstructorConnector struct {
	backend learnedConstructorDriver
	path    string
}

func (connector learnedConstructorConnector) Driver() driver.Driver { return connector.backend }

func (connector learnedConstructorConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection, err := connector.backend.Open(connector.path)
	if err != nil {
		return nil, err
	}
	// Return the acquired handle even when cancellation arrived during native
	// connection admission; the production connector must reject and close it.
	phase := "connect"
	if connector.backend.barrier.phase == "connect_error" {
		phase = "connect_error"
	}
	_ = connector.backend.barrier.block(ctx, phase)
	return connection, connector.backend.barrier.connectErr
}

type learnedConstructorConnection struct {
	driver.Conn
	barrier *learnedConstructorBarrier
}

func (connection *learnedConstructorConnection) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if pinger, ok := connection.Conn.(driver.Pinger); ok {
		if err := pinger.Ping(ctx); err != nil {
			return err
		}
	}
	return connection.barrier.block(ctx, "ping")
}

func (connection *learnedConstructorConnection) ExecContext(ctx context.Context, query string, arguments []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := connection.Conn.(driver.ExecerContext).ExecContext(ctx, query, arguments)
	if err != nil {
		return result, err
	}
	if query == "ROLLBACK" && connection.barrier.rollbackErr != nil {
		return nil, connection.barrier.rollbackErr
	}
	phase := ""
	switch {
	case strings.HasPrefix(query, "PRAGMA "):
		phase = "pragma"
	case strings.Contains(query, "CREATE TABLE IF NOT EXISTS learned_patterns"):
		phase = "schema"
	case strings.HasPrefix(query, "DROP TABLE IF EXISTS vec_learned"):
		phase = "vector"
	case strings.HasPrefix(query, "INSERT INTO vec_learned"):
		phase = "backfill"
	}
	if phase == "schema" && connection.barrier.operationErr != nil {
		return nil, connection.barrier.operationErr
	}
	if err := connection.barrier.block(ctx, phase); err != nil {
		return nil, err
	}
	return result, nil
}

func (connection *learnedConstructorConnection) QueryContext(ctx context.Context, query string, arguments []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := connection.Conn.(driver.QueryerContext).QueryContext(ctx, query, arguments)
	if err != nil {
		return nil, err
	}
	if err := connection.barrier.block(ctx, "backfill_query"); err != nil {
		return nil, errors.Join(err, rows.Close())
	}
	return rows, nil
}

func (connection *learnedConstructorConnection) Close() error {
	err := connection.Conn.Close()
	if connection.barrier.closes.Add(1) == 1 {
		close(connection.barrier.closeEntered)
		<-connection.barrier.closeRelease
		close(connection.barrier.closeDrained)
	}
	return errors.Join(err, connection.barrier.cleanupErr)
}

func awaitLearnedConstructor[Value any](test *testing.T, events <-chan Value) Value {
	test.Helper()
	select {
	case value := <-events:
		return value
	case <-time.After(20 * time.Second):
		test.Fatal("learned constructor barrier did not complete")
		var zero Value
		return zero
	}
}

func learnedConstructorSnapshot(test *testing.T, path string) []byte {
	test.Helper()
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		test.Fatal(err)
	}
	defer database.Close()
	var tables [][][]any
	for _, query := range []string{
		"SELECT type, name, tbl_name, sql FROM sqlite_master ORDER BY name",
		"SELECT * FROM learned_patterns ORDER BY id",
		"SELECT rowid, embedding, pattern, verb FROM vec_learned ORDER BY rowid",
	} {
		rows, err := database.Query(query)
		if err != nil {
			test.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			test.Fatal(err)
		}
		var records [][]any
		for rows.Next() {
			values := make([]any, len(columns))
			destinations := make([]any, len(columns))
			for index := range values {
				destinations[index] = &values[index]
			}
			if err := rows.Scan(destinations...); err != nil {
				_ = rows.Close()
				test.Fatal(err)
			}
			records = append(records, values)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			test.Fatal(err)
		}
		tables = append(tables, records)
	}
	encoded, err := json.Marshal(tables)
	if err != nil {
		test.Fatal(err)
	}
	return encoded
}

func TestLearnedConstructorContext_SQLCancellationDrainsAndPreservesCorpus(test *testing.T) {
	for _, phase := range []string{"connect", "connect_error", "pragma", "ping", "schema", "vector", "backfill_query", "backfill", "deadline"} {
		test.Run(phase, func(test *testing.T) {
			path := filepath.Join(test.TempDir(), "learned.db")
			engine := &MockEmbeddingEngine{}
			seed, err := NewLearnedCorpusStore(path, engine)
			if err != nil {
				test.Fatal(err)
			}
			for _, pattern := range []string{"create application", "fix application"} {
				if err := seed.AddPattern(context.Background(), pattern, "create", "application", "", 0.9); err != nil {
					_ = seed.Close()
					test.Fatal(err)
				}
			}
			if _, err := seed.db.Exec("INSERT INTO vec_learned (embedding, pattern, verb) VALUES (?, ?, ?)", encodeFloat32SliceToBlob([]float32{1, 0, 0, 0}), "existing vector sentinel", "review"); err != nil {
				_ = seed.Close()
				test.Fatal(err)
			}
			if err := seed.Close(); err != nil {
				test.Fatal(err)
			}
			before := learnedConstructorSnapshot(test, path)
			cleanupFailure := errors.New("learned constructor close failure")
			rollbackFailure := errors.New("learned constructor rollback failure")
			connectFailure := errors.New("learned connection admission failure")
			barrier := &learnedConstructorBarrier{
				phase: phase, entered: make(chan context.Context, 1), canceled: make(chan struct{}),
				release: make(chan struct{}), drained: make(chan struct{}),
				closeEntered: make(chan struct{}), closeRelease: make(chan struct{}), closeDrained: make(chan struct{}),
				cleanupErr: cleanupFailure, rollbackErr: rollbackFailure,
			}
			if phase == "connect_error" {
				barrier.connectErr = connectFailure
			}
			parent, cancel := context.WithCancel(context.Background())
			expected := error(context.Canceled)
			if phase == "deadline" {
				cancel()
				parent, cancel = context.WithTimeout(context.Background(), 2*time.Second)
				barrier.phase = "schema"
				expected = context.DeadlineExceeded
			}
			type result struct {
				corpus *LearnedCorpusStore
				err    error
			}
			finished := make(chan result, 1)
			done := make(chan struct{})
			test.Cleanup(func() {
				cancel()
				barrier.unblock()
				awaitLearnedConstructor(test, done)
			})
			go func() {
				defer close(done)
				corpus, err := newLearnedCorpusStoreWithDriver(parent, path, engine, learnedConstructorDriver{barrier: barrier})
				finished <- result{corpus: corpus, err: err}
			}()
			observed := awaitLearnedConstructor(test, barrier.entered)
			if observed != parent {
				test.Fatal("actual SQL operation did not receive the caller context")
			}
			if phase != "deadline" {
				cancel()
			}
			awaitLearnedConstructor(test, barrier.canceled)
			select {
			case outcome := <-finished:
				test.Fatalf("constructor returned before SQL drained: %+v", outcome)
			default:
			}
			barrier.releaseOnce.Do(func() { close(barrier.release) })
			awaitLearnedConstructor(test, barrier.drained)
			awaitLearnedConstructor(test, barrier.closeEntered)
			select {
			case outcome := <-finished:
				test.Fatalf("constructor returned before handle cleanup drained: %+v", outcome)
			default:
			}
			barrier.unblock()
			outcome := awaitLearnedConstructor(test, finished)
			awaitLearnedConstructor(test, done)
			awaitLearnedConstructor(test, barrier.closeDrained)
			if outcome.corpus != nil || !errors.Is(outcome.err, expected) || !errors.Is(outcome.err, cleanupFailure) || barrier.closes.Load() != 1 {
				test.Fatalf("constructor=(%v, %v), closes=%d", outcome.corpus, outcome.err, barrier.closes.Load())
			}
			if phase != "connect" && phase != "connect_error" && phase != "pragma" && phase != "ping" && !errors.Is(outcome.err, rollbackFailure) {
				test.Fatalf("constructor discarded rollback cleanup failure: %v", outcome.err)
			}
			if phase == "connect_error" && !errors.Is(outcome.err, connectFailure) {
				test.Fatalf("constructor discarded the driver's admission failure: %v", outcome.err)
			}
			if after := learnedConstructorSnapshot(test, path); !reflect.DeepEqual(before, after) {
				test.Fatalf("canceled constructor changed existing schema, patterns or vectors\nbefore=%s\nafter=%s", before, after)
			}
			reopened, err := NewLearnedCorpusStoreWithContext(context.Background(), path, engine)
			if err != nil {
				test.Fatal(err)
			}
			matches, searchErr := reopened.Search([]float32{0.1, 0.2, 0.3, 0.4}, 5)
			if err := errors.Join(searchErr, reopened.Close()); err != nil || len(matches) != 2 {
				test.Fatalf("healthy retry lost learned patterns: matches=%v err=%v", matches, err)
			}
			if err := os.Rename(path, path+".reopened"); err != nil {
				test.Fatalf("failed constructor or retry retained the database handle: %v", err)
			}
		})
	}
}

func TestLearnedConstructorContext_CancellationDuringCleanupRetainsParentError(test *testing.T) {
	operationFailure := errors.New("learned schema failure")
	cleanupFailure := errors.New("learned close failure")
	barrier := &learnedConstructorBarrier{
		phase: "unblocked", entered: make(chan context.Context, 1), canceled: make(chan struct{}),
		release: make(chan struct{}), drained: make(chan struct{}),
		closeEntered: make(chan struct{}), closeRelease: make(chan struct{}), closeDrained: make(chan struct{}),
		operationErr: operationFailure, cleanupErr: cleanupFailure,
	}
	parent, cancel := context.WithCancel(context.Background())
	path := filepath.Join(test.TempDir(), "learned.db")
	finished := make(chan error, 1)
	done := make(chan struct{})
	test.Cleanup(func() {
		cancel()
		barrier.unblock()
		awaitLearnedConstructor(test, done)
	})
	go func() {
		defer close(done)
		corpus, err := newLearnedCorpusStoreWithDriver(parent, path, nil, learnedConstructorDriver{barrier: barrier})
		if corpus != nil {
			err = errors.Join(err, corpus.Close(), errors.New("failed constructor published a store"))
		}
		finished <- err
	}()
	awaitLearnedConstructor(test, barrier.closeEntered)
	cancel()
	barrier.unblock()
	err := awaitLearnedConstructor(test, finished)
	awaitLearnedConstructor(test, done)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, operationFailure) || !errors.Is(err, cleanupFailure) {
		test.Fatalf("cleanup cancellation discarded an error: %v", err)
	}
}

func TestLearnedConstructorContext_PreCanceledHasNoFilesystemEffects(test *testing.T) {
	for _, deadline := range []bool{false, true} {
		test.Run(fmt.Sprintf("deadline=%v", deadline), func(test *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			expected := error(context.Canceled)
			if deadline {
				cancel()
				parent, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				expected = context.DeadlineExceeded
			} else {
				cancel()
			}
			defer cancel()
			path := filepath.Join(test.TempDir(), "uncreated", "learned.db")
			corpus, err := NewLearnedCorpusStoreWithContext(parent, path, nil)
			if corpus != nil || !errors.Is(err, expected) {
				test.Fatalf("pre-canceled constructor=(%v, %v)", corpus, err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				test.Fatalf("pre-canceled constructor touched the filesystem: %v", err)
			}
		})
	}
}

func TestLearnedConstructorContext_PragmaProfileAppliesToEveryConnection(test *testing.T) {
	path := filepath.Join(test.TempDir(), "learned.db")
	corpus, err := NewLearnedCorpusStoreWithContext(context.Background(), path, nil)
	if err != nil {
		test.Fatal(err)
	}
	defer corpus.Close()
	first, err := corpus.db.Conn(context.Background())
	if err != nil {
		test.Fatal(err)
	}
	defer first.Close()
	second, err := corpus.db.Conn(context.Background())
	if err != nil {
		test.Fatal(err)
	}
	defer second.Close()
	for _, connection := range []*sql.Conn{first, second} {
		var journal string
		var timeout, temporary int
		if err := connection.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journal); err != nil {
			test.Fatal(err)
		}
		if err := connection.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&timeout); err != nil {
			test.Fatal(err)
		}
		if err := connection.QueryRowContext(context.Background(), "PRAGMA temp_store").Scan(&temporary); err != nil {
			test.Fatal(err)
		}
		if journal != "wal" || timeout != 10000 || temporary != 2 {
			test.Fatalf("connection profile journal=%q busy_timeout=%d temp_store=%d", journal, timeout, temporary)
		}
	}
}
