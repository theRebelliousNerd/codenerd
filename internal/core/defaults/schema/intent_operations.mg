# Intent Definitions - System Operations
# SECTIONS 12-22: Research, Explain, Tool Generation, Campaigns, Git, Search, Explore, Config, Knowledge, Shadow, Misc
# Various system operations and queries.

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
