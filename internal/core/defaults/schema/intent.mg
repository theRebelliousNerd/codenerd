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
