# Mock Generation Automation for TesterShard

## Overview

Implemented automatic mock generation and detection capabilities for TesterShard, enabling it to automatically detect and regenerate stale mocks when tests fail due to interface changes.

## Files Created/Modified

### Created Files

1. **`internal/shards/tester_mocks.go`** (~600 lines)
   - Core mock generation functionality
   - Stale mock detection
   - Multi-language support (Go, TypeScript/Jest, Python)

2. **`internal/shards/tester_mocks_test.go`** (~350 lines)
   - Comprehensive unit tests for mock functionality
   - Tests for detection, extraction, and parsing

### Modified Files

1. **`internal/shards/tester.go`**
   - Added mock error detection in `runTests()`
   - Added task handlers for `regenerate_mocks` and `detect_stale_mocks`
   - Updated task parser to recognize new actions
   - Integrated automatic mock regeneration into TDD loop

## Key Features

### 1. Stale Mock Detection

**Method:** `detectStaleMocks(ctx, testFile) ([]string, error)`

Scans test files for:
- Mock file references (`mock_*.go`)
- Mock import statements
- gomock usage patterns
- Mock type references (`*MockInterface`)

Checks staleness by:
- Verifying mock file exists
- Comparing interface methods with mock methods
- Identifying missing methods in mocks

### 2. Mock Regeneration

**Method:** `regenerateMock(ctx, interfacePath) error`

Supports multiple approaches:
1. **mockgen (preferred)** - Uses `go get github.com/golang/mock/mockgen` if available
2. **LLM fallback** - Generates mocks via AI when mockgen unavailable
3. **Multi-language** - Framework-specific generation for Go, Jest, Python

### 3. Framework Support

#### Go (gotest)
- Uses `mockgen -source=` command
- Generates gomock-compatible mocks
- Falls back to LLM if mockgen not installed

#### TypeScript/Jest
- Creates mocks in `__mocks__/` directory
- LLM-generated Jest-compatible mocks

#### Python (pytest)
- Generates mocks using unittest.mock patterns
- Creates mock factory classes

### 4. Automatic Integration

The TDD loop now automatically:
1. Detects mock-related test failures
2. Identifies stale mocks in the test file
3. Regenerates outdated mocks
4. Re-runs tests with updated mocks

### 5. Task Commands

New task formats:
```
# Detect stale mocks in a test file
detect_stale_mocks file:internal/core/kernel_test.go

# Regenerate mocks for an interface
regenerate_mocks file:internal/core/interfaces.go

# Aliases
check_mocks file:test.go
regen_mocks file:service.go
update_mocks file:repo.go
```

## Implementation Details

### Mock Error Detection

Identifies mock-related failures by checking for keywords:
- `mock`, `gomock`, `mockgen`
- `unexpected call`, `missing call`
- `interface not implemented`
- `method has a pointer receiver`
- `undefined: Mock*`

### Interface Method Extraction

Uses regex patterns to extract:
- Interface definitions: `type InterfaceName interface`
- Method signatures: `MethodName(params) returns`
- Package information

### Mock Method Extraction

Parses mock implementations:
- Mock receiver methods: `func (m *MockType) MethodName`
- Validates method coverage

### LLM Mock Generation

When mockgen unavailable:
- Reads interface definition
- Generates system prompt with framework guidelines
- Produces production-ready mock code
- Writes to appropriate mock file location

## Usage Examples

### Example 1: Automatic Mock Update in TDD Loop

```go
// Interface changes
type UserService interface {
    GetUser(id string) (*User, error)
    UpdateUser(user *User) error
    DeleteUser(id string) error  // NEW METHOD
}

// Run TDD loop - automatically detects and updates mocks
tester.Execute(ctx, "tdd file:internal/services/user_test.go")
```

Output:
```
[TesterShard] Starting TDD loop for internal/services/user_test.go
[TesterShard] Detected mock-related test failure, checking for stale mocks...
[TesterShard] Mock missing method: DeleteUser
[TesterShard] Found 1 stale mock(s), attempting regeneration...
[TesterShard] Regenerating mock for interface: internal/services/user.go
[TesterShard] Generated mock: internal/services/mock_user.go
```

### Example 2: Manual Mock Detection

```go
tester.Execute(ctx, "detect_stale_mocks file:internal/core/kernel_test.go")
```

Output:
```
Found 2 stale mock(s):
1. internal/core/interfaces.go
2. internal/core/store.go

Use 'regenerate_mocks' to update them.
```

### Example 3: Manual Mock Regeneration

```go
tester.Execute(ctx, "regenerate_mocks file:internal/core/interfaces.go")
```

Output:
```
Successfully regenerated mock for interface: internal/core/interfaces.go
```

## Architecture

### Component Flow

```
Test Failure
    ↓
isMockError() - Detect mock-related error
    ↓
detectStaleMocks() - Find stale mocks
    ↓
    ├─ extractMockImports() - Parse test file
    ├─ isMockStale() - Check each mock
    │   ├─ extractInterfaceMethods()
    │   └─ extractMockMethods()
    ↓
regenerateMock() - Update mocks
    ↓
    ├─ regenerateGoMock() - Try mockgen
    │   └─ generateMockViaLLM() - Fallback
    ├─ regenerateJestMock() - Jest mocks
    └─ regeneratePytestMock() - Python mocks
```

### Fact Assertions

New kernel facts:
- `mock_generated/3` - Records mock generation events
- Includes: mock path, interface path, timestamp

## Testing

Comprehensive test coverage:
- `TestIsMockError` - Error detection patterns
- `TestExtractMockImports` - Import parsing
- `TestExtractInterfaceMethods` - Interface parsing
- `TestExtractMockMethods` - Mock parsing
- `TestExtractPackageName` - Go package extraction
- `TestExtractInterfaceNames` - Interface discovery
- `TestBuildMockGenSystemPrompt` - Prompt generation
- `TestParseTaskWithMockActions` - Task parsing

All tests passing ✓

## Benefits

1. **Reduced Manual Work** - No manual mock updates when interfaces change
2. **Faster TDD Cycles** - Automatic mock regeneration in test loops
3. **Multi-Language Support** - Works with Go, TypeScript, Python
4. **Graceful Degradation** - Falls back to LLM when tools unavailable
5. **Smart Detection** - Only regenerates when actually stale
6. **Kernel Integration** - Facts tracked for learning patterns

## Future Enhancements

Potential improvements:
1. Support for more frameworks (Rust, Java, C#)
2. Incremental mock updates (only add missing methods)
3. Mock usage analytics (which mocks are most problematic)
4. Auto-install mockgen if missing
5. Mock versioning and rollback
6. Custom mock templates per project

## Configuration

No special configuration required. Works with existing TesterConfig:
- Auto-detects framework
- Auto-detects mockgen availability
- Falls back to LLM generation
- Integrates with existing TDD loop settings

## Notes

- Mock regeneration preserves package structure
- LLM-generated mocks follow framework best practices
- Stale detection compares method signatures only (not implementations)
- Compatible with gomock, Jest manual mocks, unittest.mock patterns
- No external dependencies required (mockgen optional)
