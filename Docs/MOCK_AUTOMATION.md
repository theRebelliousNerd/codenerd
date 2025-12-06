# Mock Generation Automation - Quick Reference

## Overview

TesterShard now automatically detects and regenerates stale mocks when interface signatures change. This eliminates the manual work of keeping mocks in sync with interfaces during TDD cycles.

## Quick Start

### Automatic Mode (Recommended)

Mock regeneration happens automatically when running tests:

```go
// Run tests - automatically detects and fixes stale mocks
tester.Execute(ctx, "run_tests file:internal/services/user_test.go")

// Run TDD loop - includes automatic mock regeneration
tester.Execute(ctx, "tdd file:internal/services/user_test.go")
```

### Manual Commands

```go
// Check for stale mocks in a test file
tester.Execute(ctx, "detect_stale_mocks file:path/to/test.go")

// Regenerate mocks for an interface
tester.Execute(ctx, "regenerate_mocks file:path/to/interface.go")
```

## How It Works

### 1. Detection Phase

When tests fail, TesterShard:
- Checks if the failure is mock-related (undefined mock, missing methods, etc.)
- Scans the test file for mock imports and references
- Compares interface methods with mock implementations
- Identifies mocks that are missing methods

### 2. Regeneration Phase

For stale mocks, TesterShard:
- Attempts to use `mockgen` if available
- Falls back to LLM-based generation if mockgen is not installed
- Generates complete mock implementations with all interface methods
- Writes the updated mock file

### 3. Re-test Phase

After regeneration:
- Tests are automatically re-run with updated mocks
- The TDD loop continues until all tests pass

## Supported Frameworks

### Go (mockgen/gomock)

**Detection:**
- `mock_*.go` files in same directory
- `*MockInterface` type references
- `gomock.NewController` usage

**Generation:**
```bash
mockgen -source=interface.go -destination=mock_interface.go -package=pkg
```

Or LLM-generated gomock-style mocks.

### TypeScript/Jest

**Detection:**
- `__mocks__/` directory
- Jest mock syntax

**Generation:**
- Creates mocks in `__mocks__/` directory
- LLM generates Jest-compatible mocks

### Python (unittest.mock)

**Detection:**
- `mock_*.py` files
- `unittest.mock` imports

**Generation:**
- LLM generates mock classes using `unittest.mock` patterns

## Configuration

### Installing mockgen (Optional)

For best results with Go projects:

```bash
go install github.com/golang/mock/mockgen@latest
```

If not installed, TesterShard will use LLM-based generation.

### No Configuration Required

Mock automation works out-of-the-box:
- Auto-detects framework from file extension
- Auto-detects mockgen availability
- Falls back gracefully to LLM generation

## Examples

### Example 1: Interface Change

**Before:**
```go
// user.go
type UserService interface {
    GetUser(id string) (*User, error)
    UpdateUser(user *User) error
}
```

**Change:**
```go
// user.go - Added new method
type UserService interface {
    GetUser(id string) (*User, error)
    UpdateUser(user *User) error
    DeleteUser(id string) error  // NEW
}
```

**Result:**
```
[TesterShard] Running tests...
[TesterShard] Detected mock-related test failure
[TesterShard] Mock missing method: DeleteUser
[TesterShard] Regenerating mock for interface: user.go
[TesterShard] Generated mock: mock_user.go
[TesterShard] Re-running tests...
✓ PASSED
```

### Example 2: Manual Detection

```go
result := tester.Execute(ctx, "detect_stale_mocks file:internal/core/kernel_test.go")
fmt.Println(result)
```

**Output:**
```
Found 2 stale mock(s):
1. internal/core/interfaces.go
2. internal/core/store.go

Use 'regenerate_mocks' to update them.
```

### Example 3: Bulk Regeneration

```go
// Regenerate all mocks for a package
interfaces := []string{
    "internal/core/kernel.go",
    "internal/core/store.go",
    "internal/core/shard.go",
}

for _, iface := range interfaces {
    tester.Execute(ctx, "regenerate_mocks file:" + iface)
}
```

## Error Detection

TesterShard recognizes these mock-related errors:

- `undefined: MockInterface`
- `interface not implemented`
- `method has a pointer receiver`
- `gomock: expected call not found`
- `unexpected call to method`
- `wrong number of calls`
- `mock expectations not met`

## Architecture

```
┌─────────────────────────────────────────┐
│          Test Execution                  │
│  (runTests / runTDDLoop)                 │
└─────────────┬───────────────────────────┘
              │
              ├─ Tests Fail?
              │
              ├─ isMockError() ──> Yes
              │                    │
              ▼                    ▼
    ┌─────────────────┐   ┌──────────────────┐
    │ detectStaleMocks│   │  Continue normal  │
    │                 │   │  error handling   │
    └────────┬────────┘   └──────────────────┘
             │
             ├─ Extract mock imports
             ├─ Check each mock
             │  ├─ Read interface
             │  ├─ Read mock
             │  └─ Compare methods
             │
             ▼
    ┌─────────────────┐
    │ regenerateMock  │
    │                 │
    ├─ Try mockgen ───┤
    │                 │
    ├─ Fallback LLM ─┤
    │                 │
    └────────┬────────┘
             │
             ▼
    ┌─────────────────┐
    │  Write mock file│
    │  Assert facts   │
    └─────────────────┘
```

## Best Practices

1. **Use mockgen for Go** - Install mockgen for faster, more reliable mock generation
2. **Descriptive interfaces** - Well-documented interfaces help LLM generate better mocks
3. **Test files near interfaces** - Keep `*_test.go` files near interface definitions
4. **Review generated mocks** - Especially LLM-generated ones, verify they match expectations
5. **Commit generated mocks** - Check them into version control like any other code

## Troubleshooting

### Mock not detected

**Symptom:** Stale mock not found by detection

**Solutions:**
- Ensure test file imports or references the mock
- Use `*MockInterface` naming convention
- Place mock files in same directory as tests

### Mock generation fails

**Symptom:** Error during regeneration

**Solutions:**
- Check interface syntax is valid
- Ensure interface file is readable
- Try manual `mockgen` command to debug
- Check LLM client is configured if using LLM fallback

### Mock has wrong package

**Symptom:** Generated mock in wrong package

**Solutions:**
- Verify package declaration in interface file
- Check `extractPackageName()` regex matches your format
- Manually specify package if needed

## API Reference

### Public Methods

#### `detectStaleMocks(ctx, testFile) ([]string, error)`
Scans test file for stale mock references.

**Returns:** List of interface file paths with stale mocks

#### `regenerateMock(ctx, interfacePath) error`
Regenerates mock for given interface file.

**Returns:** Error if generation fails

#### `isMockError(output) bool`
Checks if test output indicates mock error.

**Returns:** True if mock-related error detected

### Task Actions

- `detect_stale_mocks` - Check for stale mocks
- `regenerate_mocks` - Regenerate mocks
- Aliases: `check_mocks`, `regen_mocks`, `update_mocks`, `stale_mocks`

## Future Enhancements

Planned improvements:

- [ ] Incremental updates (only add missing methods)
- [ ] Mock dependency analysis
- [ ] Auto-install mockgen if missing
- [ ] Custom mock templates
- [ ] Mock usage analytics
- [ ] Rust/Mockall support
- [ ] Java/Mockito support
- [ ] C#/Moq support

## Support

For issues or questions:
1. Check logs for `[TesterShard]` output
2. Verify interface syntax is valid
3. Test mockgen manually if using Go
4. Review generated mock files

## See Also

- [MOCK_GENERATION_SUMMARY.md](../MOCK_GENERATION_SUMMARY.md) - Full implementation details
- [internal/shards/tester_mocks.go](../internal/shards/tester_mocks.go) - Source code
- [internal/shards/tester_mocks_test.go](../internal/shards/tester_mocks_test.go) - Tests
