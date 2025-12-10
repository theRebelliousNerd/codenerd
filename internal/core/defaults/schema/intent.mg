# Intent Definition Schema - Encyclopedic Sentence Store
# Maps Canonical Sentences to Mangle Actions for intent classification.
#
# ARCHITECTURE:
# - Verbs with ShardType="/none" are answered DIRECTLY by the agent
# - Verbs with ShardType="/reviewer|coder|tester|researcher" spawn that shard
# - Verbs with ShardType="/tool_generator" trigger autopoiesis
# - Verbs with ShardType="/campaign" trigger campaign orchestration
#
# This file contains 400+ canonical sentences covering all codeNERD capabilities.

Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

# =============================================================================
# SECTION 1: CODEBASE STATISTICS (/stats) - DIRECT RESPONSE, NO SHARD
# These queries about codebase metrics should be answered directly using
# shell commands or the knowledge database - NOT by spawning shards.
# =============================================================================

# --- File Type Breakdown ---
intent_definition("What is the breakdown of file types?", /stats, "file_types").
intent_category("What is the breakdown of file types?", /query).

intent_definition("what is the breakdown of file types", /stats, "file_types").
intent_category("what is the breakdown of file types", /query).

intent_definition("How many files of each type are there?", /stats, "file_types").
intent_category("How many files of each type are there?", /query).

intent_definition("Show me the file type distribution.", /stats, "file_types").
intent_category("Show me the file type distribution.", /query).

intent_definition("What file types are in this codebase?", /stats, "file_types").
intent_category("What file types are in this codebase?", /query).

intent_definition("Count files by extension.", /stats, "file_types").
intent_category("Count files by extension.", /query).

intent_definition("What extensions are used in this project?", /stats, "file_types").
intent_category("What extensions are used in this project?", /query).

intent_definition("File type summary.", /stats, "file_types").
intent_category("File type summary.", /query).

intent_definition("What kinds of files are here?", /stats, "file_types").
intent_category("What kinds of files are here?", /query).

intent_definition("Show file extensions.", /stats, "file_types").
intent_category("Show file extensions.", /query).

intent_definition("List all file types.", /stats, "file_types").
intent_category("List all file types.", /query).

intent_definition("How many .go files vs .md files?", /stats, "file_types").
intent_category("How many .go files vs .md files?", /query).

intent_definition("What's the ratio of code to documentation?", /stats, "file_types").
intent_category("What's the ratio of code to documentation?", /query).

intent_definition("Break down the codebase by language.", /stats, "file_types").
intent_category("Break down the codebase by language.", /query).

intent_definition("What languages are used?", /stats, "file_types").
intent_category("What languages are used?", /query).

# --- File Counts ---
intent_definition("How many Go files are there?", /stats, "go_files").
intent_category("How many Go files are there?", /query).

intent_definition("How many files are in this project?", /stats, "file_count").
intent_category("How many files are in this project?", /query).

intent_definition("Count the files in the codebase.", /stats, "file_count").
intent_category("Count the files in the codebase.", /query).

intent_definition("How many markdown files?", /stats, "md_files").
intent_category("How many markdown files?", /query).

intent_definition("How many test files are there?", /stats, "test_files").
intent_category("How many test files are there?", /query).

intent_definition("Total file count.", /stats, "file_count").
intent_category("Total file count.", /query).

intent_definition("How big is this codebase?", /stats, "file_count").
intent_category("How big is this codebase?", /query).

intent_definition("Number of source files.", /stats, "file_count").
intent_category("Number of source files.", /query).

intent_definition("How many Python files?", /stats, "py_files").
intent_category("How many Python files?", /query).

intent_definition("How many TypeScript files?", /stats, "ts_files").
intent_category("How many TypeScript files?", /query).

intent_definition("How many JavaScript files?", /stats, "js_files").
intent_category("How many JavaScript files?", /query).

intent_definition("How many Rust files?", /stats, "rs_files").
intent_category("How many Rust files?", /query).

intent_definition("How many YAML files?", /stats, "yaml_files").
intent_category("How many YAML files?", /query).

intent_definition("How many JSON files?", /stats, "json_files").
intent_category("How many JSON files?", /query).

intent_definition("How many config files?", /stats, "config_files").
intent_category("How many config files?", /query).

intent_definition("Count Go files.", /stats, "go_files").
intent_category("Count Go files.", /query).

intent_definition("Count all files.", /stats, "file_count").
intent_category("Count all files.", /query).

intent_definition("How many Mangle files?", /stats, "mg_files").
intent_category("How many Mangle files?", /query).

intent_definition("How many .mg files?", /stats, "mg_files").
intent_category("How many .mg files?", /query).

# --- Lines of Code ---
intent_definition("How many lines of code?", /stats, "loc").
intent_category("How many lines of code?", /query).

intent_definition("What is the total LOC?", /stats, "loc").
intent_category("What is the total LOC?", /query).

intent_definition("Count the lines of code.", /stats, "loc").
intent_category("Count the lines of code.", /query).

intent_definition("How many lines in total?", /stats, "loc").
intent_category("How many lines in total?", /query).

intent_definition("LOC count.", /stats, "loc").
intent_category("LOC count.", /query).

intent_definition("Total lines.", /stats, "loc").
intent_category("Total lines.", /query).

intent_definition("How much code is there?", /stats, "loc").
intent_category("How much code is there?", /query).

intent_definition("Lines of Go code.", /stats, "loc_go").
intent_category("Lines of Go code.", /query).

intent_definition("How many lines of Go?", /stats, "loc_go").
intent_category("How many lines of Go?", /query).

intent_definition("Code size.", /stats, "loc").
intent_category("Code size.", /query).

intent_definition("Codebase size.", /stats, "loc").
intent_category("Codebase size.", /query).

intent_definition("How large is the codebase?", /stats, "loc").
intent_category("How large is the codebase?", /query).

# --- Project Structure ---
intent_definition("What is the project structure?", /stats, "structure").
intent_category("What is the project structure?", /query).

intent_definition("Show me the directory structure.", /stats, "structure").
intent_category("Show me the directory structure.", /query).

intent_definition("What directories are in this project?", /stats, "structure").
intent_category("What directories are in this project?", /query).

intent_definition("List the main folders.", /stats, "structure").
intent_category("List the main folders.", /query).

intent_definition("Project layout.", /stats, "structure").
intent_category("Project layout.", /query).

intent_definition("Folder structure.", /stats, "structure").
intent_category("Folder structure.", /query).

intent_definition("Directory tree.", /stats, "structure").
intent_category("Directory tree.", /query).

intent_definition("Show the tree.", /stats, "structure").
intent_category("Show the tree.", /query).

intent_definition("What's in the root directory?", /stats, "structure").
intent_category("What's in the root directory?", /query).

intent_definition("List top-level directories.", /stats, "structure").
intent_category("List top-level directories.", /query).

intent_definition("Show project hierarchy.", /stats, "structure").
intent_category("Show project hierarchy.", /query).

intent_definition("How is the code organized?", /stats, "structure").
intent_category("How is the code organized?", /query).

intent_definition("Show me the architecture.", /stats, "structure").
intent_category("Show me the architecture.", /query).

intent_definition("What packages exist?", /stats, "structure").
intent_category("What packages exist?", /query).

intent_definition("List all packages.", /stats, "structure").
intent_category("List all packages.", /query).

intent_definition("What modules are there?", /stats, "structure").
intent_category("What modules are there?", /query).

intent_definition("Show the module structure.", /stats, "structure").
intent_category("Show the module structure.", /query).

intent_definition("What's in internal/?", /stats, "structure").
intent_category("What's in internal/?", /query).

intent_definition("What's in cmd/?", /stats, "structure").
intent_category("What's in cmd/?", /query).

# --- Codebase Overview ---
intent_definition("Give me an overview of the codebase.", /stats, "overview").
intent_category("Give me an overview of the codebase.", /query).

intent_definition("Summarize this project.", /stats, "overview").
intent_category("Summarize this project.", /query).

intent_definition("What is this codebase about?", /stats, "overview").
intent_category("What is this codebase about?", /query).

intent_definition("Describe the project.", /stats, "overview").
intent_category("Describe the project.", /query).

intent_definition("Project summary.", /stats, "overview").
intent_category("Project summary.", /query).

intent_definition("What does this project do?", /stats, "overview").
intent_category("What does this project do?", /query).

intent_definition("Tell me about this codebase.", /stats, "overview").
intent_category("Tell me about this codebase.", /query).

intent_definition("What am I looking at?", /stats, "overview").
intent_category("What am I looking at?", /query).

intent_definition("Codebase summary.", /stats, "overview").
intent_category("Codebase summary.", /query).

intent_definition("High-level overview.", /stats, "overview").
intent_category("High-level overview.", /query).

intent_definition("What's the purpose of this project?", /stats, "overview").
intent_category("What's the purpose of this project?", /query).

# --- Dependency Stats ---
intent_definition("How many dependencies?", /stats, "deps").
intent_category("How many dependencies?", /query).

intent_definition("List dependencies.", /stats, "deps").
intent_category("List dependencies.", /query).

intent_definition("What packages are imported?", /stats, "deps").
intent_category("What packages are imported?", /query).

intent_definition("Show go.mod dependencies.", /stats, "deps").
intent_category("Show go.mod dependencies.", /query).

intent_definition("External dependencies.", /stats, "deps").
intent_category("External dependencies.", /query).

intent_definition("Third-party packages.", /stats, "deps").
intent_category("Third-party packages.", /query).

# --- Test Stats ---
intent_definition("How many tests?", /stats, "test_count").
intent_category("How many tests?", /query).

intent_definition("Test count.", /stats, "test_count").
intent_category("Test count.", /query).

intent_definition("Number of test functions.", /stats, "test_count").
intent_category("Number of test functions.", /query).

intent_definition("How many _test.go files?", /stats, "test_files").
intent_category("How many _test.go files?", /query).

intent_definition("Test file count.", /stats, "test_files").
intent_category("Test file count.", /query).

# --- Function/Symbol Stats ---
intent_definition("How many functions?", /stats, "func_count").
intent_category("How many functions?", /query).

intent_definition("Function count.", /stats, "func_count").
intent_category("Function count.", /query).

intent_definition("How many types are defined?", /stats, "type_count").
intent_category("How many types are defined?", /query).

intent_definition("How many structs?", /stats, "struct_count").
intent_category("How many structs?", /query).

intent_definition("How many interfaces?", /stats, "interface_count").
intent_category("How many interfaces?", /query).

# =============================================================================
# SECTION 2: CAPABILITIES & HELP (/help) - DIRECT RESPONSE, NO SHARD
# Questions about what the agent can do - answered directly.
# =============================================================================

intent_definition("What can you do?", /help, "capabilities").
intent_category("What can you do?", /query).

intent_definition("What are your capabilities?", /help, "capabilities").
intent_category("What are your capabilities?", /query).

intent_definition("Help.", /help, "general").
intent_category("Help.", /query).

intent_definition("Help me.", /help, "general").
intent_category("Help me.", /query).

intent_definition("What commands are available?", /help, "commands").
intent_category("What commands are available?", /query).

intent_definition("List commands.", /help, "commands").
intent_category("List commands.", /query).

intent_definition("Show me what you can do.", /help, "capabilities").
intent_category("Show me what you can do.", /query).

intent_definition("What features do you have?", /help, "capabilities").
intent_category("What features do you have?", /query).

intent_definition("How do I use you?", /help, "usage").
intent_category("How do I use you?", /query).

intent_definition("Getting started.", /help, "usage").
intent_category("Getting started.", /query).

intent_definition("Tutorial.", /help, "usage").
intent_category("Tutorial.", /query).

intent_definition("How does this work?", /help, "usage").
intent_category("How does this work?", /query).

intent_definition("Can you review code?", /help, "capabilities").
intent_category("Can you review code?", /query).

intent_definition("Can you write code?", /help, "capabilities").
intent_category("Can you write code?", /query).

intent_definition("Can you run tests?", /help, "capabilities").
intent_category("Can you run tests?", /query).

intent_definition("Can you search files?", /help, "capabilities").
intent_category("Can you search files?", /query).

intent_definition("Do you have access to the file system?", /help, "capabilities").
intent_category("Do you have access to the file system?", /query).

intent_definition("Can you execute commands?", /help, "capabilities").
intent_category("Can you execute commands?", /query).

intent_definition("What shards do you have?", /help, "shards").
intent_category("What shards do you have?", /query).

intent_definition("What agents are available?", /help, "shards").
intent_category("What agents are available?", /query).

intent_definition("List available agents.", /help, "shards").
intent_category("List available agents.", /query).

intent_definition("What specialists exist?", /help, "shards").
intent_category("What specialists exist?", /query).

intent_definition("How do I start a campaign?", /help, "campaign").
intent_category("How do I start a campaign?", /query).

intent_definition("What is a campaign?", /help, "campaign").
intent_category("What is a campaign?", /query).

intent_definition("How do I define a new agent?", /help, "define_agent").
intent_category("How do I define a new agent?", /query).

intent_definition("How do I create a specialist?", /help, "define_agent").
intent_category("How do I create a specialist?", /query).

intent_definition("What is autopoiesis?", /help, "autopoiesis").
intent_category("What is autopoiesis?", /query).

intent_definition("Can you learn?", /help, "learning").
intent_category("Can you learn?", /query).

intent_definition("Do you remember things?", /help, "memory").
intent_category("Do you remember things?", /query).

intent_definition("What is Mangle?", /help, "mangle").
intent_category("What is Mangle?", /query).

intent_definition("What is the kernel?", /help, "kernel").
intent_category("What is the kernel?", /query).

intent_definition("Explain your architecture.", /help, "architecture").
intent_category("Explain your architecture.", /query).

intent_definition("How are you built?", /help, "architecture").
intent_category("How are you built?", /query).

# =============================================================================
# SECTION 3: GREETINGS & CONVERSATION (/greet) - DIRECT RESPONSE, NO SHARD
# Social interactions that don't require any code actions.
# =============================================================================

intent_definition("Hello.", /greet, "hello").
intent_category("Hello.", /query).

intent_definition("Hi.", /greet, "hello").
intent_category("Hi.", /query).

intent_definition("Hi there.", /greet, "hello").
intent_category("Hi there.", /query).

intent_definition("Hey.", /greet, "hello").
intent_category("Hey.", /query).

intent_definition("Hey there.", /greet, "hello").
intent_category("Hey there.", /query).

intent_definition("Good morning.", /greet, "hello").
intent_category("Good morning.", /query).

intent_definition("Good afternoon.", /greet, "hello").
intent_category("Good afternoon.", /query).

intent_definition("Good evening.", /greet, "hello").
intent_category("Good evening.", /query).

intent_definition("Howdy.", /greet, "hello").
intent_category("Howdy.", /query).

intent_definition("What's up?", /greet, "hello").
intent_category("What's up?", /query).

intent_definition("Sup.", /greet, "hello").
intent_category("Sup.", /query).

intent_definition("Yo.", /greet, "hello").
intent_category("Yo.", /query).

intent_definition("Thanks!", /greet, "thanks").
intent_category("Thanks!", /query).

intent_definition("Thank you.", /greet, "thanks").
intent_category("Thank you.", /query).

intent_definition("Thanks a lot.", /greet, "thanks").
intent_category("Thanks a lot.", /query).

intent_definition("Much appreciated.", /greet, "thanks").
intent_category("Much appreciated.", /query).

intent_definition("Cheers.", /greet, "thanks").
intent_category("Cheers.", /query).

intent_definition("Awesome, thanks.", /greet, "thanks").
intent_category("Awesome, thanks.", /query).

intent_definition("Perfect, thank you.", /greet, "thanks").
intent_category("Perfect, thank you.", /query).

intent_definition("Goodbye.", /greet, "bye").
intent_category("Goodbye.", /query).

intent_definition("Bye.", /greet, "bye").
intent_category("Bye.", /query).

intent_definition("See you.", /greet, "bye").
intent_category("See you.", /query).

intent_definition("Later.", /greet, "bye").
intent_category("Later.", /query).

intent_definition("Good work.", /greet, "praise").
intent_category("Good work.", /query).

intent_definition("Nice job.", /greet, "praise").
intent_category("Nice job.", /query).

intent_definition("Well done.", /greet, "praise").
intent_category("Well done.", /query).

intent_definition("That's great.", /greet, "praise").
intent_category("That's great.", /query).

intent_definition("Okay.", /greet, "ack").
intent_category("Okay.", /query).

intent_definition("OK.", /greet, "ack").
intent_category("OK.", /query).

intent_definition("Got it.", /greet, "ack").
intent_category("Got it.", /query).

intent_definition("I see.", /greet, "ack").
intent_category("I see.", /query).

intent_definition("Makes sense.", /greet, "ack").
intent_category("Makes sense.", /query).

intent_definition("Understood.", /greet, "ack").
intent_category("Understood.", /query).

# =============================================================================
# SECTION 4: CODE REVIEW (/review) - REVIEWER SHARD
# Requests for code review, quality checks, and audits.
# =============================================================================

intent_definition("Review this file.", /review, "context_file").
intent_category("Review this file.", /query).

intent_definition("Review this file for bugs.", /review, "context_file").
intent_category("Review this file for bugs.", /query).

intent_definition("Review my code.", /review, "context_file").
intent_category("Review my code.", /query).

intent_definition("Code review this.", /review, "context_file").
intent_category("Code review this.", /query).

intent_definition("Check this file for issues.", /review, "context_file").
intent_category("Check this file for issues.", /query).

intent_definition("Look over this code.", /review, "context_file").
intent_category("Look over this code.", /query).

intent_definition("Can you review my changes?", /review, "changes").
intent_category("Can you review my changes?", /query).

intent_definition("Review the pull request.", /review, "pr").
intent_category("Review the pull request.", /query).

intent_definition("Review this PR.", /review, "pr").
intent_category("Review this PR.", /query).

intent_definition("Check this code.", /review, "context_file").
intent_category("Check this code.", /query).

intent_definition("Audit this file.", /review, "context_file").
intent_category("Audit this file.", /query).

intent_definition("Is this code good?", /review, "context_file").
intent_category("Is this code good?", /query).

intent_definition("Any issues with this?", /review, "context_file").
intent_category("Any issues with this?", /query).

intent_definition("Find problems in this code.", /review, "context_file").
intent_category("Find problems in this code.", /query).

intent_definition("What's wrong with this code?", /review, "context_file").
intent_category("What's wrong with this code?", /query).

intent_definition("Review for best practices.", /review, "best_practices").
intent_category("Review for best practices.", /query).

intent_definition("Check code quality.", /review, "quality").
intent_category("Check code quality.", /query).

intent_definition("Code quality check.", /review, "quality").
intent_category("Code quality check.", /query).

intent_definition("Static analysis.", /review, "static_analysis").
intent_category("Static analysis.", /query).

intent_definition("Run static analysis.", /review, "static_analysis").
intent_category("Run static analysis.", /query).

intent_definition("Lint this file.", /review, "lint").
intent_category("Lint this file.", /query).

intent_definition("Check for code smells.", /review, "code_smells").
intent_category("Check for code smells.", /query).

intent_definition("Find code smells.", /review, "code_smells").
intent_category("Find code smells.", /query).

intent_definition("Review for performance.", /review, "performance").
intent_category("Review for performance.", /query).

intent_definition("Performance review.", /review, "performance").
intent_category("Performance review.", /query).

intent_definition("Check for memory leaks.", /review, "memory").
intent_category("Check for memory leaks.", /query).

intent_definition("Review error handling.", /review, "error_handling").
intent_category("Review error handling.", /query).

intent_definition("Check error handling.", /review, "error_handling").
intent_category("Check error handling.", /query).

intent_definition("Review this function.", /review, "function").
intent_category("Review this function.", /query).

intent_definition("Review this package.", /review, "package").
intent_category("Review this package.", /query).

intent_definition("Review internal/core.", /review, "internal/core").
intent_category("Review internal/core.", /query).

intent_definition("Review the authentication code.", /review, "auth").
intent_category("Review the authentication code.", /query).

intent_definition("Review the API handlers.", /review, "api").
intent_category("Review the API handlers.", /query).

intent_definition("Give me feedback on this.", /review, "context_file").
intent_category("Give me feedback on this.", /query).

intent_definition("Critique this code.", /review, "context_file").
intent_category("Critique this code.", /query).

# =============================================================================
# SECTION 5: SECURITY ANALYSIS (/security) - REVIEWER SHARD
# Security-focused reviews and vulnerability scanning.
# =============================================================================

intent_definition("Check my code for security issues.", /security, "codebase").
intent_category("Check my code for security issues.", /query).

intent_definition("Find security vulnerabilities.", /security, "codebase").
intent_category("Find security vulnerabilities.", /query).

intent_definition("Is this code secure?", /security, "context_file").
intent_category("Is this code secure?", /query).

intent_definition("Security audit this file.", /security, "context_file").
intent_category("Security audit this file.", /query).

intent_definition("Check for SQL injection.", /security, "sql_injection").
intent_category("Check for SQL injection.", /query).

intent_definition("Look for XSS vulnerabilities.", /security, "xss").
intent_category("Look for XSS vulnerabilities.", /query).

intent_definition("Scan for OWASP top 10.", /security, "owasp").
intent_category("Scan for OWASP top 10.", /query).

intent_definition("Security scan.", /security, "codebase").
intent_category("Security scan.", /query).

intent_definition("Security check.", /security, "codebase").
intent_category("Security check.", /query).

intent_definition("Find vulnerabilities.", /security, "codebase").
intent_category("Find vulnerabilities.", /query).

intent_definition("Vulnerability scan.", /security, "codebase").
intent_category("Vulnerability scan.", /query).

intent_definition("Check for injection vulnerabilities.", /security, "injection").
intent_category("Check for injection vulnerabilities.", /query).

intent_definition("Is this vulnerable?", /security, "context_file").
intent_category("Is this vulnerable?", /query).

intent_definition("Check for command injection.", /security, "command_injection").
intent_category("Check for command injection.", /query).

intent_definition("Check for path traversal.", /security, "path_traversal").
intent_category("Check for path traversal.", /query).

intent_definition("Check authentication security.", /security, "auth").
intent_category("Check authentication security.", /query).

intent_definition("Review security of auth flow.", /security, "auth").
intent_category("Review security of auth flow.", /query).

intent_definition("Check for hardcoded secrets.", /security, "secrets").
intent_category("Check for hardcoded secrets.", /query).

intent_definition("Find hardcoded passwords.", /security, "secrets").
intent_category("Find hardcoded passwords.", /query).

intent_definition("Check for exposed API keys.", /security, "secrets").
intent_category("Check for exposed API keys.", /query).

intent_definition("Security best practices check.", /security, "best_practices").
intent_category("Security best practices check.", /query).

intent_definition("Check for CSRF vulnerabilities.", /security, "csrf").
intent_category("Check for CSRF vulnerabilities.", /query).

intent_definition("Check input validation.", /security, "input_validation").
intent_category("Check input validation.", /query).

intent_definition("Audit security.", /security, "codebase").
intent_category("Audit security.", /query).

intent_definition("Penetration test this.", /security, "pentest").
intent_category("Penetration test this.", /query).

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
# SECTION 7: DEBUGGING (/debug) - CODER SHARD
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
# SECTION 12: RESEARCH (/research) - RESEARCHER SHARD
# Documentation and research requests.
# =============================================================================

intent_definition("Research this.", /research, "topic").
intent_category("Research this.", /query).

intent_definition("Research how to use X.", /research, "X").
intent_category("Research how to use X.", /query).

intent_definition("How do I use this library?", /research, "library").
intent_category("How do I use this library?", /query).

intent_definition("Find documentation for this.", /research, "documentation").
intent_category("Find documentation for this.", /query).

intent_definition("Look up best practices.", /research, "best_practices").
intent_category("Look up best practices.", /query).

intent_definition("What is the idiomatic way to do this?", /research, "idiomatic").
intent_category("What is the idiomatic way to do this?", /query).

intent_definition("Research the API.", /research, "api").
intent_category("Research the API.", /query).

intent_definition("Look up the docs.", /research, "docs").
intent_category("Look up the docs.", /query).

intent_definition("Find examples.", /research, "examples").
intent_category("Find examples.", /query).

intent_definition("Show me examples.", /research, "examples").
intent_category("Show me examples.", /query).

intent_definition("How do I do X in Go?", /research, "go").
intent_category("How do I do X in Go?", /query).

intent_definition("What's the best way to do this?", /research, "best_way").
intent_category("What's the best way to do this?", /query).

intent_definition("Research error handling in Go.", /research, "error_handling").
intent_category("Research error handling in Go.", /query).

intent_definition("Look up context usage.", /research, "context").
intent_category("Look up context usage.", /query).

intent_definition("Research goroutine patterns.", /research, "goroutines").
intent_category("Research goroutine patterns.", /query).

intent_definition("Find channel examples.", /research, "channels").
intent_category("Find channel examples.", /query).

intent_definition("Research Mangle syntax.", /research, "mangle").
intent_category("Research Mangle syntax.", /query).

intent_definition("Look up Datalog.", /research, "datalog").
intent_category("Look up Datalog.", /query).

intent_definition("Research Bubbletea.", /research, "bubbletea").
intent_category("Research Bubbletea.", /query).

intent_definition("How does Rod work?", /research, "rod").
intent_category("How does Rod work?", /query).

intent_definition("Research web scraping.", /research, "web_scraping").
intent_category("Research web scraping.", /query).

intent_definition("Find the documentation.", /research, "documentation").
intent_category("Find the documentation.", /query).

# =============================================================================
# SECTION 13: EXPLANATION (/explain) - Context-dependent
# Code explanation requests - can be direct or shard-based.
# =============================================================================

intent_definition("Explain this code.", /explain, "context_file").
intent_category("Explain this code.", /query).

intent_definition("Explain this.", /explain, "context_file").
intent_category("Explain this.", /query).

intent_definition("What does this function do?", /explain, "focused_symbol").
intent_category("What does this function do?", /query).

intent_definition("How does this work?", /explain, "context_file").
intent_category("How does this work?", /query).

intent_definition("What is this?", /explain, "context_file").
intent_category("What is this?", /query).

intent_definition("Tell me about this function.", /explain, "focused_symbol").
intent_category("Tell me about this function.", /query).

intent_definition("Help me understand this.", /explain, "context_file").
intent_category("Help me understand this.", /query).

intent_definition("Walk me through this.", /explain, "context_file").
intent_category("Walk me through this.", /query).

intent_definition("Explain the logic.", /explain, "logic").
intent_category("Explain the logic.", /query).

intent_definition("What's happening here?", /explain, "context_file").
intent_category("What's happening here?", /query).

intent_definition("Break down this code.", /explain, "context_file").
intent_category("Break down this code.", /query).

intent_definition("Explain step by step.", /explain, "context_file").
intent_category("Explain step by step.", /query).

intent_definition("What does this variable do?", /explain, "variable").
intent_category("What does this variable do?", /query).

intent_definition("Explain the algorithm.", /explain, "algorithm").
intent_category("Explain the algorithm.", /query).

intent_definition("How does this algorithm work?", /explain, "algorithm").
intent_category("How does this algorithm work?", /query).

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
# SECTION 15: CAMPAIGNS (/campaign) - CAMPAIGN SYSTEM
# Multi-phase, long-running task requests.
# =============================================================================

intent_definition("Start a campaign.", /campaign, "start").
intent_category("Start a campaign.", /mutation).

intent_definition("Start a campaign to rewrite auth.", /campaign, "rewrite auth").
intent_category("Start a campaign to rewrite auth.", /mutation).

intent_definition("I want to refactor the entire codebase.", /campaign, "refactor").
intent_category("I want to refactor the entire codebase.", /mutation).

intent_definition("Help me migrate to a new framework.", /campaign, "migration").
intent_category("Help me migrate to a new framework.", /mutation).

intent_definition("Let's do a major feature.", /campaign, "feature").
intent_category("Let's do a major feature.", /mutation).

intent_definition("This is going to be a big task.", /campaign, "big_task").
intent_category("This is going to be a big task.", /mutation).

intent_definition("Launch a campaign.", /campaign, "start").
intent_category("Launch a campaign.", /mutation).

intent_definition("Begin campaign mode.", /campaign, "start").
intent_category("Begin campaign mode.", /mutation).

intent_definition("Start a multi-phase project.", /campaign, "project").
intent_category("Start a multi-phase project.", /mutation).

intent_definition("Campaign status.", /campaign, "status").
intent_category("Campaign status.", /query).

intent_definition("Show campaign progress.", /campaign, "progress").
intent_category("Show campaign progress.", /query).

intent_definition("What phase are we on?", /campaign, "phase").
intent_category("What phase are we on?", /query).

intent_definition("Continue the campaign.", /campaign, "continue").
intent_category("Continue the campaign.", /mutation).

intent_definition("Pause the campaign.", /campaign, "pause").
intent_category("Pause the campaign.", /mutation).

intent_definition("Cancel the campaign.", /campaign, "cancel").
intent_category("Cancel the campaign.", /mutation).

intent_definition("Abort campaign.", /campaign, "abort").
intent_category("Abort campaign.", /mutation).

# =============================================================================
# SECTION 16: GIT OPERATIONS (/git) - Direct or Git integration
# Git-related queries and commands.
# =============================================================================

intent_definition("Show git status.", /git, "status").
intent_category("Show git status.", /query).

intent_definition("Git status.", /git, "status").
intent_category("Git status.", /query).

intent_definition("What's the git status?", /git, "status").
intent_category("What's the git status?", /query).

intent_definition("What changed?", /git, "diff").
intent_category("What changed?", /query).

intent_definition("What changed recently?", /git, "diff").
intent_category("What changed recently?", /query).

intent_definition("Show the diff.", /git, "diff").
intent_category("Show the diff.", /query).

intent_definition("Git diff.", /git, "diff").
intent_category("Git diff.", /query).

intent_definition("Show me the changes.", /git, "diff").
intent_category("Show me the changes.", /query).

intent_definition("What files changed?", /git, "changed_files").
intent_category("What files changed?", /query).

intent_definition("Show commit history.", /git, "log").
intent_category("Show commit history.", /query).

intent_definition("Git log.", /git, "log").
intent_category("Git log.", /query).

intent_definition("Recent commits.", /git, "log").
intent_category("Recent commits.", /query).

intent_definition("Show recent commits.", /git, "log").
intent_category("Show recent commits.", /query).

intent_definition("What was the last commit?", /git, "last_commit").
intent_category("What was the last commit?", /query).

intent_definition("Create a commit.", /git, "commit").
intent_category("Create a commit.", /mutation).

intent_definition("Commit this.", /git, "commit").
intent_category("Commit this.", /mutation).

intent_definition("Commit the changes.", /git, "commit").
intent_category("Commit the changes.", /mutation).

intent_definition("Stage the changes.", /git, "add").
intent_category("Stage the changes.", /mutation).

intent_definition("Git add.", /git, "add").
intent_category("Git add.", /mutation).

intent_definition("Push to remote.", /git, "push").
intent_category("Push to remote.", /mutation).

intent_definition("Git push.", /git, "push").
intent_category("Git push.", /mutation).

intent_definition("Pull latest.", /git, "pull").
intent_category("Pull latest.", /mutation).

intent_definition("Git pull.", /git, "pull").
intent_category("Git pull.", /mutation).

intent_definition("What branch am I on?", /git, "branch").
intent_category("What branch am I on?", /query).

intent_definition("Current branch.", /git, "branch").
intent_category("Current branch.", /query).

intent_definition("List branches.", /git, "branches").
intent_category("List branches.", /query).

intent_definition("Create a branch.", /git, "create_branch").
intent_category("Create a branch.", /mutation).

intent_definition("Switch branch.", /git, "checkout").
intent_category("Switch branch.", /mutation).

intent_definition("Checkout main.", /git, "checkout_main").
intent_category("Checkout main.", /mutation).

intent_definition("Show staged changes.", /git, "staged").
intent_category("Show staged changes.", /query).

intent_definition("What's staged?", /git, "staged").
intent_category("What's staged?", /query).

# =============================================================================
# SECTION 17: SEARCH (/search) - Direct or Researcher
# Code and file search requests.
# =============================================================================

intent_definition("Search for this.", /search, "query").
intent_category("Search for this.", /query).

intent_definition("Search the codebase.", /search, "codebase").
intent_category("Search the codebase.", /query).

intent_definition("Find this.", /search, "query").
intent_category("Find this.", /query).

intent_definition("Find all uses of this function.", /search, "function_uses").
intent_category("Find all uses of this function.", /query).

intent_definition("Find where this is defined.", /search, "definition").
intent_category("Find where this is defined.", /query).

intent_definition("Grep for this pattern.", /search, "pattern").
intent_category("Grep for this pattern.", /query).

intent_definition("Grep for X.", /search, "X").
intent_category("Grep for X.", /query).

intent_definition("Find all TODO comments.", /search, "todos").
intent_category("Find all TODO comments.", /query).

intent_definition("Search for TODO.", /search, "todo").
intent_category("Search for TODO.", /query).

intent_definition("Find FIXME.", /search, "fixme").
intent_category("Find FIXME.", /query).

intent_definition("Where is X defined?", /search, "definition").
intent_category("Where is X defined?", /query).

intent_definition("Find references to X.", /search, "references").
intent_category("Find references to X.", /query).

intent_definition("Find callers of this function.", /search, "callers").
intent_category("Find callers of this function.", /query).

intent_definition("Find all imports of X.", /search, "imports").
intent_category("Find all imports of X.", /query).

intent_definition("Search for error handling.", /search, "error_handling").
intent_category("Search for error handling.", /query).

intent_definition("Find files containing X.", /search, "files").
intent_category("Find files containing X.", /query).

intent_definition("Search for this string.", /search, "string").
intent_category("Search for this string.", /query).

intent_definition("Find all occurrences.", /search, "occurrences").
intent_category("Find all occurrences.", /query).

# =============================================================================
# SECTION 18: EXPLORE (/explore) - Researcher or Direct
# Codebase exploration requests.
# =============================================================================

intent_definition("Explore the codebase.", /explore, "codebase").
intent_category("Explore the codebase.", /query).

intent_definition("Explore this package.", /explore, "package").
intent_category("Explore this package.", /query).

intent_definition("Show me how the modules connect.", /explore, "dependencies").
intent_category("Show me how the modules connect.", /query).

intent_definition("What imports what?", /explore, "imports").
intent_category("What imports what?", /query).

intent_definition("Show dependency graph.", /explore, "dep_graph").
intent_category("Show dependency graph.", /query).

intent_definition("Explore dependencies.", /explore, "dependencies").
intent_category("Explore dependencies.", /query).

intent_definition("What uses this?", /explore, "usage").
intent_category("What uses this?", /query).

intent_definition("What calls this function?", /explore, "callers").
intent_category("What calls this function?", /query).

intent_definition("Trace the call graph.", /explore, "call_graph").
intent_category("Trace the call graph.", /query).

intent_definition("Show the architecture.", /explore, "architecture").
intent_category("Show the architecture.", /query).

intent_definition("Explore internal/core.", /explore, "internal/core").
intent_category("Explore internal/core.", /query).

intent_definition("What's in this directory?", /explore, "directory").
intent_category("What's in this directory?", /query).

intent_definition("List files in this folder.", /explore, "files").
intent_category("List files in this folder.", /query).

# =============================================================================
# SECTION 19: CONFIGURATION (/configure) - DIRECT RESPONSE
# Settings and preference changes.
# =============================================================================

intent_definition("Configure settings.", /configure, "settings").
intent_category("Configure settings.", /instruction).

intent_definition("Configure the agent to be verbose.", /configure, "verbosity").
intent_category("Configure the agent to be verbose.", /instruction).

intent_definition("Change the settings.", /configure, "settings").
intent_category("Change the settings.", /instruction).

intent_definition("Set the theme to dark.", /configure, "theme").
intent_category("Set the theme to dark.", /instruction).

intent_definition("Set the theme to light.", /configure, "theme").
intent_category("Set the theme to light.", /instruction).

intent_definition("Always use tabs.", /configure, "tabs").
intent_category("Always use tabs.", /instruction).

intent_definition("Prefer spaces over tabs.", /configure, "spaces").
intent_category("Prefer spaces over tabs.", /instruction).

intent_definition("Set model to X.", /configure, "model").
intent_category("Set model to X.", /instruction).

intent_definition("Change the LLM provider.", /configure, "provider").
intent_category("Change the LLM provider.", /instruction).

intent_definition("Set API key.", /configure, "api_key").
intent_category("Set API key.", /instruction).

intent_definition("Configure embedding.", /configure, "embedding").
intent_category("Configure embedding.", /instruction).

intent_definition("Enable verbose mode.", /configure, "verbose").
intent_category("Enable verbose mode.", /instruction).

intent_definition("Disable verbose mode.", /configure, "verbose_off").
intent_category("Disable verbose mode.", /instruction).

intent_definition("Remember this preference.", /configure, "preference").
intent_category("Remember this preference.", /instruction).

intent_definition("Set context window size.", /configure, "context_window").
intent_category("Set context window size.", /instruction).

# =============================================================================
# SECTION 20: KNOWLEDGE DB (/knowledge) - DIRECT RESPONSE
# Queries about the agent's knowledge and memory.
# =============================================================================

intent_definition("What do you know about X?", /knowledge, "X").
intent_category("What do you know about X?", /query).

intent_definition("Query the knowledge base.", /knowledge, "query").
intent_category("Query the knowledge base.", /query).

intent_definition("Search knowledge.", /knowledge, "search").
intent_category("Search knowledge.", /query).

intent_definition("What have you learned?", /knowledge, "learned").
intent_category("What have you learned?", /query).

intent_definition("Show learned patterns.", /knowledge, "patterns").
intent_category("Show learned patterns.", /query).

intent_definition("What preferences have you saved?", /knowledge, "preferences").
intent_category("What preferences have you saved?", /query).

intent_definition("Show session history.", /knowledge, "history").
intent_category("Show session history.", /query).

intent_definition("What did we discuss?", /knowledge, "conversation").
intent_category("What did we discuss?", /query).

intent_definition("What was my last request?", /knowledge, "last_request").
intent_category("What was my last request?", /query).

intent_definition("List knowledge atoms.", /knowledge, "atoms").
intent_category("List knowledge atoms.", /query).

intent_definition("Query knowledge DB.", /knowledge, "db").
intent_category("Query knowledge DB.", /query).

# =============================================================================
# SECTION 21: SHADOW MODE (/shadow) - DIRECT RESPONSE
# What-if analysis and simulation.
# =============================================================================

intent_definition("What if I change this?", /shadow, "whatif").
intent_category("What if I change this?", /query).

intent_definition("Simulate this change.", /shadow, "simulate").
intent_category("Simulate this change.", /query).

intent_definition("What would happen if?", /shadow, "whatif").
intent_category("What would happen if?", /query).

intent_definition("Impact analysis.", /shadow, "impact").
intent_category("Impact analysis.", /query).

intent_definition("What breaks if I change X?", /shadow, "impact").
intent_category("What breaks if I change X?", /query).

intent_definition("Shadow mode.", /shadow, "enable").
intent_category("Shadow mode.", /query).

intent_definition("Enter shadow mode.", /shadow, "enable").
intent_category("Enter shadow mode.", /query).

intent_definition("Safe simulation.", /shadow, "simulate").
intent_category("Safe simulation.", /query).

intent_definition("Dry run.", /shadow, "dryrun").
intent_category("Dry run.", /query).

intent_definition("Preview changes.", /shadow, "preview").
intent_category("Preview changes.", /query).

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

# =============================================================================
# SECTION 23: MULTI-STEP TASK PATTERNS (/multi_step)
# Encyclopedic corpus for detecting and decomposing multi-step requests.
# These patterns trigger task decomposition into multiple sequential steps.
# =============================================================================

# --- Multi-Step Pattern Declarations ---
Decl multistep_pattern(Pattern, Category, Relation, Priority).
Decl multistep_keyword(Pattern, Keyword).
Decl multistep_verb_pair(Pattern, Verb1, Verb2).
Decl multistep_example(Pattern, Example).

# =============================================================================
# SEQUENTIAL EXPLICIT PATTERNS
# "first X, then Y, finally Z" style requests
# =============================================================================

multistep_pattern("explicit_first_then", "sequential_explicit", "sequential", 100).
multistep_keyword("explicit_first_then", "first").
multistep_keyword("explicit_first_then", "then").
multistep_keyword("explicit_first_then", "finally").
multistep_keyword("explicit_first_then", "start by").
multistep_keyword("explicit_first_then", "begin with").
multistep_example("explicit_first_then", "first review the code, then fix any issues").
multistep_example("explicit_first_then", "first create the file, then add tests, finally commit").
multistep_example("explicit_first_then", "start by analyzing the codebase, then refactor the hot spots").

multistep_pattern("explicit_step_numbers", "sequential_explicit", "sequential", 95).
multistep_keyword("explicit_step_numbers", "step 1").
multistep_keyword("explicit_step_numbers", "step 2").
multistep_keyword("explicit_step_numbers", "step 3").
multistep_keyword("explicit_step_numbers", "1.").
multistep_keyword("explicit_step_numbers", "2.").
multistep_keyword("explicit_step_numbers", "3.").
multistep_example("explicit_step_numbers", "1. create the handler 2. add tests 3. update the router").
multistep_example("explicit_step_numbers", "step 1: review, step 2: fix, step 3: test").

multistep_pattern("explicit_after_that", "sequential_explicit", "sequential", 90).
multistep_keyword("explicit_after_that", "after that").
multistep_keyword("explicit_after_that", "afterward").
multistep_keyword("explicit_after_that", "afterwards").
multistep_keyword("explicit_after_that", "following that").
multistep_example("explicit_after_that", "fix the bug, after that run the tests").
multistep_example("explicit_after_that", "refactor the function and afterward update the docs").

multistep_pattern("explicit_next", "sequential_explicit", "sequential", 85).
multistep_keyword("explicit_next", "next").
multistep_keyword("explicit_next", "subsequently").
multistep_example("explicit_next", "create the interface, next implement it").
multistep_example("explicit_next", "review the PR and next merge it").

multistep_pattern("explicit_once_done", "sequential_explicit", "sequential", 88).
multistep_keyword("explicit_once_done", "once done").
multistep_keyword("explicit_once_done", "when done").
multistep_keyword("explicit_once_done", "when finished").
multistep_keyword("explicit_once_done", "after done").
multistep_keyword("explicit_once_done", "after finished").
multistep_example("explicit_once_done", "fix the tests, once done commit the changes").
multistep_example("explicit_once_done", "refactor that function and when you're done run the benchmarks").

# =============================================================================
# REVIEW-THEN-FIX PATTERNS
# "review X and fix issues" - implicit sequential dependency
# =============================================================================

multistep_pattern("implicit_review_fix", "review_then_fix", "sequential", 92).
multistep_keyword("implicit_review_fix", "review and fix").
multistep_keyword("implicit_review_fix", "check and fix").
multistep_keyword("implicit_review_fix", "find and fix").
multistep_keyword("implicit_review_fix", "audit and fix").
multistep_verb_pair("implicit_review_fix", /review, /fix).
multistep_verb_pair("implicit_review_fix", /analyze, /fix).
multistep_verb_pair("implicit_review_fix", /security, /fix).
multistep_example("implicit_review_fix", "review auth.go and fix any issues").
multistep_example("implicit_review_fix", "check the handlers and fix any bugs").
multistep_example("implicit_review_fix", "find and fix all security issues").

# =============================================================================
# CREATE-THEN-VALIDATE PATTERNS
# "create X and test it" - mutation followed by verification
# =============================================================================

multistep_pattern("implicit_create_test", "create_then_validate", "sequential", 90).
multistep_keyword("implicit_create_test", "create and test").
multistep_keyword("implicit_create_test", "implement and test").
multistep_keyword("implicit_create_test", "add and test").
multistep_keyword("implicit_create_test", "build and test").
multistep_keyword("implicit_create_test", "make sure it works").
multistep_verb_pair("implicit_create_test", /create, /test).
multistep_verb_pair("implicit_create_test", /fix, /test).
multistep_verb_pair("implicit_create_test", /refactor, /test).
multistep_example("implicit_create_test", "create a new handler and test it").
multistep_example("implicit_create_test", "implement the feature and write tests").
multistep_example("implicit_create_test", "add the endpoint and make sure it works").

# =============================================================================
# VERIFY-AFTER-MUTATION PATTERNS
# "fix X and verify/run tests" - mutation followed by verification
# =============================================================================

multistep_pattern("implicit_fix_verify", "verify_after_mutation", "sequential", 88).
multistep_keyword("implicit_fix_verify", "fix and verify").
multistep_keyword("implicit_fix_verify", "change and test").
multistep_keyword("implicit_fix_verify", "update and check").
multistep_keyword("implicit_fix_verify", "fix and run tests").
multistep_verb_pair("implicit_fix_verify", /fix, /test).
multistep_verb_pair("implicit_fix_verify", /refactor, /test).
multistep_verb_pair("implicit_fix_verify", /create, /test).
multistep_example("implicit_fix_verify", "fix the authentication and verify it works").
multistep_example("implicit_fix_verify", "change the handler and run the tests").
multistep_example("implicit_fix_verify", "update the config and make sure nothing breaks").

# =============================================================================
# RESEARCH-THEN-ACT PATTERNS
# "figure out X then implement" - learning followed by action
# =============================================================================

multistep_pattern("implicit_research_implement", "research_then_act", "sequential", 85).
multistep_keyword("implicit_research_implement", "research and implement").
multistep_keyword("implicit_research_implement", "figure out and").
multistep_keyword("implicit_research_implement", "learn how to and").
multistep_keyword("implicit_research_implement", "understand and then").
multistep_verb_pair("implicit_research_implement", /research, /create).
multistep_verb_pair("implicit_research_implement", /research, /fix).
multistep_verb_pair("implicit_research_implement", /explore, /create).
multistep_example("implicit_research_implement", "research how to implement OAuth and then add it").
multistep_example("implicit_research_implement", "figure out the API and implement the client").
multistep_example("implicit_research_implement", "understand the codebase structure and then refactor").

# =============================================================================
# ANALYZE-THEN-OPTIMIZE PATTERNS
# "analyze X and improve" - analysis followed by improvement
# =============================================================================

multistep_pattern("implicit_analyze_optimize", "analyze_then_optimize", "sequential", 85).
multistep_keyword("implicit_analyze_optimize", "analyze and optimize").
multistep_keyword("implicit_analyze_optimize", "profile and improve").
multistep_keyword("implicit_analyze_optimize", "find bottlenecks and fix").
multistep_verb_pair("implicit_analyze_optimize", /analyze, /refactor).
multistep_verb_pair("implicit_analyze_optimize", /analyze, /fix).
multistep_example("implicit_analyze_optimize", "analyze the performance and optimize the hot paths").
multistep_example("implicit_analyze_optimize", "profile the API and improve response times").
multistep_example("implicit_analyze_optimize", "find bottlenecks in the database layer and fix them").

# =============================================================================
# SECURITY-AUDIT-FIX PATTERNS
# "security scan and fix" - security analysis followed by remediation
# =============================================================================

multistep_pattern("implicit_security_fix", "security_audit_fix", "sequential", 93).
multistep_keyword("implicit_security_fix", "security scan and fix").
multistep_keyword("implicit_security_fix", "audit and fix").
multistep_keyword("implicit_security_fix", "find vulnerabilities and fix").
multistep_verb_pair("implicit_security_fix", /security, /fix).
multistep_example("implicit_security_fix", "security scan the API handlers and fix any vulnerabilities").
multistep_example("implicit_security_fix", "audit the auth module and patch any issues").
multistep_example("implicit_security_fix", "find security issues in the input validation and fix them").

# =============================================================================
# DOCUMENT-AFTER-CHANGE PATTERNS
# "change X and update docs" - mutation followed by documentation
# =============================================================================

multistep_pattern("implicit_change_document", "document_after_change", "sequential", 80).
multistep_keyword("implicit_change_document", "and update docs").
multistep_keyword("implicit_change_document", "and document").
multistep_keyword("implicit_change_document", "and add comments").
multistep_verb_pair("implicit_change_document", /refactor, /document).
multistep_verb_pair("implicit_change_document", /create, /document).
multistep_verb_pair("implicit_change_document", /fix, /document).
multistep_example("implicit_change_document", "refactor the handler and update the documentation").
multistep_example("implicit_change_document", "add the new endpoint and document it").
multistep_example("implicit_change_document", "change the algorithm and add comments explaining it").

# =============================================================================
# TEST-DRIVEN FLOW PATTERNS
# "write tests first then implement" - TDD style
# =============================================================================

multistep_pattern("tdd_test_first", "test_driven_flow", "sequential", 88).
multistep_keyword("tdd_test_first", "write tests first").
multistep_keyword("tdd_test_first", "tdd").
multistep_keyword("tdd_test_first", "test-driven").
multistep_keyword("tdd_test_first", "tests then implement").
multistep_verb_pair("tdd_test_first", /test, /create).
multistep_example("tdd_test_first", "write tests for the parser first, then implement it").
multistep_example("tdd_test_first", "TDD the new authentication flow").
multistep_example("tdd_test_first", "create tests and then the implementation for the cache").

# =============================================================================
# CONDITIONAL SUCCESS PATTERNS
# "X, if successful, Y" - conditional execution on success
# =============================================================================

multistep_pattern("conditional_if_success", "conditional_success", "conditional", 85).
multistep_keyword("conditional_if_success", "if it works").
multistep_keyword("conditional_if_success", "if successful").
multistep_keyword("conditional_if_success", "if it passes").
multistep_keyword("conditional_if_success", "on success").
multistep_keyword("conditional_if_success", "assuming it works").
multistep_example("conditional_if_success", "run the tests, if they pass, commit").
multistep_example("conditional_if_success", "fix the bug and if it works deploy to staging").
multistep_example("conditional_if_success", "refactor and on success merge the PR").

multistep_pattern("conditional_tests_pass", "conditional_success", "conditional", 87).
multistep_keyword("conditional_tests_pass", "if tests pass").
multistep_keyword("conditional_tests_pass", "when tests pass").
multistep_keyword("conditional_tests_pass", "once tests are green").
multistep_example("conditional_tests_pass", "fix the handler, if the tests pass, push to main").
multistep_example("conditional_tests_pass", "refactor and when tests are green merge").

# =============================================================================
# CONDITIONAL FAILURE / FALLBACK PATTERNS
# "try X, if fails, Y" - fallback on failure
# =============================================================================

multistep_pattern("fallback_if_fails", "conditional_failure", "fallback", 83).
multistep_keyword("fallback_if_fails", "if it fails").
multistep_keyword("fallback_if_fails", "if it doesn't work").
multistep_keyword("fallback_if_fails", "otherwise").
multistep_keyword("fallback_if_fails", "or else").
multistep_keyword("fallback_if_fails", "on failure").
multistep_example("fallback_if_fails", "try the migration, if it fails, rollback").
multistep_example("fallback_if_fails", "apply the patch, otherwise revert to the backup").
multistep_example("fallback_if_fails", "run the deployment and on failure alert the team").

multistep_pattern("fallback_try_revert", "undo_recovery", "fallback", 82).
multistep_keyword("fallback_try_revert", "revert if fails").
multistep_keyword("fallback_try_revert", "rollback if").
multistep_keyword("fallback_try_revert", "undo if fails").
multistep_keyword("fallback_try_revert", "revert if needed").
multistep_example("fallback_try_revert", "try the database migration, revert if it fails").
multistep_example("fallback_try_revert", "apply the changes but be ready to rollback if needed").
multistep_example("fallback_try_revert", "deploy to production and rollback if something goes wrong").

# =============================================================================
# PARALLEL / INDEPENDENT PATTERNS
# "X and Y" where both can run concurrently
# =============================================================================

multistep_pattern("parallel_independent_and", "parallel_independent", "parallel", 75).
multistep_keyword("parallel_independent_and", "review X and review Y").
multistep_keyword("parallel_independent_and", "scan X and scan Y").
multistep_example("parallel_independent_and", "review auth.go and review handler.go").
multistep_example("parallel_independent_and", "analyze the frontend and the backend").
multistep_example("parallel_independent_and", "scan the API and scan the database layer").

multistep_pattern("parallel_also_additionally", "parallel_independent", "parallel", 70).
multistep_keyword("parallel_also_additionally", "also").
multistep_keyword("parallel_also_additionally", "additionally").
multistep_keyword("parallel_also_additionally", "at the same time").
multistep_keyword("parallel_also_additionally", "simultaneously").
multistep_keyword("parallel_also_additionally", "in parallel").
multistep_example("parallel_also_additionally", "review the API, also check the tests").
multistep_example("parallel_also_additionally", "fix the bug, additionally update the changelog").
multistep_example("parallel_also_additionally", "run lint and at the same time run tests").

# =============================================================================
# COMPOUND WITH REFERENCE PATTERNS
# "X and Y it" - pronoun reference to target
# =============================================================================

multistep_pattern("compound_pronoun_ref", "compound_with_ref", "sequential", 88).
multistep_keyword("compound_pronoun_ref", "and test it").
multistep_keyword("compound_pronoun_ref", "and commit it").
multistep_keyword("compound_pronoun_ref", "and deploy it").
multistep_keyword("compound_pronoun_ref", "and document it").
multistep_keyword("compound_pronoun_ref", "and run them").
multistep_example("compound_pronoun_ref", "create the handler and test it").
multistep_example("compound_pronoun_ref", "fix the bug and commit it").
multistep_example("compound_pronoun_ref", "implement the feature and deploy it").
multistep_example("compound_pronoun_ref", "write the tests and run them").

# =============================================================================
# ITERATIVE / BATCH PATTERNS
# "do X to each/every/all Y"
# =============================================================================

multistep_pattern("iterative_each_every", "iterative_collection", "iterative", 80).
multistep_keyword("iterative_each_every", "each").
multistep_keyword("iterative_each_every", "every").
multistep_keyword("iterative_each_every", "all the").
multistep_keyword("iterative_each_every", "for each").
multistep_keyword("iterative_each_every", "for every").
multistep_example("iterative_each_every", "review each handler in cmd/api/").
multistep_example("iterative_each_every", "fix every failing test").
multistep_example("iterative_each_every", "refactor all the deprecated functions").
multistep_example("iterative_each_every", "for each model, add validation").

multistep_pattern("batch_all_files", "batch_operation", "iterative", 78).
multistep_keyword("batch_all_files", "all files").
multistep_keyword("batch_all_files", "entire codebase").
multistep_keyword("batch_all_files", "whole project").
multistep_keyword("batch_all_files", "all go files").
multistep_keyword("batch_all_files", "all typescript files").
multistep_example("batch_all_files", "format all go files").
multistep_example("batch_all_files", "lint the entire codebase").
multistep_example("batch_all_files", "review all typescript files").

# =============================================================================
# PIPELINE / CHAIN PATTERNS
# "X then pass results to Y"
# =============================================================================

multistep_pattern("pipeline_pass_output", "pipeline_chain", "sequential", 85).
multistep_keyword("pipeline_pass_output", "pass the results to").
multistep_keyword("pipeline_pass_output", "feed output to").
multistep_keyword("pipeline_pass_output", "use the results to").
multistep_keyword("pipeline_pass_output", "pipe to").
multistep_example("pipeline_pass_output", "analyze the code and pass the results to the optimizer").
multistep_example("pipeline_pass_output", "review for security issues and use the findings to fix").
multistep_example("pipeline_pass_output", "run static analysis and feed output to the report generator").

multistep_pattern("pipeline_based_on", "pipeline_chain", "sequential", 86).
multistep_keyword("pipeline_based_on", "based on the results").
multistep_keyword("pipeline_based_on", "according to findings").
multistep_keyword("pipeline_based_on", "based on issues").
multistep_example("pipeline_based_on", "review the handlers and then fix based on the findings").
multistep_example("pipeline_based_on", "analyze complexity and refactor according to the results").

# =============================================================================
# COMPARE AND CHOOSE PATTERNS
# "compare X and Y, pick best"
# =============================================================================

multistep_pattern("compare_and_choose", "compare_and_choose", "sequential", 75).
multistep_keyword("compare_and_choose", "compare and pick").
multistep_keyword("compare_and_choose", "compare and choose").
multistep_keyword("compare_and_choose", "evaluate and recommend").
multistep_example("compare_and_choose", "compare the two implementations and pick the best").
multistep_example("compare_and_choose", "evaluate approach A vs B and recommend one").

# =============================================================================
# CONSTRAINT / EXCLUSION PATTERNS
# "do X but not Y" or "do X while keeping Y"
# =============================================================================

multistep_pattern("constraint_but_not", "refactor_preserve", "sequential", 82).
multistep_keyword("constraint_but_not", "but not").
multistep_keyword("constraint_but_not", "but skip").
multistep_keyword("constraint_but_not", "except").
multistep_keyword("constraint_but_not", "excluding").
multistep_keyword("constraint_but_not", "while keeping").
multistep_keyword("constraint_but_not", "preserving").
multistep_example("constraint_but_not", "refactor the handlers but not the middleware").
multistep_example("constraint_but_not", "review all files except tests").
multistep_example("constraint_but_not", "update the API while keeping backwards compatibility").
multistep_example("constraint_but_not", "fix the auth but don't touch the session logic").

# =============================================================================
# GIT WORKFLOW PATTERNS
# "commit and push", "add, commit, and push"
# =============================================================================

multistep_pattern("git_commit_push", "sequential_implicit", "sequential", 85).
multistep_keyword("git_commit_push", "commit and push").
multistep_keyword("git_commit_push", "add and commit").
multistep_keyword("git_commit_push", "stage and commit and push").
multistep_verb_pair("git_commit_push", /git, /git).
multistep_example("git_commit_push", "commit and push").
multistep_example("git_commit_push", "add the changes, commit, and push").
multistep_example("git_commit_push", "stage everything and commit and push to origin").

multistep_pattern("git_branch_workflow", "sequential_implicit", "sequential", 80).
multistep_keyword("git_branch_workflow", "create branch and").
multistep_keyword("git_branch_workflow", "checkout and").
multistep_keyword("git_branch_workflow", "switch to and").
multistep_example("git_branch_workflow", "create a new branch for the feature and start working").
multistep_example("git_branch_workflow", "checkout main and pull the latest").

# =============================================================================
# MULTI-STEP INTENT DEFINITIONS (Sentence-Level)
# =============================================================================

# --- Sequential Explicit ---
intent_definition("First review the code, then fix any issues.", /multi_step, "review_fix").
intent_category("First review the code, then fix any issues.", /mutation).

intent_definition("First create the file, then add tests, finally commit.", /multi_step, "create_test_commit").
intent_category("First create the file, then add tests, finally commit.", /mutation).

intent_definition("Start by analyzing the codebase, then refactor.", /multi_step, "analyze_refactor").
intent_category("Start by analyzing the codebase, then refactor.", /mutation).

intent_definition("Fix the bug, after that run the tests.", /multi_step, "fix_test").
intent_category("Fix the bug, after that run the tests.", /mutation).

intent_definition("Create the interface, next implement it.", /multi_step, "create_implement").
intent_category("Create the interface, next implement it.", /mutation).

intent_definition("Fix the tests, once done commit the changes.", /multi_step, "fix_commit").
intent_category("Fix the tests, once done commit the changes.", /mutation).

# --- Review Then Fix ---
intent_definition("Review auth.go and fix any issues.", /multi_step, "review_fix_file").
intent_category("Review auth.go and fix any issues.", /mutation).

intent_definition("Check the handlers and fix any bugs.", /multi_step, "check_fix").
intent_category("Check the handlers and fix any bugs.", /mutation).

intent_definition("Find and fix all security issues.", /multi_step, "find_fix_security").
intent_category("Find and fix all security issues.", /mutation).

intent_definition("Review and fix issues in the codebase.", /multi_step, "review_fix_codebase").
intent_category("Review and fix issues in the codebase.", /mutation).

# --- Create Then Test ---
intent_definition("Create a new handler and test it.", /multi_step, "create_test").
intent_category("Create a new handler and test it.", /mutation).

intent_definition("Implement the feature and write tests.", /multi_step, "implement_test").
intent_category("Implement the feature and write tests.", /mutation).

intent_definition("Add the endpoint and make sure it works.", /multi_step, "add_verify").
intent_category("Add the endpoint and make sure it works.", /mutation).

intent_definition("Fix it and test it.", /multi_step, "fix_test").
intent_category("Fix it and test it.", /mutation).

intent_definition("Refactor and test.", /multi_step, "refactor_test").
intent_category("Refactor and test.", /mutation).

# --- Security Audit and Fix ---
intent_definition("Security scan and fix vulnerabilities.", /multi_step, "security_fix").
intent_category("Security scan and fix vulnerabilities.", /mutation).

intent_definition("Audit the auth module and patch any issues.", /multi_step, "audit_patch").
intent_category("Audit the auth module and patch any issues.", /mutation).

intent_definition("Scan for security issues and fix them.", /multi_step, "scan_fix").
intent_category("Scan for security issues and fix them.", /mutation).

# --- Research Then Implement ---
intent_definition("Research how to implement OAuth and then add it.", /multi_step, "research_implement").
intent_category("Research how to implement OAuth and then add it.", /mutation).

intent_definition("Figure out the API and implement the client.", /multi_step, "figure_implement").
intent_category("Figure out the API and implement the client.", /mutation).

intent_definition("Understand the codebase and then refactor.", /multi_step, "understand_refactor").
intent_category("Understand the codebase and then refactor.", /mutation).

# --- Analyze Then Optimize ---
intent_definition("Analyze performance and optimize.", /multi_step, "analyze_optimize").
intent_category("Analyze performance and optimize.", /mutation).

intent_definition("Profile the API and improve response times.", /multi_step, "profile_improve").
intent_category("Profile the API and improve response times.", /mutation).

intent_definition("Find bottlenecks and fix them.", /multi_step, "find_fix_bottlenecks").
intent_category("Find bottlenecks and fix them.", /mutation).

# --- Conditional Patterns ---
intent_definition("Run the tests, if they pass, commit.", /multi_step, "test_commit_conditional").
intent_category("Run the tests, if they pass, commit.", /mutation).

intent_definition("Fix the bug and if it works deploy to staging.", /multi_step, "fix_deploy_conditional").
intent_category("Fix the bug and if it works deploy to staging.", /mutation).

intent_definition("Try the migration, if it fails, rollback.", /multi_step, "migrate_rollback").
intent_category("Try the migration, if it fails, rollback.", /mutation).

intent_definition("Apply the patch, otherwise revert.", /multi_step, "patch_revert").
intent_category("Apply the patch, otherwise revert.", /mutation).

# --- Parallel Patterns ---
intent_definition("Review auth.go and review handler.go.", /multi_step, "review_parallel").
intent_category("Review auth.go and review handler.go.", /query).

intent_definition("Run lint and at the same time run tests.", /multi_step, "lint_test_parallel").
intent_category("Run lint and at the same time run tests.", /mutation).

intent_definition("Fix the bug, also update the changelog.", /multi_step, "fix_update_parallel").
intent_category("Fix the bug, also update the changelog.", /mutation).

# --- Pronoun Reference ---
intent_definition("Create the handler and test it.", /multi_step, "create_test_it").
intent_category("Create the handler and test it.", /mutation).

intent_definition("Fix the bug and commit it.", /multi_step, "fix_commit_it").
intent_category("Fix the bug and commit it.", /mutation).

intent_definition("Write the tests and run them.", /multi_step, "write_run_tests").
intent_category("Write the tests and run them.", /mutation).

# --- TDD Patterns ---
intent_definition("Write tests first, then implement.", /multi_step, "tdd_flow").
intent_category("Write tests first, then implement.", /mutation).

intent_definition("TDD the new feature.", /multi_step, "tdd").
intent_category("TDD the new feature.", /mutation).

intent_definition("Test-driven development for the parser.", /multi_step, "tdd_parser").
intent_category("Test-driven development for the parser.", /mutation).

# --- Iterative/Batch Patterns ---
intent_definition("Review each handler in the API.", /multi_step, "review_each").
intent_category("Review each handler in the API.", /query).

intent_definition("Fix every failing test.", /multi_step, "fix_every_test").
intent_category("Fix every failing test.", /mutation).

intent_definition("Refactor all deprecated functions.", /multi_step, "refactor_all").
intent_category("Refactor all deprecated functions.", /mutation).

intent_definition("Format all go files.", /multi_step, "format_all").
intent_category("Format all go files.", /mutation).

intent_definition("Lint the entire codebase.", /multi_step, "lint_all").
intent_category("Lint the entire codebase.", /query).

# --- Git Workflow Patterns ---
intent_definition("Commit and push.", /multi_step, "commit_push").
intent_category("Commit and push.", /mutation).

intent_definition("Add, commit, and push.", /multi_step, "add_commit_push").
intent_category("Add, commit, and push.", /mutation).

intent_definition("Stage the changes and commit.", /multi_step, "stage_commit").
intent_category("Stage the changes and commit.", /mutation).

intent_definition("Create a branch and start working.", /multi_step, "branch_work").
intent_category("Create a branch and start working.", /mutation).

intent_definition("Checkout main and pull.", /multi_step, "checkout_pull").
intent_category("Checkout main and pull.", /mutation).

# --- Document After Change ---
intent_definition("Refactor and update the documentation.", /multi_step, "refactor_document").
intent_category("Refactor and update the documentation.", /mutation).

intent_definition("Add the endpoint and document it.", /multi_step, "add_document").
intent_category("Add the endpoint and document it.", /mutation).

intent_definition("Change the algorithm and add comments.", /multi_step, "change_comment").
intent_category("Change the algorithm and add comments.", /mutation).

# --- Pipeline Patterns ---
intent_definition("Review and fix based on findings.", /multi_step, "review_fix_pipeline").
intent_category("Review and fix based on findings.", /mutation).

intent_definition("Analyze and pass results to the optimizer.", /multi_step, "analyze_optimize_pipeline").
intent_category("Analyze and pass results to the optimizer.", /mutation).

# --- Constraint Patterns ---
intent_definition("Refactor but not the middleware.", /multi_step, "refactor_constrained").
intent_category("Refactor but not the middleware.", /mutation).

intent_definition("Review all files except tests.", /multi_step, "review_except").
intent_category("Review all files except tests.", /query).

intent_definition("Update the API while keeping backwards compatibility.", /multi_step, "update_preserve").
intent_category("Update the API while keeping backwards compatibility.", /mutation).

# =============================================================================
# INFERENCE RULES FOR MULTI-STEP DETECTION
# =============================================================================

# Check if input contains a multi-step keyword
# NOTE: fn:string_contains is not a Mangle built-in. These rules need to be
# implemented as virtual predicates in Go that perform string matching.
Decl is_multistep_input(Input).
# is_multistep_input(Input) :-
#     multistep_keyword(Pattern, Keyword),
#     fn:string_contains(Input, Keyword).

# Get the best matching pattern for input
Decl best_multistep_pattern(Input, Pattern, Priority).
# best_multistep_pattern(Input, Pattern, Priority) :-
#     multistep_keyword(Pattern, Keyword),
#     fn:string_contains(Input, Keyword),
#     multistep_pattern(Pattern, _, _, Priority).

# Get verb pairs for a pattern
Decl pattern_verb_pair(Pattern, Verb1, Verb2).
pattern_verb_pair(Pattern, Verb1, Verb2) :-
    multistep_verb_pair(Pattern, Verb1, Verb2).

# Get relation type for a pattern
Decl pattern_relation(Pattern, Relation).
pattern_relation(Pattern, Relation) :-
    multistep_pattern(Pattern, _, Relation, _).

# =============================================================================
# SECTION 24: ENCYCLOPEDIC MULTI-STEP SENTENCE CORPUS
# Complete coverage of multi-step request phrasings
# =============================================================================

# ---------------------------------------------------------------------------
# SEQUENTIAL EXPLICIT - "First X, then Y" patterns
# ---------------------------------------------------------------------------

# First-Then-Finally patterns
intent_definition("First review the code, then fix any issues, finally run the tests.", /multi_step, "review_fix_test").
intent_category("First review the code, then fix any issues, finally run the tests.", /mutation).

intent_definition("First analyze the performance, then optimize the bottlenecks.", /multi_step, "analyze_optimize").
intent_category("First analyze the performance, then optimize the bottlenecks.", /mutation).

intent_definition("First understand how it works, then refactor it.", /multi_step, "understand_refactor").
intent_category("First understand how it works, then refactor it.", /mutation).

intent_definition("First check for bugs, then fix them, finally commit.", /multi_step, "check_fix_commit").
intent_category("First check for bugs, then fix them, finally commit.", /mutation).

intent_definition("First research the library, then implement the integration.", /multi_step, "research_implement").
intent_category("First research the library, then implement the integration.", /mutation).

intent_definition("First scan for security issues, then patch them.", /multi_step, "scan_patch").
intent_category("First scan for security issues, then patch them.", /mutation).

intent_definition("First explore the codebase, then identify refactoring opportunities.", /multi_step, "explore_identify").
intent_category("First explore the codebase, then identify refactoring opportunities.", /query).

intent_definition("First run the tests, then analyze failures, finally fix them.", /multi_step, "test_analyze_fix").
intent_category("First run the tests, then analyze failures, finally fix them.", /mutation).

intent_definition("First backup the database, then run the migration.", /multi_step, "backup_migrate").
intent_category("First backup the database, then run the migration.", /mutation).

intent_definition("First create a branch, then make the changes, finally open a PR.", /multi_step, "branch_change_pr").
intent_category("First create a branch, then make the changes, finally open a PR.", /mutation).

# Start-by patterns
intent_definition("Start by reviewing the authentication module.", /multi_step, "start_review").
intent_category("Start by reviewing the authentication module.", /query).

intent_definition("Start by analyzing the dependencies, then upgrade them.", /multi_step, "start_analyze_upgrade").
intent_category("Start by analyzing the dependencies, then upgrade them.", /mutation).

intent_definition("Start by creating the interface, then implement it.", /multi_step, "start_create_implement").
intent_category("Start by creating the interface, then implement it.", /mutation).

intent_definition("Start by writing failing tests, then make them pass.", /multi_step, "start_tdd").
intent_category("Start by writing failing tests, then make them pass.", /mutation).

intent_definition("Begin with security analysis, then address the findings.", /multi_step, "begin_security_fix").
intent_category("Begin with security analysis, then address the findings.", /mutation).

intent_definition("Begin by profiling the code, then optimize hot paths.", /multi_step, "begin_profile_optimize").
intent_category("Begin by profiling the code, then optimize hot paths.", /mutation).

# After-that patterns
intent_definition("Fix the null pointer, after that add proper error handling.", /multi_step, "fix_add_handling").
intent_category("Fix the null pointer, after that add proper error handling.", /mutation).

intent_definition("Refactor the function, afterward update the callers.", /multi_step, "refactor_update_callers").
intent_category("Refactor the function, afterward update the callers.", /mutation).

intent_definition("Create the model, afterwards write the repository.", /multi_step, "create_model_repo").
intent_category("Create the model, afterwards write the repository.", /mutation).

intent_definition("Implement the feature, following that write integration tests.", /multi_step, "implement_integration_test").
intent_category("Implement the feature, following that write integration tests.", /mutation).

# Once-done patterns
intent_definition("Fix all the lint errors, once done run the full test suite.", /multi_step, "fix_lint_test").
intent_category("Fix all the lint errors, once done run the full test suite.", /mutation).

intent_definition("Refactor the database layer, when finished update the documentation.", /multi_step, "refactor_document").
intent_category("Refactor the database layer, when finished update the documentation.", /mutation).

intent_definition("Complete the API endpoints, once complete deploy to staging.", /multi_step, "complete_deploy").
intent_category("Complete the API endpoints, once complete deploy to staging.", /mutation).

intent_definition("Finish the migration, when done verify data integrity.", /multi_step, "finish_verify").
intent_category("Finish the migration, when done verify data integrity.", /mutation).

# Numbered step patterns
intent_definition("1. Review the PR 2. Leave comments 3. Approve or request changes.", /multi_step, "numbered_pr_review").
intent_category("1. Review the PR 2. Leave comments 3. Approve or request changes.", /query).

intent_definition("Step 1: Create the handler. Step 2: Add routes. Step 3: Write tests.", /multi_step, "step_handler_routes_tests").
intent_category("Step 1: Create the handler. Step 2: Add routes. Step 3: Write tests.", /mutation).

intent_definition("1. Backup 2. Migrate 3. Verify 4. Deploy.", /multi_step, "numbered_backup_deploy").
intent_category("1. Backup 2. Migrate 3. Verify 4. Deploy.", /mutation).

intent_definition("1) Analyze the issue 2) Create a fix 3) Test the fix 4) Commit.", /multi_step, "numbered_analyze_commit").
intent_category("1) Analyze the issue 2) Create a fix 3) Test the fix 4) Commit.", /mutation).

# ---------------------------------------------------------------------------
# REVIEW-THEN-FIX - Analysis followed by remediation
# ---------------------------------------------------------------------------

intent_definition("Review the handler and fix any issues you find.", /multi_step, "review_fix_handler").
intent_category("Review the handler and fix any issues you find.", /mutation).

intent_definition("Check the error handling and improve where needed.", /multi_step, "check_improve_errors").
intent_category("Check the error handling and improve where needed.", /mutation).

intent_definition("Audit the authentication flow and fix vulnerabilities.", /multi_step, "audit_fix_auth").
intent_category("Audit the authentication flow and fix vulnerabilities.", /mutation).

intent_definition("Review the database queries and optimize slow ones.", /multi_step, "review_optimize_queries").
intent_category("Review the database queries and optimize slow ones.", /mutation).

intent_definition("Check the API endpoints for security issues and patch them.", /multi_step, "check_patch_api").
intent_category("Check the API endpoints for security issues and patch them.", /mutation).

intent_definition("Analyze the memory usage and fix any leaks.", /multi_step, "analyze_fix_leaks").
intent_category("Analyze the memory usage and fix any leaks.", /mutation).

intent_definition("Find all TODO comments and address them.", /multi_step, "find_address_todos").
intent_category("Find all TODO comments and address them.", /mutation).

intent_definition("Look for code duplication and refactor it out.", /multi_step, "find_refactor_duplication").
intent_category("Look for code duplication and refactor it out.", /mutation).

intent_definition("Identify dead code and remove it.", /multi_step, "identify_remove_dead_code").
intent_category("Identify dead code and remove it.", /mutation).

intent_definition("Find race conditions and fix them.", /multi_step, "find_fix_race_conditions").
intent_category("Find race conditions and fix them.", /mutation).

intent_definition("Review for OWASP top 10 and remediate.", /multi_step, "review_remediate_owasp").
intent_category("Review for OWASP top 10 and remediate.", /mutation).

intent_definition("Check for hardcoded secrets and extract to config.", /multi_step, "check_extract_secrets").
intent_category("Check for hardcoded secrets and extract to config.", /mutation).

# ---------------------------------------------------------------------------
# CREATE-THEN-VALIDATE - Creation followed by testing
# ---------------------------------------------------------------------------

intent_definition("Create a new endpoint and write tests for it.", /multi_step, "create_test_endpoint").
intent_category("Create a new endpoint and write tests for it.", /mutation).

intent_definition("Implement the service and verify it works.", /multi_step, "implement_verify_service").
intent_category("Implement the service and verify it works.", /mutation).

intent_definition("Add the feature and make sure tests pass.", /multi_step, "add_ensure_tests").
intent_category("Add the feature and make sure tests pass.", /mutation).

intent_definition("Write the migration and verify it applies correctly.", /multi_step, "write_verify_migration").
intent_category("Write the migration and verify it applies correctly.", /mutation).

intent_definition("Create the validator and test edge cases.", /multi_step, "create_test_validator").
intent_category("Create the validator and test edge cases.", /mutation).

intent_definition("Build the parser and verify it handles all formats.", /multi_step, "build_verify_parser").
intent_category("Build the parser and verify it handles all formats.", /mutation).

intent_definition("Implement caching and benchmark the improvement.", /multi_step, "implement_benchmark_cache").
intent_category("Implement caching and benchmark the improvement.", /mutation).

intent_definition("Add rate limiting and test under load.", /multi_step, "add_test_rate_limiting").
intent_category("Add rate limiting and test under load.", /mutation).

intent_definition("Create the middleware and verify it intercepts requests.", /multi_step, "create_verify_middleware").
intent_category("Create the middleware and verify it intercepts requests.", /mutation).

intent_definition("Implement retry logic and test failure scenarios.", /multi_step, "implement_test_retry").
intent_category("Implement retry logic and test failure scenarios.", /mutation).

# ---------------------------------------------------------------------------
# CONDITIONAL SUCCESS - Action contingent on success
# ---------------------------------------------------------------------------

intent_definition("Run the tests, if they pass, push to main.", /multi_step, "test_push_conditional").
intent_category("Run the tests, if they pass, push to main.", /mutation).

intent_definition("Fix the bug, and if it works, deploy to staging.", /multi_step, "fix_deploy_staging").
intent_category("Fix the bug, and if it works, deploy to staging.", /mutation).

intent_definition("Refactor the function, if successful, update the docs.", /multi_step, "refactor_docs_conditional").
intent_category("Refactor the function, if successful, update the docs.", /mutation).

intent_definition("Run the migration, if it succeeds, notify the team.", /multi_step, "migrate_notify_conditional").
intent_category("Run the migration, if it succeeds, notify the team.", /mutation).

intent_definition("Apply the patch, on success, merge the PR.", /multi_step, "patch_merge_conditional").
intent_category("Apply the patch, on success, merge the PR.", /mutation).

intent_definition("Build the project, if no errors, create a release.", /multi_step, "build_release_conditional").
intent_category("Build the project, if no errors, create a release.", /mutation).

intent_definition("Run lint, when tests pass, deploy.", /multi_step, "lint_test_deploy").
intent_category("Run lint, when tests pass, deploy.", /mutation).

intent_definition("Fix the flaky test, once it's green, enable CI.", /multi_step, "fix_enable_ci").
intent_category("Fix the flaky test, once it's green, enable CI.", /mutation).

intent_definition("Run the benchmarks, if performance improves, merge.", /multi_step, "benchmark_merge_conditional").
intent_category("Run the benchmarks, if performance improves, merge.", /mutation).

intent_definition("Test the integration, assuming it works, document it.", /multi_step, "test_document_conditional").
intent_category("Test the integration, assuming it works, document it.", /mutation).

# ---------------------------------------------------------------------------
# CONDITIONAL FAILURE / FALLBACK - Action contingent on failure
# ---------------------------------------------------------------------------

intent_definition("Try the migration, if it fails, rollback immediately.", /multi_step, "migrate_rollback").
intent_category("Try the migration, if it fails, rollback immediately.", /mutation).

intent_definition("Deploy to production, otherwise revert to the previous version.", /multi_step, "deploy_revert").
intent_category("Deploy to production, otherwise revert to the previous version.", /mutation).

intent_definition("Apply the fix, on failure, restore from backup.", /multi_step, "fix_restore_backup").
intent_category("Apply the fix, on failure, restore from backup.", /mutation).

intent_definition("Run the update, if it breaks anything, undo all changes.", /multi_step, "update_undo").
intent_category("Run the update, if it breaks anything, undo all changes.", /mutation).

intent_definition("Try the refactoring, revert if tests fail.", /multi_step, "refactor_revert_conditional").
intent_category("Try the refactoring, revert if tests fail.", /mutation).

intent_definition("Attempt the upgrade, rollback if needed.", /multi_step, "upgrade_rollback").
intent_category("Attempt the upgrade, rollback if needed.", /mutation).

intent_definition("Push the changes, if it fails, fix and retry.", /multi_step, "push_retry").
intent_category("Push the changes, if it fails, fix and retry.", /mutation).

intent_definition("Deploy to staging, if something goes wrong, alert the team.", /multi_step, "deploy_alert").
intent_category("Deploy to staging, if something goes wrong, alert the team.", /mutation).

intent_definition("Run the script, on error, log and continue.", /multi_step, "run_log_continue").
intent_category("Run the script, on error, log and continue.", /mutation).

intent_definition("Apply the patch, if errors occur, open an issue.", /multi_step, "patch_open_issue").
intent_category("Apply the patch, if errors occur, open an issue.", /mutation).

# ---------------------------------------------------------------------------
# PARALLEL - Independent concurrent operations
# ---------------------------------------------------------------------------

intent_definition("Review the frontend and review the backend.", /multi_step, "review_frontend_backend").
intent_category("Review the frontend and review the backend.", /query).

intent_definition("Run unit tests and integration tests in parallel.", /multi_step, "test_parallel").
intent_category("Run unit tests and integration tests in parallel.", /mutation).

intent_definition("Lint the code, also run type checking.", /multi_step, "lint_typecheck_parallel").
intent_category("Lint the code, also run type checking.", /mutation).

intent_definition("Analyze the API, additionally check the database schema.", /multi_step, "analyze_api_db").
intent_category("Analyze the API, additionally check the database schema.", /query).

intent_definition("Review security at the same time as reviewing performance.", /multi_step, "review_security_performance").
intent_category("Review security at the same time as reviewing performance.", /query).

intent_definition("Fix the bug, simultaneously update the changelog.", /multi_step, "fix_changelog_parallel").
intent_category("Fix the bug, simultaneously update the changelog.", /mutation).

intent_definition("Deploy to staging and production in parallel.", /multi_step, "deploy_parallel").
intent_category("Deploy to staging and production in parallel.", /mutation).

intent_definition("Run tests across multiple environments simultaneously.", /multi_step, "test_multi_env").
intent_category("Run tests across multiple environments simultaneously.", /mutation).

intent_definition("Build for Linux and Windows at the same time.", /multi_step, "build_multi_platform").
intent_category("Build for Linux and Windows at the same time.", /mutation).

intent_definition("Scan for vulnerabilities while running performance tests.", /multi_step, "scan_perf_parallel").
intent_category("Scan for vulnerabilities while running performance tests.", /mutation).

# ---------------------------------------------------------------------------
# ITERATIVE / BATCH - Operations over collections
# ---------------------------------------------------------------------------

intent_definition("Review each file in the handlers directory.", /multi_step, "review_each_handler").
intent_category("Review each file in the handlers directory.", /query).

intent_definition("Fix every failing test in the test suite.", /multi_step, "fix_every_failing_test").
intent_category("Fix every failing test in the test suite.", /mutation).

intent_definition("Refactor all deprecated functions to use the new API.", /multi_step, "refactor_all_deprecated").
intent_category("Refactor all deprecated functions to use the new API.", /mutation).

intent_definition("Update all config files to the new format.", /multi_step, "update_all_configs").
intent_category("Update all config files to the new format.", /mutation).

intent_definition("For each endpoint, add rate limiting.", /multi_step, "foreach_rate_limit").
intent_category("For each endpoint, add rate limiting.", /mutation).

intent_definition("For every model, add validation.", /multi_step, "foreach_validation").
intent_category("For every model, add validation.", /mutation).

intent_definition("Review all Go files in the project.", /multi_step, "review_all_go").
intent_category("Review all Go files in the project.", /query).

intent_definition("Fix all lint errors across the codebase.", /multi_step, "fix_all_lint").
intent_category("Fix all lint errors across the codebase.", /mutation).

intent_definition("Add logging to each handler one by one.", /multi_step, "add_logging_each").
intent_category("Add logging to each handler one by one.", /mutation).

intent_definition("Review the entire authentication module.", /multi_step, "review_entire_auth").
intent_category("Review the entire authentication module.", /query).

intent_definition("Test all edge cases for the parser.", /multi_step, "test_all_edge_cases").
intent_category("Test all edge cases for the parser.", /mutation).

intent_definition("Check throughout the codebase for SQL injection.", /multi_step, "check_throughout_sql").
intent_category("Check throughout the codebase for SQL injection.", /query).

# ---------------------------------------------------------------------------
# RESEARCH-THEN-ACT - Learning followed by implementation
# ---------------------------------------------------------------------------

intent_definition("Research how to implement WebSockets and then add them.", /multi_step, "research_websockets").
intent_category("Research how to implement WebSockets and then add them.", /mutation).

intent_definition("Figure out the OAuth flow and implement it.", /multi_step, "figure_oauth").
intent_category("Figure out the OAuth flow and implement it.", /mutation).

intent_definition("Learn how the caching works and then optimize it.", /multi_step, "learn_optimize_cache").
intent_category("Learn how the caching works and then optimize it.", /mutation).

intent_definition("Understand the event system and then add new events.", /multi_step, "understand_add_events").
intent_category("Understand the event system and then add new events.", /mutation).

intent_definition("Look up the API documentation and then integrate.", /multi_step, "lookup_integrate_api").
intent_category("Look up the API documentation and then integrate.", /mutation).

intent_definition("Investigate the bug and then fix it.", /multi_step, "investigate_fix_bug").
intent_category("Investigate the bug and then fix it.", /mutation).

intent_definition("Study the architecture and then propose improvements.", /multi_step, "study_propose_improvements").
intent_category("Study the architecture and then propose improvements.", /query).

intent_definition("Find out how the tests are structured and add new ones.", /multi_step, "findout_add_tests").
intent_category("Find out how the tests are structured and add new ones.", /mutation).

intent_definition("Explore the plugin system and then create a plugin.", /multi_step, "explore_create_plugin").
intent_category("Explore the plugin system and then create a plugin.", /mutation).

intent_definition("Read about best practices and then apply them.", /multi_step, "read_apply_practices").
intent_category("Read about best practices and then apply them.", /mutation).

# ---------------------------------------------------------------------------
# GIT WORKFLOW - Version control operations
# ---------------------------------------------------------------------------

intent_definition("Stage all changes and commit with a message.", /multi_step, "stage_commit").
intent_category("Stage all changes and commit with a message.", /mutation).

intent_definition("Commit the changes and push to origin.", /multi_step, "commit_push").
intent_category("Commit the changes and push to origin.", /mutation).

intent_definition("Create a feature branch and start implementing.", /multi_step, "branch_implement").
intent_category("Create a feature branch and start implementing.", /mutation).

intent_definition("Checkout main, pull latest, then create a branch.", /multi_step, "checkout_pull_branch").
intent_category("Checkout main, pull latest, then create a branch.", /mutation).

intent_definition("Stash changes, pull updates, then pop the stash.", /multi_step, "stash_pull_pop").
intent_category("Stash changes, pull updates, then pop the stash.", /mutation).

intent_definition("Rebase onto main and resolve any conflicts.", /multi_step, "rebase_resolve").
intent_category("Rebase onto main and resolve any conflicts.", /mutation).

intent_definition("Squash the commits and force push.", /multi_step, "squash_force_push").
intent_category("Squash the commits and force push.", /mutation).

intent_definition("Tag the release and push tags.", /multi_step, "tag_push").
intent_category("Tag the release and push tags.", /mutation).

intent_definition("Cherry-pick the commit and push to the release branch.", /multi_step, "cherrypick_push").
intent_category("Cherry-pick the commit and push to the release branch.", /mutation).

intent_definition("Merge the PR and delete the branch.", /multi_step, "merge_delete_branch").
intent_category("Merge the PR and delete the branch.", /mutation).

# ---------------------------------------------------------------------------
# PRONOUN REFERENCE - "X and Y it" patterns
# ---------------------------------------------------------------------------

intent_definition("Create the endpoint and test it thoroughly.", /multi_step, "create_test_it").
intent_category("Create the endpoint and test it thoroughly.", /mutation).

intent_definition("Fix the issue and verify it's resolved.", /multi_step, "fix_verify_it").
intent_category("Fix the issue and verify it's resolved.", /mutation).

intent_definition("Implement the feature and document it.", /multi_step, "implement_document_it").
intent_category("Implement the feature and document it.", /mutation).

intent_definition("Write the function and unit test it.", /multi_step, "write_unittest_it").
intent_category("Write the function and unit test it.", /mutation).

intent_definition("Create the migration and run it.", /multi_step, "create_run_it").
intent_category("Create the migration and run it.", /mutation).

intent_definition("Build the module and publish it.", /multi_step, "build_publish_it").
intent_category("Build the module and publish it.", /mutation).

intent_definition("Write the script and execute it.", /multi_step, "write_execute_it").
intent_category("Write the script and execute it.", /mutation).

intent_definition("Create the tests and run them.", /multi_step, "create_run_them").
intent_category("Create the tests and run them.", /mutation).

intent_definition("Find the bugs and fix them all.", /multi_step, "find_fix_them").
intent_category("Find the bugs and fix them all.", /mutation).

intent_definition("Generate the mocks and use them in tests.", /multi_step, "generate_use_them").
intent_category("Generate the mocks and use them in tests.", /mutation).

# ---------------------------------------------------------------------------
# CONSTRAINT PATTERNS - Exclusion and preservation
# ---------------------------------------------------------------------------

intent_definition("Refactor the handlers but not the middleware.", /multi_step, "refactor_not_middleware").
intent_category("Refactor the handlers but not the middleware.", /mutation).

intent_definition("Review all files except the generated ones.", /multi_step, "review_except_generated").
intent_category("Review all files except the generated ones.", /query).

intent_definition("Fix the bugs but skip the known issues.", /multi_step, "fix_skip_known").
intent_category("Fix the bugs but skip the known issues.", /mutation).

intent_definition("Update the dependencies excluding dev dependencies.", /multi_step, "update_exclude_dev").
intent_category("Update the dependencies excluding dev dependencies.", /mutation).

intent_definition("Refactor while keeping the public API stable.", /multi_step, "refactor_keep_api").
intent_category("Refactor while keeping the public API stable.", /mutation).

intent_definition("Update the code while preserving backwards compatibility.", /multi_step, "update_preserve_compat").
intent_category("Update the code while preserving backwards compatibility.", /mutation).

intent_definition("Optimize the function without changing its behavior.", /multi_step, "optimize_without_change").
intent_category("Optimize the function without changing its behavior.", /mutation).

intent_definition("Clean up the code without breaking tests.", /multi_step, "cleanup_without_break").
intent_category("Clean up the code without breaking tests.", /mutation).

intent_definition("Only update the authentication module.", /multi_step, "only_auth").
intent_category("Only update the authentication module.", /mutation).

intent_definition("Just fix the critical bugs.", /multi_step, "just_critical").
intent_category("Just fix the critical bugs.", /mutation).

# ---------------------------------------------------------------------------
# PIPELINE PATTERNS - Output passing
# ---------------------------------------------------------------------------

intent_definition("Analyze the code and use the results to prioritize fixes.", /multi_step, "analyze_prioritize").
intent_category("Analyze the code and use the results to prioritize fixes.", /mutation).

intent_definition("Run static analysis and feed the output to the fixer.", /multi_step, "static_feed_fixer").
intent_category("Run static analysis and feed the output to the fixer.", /mutation).

intent_definition("Profile the application and based on the results, optimize.", /multi_step, "profile_based_optimize").
intent_category("Profile the application and based on the results, optimize.", /mutation).

intent_definition("Review for security issues and according to findings, patch.", /multi_step, "review_according_patch").
intent_category("Review for security issues and according to findings, patch.", /mutation).

intent_definition("Collect metrics and using the output, generate a report.", /multi_step, "collect_using_report").
intent_category("Collect metrics and using the output, generate a report.", /query).

intent_definition("Run benchmarks and pass results to the optimizer.", /multi_step, "benchmark_pass_optimizer").
intent_category("Run benchmarks and pass results to the optimizer.", /mutation).

# ---------------------------------------------------------------------------
# TDD PATTERNS - Test-driven development
# ---------------------------------------------------------------------------

intent_definition("Write the tests first, then make them pass.", /multi_step, "tdd_write_pass").
intent_category("Write the tests first, then make them pass.", /mutation).

intent_definition("TDD the new authentication system.", /multi_step, "tdd_auth").
intent_category("TDD the new authentication system.", /mutation).

intent_definition("Test-driven develop the payment integration.", /multi_step, "tdd_payment").
intent_category("Test-driven develop the payment integration.", /mutation).

intent_definition("Start with failing tests, then implement until green.", /multi_step, "tdd_failing_green").
intent_category("Start with failing tests, then implement until green.", /mutation).

intent_definition("Red-green-refactor the new feature.", /multi_step, "tdd_red_green_refactor").
intent_category("Red-green-refactor the new feature.", /mutation).

intent_definition("Write acceptance tests first, then build the feature.", /multi_step, "bdd_acceptance_build").
intent_category("Write acceptance tests first, then build the feature.", /mutation).

# ---------------------------------------------------------------------------
# SECURITY PATTERNS - Security audit and fix
# ---------------------------------------------------------------------------

intent_definition("Scan for OWASP vulnerabilities and fix all critical ones.", /multi_step, "scan_fix_owasp").
intent_category("Scan for OWASP vulnerabilities and fix all critical ones.", /mutation).

intent_definition("Audit the input validation and harden it.", /multi_step, "audit_harden_input").
intent_category("Audit the input validation and harden it.", /mutation).

intent_definition("Check for injection vulnerabilities and sanitize inputs.", /multi_step, "check_sanitize_injection").
intent_category("Check for injection vulnerabilities and sanitize inputs.", /mutation).

intent_definition("Review authentication and add MFA support.", /multi_step, "review_add_mfa").
intent_category("Review authentication and add MFA support.", /mutation).

intent_definition("Find exposed secrets and rotate them.", /multi_step, "find_rotate_secrets").
intent_category("Find exposed secrets and rotate them.", /mutation).

intent_definition("Check for XSS vulnerabilities and add escaping.", /multi_step, "check_add_escaping").
intent_category("Check for XSS vulnerabilities and add escaping.", /mutation).

intent_definition("Audit the session management and fix weaknesses.", /multi_step, "audit_fix_sessions").
intent_category("Audit the session management and fix weaknesses.", /mutation).

intent_definition("Review CORS configuration and tighten it.", /multi_step, "review_tighten_cors").
intent_category("Review CORS configuration and tighten it.", /mutation).

# ---------------------------------------------------------------------------
# DOCUMENTATION PATTERNS - Change and document
# ---------------------------------------------------------------------------

intent_definition("Refactor the API and update the OpenAPI spec.", /multi_step, "refactor_update_openapi").
intent_category("Refactor the API and update the OpenAPI spec.", /mutation).

intent_definition("Add the feature and write user documentation.", /multi_step, "add_write_docs").
intent_category("Add the feature and write user documentation.", /mutation).

intent_definition("Change the configuration format and update the README.", /multi_step, "change_update_readme").
intent_category("Change the configuration format and update the README.", /mutation).

intent_definition("Rename the function and update all references in docs.", /multi_step, "rename_update_docs").
intent_category("Rename the function and update all references in docs.", /mutation).

intent_definition("Deprecate the old API and document the migration path.", /multi_step, "deprecate_document_migration").
intent_category("Deprecate the old API and document the migration path.", /mutation).

intent_definition("Add inline comments explaining the algorithm.", /multi_step, "add_comments_algorithm").
intent_category("Add inline comments explaining the algorithm.", /mutation).

# ---------------------------------------------------------------------------
# COMPARE AND CHOOSE PATTERNS
# ---------------------------------------------------------------------------

intent_definition("Compare the two approaches and pick the better one.", /multi_step, "compare_pick").
intent_category("Compare the two approaches and pick the better one.", /query).

intent_definition("Evaluate both implementations and recommend one.", /multi_step, "evaluate_recommend").
intent_category("Evaluate both implementations and recommend one.", /query).

intent_definition("Benchmark both solutions and choose the faster one.", /multi_step, "benchmark_choose").
intent_category("Benchmark both solutions and choose the faster one.", /mutation).

intent_definition("Analyze the trade-offs and suggest the best option.", /multi_step, "analyze_suggest").
intent_category("Analyze the trade-offs and suggest the best option.", /query).

intent_definition("Compare memory usage of both and select the efficient one.", /multi_step, "compare_select_memory").
intent_category("Compare memory usage of both and select the efficient one.", /query).
