package autopoiesis

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
)

// Thunderdome is the adversarial testing arena where tools fight for survival.
// It runs attack vectors against compiled tools in isolated sandboxes.
type Thunderdome struct {
	config ThunderdomeConfig
	mu     sync.Mutex
	stats  ThunderdomeStats
}

// ThunderdomeConfig holds configuration for the Thunderdome.
type ThunderdomeConfig struct {
	// Timeout is the maximum duration for a single attack.
	Timeout time.Duration
	// MaxMemoryMB is the memory limit for tool execution.
	MaxMemoryMB int
	// WorkDir is the directory for temporary files.
	WorkDir string
	// KeepArtifacts keeps test artifacts for debugging.
	KeepArtifacts bool
	// ParallelAttacks is the number of attacks to run concurrently.
	ParallelAttacks int
}

// DefaultThunderdomeConfig returns sensible defaults.
func DefaultThunderdomeConfig() ThunderdomeConfig {
	return ThunderdomeConfig{
		Timeout:         5 * time.Second,
		MaxMemoryMB:     100,
		WorkDir:         filepath.Join(os.TempDir(), "thunderdome"),
		KeepArtifacts:   false,
		ParallelAttacks: 1, // Sequential by default for cleaner failure analysis
	}
}

// ThunderdomeStats tracks combat statistics.
type ThunderdomeStats struct {
	TotalBattles   int
	ToolsSurvived  int
	ToolsDefeated  int
	AttacksRun     int
	AttacksFailed  int
	AverageTimeMS  int64
	LongestBattle  time.Duration
	MostDeadlyType string
}

// BattleResult represents the outcome of a tool's Thunderdome trial.
type BattleResult struct {
	ToolName     string
	Survived     bool
	TotalAttacks int
	Failures     int
	Results      []AttackResult
	Duration     time.Duration
	FatalAttack  *AttackVector // The attack that killed the tool (if any)
}

// NewThunderdome creates a new Thunderdome arena.
func NewThunderdome() *Thunderdome {
	return NewThunderdomeWithConfig(DefaultThunderdomeConfig())
}

// NewThunderdomeWithConfig creates a new Thunderdome with custom configuration.
func NewThunderdomeWithConfig(config ThunderdomeConfig) *Thunderdome {
	logging.Autopoiesis("Initializing Thunderdome: timeout=%v, maxMemory=%dMB, parallel=%d",
		config.Timeout, config.MaxMemoryMB, config.ParallelAttacks)

	// Ensure work directory exists
	if err := os.MkdirAll(config.WorkDir, 0755); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to create Thunderdome work dir: %v", err)
	}

	return &Thunderdome{
		config: config,
	}
}

// Battle runs a tool through the gauntlet of attacks.
// Returns the battle result indicating survival or defeat.
func (t *Thunderdome) Battle(ctx context.Context, tool *GeneratedTool, attacks []AttackVector) (*BattleResult, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "Thunderdome.Battle")
	defer timer.Stop()

	logging.Autopoiesis("ENTERING THE THUNDERDOME: Tool=%s, Attacks=%d", tool.Name, len(attacks))

	t.mu.Lock()
	t.stats.TotalBattles++
	t.mu.Unlock()

	result := &BattleResult{
		ToolName:     tool.Name,
		Survived:     true,
		TotalAttacks: len(attacks),
		Results:      make([]AttackResult, 0, len(attacks)),
	}

	startTime := time.Now()

	// Compile the tool for arena combat
	arenaDir, binaryPath, err := t.prepareArena(ctx, tool)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to prepare arena: %v", err)
		return nil, fmt.Errorf("failed to prepare arena: %w", err)
	}

	if !t.config.KeepArtifacts {
		defer os.RemoveAll(arenaDir)
	}

	// Run each attack
	for i, attack := range attacks {
		logging.Autopoiesis("Attack %d/%d: %s (%s)", i+1, len(attacks), attack.Name, attack.Category)

		attackResult := t.runAttack(ctx, binaryPath, attack)
		result.Results = append(result.Results, attackResult)

		t.mu.Lock()
		t.stats.AttacksRun++
		if !attackResult.Survived {
			t.stats.AttacksFailed++
		}
		t.mu.Unlock()

		if !attackResult.Survived {
			logging.Autopoiesis("FATAL: Tool killed by %s (%s)", attack.Name, attackResult.Failure)
			result.Survived = false
			result.Failures++
			result.FatalAttack = &attack

			// Fail fast - no need to run more attacks
			break
		} else {
			logging.AutopoiesisDebug("Survived attack: %s", attack.Name)
		}
	}

	result.Duration = time.Since(startTime)

	// Update stats
	t.mu.Lock()
	if result.Survived {
		t.stats.ToolsSurvived++
		logging.Autopoiesis("THUNDERDOME RESULT: SURVIVED (attacks=%d, time=%v)", result.TotalAttacks, result.Duration)
	} else {
		t.stats.ToolsDefeated++
		logging.Autopoiesis("THUNDERDOME RESULT: DEFEATED by %s (failures=%d/%d, time=%v)",
			result.FatalAttack.Name, result.Failures, result.TotalAttacks, result.Duration)
	}
	if result.Duration > t.stats.LongestBattle {
		t.stats.LongestBattle = result.Duration
	}
	t.mu.Unlock()

	return result, nil
}

// prepareArena sets up the sandboxed environment for combat.
func (t *Thunderdome) prepareArena(ctx context.Context, tool *GeneratedTool) (arenaDir string, binaryPath string, err error) {
	// Create unique arena directory
	arenaDir = filepath.Join(t.config.WorkDir, fmt.Sprintf("arena_%s_%d", tool.Name, time.Now().UnixNano()))
	if err := os.MkdirAll(arenaDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create arena dir: %w", err)
	}

	logging.AutopoiesisDebug("Arena prepared at: %s", arenaDir)

	// FIX: Normalize package name to "tools" to match the harness
	// This fixes the "Package Schism" bug where tools with "package main"
	// would fail to compile alongside the "package tools" harness.
	toolCode := t.normalizePackage(tool.Code)

	// Detect the entry point function using AST parsing
	entryPoint, err := t.findEntryPoint(toolCode)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to find entry point, defaulting to 'Execute': %v", err)
		entryPoint = "Execute"
	}
	logging.AutopoiesisDebug("Detected entry point function: %s", entryPoint)

	// Write the normalized tool code to arena
	sourcePath := filepath.Join(arenaDir, "tool.go")
	if err := os.WriteFile(sourcePath, []byte(toolCode), 0644); err != nil {
		return arenaDir, "", fmt.Errorf("failed to write tool source: %w", err)
	}

	// Create a test harness that wraps the tool and ACTUALLY CALLS IT
	harnessCode := t.generateTestHarness(tool, entryPoint)
	harnessPath := filepath.Join(arenaDir, "harness_test.go")
	if err := os.WriteFile(harnessPath, []byte(harnessCode), 0644); err != nil {
		return arenaDir, "", fmt.Errorf("failed to write test harness: %w", err)
	}

	// Create go.mod
	goModContent := fmt.Sprintf(`module thunderdome_%s

go 1.21
`, tool.Name)
	goModPath := filepath.Join(arenaDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		return arenaDir, "", fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Compile the test binary
	binaryPath = filepath.Join(arenaDir, "arena.test")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-c", "-o", binaryPath, ".")
	cmd.Dir = arenaDir
	// Use unified build environment but disable CGO for sandbox isolation
	cmd.Env = build.MergeEnv(build.GetBuildEnv(nil, arenaDir), "CGO_ENABLED=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	logging.AutopoiesisDebug("Compiling arena binary: %s", binaryPath)
	if err := cmd.Run(); err != nil {
		return arenaDir, "", fmt.Errorf("failed to compile arena binary: %w\nstderr: %s", err, stderr.String())
	}

	logging.AutopoiesisDebug("Arena binary compiled successfully")
	return arenaDir, binaryPath, nil
}

// generateTestHarness creates Go test code that wraps the tool for attack execution.
// FIX: Now accepts entryPoint parameter to actually call the tool's function with attack input.
// This fixes the "Phantom Punch" bug where attack inputs were being discarded.
func (t *Thunderdome) generateTestHarness(_ *GeneratedTool, entryPoint string) string {
	// Generate a test harness that reads attack input from stdin
	// and ACTUALLY executes the tool's entry point with that input
	// NOTE: Must match the tool's package (tools) for Go test to work
	return fmt.Sprintf(`package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestThunderdomeArena(t *testing.T) {
	// Set memory limit
	debug.SetMemoryLimit(%d * 1024 * 1024)

	// Read attack input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	// Use larger buffer for potential attack payloads
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	var input string
	if scanner.Scan() {
		input = scanner.Text()
	}

	// Set up panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "PANIC: %%v\n", r)
			fmt.Fprintf(os.Stderr, "STACK:\n%%s\n", debug.Stack())
			os.Exit(1)
		}
	}()

	// Set up timeout goroutine
	timeoutCh := time.After(%d * time.Second)

	// Monitor memory in background
	go func() {
		for {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.Alloc > %d*1024*1024 {
				fmt.Fprintf(os.Stderr, "OOM: Memory exceeded %%d MB\n", m.Alloc/1024/1024)
				os.Exit(3)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Execute the tool's ACTUAL entry point with the attack input
	done := make(chan struct{})
	var toolErr error
	go func() {
		defer close(done)
		// Create context with timeout for the tool execution
		ctx, cancel := context.WithTimeout(context.Background(), %d*time.Second)
		defer cancel()

		// *** FIX: ACTUALLY CALL THE TOOL'S ENTRY POINT ***
		// This invokes the tool's logic with the attack payload
		_, toolErr = %s(ctx, input)
	}()

	// Wait for completion or timeout
	select {
	case <-done:
		if toolErr != nil {
			// Tool returned an error - this is fine, not a crash
			fmt.Fprintf(os.Stderr, "TOOL_ERROR: %%v\n", toolErr)
		}
		fmt.Println("SURVIVED")
	case <-timeoutCh:
		fmt.Fprintln(os.Stderr, "TIMEOUT: Operation exceeded time limit")
		os.Exit(2)
	}
}
`, t.config.MaxMemoryMB, int(t.config.Timeout.Seconds()), t.config.MaxMemoryMB, int(t.config.Timeout.Seconds()), entryPoint)
}

// normalizePackage ensures the tool code uses "package tools" to match the harness.
// This fixes the "Package Schism" bug where mismatched packages cause compilation failures.
func (t *Thunderdome) normalizePackage(code string) string {
	// Pattern to match package declaration
	packagePattern := regexp.MustCompile(`(?m)^package\s+(\w+)`)
	match := packagePattern.FindStringSubmatch(code)

	if len(match) < 2 {
		// No package found, prepend one
		return "package tools\n\n" + code
	}

	if match[1] == "tools" {
		// Already correct
		return code
	}

	// Replace the package name with "tools"
	logging.AutopoiesisDebug("Normalizing package from '%s' to 'tools'", match[1])
	return packagePattern.ReplaceAllString(code, "package tools")
}

// findEntryPoint uses AST parsing to locate the tool's main entry function.
// It looks for exported functions with context + string signatures that match
// the standard tool interface: func Name(ctx context.Context, input string) (string, error)
func (t *Thunderdome) findEntryPoint(code string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "tool.go", code, 0)
	if err != nil {
		return "", fmt.Errorf("failed to parse tool code: %w", err)
	}

	var bestFunc string
	var bestScore int

	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Skip main, init, and test functions
		name := fn.Name.Name
		if name == "main" || name == "init" || strings.HasPrefix(name, "Test") ||
			strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Register") {
			return true
		}

		score := 0

		// Exported functions are preferred
		if fn.Name.IsExported() {
			score += 10
		}

		// Check parameters - prefer (context.Context, string)
		if fn.Type.Params != nil {
			params := fn.Type.Params.List
			if len(params) >= 2 {
				// First param should be context
				if t.isContextParam(params[0]) {
					score += 15
				}
				// Second param should be string
				if t.isStringParam(params[1]) {
					score += 10
				}
			} else if len(params) == 1 {
				// Single param functions are acceptable
				if t.isContextParam(params[0]) {
					score += 5
				} else if t.isStringParam(params[0]) {
					score += 5
				}
			}
		}

		// Check return type - prefer (string, error) or (T, error)
		if fn.Type.Results != nil {
			results := fn.Type.Results.List
			if len(results) == 2 {
				score += 10
				// Check if second return is error
				if t.isErrorResult(results[1]) {
					score += 5
				}
			} else if len(results) == 1 {
				score += 3
			}
		}

		// Prefer common tool function names
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "execute") ||
			strings.Contains(lowerName, "run") ||
			strings.Contains(lowerName, "process") ||
			strings.Contains(lowerName, "handle") {
			score += 8
		}

		if score > bestScore {
			bestScore = score
			bestFunc = name
		}

		return true
	})

	if bestFunc == "" {
		return "", fmt.Errorf("no suitable entry point function found in tool code")
	}

	logging.AutopoiesisDebug("Found entry point '%s' with score %d", bestFunc, bestScore)
	return bestFunc, nil
}

// isContextParam checks if a parameter is context.Context
func (t *Thunderdome) isContextParam(field *ast.Field) bool {
	sel, ok := field.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "context" && sel.Sel.Name == "Context"
}

// isStringParam checks if a parameter is a string type
func (t *Thunderdome) isStringParam(field *ast.Field) bool {
	ident, ok := field.Type.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "string"
}

// isErrorResult checks if a result type is error
func (t *Thunderdome) isErrorResult(field *ast.Field) bool {
	ident, ok := field.Type.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "error"
}

// runAttack executes a single attack against the tool.
func (t *Thunderdome) runAttack(ctx context.Context, binaryPath string, attack AttackVector) AttackResult {
	result := AttackResult{
		Vector:   attack,
		Survived: true,
	}

	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, t.config.Timeout+2*time.Second)
	defer cancel()

	// Run the test binary with attack input
	cmd := exec.CommandContext(ctx, binaryPath, "-test.run=TestThunderdomeArena", "-test.v")

	// Pipe attack input via stdin
	cmd.Stdin = strings.NewReader(attack.Input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.Duration = time.Since(startTime).Milliseconds()

	// Analyze the result
	if err != nil {
		result.Survived = false
		stderrStr := stderr.String()

		// Categorize the failure
		switch {
		case strings.Contains(stderrStr, "PANIC:"):
			result.Failure = "panic"
			// Extract stack trace
			if idx := strings.Index(stderrStr, "STACK:"); idx != -1 {
				result.StackDump = stderrStr[idx:]
			}
		case strings.Contains(stderrStr, "TIMEOUT:"):
			result.Failure = "timeout"
		case strings.Contains(stderrStr, "OOM:"):
			result.Failure = "oom"
		case strings.Contains(stderrStr, "deadlock"):
			result.Failure = "deadlock"
		case ctx.Err() == context.DeadlineExceeded:
			result.Failure = "timeout (context)"
		default:
			result.Failure = fmt.Sprintf("exit error: %v", err)
		}

		logging.AutopoiesisDebug("Attack '%s' caused failure: %s", attack.Name, result.Failure)
	} else {
		logging.AutopoiesisDebug("Attack '%s' defended successfully", attack.Name)
	}

	return result
}

// GetStats returns the current Thunderdome statistics.
func (t *Thunderdome) GetStats() ThunderdomeStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.stats
}

// ResetStats clears the statistics.
func (t *Thunderdome) ResetStats() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats = ThunderdomeStats{}
}

// FormatBattleResultForFeedback creates a feedback string for tool regeneration.
func (t *Thunderdome) FormatBattleResultForFeedback(result *BattleResult) string {
	if result.Survived {
		return fmt.Sprintf("Tool '%s' SURVIVED The Thunderdome (%d attacks defended, %v)",
			result.ToolName, result.TotalAttacks, result.Duration)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tool '%s' was DEFEATED in The Thunderdome\n\n", result.ToolName))
	sb.WriteString(fmt.Sprintf("Fatal Attack: %s (%s)\n", result.FatalAttack.Name, result.FatalAttack.Category))
	sb.WriteString(fmt.Sprintf("Input: %s\n", truncateString(result.FatalAttack.Input, 200)))
	sb.WriteString(fmt.Sprintf("Failure Mode: %s\n", result.Results[len(result.Results)-1].Failure))

	if stackDump := result.Results[len(result.Results)-1].StackDump; stackDump != "" {
		sb.WriteString(fmt.Sprintf("\nStack Trace:\n%s\n", truncateString(stackDump, 1000)))
	}

	sb.WriteString("\nThe tool must be regenerated with fixes for this vulnerability.\n")

	return sb.String()
}
