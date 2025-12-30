# Go Concurrency Patterns

This reference covers production-ready concurrency patterns in Go, with emphasis on patterns that AI agents commonly get wrong.

## Fundamentals

### Goroutine Lifecycle Management

Every goroutine MUST have a guaranteed termination path. The three termination mechanisms:

1. **Normal completion**: Function returns naturally
2. **Context cancellation**: Parent signals shutdown via `ctx.Done()`
3. **Channel close**: Upstream signals no more data

```go
// Pattern: Guaranteed termination via context
func worker(ctx context.Context, jobs <-chan Job) {
    for {
        select {
        case job, ok := <-jobs:
            if !ok {
                return // Channel closed
            }
            process(job)
        case <-ctx.Done():
            return // Context cancelled
        }
    }
}
```

### Channel Buffer Sizing

| Buffer Size | Use Case | Semantics |
|-------------|----------|-----------|
| 0 (unbuffered) | Synchronous handoff | Blocks until receiver ready |
| 1 | Decoupled send/receive | Sender never blocks if receiver catches up |
| N | Known work queue depth | Buffers up to N items |

**Rule**: Default to unbuffered. Use buffer of 1 when sender might be abandoned. Use larger buffers only with explicit capacity planning.

## Worker Pool Pattern

### Basic Worker Pool

```go
func WorkerPool(ctx context.Context, jobs <-chan Job, results chan<- Result, workers int) {
    var wg sync.WaitGroup

    for i := 0; i < workers; i++ {
        wg.Add(1) // BEFORE go statement
        go func(workerID int) {
            defer wg.Done()
            for {
                select {
                case job, ok := <-jobs:
                    if !ok {
                        return // Jobs channel closed
                    }
                    result := processJob(job)
                    select {
                    case results <- result:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return // Shutdown signal
                }
            }
        }(i)
    }

    // Wait for all workers then close results
    go func() {
        wg.Wait()
        close(results)
    }()
}
```

### Bounded Worker Pool with Semaphore

```go
func BoundedParallel(ctx context.Context, items []Item, maxWorkers int) []Result {
    sem := make(chan struct{}, maxWorkers)
    results := make([]Result, len(items))
    var wg sync.WaitGroup

    for i, item := range items {
        wg.Add(1)
        go func(i int, item Item) {
            defer wg.Done()

            // Acquire semaphore
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }() // Release
            case <-ctx.Done():
                return
            }

            results[i] = process(item)
        }(i, item)
    }

    wg.Wait()
    return results
}
```

## Pipeline Pattern

### Linear Pipeline

```go
// Stage 1: Generate
func generate(ctx context.Context, nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for _, n := range nums {
            select {
            case out <- n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 2: Square
func square(ctx context.Context, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out)
        for n := range in {
            select {
            case out <- n * n:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 3: Consume
func consume(ctx context.Context, in <-chan int) {
    for n := range in {
        fmt.Println(n)
    }
}

// Usage
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Pipeline: generate -> square -> consume
    nums := generate(ctx, 1, 2, 3, 4, 5)
    squares := square(ctx, nums)
    consume(ctx, squares)
}
```

### Fan-Out/Fan-In

```go
// Fan-out: Multiple goroutines reading from same channel
func fanOut(ctx context.Context, in <-chan Job, workers int) []<-chan Result {
    outs := make([]<-chan Result, workers)
    for i := 0; i < workers; i++ {
        outs[i] = worker(ctx, in)
    }
    return outs
}

// Fan-in: Merge multiple channels into one
func fanIn(ctx context.Context, channels ...<-chan Result) <-chan Result {
    var wg sync.WaitGroup
    out := make(chan Result)

    // Start output goroutine for each input channel
    output := func(c <-chan Result) {
        defer wg.Done()
        for r := range c {
            select {
            case out <- r:
            case <-ctx.Done():
                return
            }
        }
    }

    wg.Add(len(channels))
    for _, c := range channels {
        go output(c)
    }

    // Close out after all inputs done
    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

## Timeout and Cancellation

### Request Timeout

```go
func fetchWithTimeout(ctx context.Context, url string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return io.ReadAll(resp.Body)
}
```

### First Response Wins

```go
func queryFirstResponse(ctx context.Context, urls []string) ([]byte, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // Cancel other requests when first completes

    results := make(chan []byte, 1) // Buffered - first writer wins
    errs := make(chan error, len(urls))

    for _, url := range urls {
        go func(url string) {
            data, err := fetchWithTimeout(ctx, url)
            if err != nil {
                errs <- err
                return
            }
            select {
            case results <- data:
            default: // Already have result
            }
        }(url)
    }

    select {
    case data := <-results:
        return data, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### Graceful Shutdown

```go
type Server struct {
    srv      *http.Server
    shutdown chan struct{}
    done     chan struct{}
}

func (s *Server) Start() {
    go func() {
        if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
            log.Printf("HTTP server error: %v", err)
        }
        close(s.done)
    }()
}

func (s *Server) Shutdown(ctx context.Context) error {
    close(s.shutdown)

    if err := s.srv.Shutdown(ctx); err != nil {
        return err
    }

    select {
    case <-s.done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Main with signal handling
func main() {
    srv := NewServer()
    srv.Start()

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Fatalf("Shutdown error: %v", err)
    }
}
```

## Synchronization Primitives

### sync.WaitGroup

```go
func processAll(items []Item) {
    var wg sync.WaitGroup

    for _, item := range items {
        wg.Add(1)  // BEFORE go statement - CRITICAL
        go func(item Item) {
            defer wg.Done()  // Guaranteed via defer
            process(item)
        }(item)
    }

    wg.Wait()  // Blocks until all Done() calls
}
```

### sync.Mutex

```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

func (c *SafeCounter) Value() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.count
}
```

### sync.RWMutex (Read-Heavy Workloads)

```go
type Cache struct {
    mu    sync.RWMutex
    items map[string]Item
}

func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()  // Multiple readers OK
    defer c.mu.RUnlock()
    item, ok := c.items[key]
    return item, ok
}

func (c *Cache) Set(key string, item Item) {
    c.mu.Lock()  // Exclusive access
    defer c.mu.Unlock()
    c.items[key] = item
}
```

### sync.Once

```go
var (
    instance *Singleton
    once     sync.Once
)

func GetSingleton() *Singleton {
    once.Do(func() {
        instance = &Singleton{}
        instance.init()
    })
    return instance
}
```

### sync.Pool

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processData(data []byte) string {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    // Use buffer
    buf.Write(data)
    return buf.String()
}
```

### sync.Map

```go
var cache sync.Map

func GetOrCompute(key string) Value {
    if v, ok := cache.Load(key); ok {
        return v.(Value)
    }

    // Compute value (may compute multiple times for same key)
    v := compute(key)
    actual, _ := cache.LoadOrStore(key, v)
    return actual.(Value)
}
```

## Error Handling in Concurrent Code

### Collecting Errors from Goroutines

```go
func processAll(ctx context.Context, items []Item) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(items))

    for _, item := range items {
        wg.Add(1)
        go func(item Item) {
            defer wg.Done()
            if err := process(ctx, item); err != nil {
                errCh <- err
            }
        }(item)
    }

    // Wait and close error channel
    go func() {
        wg.Wait()
        close(errCh)
    }()

    // Collect errors
    var errs []error
    for err := range errCh {
        errs = append(errs, err)
    }

    if len(errs) > 0 {
        return errors.Join(errs...)
    }
    return nil
}
```

### errgroup Package

```go
import "golang.org/x/sync/errgroup"

func processAll(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)

    for _, item := range items {
        item := item // Capture loop variable
        g.Go(func() error {
            return process(ctx, item)
        })
    }

    return g.Wait() // Returns first error, cancels context
}
```

### Bounded errgroup

```go
func processAllBounded(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // Max 10 concurrent goroutines

    for _, item := range items {
        item := item
        g.Go(func() error {
            return process(ctx, item)
        })
    }

    return g.Wait()
}
```

## Rate Limiting

### Token Bucket

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(10, 1) // 10 per second, burst of 1

func processWithRateLimit(ctx context.Context, item Item) error {
    if err := limiter.Wait(ctx); err != nil {
        return err
    }
    return process(item)
}
```

### Sliding Window

```go
type SlidingWindow struct {
    mu      sync.Mutex
    times   []time.Time
    limit   int
    window  time.Duration
}

func (sw *SlidingWindow) Allow() bool {
    sw.mu.Lock()
    defer sw.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-sw.window)

    // Remove old entries
    valid := sw.times[:0]
    for _, t := range sw.times {
        if t.After(cutoff) {
            valid = append(valid, t)
        }
    }
    sw.times = valid

    if len(sw.times) >= sw.limit {
        return false
    }

    sw.times = append(sw.times, now)
    return true
}
```

## Testing Concurrent Code

### Race Detector

```bash
go test -race ./...
```

### Testing with Timeouts

```go
func TestConcurrentOperation(t *testing.T) {
    done := make(chan struct{})

    go func() {
        // Operation under test
        result := concurrentOperation()
        if result != expected {
            t.Errorf("got %v, want %v", result, expected)
        }
        close(done)
    }()

    select {
    case <-done:
        // Success
    case <-time.After(5 * time.Second):
        t.Fatal("test timed out - possible deadlock")
    }
}
```

### Leak Detection (goleak)

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

## Anti-Patterns to Avoid

### 1. Spawning Goroutines in init()

```go
// WRONG
func init() {
    go backgroundTask() // No lifecycle control
}

// CORRECT - Start in main with shutdown control
func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go backgroundTask(ctx)
    // ...
}
```

### 2. Copying Mutex

```go
// WRONG - Mutex copied
func process(m sync.Mutex) { // m is a copy!
    m.Lock()
    defer m.Unlock()
}

// CORRECT - Use pointer
func process(m *sync.Mutex) {
    m.Lock()
    defer m.Unlock()
}
```

### 3. Closing Channel Multiple Times

```go
// WRONG
go func() { close(ch) }()
go func() { close(ch) }() // PANIC

// CORRECT - Single owner closes
owner := make(chan struct{})
go func() {
    // Only owner closes
    close(ch)
}()
```

### 4. Send on Closed Channel

```go
// WRONG
close(ch)
ch <- value // PANIC

// CORRECT - Check before send or use sync primitives
select {
case ch <- value:
case <-done:
    return
}
```

### 5. Unbounded Goroutine Creation

```go
// WRONG - Creates millions of goroutines
for _, item := range millionItems {
    go process(item)
}

// CORRECT - Use worker pool
sem := make(chan struct{}, 100)
for _, item := range millionItems {
    sem <- struct{}{}
    go func(item Item) {
        defer func() { <-sem }()
        process(item)
    }(item)
}
```

## Performance Guidelines

1. **Measure first**: Use `go tool pprof` before optimizing
2. **Reduce contention**: Shard data, use read-write locks
3. **Batch operations**: Amortize goroutine/channel overhead
4. **Limit concurrency**: Unbounded parallelism isn't always faster
5. **Consider GOMAXPROCS**: Default is NumCPU, may need tuning

## References

- [Go Concurrency Patterns (Pike)](https://go.dev/blog/pipelines)
- [Advanced Concurrency Patterns](https://go.dev/blog/advanced-go-concurrency-patterns)
- [Go Memory Model](https://go.dev/ref/mem)
- [sync package documentation](https://pkg.go.dev/sync)
