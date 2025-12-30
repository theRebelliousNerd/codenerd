# Intent Mutations

# =============================================================================
# SECTION 6: BUG FIXES (/fix) - CODER SHARD
# Requests to fix bugs, errors, and issues.
# =============================================================================

intent_definition("Fix this bug.", /fix, "bug").
intent_category("Fix this bug.", /mutation).

intent_definition("Fix the bug.", /fix, "bug").
intent_category("Fix the bug.", /mutation).

intent_definition("Fix this.", /fix, "context_file").
intent_category("Fix this.", /mutation).

intent_definition("Fix the error.", /fix, "error").
intent_category("Fix the error.", /mutation).

intent_definition("Fix the compilation error.", /fix, "compiler_error").
intent_category("Fix the compilation error.", /mutation).

intent_definition("Fix the build error.", /fix, "build_error").
intent_category("Fix the build error.", /mutation).

intent_definition("Fix the syntax error.", /fix, "syntax_error").
intent_category("Fix the syntax error.", /mutation).

intent_definition("Fix the type error.", /fix, "type_error").
intent_category("Fix the type error.", /mutation).

intent_definition("Can you fix this?", /fix, "context_file").
intent_category("Can you fix this?", /mutation).

intent_definition("Resolve this issue.", /fix, "issue").
intent_category("Resolve this issue.", /mutation).

intent_definition("Fix the failing test.", /fix, "test_failure").
intent_category("Fix the failing test.", /mutation).

intent_definition("Fix the test.", /fix, "test").
intent_category("Fix the test.", /mutation).

intent_definition("Make the tests pass.", /fix, "tests").
intent_category("Make the tests pass.", /mutation).

intent_definition("Fix the lint errors.", /fix, "lint").
intent_category("Fix the lint errors.", /mutation).

intent_definition("Fix the linting issues.", /fix, "lint").
intent_category("Fix the linting issues.", /mutation).

intent_definition("Repair this code.", /fix, "context_file").
intent_category("Repair this code.", /mutation).

intent_definition("Patch this bug.", /fix, "bug").
intent_category("Patch this bug.", /mutation).

intent_definition("Fix the crash.", /fix, "crash").
intent_category("Fix the crash.", /mutation).

intent_definition("Fix the panic.", /fix, "panic").
intent_category("Fix the panic.", /mutation).

intent_definition("Fix the nil pointer.", /fix, "nil_pointer").
intent_category("Fix the nil pointer.", /mutation).

intent_definition("Fix the race condition.", /fix, "race_condition").
intent_category("Fix the race condition.", /mutation).

intent_definition("Fix the deadlock.", /fix, "deadlock").
intent_category("Fix the deadlock.", /mutation).

intent_definition("Fix the memory leak.", /fix, "memory_leak").
intent_category("Fix the memory leak.", /mutation).

intent_definition("Fix the import error.", /fix, "import_error").
intent_category("Fix the import error.", /mutation).

intent_definition("Fix the undefined error.", /fix, "undefined").
intent_category("Fix the undefined error.", /mutation).

intent_definition("Fix the null reference.", /fix, "null_reference").
intent_category("Fix the null reference.", /mutation).

intent_definition("Resolve the conflict.", /fix, "conflict").
intent_category("Resolve the conflict.", /mutation).

intent_definition("Fix merge conflicts.", /fix, "merge_conflicts").
intent_category("Fix merge conflicts.", /mutation).


# =============================================================================
# SECTION 8: REFACTORING (/refactor) - CODER SHARD
# Code improvement and restructuring requests.
# =============================================================================

intent_definition("Refactor this.", /refactor, "context_file").
intent_category("Refactor this.", /mutation).

intent_definition("Refactor this function.", /refactor, "function").
intent_category("Refactor this function.", /mutation).

intent_definition("Refactor this function to be cleaner.", /refactor, "focused_symbol").
intent_category("Refactor this function to be cleaner.", /mutation).

intent_definition("Clean up this code.", /refactor, "context_file").
intent_category("Clean up this code.", /mutation).

intent_definition("Clean this up.", /refactor, "context_file").
intent_category("Clean this up.", /mutation).

intent_definition("Improve this function.", /refactor, "focused_symbol").
intent_category("Improve this function.", /mutation).

intent_definition("Make this more readable.", /refactor, "context_file").
intent_category("Make this more readable.", /mutation).

intent_definition("Optimize this code.", /refactor, "context_file").
intent_category("Optimize this code.", /mutation).

intent_definition("Optimize this.", /refactor, "context_file").
intent_category("Optimize this.", /mutation).

intent_definition("Simplify this.", /refactor, "context_file").
intent_category("Simplify this.", /mutation).

intent_definition("Simplify this function.", /refactor, "function").
intent_category("Simplify this function.", /mutation).

intent_definition("Make this cleaner.", /refactor, "context_file").
intent_category("Make this cleaner.", /mutation).

intent_definition("Improve readability.", /refactor, "context_file").
intent_category("Improve readability.", /mutation).

intent_definition("Reduce complexity.", /refactor, "context_file").
intent_category("Reduce complexity.", /mutation).

intent_definition("Extract this into a function.", /refactor, "extract_function").
intent_category("Extract this into a function.", /mutation).

intent_definition("Extract method.", /refactor, "extract_method").
intent_category("Extract method.", /mutation).

intent_definition("Inline this function.", /refactor, "inline").
intent_category("Inline this function.", /mutation).

intent_definition("Rename this variable.", /refactor, "rename").
intent_category("Rename this variable.", /mutation).

intent_definition("Rename this function.", /refactor, "rename").
intent_category("Rename this function.", /mutation).

intent_definition("Restructure this.", /refactor, "context_file").
intent_category("Restructure this.", /mutation).

intent_definition("Reorganize this code.", /refactor, "context_file").
intent_category("Reorganize this code.", /mutation).

intent_definition("Make this DRY.", /refactor, "dry").
intent_category("Make this DRY.", /mutation).

intent_definition("Remove duplication.", /refactor, "duplication").
intent_category("Remove duplication.", /mutation).

intent_definition("Apply SOLID principles.", /refactor, "solid").
intent_category("Apply SOLID principles.", /mutation).

intent_definition("Make this more idiomatic.", /refactor, "idiomatic").
intent_category("Make this more idiomatic.", /mutation).

intent_definition("Make this more Go-like.", /refactor, "idiomatic_go").
intent_category("Make this more Go-like.", /mutation).

intent_definition("Apply Go best practices.", /refactor, "best_practices").
intent_category("Apply Go best practices.", /mutation).


# =============================================================================
# SECTION 9: CODE CREATION (/create) - CODER SHARD
# Requests to create new files, functions, or code.
# =============================================================================

intent_definition("Create a new file.", /create, "file").
intent_category("Create a new file.", /mutation).

intent_definition("Create a new file called main.go.", /create, "main.go").
intent_category("Create a new file called main.go.", /mutation).

intent_definition("Make a new file.", /create, "file").
intent_category("Make a new file.", /mutation).

intent_definition("Create a new Go file.", /create, "go_file").
intent_category("Create a new Go file.", /mutation).

intent_definition("Create a new function.", /create, "function").
intent_category("Create a new function.", /mutation).

intent_definition("Write a function that does X.", /create, "function").
intent_category("Write a function that does X.", /mutation).

intent_definition("Implement this function.", /create, "function").
intent_category("Implement this function.", /mutation).

intent_definition("Create a new struct.", /create, "struct").
intent_category("Create a new struct.", /mutation).

intent_definition("Create a new interface.", /create, "interface").
intent_category("Create a new interface.", /mutation).

intent_definition("Create a new package.", /create, "package").
intent_category("Create a new package.", /mutation).

intent_definition("Add a new endpoint.", /create, "endpoint").
intent_category("Add a new endpoint.", /mutation).

intent_definition("Create an API handler.", /create, "api_handler").
intent_category("Create an API handler.", /mutation).

intent_definition("Scaffold a new service.", /create, "service").
intent_category("Scaffold a new service.", /mutation).

intent_definition("Generate boilerplate.", /create, "boilerplate").
intent_category("Generate boilerplate.", /mutation).

intent_definition("Create a CLI command.", /create, "cli_command").
intent_category("Create a CLI command.", /mutation).

intent_definition("Add a new command.", /create, "command").
intent_category("Add a new command.", /mutation).

intent_definition("Write the implementation.", /create, "implementation").
intent_category("Write the implementation.", /mutation).

intent_definition("Implement this.", /create, "context_file").
intent_category("Implement this.", /mutation).

intent_definition("Add this feature.", /create, "feature").
intent_category("Add this feature.", /mutation).

intent_definition("Add a method to do X.", /create, "method").
intent_category("Add a method to do X.", /mutation).

intent_definition("Create a helper function.", /create, "helper").
intent_category("Create a helper function.", /mutation).

intent_definition("Create a utility function.", /create, "utility").
intent_category("Create a utility function.", /mutation).

intent_definition("Add error handling.", /create, "error_handling").
intent_category("Add error handling.", /mutation).

intent_definition("Add logging.", /create, "logging").
intent_category("Add logging.", /mutation).

intent_definition("Add validation.", /create, "validation").
intent_category("Add validation.", /mutation).


# =============================================================================
# SECTION 10: DELETE (/delete) - CODER SHARD
# Dangerous delete operations - require confirmation.
# =============================================================================

intent_definition("Delete this file.", /delete, "context_file").
intent_category("Delete this file.", /mutation).

intent_definition("Delete the database.", /delete, "database").
intent_category("Delete the database.", /mutation).

intent_definition("Remove this file.", /delete, "context_file").
intent_category("Remove this file.", /mutation).

intent_definition("Delete the tests.", /delete, "tests").
intent_category("Delete the tests.", /mutation).

intent_definition("Remove this function.", /delete, "function").
intent_category("Remove this function.", /mutation).

intent_definition("Delete this code.", /delete, "context_file").
intent_category("Delete this code.", /mutation).

intent_definition("Remove dead code.", /delete, "dead_code").
intent_category("Remove dead code.", /mutation).

intent_definition("Delete unused imports.", /delete, "unused_imports").
intent_category("Delete unused imports.", /mutation).

intent_definition("Remove unused variables.", /delete, "unused_vars").
intent_category("Remove unused variables.", /mutation).

intent_definition("Clean up unused code.", /delete, "unused_code").
intent_category("Clean up unused code.", /mutation).

intent_definition("Delete this package.", /delete, "package").
intent_category("Delete this package.", /mutation).


# =============================================================================
# SECTION 11: TESTING (/test) - TESTER SHARD
# Test-related requests.
# =============================================================================

intent_definition("Run the tests.", /test, "all").
intent_category("Run the tests.", /mutation).

intent_definition("Run tests.", /test, "all").
intent_category("Run tests.", /mutation).

intent_definition("Run all tests.", /test, "all").
intent_category("Run all tests.", /mutation).

intent_definition("Test this.", /test, "context_file").
intent_category("Test this.", /mutation).

intent_definition("Test this file.", /test, "context_file").
intent_category("Test this file.", /mutation).

intent_definition("Test this function.", /test, "function").
intent_category("Test this function.", /mutation).

intent_definition("Generate unit tests.", /test, "unit").
intent_category("Generate unit tests.", /mutation).

intent_definition("Generate unit tests for this.", /test, "context_file").
intent_category("Generate unit tests for this.", /mutation).

intent_definition("Write tests for this.", /test, "context_file").
intent_category("Write tests for this.", /mutation).

intent_definition("Write tests for this function.", /test, "focused_symbol").
intent_category("Write tests for this function.", /mutation).

intent_definition("Add test coverage.", /test, "coverage").
intent_category("Add test coverage.", /mutation).

intent_definition("Create integration tests.", /test, "integration").
intent_category("Create integration tests.", /mutation).

intent_definition("Test this code.", /test, "context_file").
intent_category("Test this code.", /mutation).

intent_definition("Run go test.", /test, "go_test").
intent_category("Run go test.", /mutation).

intent_definition("Run the unit tests.", /test, "unit").
intent_category("Run the unit tests.", /mutation).

intent_definition("Run integration tests.", /test, "integration").
intent_category("Run integration tests.", /mutation).

intent_definition("Check test coverage.", /test, "coverage").
intent_category("Check test coverage.", /query).

intent_definition("What's the coverage?", /test, "coverage").
intent_category("What's the coverage?", /query).

intent_definition("Generate a test file.", /test, "test_file").
intent_category("Generate a test file.", /mutation).

intent_definition("Create a test for this.", /test, "context_file").
intent_category("Create a test for this.", /mutation).

intent_definition("Add a test case.", /test, "test_case").
intent_category("Add a test case.", /mutation).

intent_definition("Write a table-driven test.", /test, "table_test").
intent_category("Write a table-driven test.", /mutation).

intent_definition("Generate a mock.", /test, "mock").
intent_category("Generate a mock.", /mutation).

intent_definition("Create a mock for this.", /test, "mock").
intent_category("Create a mock for this.", /mutation).

intent_definition("Add test fixtures.", /test, "fixtures").
intent_category("Add test fixtures.", /mutation).

intent_definition("Run benchmarks.", /test, "benchmark").
intent_category("Run benchmarks.", /mutation).

intent_definition("Write a benchmark.", /test, "benchmark").
intent_category("Write a benchmark.", /mutation).

intent_definition("TDD this.", /test, "tdd").
intent_category("TDD this.", /mutation).

intent_definition("Test-driven development.", /test, "tdd").
intent_category("Test-driven development.", /mutation).


# =============================================================================
# SECTION 14: TOOL GENERATION (/generate_tool) - AUTOPOIESIS
# Requests to create new tools/capabilities for the agent itself.
# =============================================================================

intent_definition("Create a ripgrep tool.", /generate_tool, "ripgrep").
intent_category("Create a ripgrep tool.", /mutation).

intent_definition("create a ripgrep tool", /generate_tool, "ripgrep").
intent_category("create a ripgrep tool", /mutation).

intent_definition("Make a tool for searching files.", /generate_tool, "file_search").
intent_category("Make a tool for searching files.", /mutation).

intent_definition("Build a grep tool.", /generate_tool, "grep").
intent_category("Build a grep tool.", /mutation).

intent_definition("Create a tool to find files.", /generate_tool, "find_files").
intent_category("Create a tool to find files.", /mutation).

intent_definition("Generate a code search tool.", /generate_tool, "code_search").
intent_category("Generate a code search tool.", /mutation).

intent_definition("I need a tool that can search the codebase.", /generate_tool, "code_search").
intent_category("I need a tool that can search the codebase.", /mutation).

intent_definition("Create a tool for yourself.", /generate_tool, "self").
intent_category("Create a tool for yourself.", /mutation).

intent_definition("Build yourself a new capability.", /generate_tool, "capability").
intent_category("Build yourself a new capability.", /mutation).

intent_definition("Make a linting tool.", /generate_tool, "linter").
intent_category("Make a linting tool.", /mutation).

intent_definition("Create a formatting tool.", /generate_tool, "formatter").
intent_category("Create a formatting tool.", /mutation).

intent_definition("Generate a new tool.", /generate_tool, "tool").
intent_category("Generate a new tool.", /mutation).

intent_definition("Create a custom tool.", /generate_tool, "custom").
intent_category("Create a custom tool.", /mutation).

intent_definition("Make yourself a tool.", /generate_tool, "self").
intent_category("Make yourself a tool.", /mutation).

intent_definition("Build a tool for X.", /generate_tool, "X").
intent_category("Build a tool for X.", /mutation).

intent_definition("I want you to create a tool.", /generate_tool, "tool").
intent_category("I want you to create a tool.", /mutation).

intent_definition("Generate a file watcher tool.", /generate_tool, "file_watcher").
intent_category("Generate a file watcher tool.", /mutation).

intent_definition("Create a dependency analyzer tool.", /generate_tool, "dep_analyzer").
intent_category("Create a dependency analyzer tool.", /mutation).

intent_definition("Build a complexity calculator.", /generate_tool, "complexity").
intent_category("Build a complexity calculator.", /mutation).

intent_definition("Create a code metrics tool.", /generate_tool, "metrics").
intent_category("Create a code metrics tool.", /mutation).

intent_definition("Make a documentation generator.", /generate_tool, "doc_generator").
intent_category("Make a documentation generator.", /mutation).

intent_definition("Build a migration tool.", /generate_tool, "migration").
intent_category("Build a migration tool.", /mutation).

intent_definition("Create a refactoring tool.", /generate_tool, "refactoring").
intent_category("Create a refactoring tool.", /mutation).

intent_definition("Generate a test generator.", /generate_tool, "test_generator").
intent_category("Generate a test generator.", /mutation).


# =============================================================================
# SECTION 22: MISC OPERATIONS
# =============================================================================

intent_definition("Read this file.", /read, "context_file").
intent_category("Read this file.", /query).

intent_definition("Show me this file.", /read, "context_file").
intent_category("Show me this file.", /query).

intent_definition("Open this file.", /read, "context_file").
intent_category("Open this file.", /query).

intent_definition("Cat this file.", /read, "context_file").
intent_category("Cat this file.", /query).

intent_definition("Display the contents.", /read, "context_file").
intent_category("Display the contents.", /query).

intent_definition("Print this file.", /read, "context_file").
intent_category("Print this file.", /query).

intent_definition("Write to this file.", /write, "context_file").
intent_category("Write to this file.", /mutation).

intent_definition("Save this.", /write, "context_file").
intent_category("Save this.", /mutation).

intent_definition("Deploy.", /deploy, "production").
intent_category("Deploy.", /mutation).

intent_definition("Deploy to production.", /deploy, "production").
intent_category("Deploy to production.", /mutation).

intent_definition("Deploy to staging.", /deploy, "staging").
intent_category("Deploy to staging.", /mutation).

intent_definition("Build the project.", /build, "project").
intent_category("Build the project.", /mutation).

intent_definition("Run go build.", /build, "go").
intent_category("Run go build.", /mutation).

intent_definition("Compile.", /build, "compile").
intent_category("Compile.", /mutation).

intent_definition("Run this.", /run, "context_file").
intent_category("Run this.", /mutation).

intent_definition("Execute this.", /run, "context_file").
intent_category("Execute this.", /mutation).

intent_definition("Run the program.", /run, "program").
intent_category("Run the program.", /mutation).


