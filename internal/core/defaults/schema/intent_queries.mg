# Intent Queries

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

# --- Review with Enhancement (creative suggestions) ---
intent_definition("Review and enhance this file.", /review_enhance, "context_file").
intent_category("Review and enhance this file.", /query).

intent_definition("Review this with suggestions.", /review_enhance, "context_file").
intent_category("Review this with suggestions.", /query).

intent_definition("Review and suggest improvements.", /review_enhance, "context_file").
intent_category("Review and suggest improvements.", /query).

intent_definition("Deep review with enhancement.", /review_enhance, "context_file").
intent_category("Deep review with enhancement.", /query).

intent_definition("Review this creatively.", /review_enhance, "context_file").
intent_category("Review this creatively.", /query).

intent_definition("Give me creative feedback.", /review_enhance, "context_file").
intent_category("Give me creative feedback.", /query).

intent_definition("Review with feature ideas.", /review_enhance, "context_file").
intent_category("Review with feature ideas.", /query).

intent_definition("Suggest improvements for this code.", /review_enhance, "context_file").
intent_category("Suggest improvements for this code.", /query).

intent_definition("What could be improved here?", /review_enhance, "context_file").
intent_category("What could be improved here?", /query).

intent_definition("How can I make this better?", /review_enhance, "context_file").
intent_category("How can I make this better?", /query).


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


