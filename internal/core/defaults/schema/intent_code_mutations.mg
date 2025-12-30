# Intent Definitions - Code Mutations
# SECTIONS 6-10: BUG FIXES, DEBUGGING, REFACTORING, CODE CREATION, DELETE - CODER SHARD
# Requests to fix bugs, debug, refactor, create code, and delete code.

# =============================================================================
# SECTION 6: BUG FIXES (/fix)
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
# SECTION 7: DEBUGGING (/debug)
# Debugging and troubleshooting requests.
# =============================================================================

intent_definition("Debug this.", /debug, "context_file").
intent_category("Debug this.", /query).

intent_definition("Debug this error.", /debug, "error").
intent_category("Debug this error.", /query).

intent_definition("Debug the crash.", /debug, "crash").
intent_category("Debug the crash.", /query).

intent_definition("Why is this failing?", /debug, "failure").
intent_category("Why is this failing?", /query).

intent_definition("Why doesn't this work?", /debug, "context_file").
intent_category("Why doesn't this work?", /query).

intent_definition("What's causing this error?", /debug, "error").
intent_category("What's causing this error?", /query).

intent_definition("Find the root cause.", /debug, "root_cause").
intent_category("Find the root cause.", /query).

intent_definition("Root cause analysis.", /debug, "root_cause").
intent_category("Root cause analysis.", /query).

intent_definition("Why is this crashing?", /debug, "crash").
intent_category("Why is this crashing?", /query).

intent_definition("Troubleshoot this.", /debug, "context_file").
intent_category("Troubleshoot this.", /query).

intent_definition("Help me debug this.", /debug, "context_file").
intent_category("Help me debug this.", /query).

intent_definition("What's wrong here?", /debug, "context_file").
intent_category("What's wrong here?", /query).

intent_definition("Find the bug.", /debug, "bug").
intent_category("Find the bug.", /query).

intent_definition("Locate the issue.", /debug, "issue").
intent_category("Locate the issue.", /query).

intent_definition("Why is this returning nil?", /debug, "nil").
intent_category("Why is this returning nil?", /query).

intent_definition("Why is this returning null?", /debug, "null").
intent_category("Why is this returning null?", /query).

intent_definition("Why is this slow?", /debug, "performance").
intent_category("Why is this slow?", /query).

intent_definition("Why is this hanging?", /debug, "hang").
intent_category("Why is this hanging?", /query).

intent_definition("Debug the timeout.", /debug, "timeout").
intent_category("Debug the timeout.", /query).

intent_definition("Analyze this stack trace.", /debug, "stacktrace").
intent_category("Analyze this stack trace.", /query).

intent_definition("Interpret this error message.", /debug, "error_message").
intent_category("Interpret this error message.", /query).

# =============================================================================
# SECTION 8: REFACTORING (/refactor)
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
# SECTION 9: CODE CREATION (/create)
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
# SECTION 10: DELETE (/delete)
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
