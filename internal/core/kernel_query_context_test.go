package core

import (
	"context"
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

func TestQueryContextCompatibility(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
Decl query_context_probe(Value, Row) bound [/any, /string].
query_context_probe(/alice, "name-row").
query_context_probe("/alice", "string-row").
`)
	for _, query := range []string{
		`query_context_probe(/alice, Row)`, `query_context_probe("/alice", Row)`,
		`query_context_probe(Value, Row)`, `query_context_probe`,
		`query_context_probe(`, `query_context_probe(/alice, Row) ignored`,
		`query_context_probe(/alice)`, `unknown_context_probe(Value)`,
	} {
		t.Run(query, func(t *testing.T) {
			ordinary, err := k.Query(query)
			if err != nil {
				t.Fatal(err)
			}
			contextual, err := k.QueryContext(context.Background(), query)
			sort.Slice(ordinary, func(i, j int) bool { return ordinary[i].String() < ordinary[j].String() })
			sort.Slice(contextual, func(i, j int) bool { return contextual[i].String() < contextual[j].String() })
			if err != nil || !reflect.DeepEqual(contextual, ordinary) {
				t.Fatalf("context changed query: got %v, %v; ordinary %v", contextual, err, ordinary)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			rows, err := k.QueryContext(ctx, query)
			if !errors.Is(err, context.Canceled) || rows != nil {
				t.Fatalf("canceled query broadened or succeeded: %v, %v", rows, err)
			}
		})
	}
}

type queryContextHeldReader struct {
	io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (r *queryContextHeldReader) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.entered) })
	<-r.release
	return r.Reader.Read(p)
}

// Production boot supplies a Cortex, not a bare RealKernel. Exercise every
// route while the actual upstream parser is owned by a different operation.
func TestQueryContextRouting(t *testing.T) {
	for _, mode := range []string{"local", "router_owner", "router_fallback", "cortex_owner", "cortex_replica", "cortex_fanout"} {
		t.Run(mode, func(t *testing.T) {
			newShard := func(domain string, owns bool, value int64) *KernelShard {
				owned := []string(nil)
				if owns {
					owned = []string{"query_context_route_probe"}
				}
				s, err := NewKernelShard(KernelShardConfig{Domain: domain, OwnedPredicates: owned, WorkspaceRoot: t.TempDir()})
				if err != nil {
					t.Fatal(err)
				}
				s.AppendPolicy("Decl query_context_route_probe(Value) bound [/number].")
				if mode != "cortex_replica" {
					if err := s.Assert(types.Fact{Predicate: "query_context_route_probe", Args: []any{value}}); err != nil {
						t.Fatal(err)
					}
				}
				return s
			}
			a := newShard("context_a", mode == "cortex_owner", 1)
			b := newShard("context_b", mode == "router_owner", 2)
			query := a.QueryContext
			want := 1
			if strings.HasPrefix(mode, "router_") {
				r := NewShardFactRouter()
				if err := r.RegisterOwner(a, a.OwnedPredicateList()); err != nil {
					t.Fatal(err)
				}
				if err := r.RegisterOwner(b, b.OwnedPredicateList()); err != nil {
					t.Fatal(err)
				}
				a.SetRouter(r)
			}
			if strings.HasPrefix(mode, "cortex_") {
				c := NewCortexKernel(a.Domain())
				if err := c.RegisterShard(a); err != nil {
					t.Fatal(err)
				}
				if err := c.RegisterShard(b); err != nil {
					t.Fatal(err)
				}
				if mode == "cortex_replica" {
					if err := c.SetSharedPredicates([]string{"query_context_route_probe"}); err != nil {
						t.Fatal(err)
					}
					for _, value := range []int64{1, 2} {
						if err := c.Assert(types.Fact{Predicate: "query_context_route_probe", Args: []any{value}}); err != nil {
							t.Fatal(err)
						}
					}
					want = 2
				}
				if mode == "cortex_fanout" {
					want = 2
				}
				query = c.QueryContext
			}
			rows, err := query(context.Background(), "query_context_route_probe(Value)")
			if err != nil || len(rows) != want {
				t.Fatalf("normal route: %v, %v, want %d", rows, err, want)
			}
			values := make([]int64, 0, len(rows))
			for _, row := range rows {
				value, ok := types.ExtractInt64(row.Args[0])
				if !ok {
					t.Fatalf("invalid routed value: %v", row)
				}
				values = append(values, value)
			}
			sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
			expected := []int64{1}
			if mode == "router_owner" {
				expected = []int64{2}
			}
			if want == 2 {
				expected = []int64{1, 2}
			}
			if !reflect.DeepEqual(values, expected) {
				t.Fatalf("wrong route/deduplication: values=%v want=%v", values, expected)
			}
			holder := &queryContextHeldReader{Reader: strings.NewReader("held_context_parser(/ok)."), entered: make(chan struct{}), release: make(chan struct{})}
			parsed := make(chan error, 1)
			go func() { _, err := mangle.ParseUnit(holder); parsed <- err }()
			var release sync.Once
			unblock := func() { release.Do(func() { close(holder.release) }) }
			t.Cleanup(func() {
				unblock()
				select {
				case err := <-parsed:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(5 * time.Second):
					t.Error("parser holder did not join")
				}
			})
			select {
			case <-holder.entered:
			case <-time.After(time.Second):
				t.Fatal("parser holder did not enter")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				rows, err := query(ctx, "query_context_route_probe(Value)")
				if rows != nil {
					err = errors.New("canceled query returned rows")
				}
				finished <- err
			}()
			select {
			case err := <-finished:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("cancellation lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Error("query route lost cancellation while parser owned")
				unblock()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Fatal("query workers did not join")
				}
			}
			unblock()
			after, err := query(context.Background(), "query_context_route_probe(Value)")
			sort.Slice(rows, func(i, j int) bool { return rows[i].String() < rows[j].String() })
			sort.Slice(after, func(i, j int) bool { return after[i].String() < after[j].String() })
			if err != nil || !reflect.DeepEqual(after, rows) {
				t.Fatalf("route changed after cancellation: %v, %v; before %v", after, err, rows)
			}
		})
	}
}
