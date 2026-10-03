package perception

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	storepkg "codenerd/internal/store"
)

type learnedBootstrapSQLContext struct {
	context.Context
	entered     chan struct{}
	schemaCalls atomic.Int64
}

func (parent *learnedBootstrapSQLContext) Done() <-chan struct{} {
	var callers [32]uintptr
	frames := runtime.CallersFrames(callers[:runtime.Callers(2, callers[:])])
	statement, schema := false, false
	for {
		frame, more := frames.Next()
		statement = statement || strings.Contains(frame.Function, "go-sqlite3.(*SQLiteStmt).exec")
		schema = schema || strings.Contains(frame.Function, "store.(*LearnedCorpusStore).initializeSchemaWithContext")
		if !more {
			break
		}
	}
	// The second Done call is the driver's select after launching sqlite3_step.
	// Observe the real schema operation, rather than an earlier pool admission.
	if statement && schema && parent.schemaCalls.Add(1) == 2 {
		parent.entered <- struct{}{}
	}
	return parent.Context.Done()
}

type learnedBootstrapEngine struct {
	embeds   atomic.Int64
	closes   atomic.Int64
	closeErr error
}

func (engine *learnedBootstrapEngine) Name() string    { return "learned-bootstrap-test" }
func (engine *learnedBootstrapEngine) Dimensions() int { return 2 }
func (engine *learnedBootstrapEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	engine.embeds.Add(1)
	return []float32{1, 2}, nil
}
func (engine *learnedBootstrapEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for index, text := range texts {
		vector, err := engine.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[index] = vector
	}
	return vectors, nil
}
func (engine *learnedBootstrapEngine) Close() error {
	engine.closes.Add(1)
	return engine.closeErr
}

func awaitLearnedBootstrap[Value any](test *testing.T, events <-chan Value) Value {
	test.Helper()
	select {
	case value := <-events:
		return value
	case <-time.After(20 * time.Second):
		test.Fatal("learned SQL bootstrap did not reach its barrier")
		var zero Value
		return zero
	}
}

func learnedBootstrapSnapshot(test *testing.T, path string) []byte {
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

func TestLearnedBootstrapContext_RealSQLLockCancellationAndRetry(test *testing.T) {
	for _, route := range []string{"corpus", "classifier"} {
		for _, cause := range []string{"cancel", "deadline"} {
			test.Run(route+"/"+cause, func(test *testing.T) {
				workspace := coldClassifierWorkspace(test)
				path := filepath.Join(workspace, ".nerd", "learned_patterns.db")
				cleanupFailure := errors.New("learned bootstrap engine cleanup failure")
				engine := &learnedBootstrapEngine{closeErr: cleanupFailure}
				seed, err := storepkg.NewLearnedCorpusStore(path, engine)
				if err != nil {
					test.Fatal(err)
				}
				if err := seed.AddPattern(context.Background(), "fix the application", "fix", "application", "", 0.9); err != nil {
					_ = seed.Close()
					test.Fatal(err)
				}
				if err := seed.Close(); err != nil {
					test.Fatal(err)
				}
				before := learnedBootstrapSnapshot(test, path)
				beforeEmbeds := engine.embeds.Load()
				lockDB, err := sql.Open("sqlite3", path)
				if err != nil {
					test.Fatal(err)
				}
				lockConnection, err := lockDB.Conn(context.Background())
				if err != nil {
					_ = lockDB.Close()
					test.Fatal(err)
				}
				var unlockOnce sync.Once
				var unlockErr error
				unlock := func() {
					unlockOnce.Do(func() {
						_, rollbackErr := lockConnection.ExecContext(context.Background(), "ROLLBACK")
						unlockErr = errors.Join(rollbackErr, lockConnection.Close(), lockDB.Close())
					})
				}
				test.Cleanup(unlock)
				if _, err := lockConnection.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
					test.Fatal(err)
				}
				base, cancel := context.WithCancel(context.Background())
				expected := error(context.Canceled)
				if cause == "deadline" {
					cancel()
					base, cancel = context.WithTimeout(context.Background(), 4*time.Second)
					expected = context.DeadlineExceeded
				}
				parent := &learnedBootstrapSQLContext{Context: base, entered: make(chan struct{}, 1)}
				type result struct {
					corpus     *LearnedCorpusStore
					classifier *SemanticClassifier
					err        error
				}
				finished := make(chan result, 1)
				done := make(chan struct{})
				test.Cleanup(func() {
					cancel()
					unlock()
					awaitLearnedBootstrap(test, done)
				})
				go func() {
					defer close(done)
					if route == "corpus" {
						corpus, err := NewLearnedCorpusStoreWithContext(parent, &config.UserConfig{}, 2, engine)
						finished <- result{corpus: corpus, err: err}
						return
					}
					classifier, err := newSemanticClassifierFromFactory(parent, coldClassifierKernel(0), &config.UserConfig{}, intentHydrateTimeout,
						func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) { return engine, nil })
					finished <- result{classifier: classifier, err: err}
				}()
				awaitLearnedBootstrap(test, parent.entered)
				select {
				case outcome := <-finished:
					test.Fatalf("constructor bypassed the real SQL write lock: %+v", outcome)
				default:
				}
				if cause == "cancel" {
					cancel()
				}
				awaitLearnedBootstrap(test, base.Done())
				unlock()
				if unlockErr != nil {
					test.Fatal(unlockErr)
				}
				outcome := awaitLearnedBootstrap(test, finished)
				awaitLearnedBootstrap(test, done)
				if outcome.corpus != nil || outcome.classifier != nil || !errors.Is(outcome.err, expected) || engine.embeds.Load() != beforeEmbeds {
					test.Fatalf("canceled SQL bootstrap published state or embedded again: %+v, embeds=%d", outcome, engine.embeds.Load())
				}
				if route == "classifier" && (!errors.Is(outcome.err, cleanupFailure) || engine.closes.Load() != 1) {
					test.Fatalf("classifier lost engine cleanup: err=%v closes=%d", outcome.err, engine.closes.Load())
				}
				if after := learnedBootstrapSnapshot(test, path); !reflect.DeepEqual(before, after) {
					test.Fatalf("canceled normal learned bootstrap changed schema, patterns or vectors\nbefore=%s\nafter=%s", before, after)
				}
				retryEngine := &learnedBootstrapEngine{}
				retried, err := NewLearnedCorpusStoreWithContext(context.Background(), &config.UserConfig{}, 2, retryEngine)
				if err != nil || retried == nil || !retried.HasBackend() {
					test.Fatalf("healthy learned bootstrap retry=(%v, %v)", retried, err)
				}
				matches, searchErr := retried.Search([]float32{1, 2}, 5)
				if err := errors.Join(searchErr, retried.Close()); err != nil || len(matches) != 1 || matches[0].TextContent != "fix the application" {
					test.Fatalf("retry lost durable learned content: matches=%v err=%v", matches, err)
				}
				if err := os.Rename(path, path+".reopened"); err != nil {
					test.Fatalf("bootstrap retained a learned database handle after return: %v", err)
				}
			})
		}
	}
}

func TestLearnedBootstrapContext_PreCanceledHasNoFilesystemEffects(test *testing.T) {
	for _, cause := range []string{"cancel", "deadline"} {
		test.Run(cause, func(test *testing.T) {
			workspace := coldClassifierWorkspace(test)
			parent, cancel := context.WithCancel(context.Background())
			expected := error(context.Canceled)
			if cause == "deadline" {
				cancel()
				parent, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				expected = context.DeadlineExceeded
			} else {
				cancel()
			}
			defer cancel()
			engine := &learnedBootstrapEngine{}
			corpus, err := NewLearnedCorpusStoreWithContext(parent, &config.UserConfig{}, 2, engine)
			if corpus != nil || !errors.Is(err, expected) || engine.embeds.Load() != 0 {
				test.Fatalf("pre-canceled learned bootstrap=(%v, %v)", corpus, err)
			}
			if _, err := os.Stat(filepath.Join(workspace, ".nerd")); !errors.Is(err, os.ErrNotExist) {
				test.Fatalf("pre-canceled learned bootstrap touched the filesystem: %v", err)
			}
		})
	}
}

func TestLearnedBootstrapContext_AliveParentUnavailablePersistenceDegrades(test *testing.T) {
	workspace := coldClassifierWorkspace(test)
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd", "learned_patterns.db"), 0o755); err != nil {
		test.Fatal(err)
	}
	engine := &learnedBootstrapEngine{}
	classifier, err := newSemanticClassifierFromFactory(context.Background(), coldClassifierKernel(0), &config.UserConfig{}, intentHydrateTimeout,
		func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) { return engine, nil })
	if err != nil || classifier == nil || classifier.learnedStore != nil || classifier.embedEngine != engine || engine.closes.Load() != 0 {
		test.Fatalf("alive-parent persistence outage did not retain the optional classifier: classifier=%v err=%v closes=%d", classifier, err, engine.closes.Load())
	}
	if err := classifier.Close(); err != nil || engine.closes.Load() != 1 {
		test.Fatalf("degraded classifier did not release its engine: err=%v closes=%d", err, engine.closes.Load())
	}
}
