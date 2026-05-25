## ⚡ perf(autopoiesis): Extract regexes to global variables in traces.go

### 💡 What
Refactored `internal/autopoiesis/traces.go` to extract 17 regular expressions (previously compiled dynamically via `regexp.MustCompile` inside functions like `extractDecisions`, `extractThoughtSteps`, `injectExitLogging`, etc.) into package-level global variables.

### 🎯 Why
`regexp.MustCompile` is an expensive operation that parses a regex pattern string and compiles it into a state machine. When placed inside repeatedly called functions (like the trace extraction helpers or log injectors which run on arbitrary code/responses), the application spends significant CPU time and memory continuously recompiling the exact same regular expressions over and over again. Extracting these to global variables ensures they are compiled exactly once at application startup.

### 📊 Measured Improvement
I wrote dedicated benchmark tests (`traces_benchmark_test.go`) to measure the impact of this optimization on the key parsing functions.

**Baseline (Repeated `MustCompile`):**
```
BenchmarkExtractDecisions-4             16656      76071 ns/op    23482 B/op      164 allocs/op
BenchmarkExtractThoughtSteps-4          95248      13200 ns/op     3844 B/op       37 allocs/op
BenchmarkExtractAssumptions-4           18994      58160 ns/op    18365 B/op      158 allocs/op
BenchmarkExtractAlternatives-4          16844      70780 ns/op    27375 B/op      190 allocs/op
BenchmarkLogInjectorInjectLogging-4     16453      76546 ns/op    41829 B/op      324 allocs/op
BenchmarkLogInjectorValidateLogging-4  125636       8879 ns/op     5702 B/op       49 allocs/op
```

**Optimized (Global `*regexp.Regexp`):**
```
BenchmarkExtractDecisions-4             32180      36788 ns/op     2121 B/op       16 allocs/op
BenchmarkExtractThoughtSteps-4         194541       6392 ns/op        0 B/op        0 allocs/op
BenchmarkExtractAssumptions-4           48156      27485 ns/op        0 B/op        0 allocs/op
BenchmarkExtractAlternatives-4          42109      28414 ns/op        0 B/op        0 allocs/op
BenchmarkLogInjectorInjectLogging-4     86384      12961 ns/op     5893 B/op       40 allocs/op
BenchmarkLogInjectorValidateLogging-4 1579423        672 ns/op      178 B/op        4 allocs/op
```

**Results:**
- **~2x to 13x Faster Execution Time:** Operations are significantly faster. `ValidateLogging`, which could be called frequently, went from 8,879 ns/op to 672 ns/op (a **~92% reduction** in execution time).
- **Massive Reduction in Allocations:** Several trace parsing functions like `extractThoughtSteps` and `extractAlternatives` went from generating up to 190 allocations per execution down to **0 allocations** directly caused by the regex compilation step, greatly reducing garbage collection pressure.
