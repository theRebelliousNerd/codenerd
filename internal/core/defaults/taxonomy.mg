Decl verb_def(Verb, Category, Shard, Priority).
Descr verb_def(mode('-', '-', '-', '-')).

Decl verb_synonym(Verb, Synonym).
Descr verb_synonym(mode('-', '-')).

Decl verb_pattern(Verb, Regex).
Descr verb_pattern(mode('-', '-')).

# =========================================================================
# CODE REVIEW & ANALYSIS (Reviewer)
# =========================================================================

# /review
verb_def(/review, /query, /reviewer, 100).
verb_synonym(/review, "review").
verb_synonym(/review, "code review").
verb_synonym(/review, "pr review").
verb_synonym(/review, "check code").
verb_synonym(/review, "audit").
verb_synonym(/review, "evaluate").
verb_synonym(/review, "critique").
# Regexes simplified to avoid backslash hell - using dot for wildcard
verb_pattern(/review, "(?i)review.*code").
verb_pattern(/review, "(?i)can.*you.*review").
verb_pattern(/review, "(?i)check.*(this|the).*file").

# /security
verb_def(/security, /query, /reviewer, 105).
verb_synonym(/security, "security").
verb_synonym(/security, "security scan").
verb_synonym(/security, "vulnerability").
verb_synonym(/security, "injection").
verb_synonym(/security, "xss").
verb_pattern(/security, "(?i)security.*scan").
verb_pattern(/security, "(?i)check.*for.*vuln").
verb_pattern(/security, "(?i)find.*vulnerabilities").

# /analyze
verb_def(/analyze, /query, /reviewer, 95).
verb_synonym(/analyze, "analyze").
verb_synonym(/analyze, "complexity").
verb_synonym(/analyze, "metrics").
verb_synonym(/analyze, "lint").
verb_synonym(/analyze, "code smell").
verb_pattern(/analyze, "(?i)analyze.*code").
verb_pattern(/analyze, "(?i)static.*analysis").

# =========================================================================
# UNDERSTANDING (Researcher/None)
# =========================================================================

# /explain
verb_def(/explain, /query, /none, 80).
verb_synonym(/explain, "explain").
verb_synonym(/explain, "describe").
verb_synonym(/explain, "what is").
verb_synonym(/explain, "how does").
verb_synonym(/explain, "help me understand").
verb_pattern(/explain, "(?i)explain.*this").
verb_pattern(/explain, "(?i)tell.*me.*about").
verb_pattern(/explain, "(?i)help.*understand").

# /explore
verb_def(/explore, /query, /researcher, 75).
verb_synonym(/explore, "explore").
verb_synonym(/explore, "browse").
verb_synonym(/explore, "show structure").
verb_synonym(/explore, "list files").
verb_pattern(/explore, "(?i)show.*structure").
verb_pattern(/explore, "(?i)explore.*codebase").

# /search
verb_def(/search, /query, /researcher, 85).
verb_synonym(/search, "search").
verb_synonym(/search, "find").
verb_synonym(/search, "grep").
verb_synonym(/search, "occurrences").
verb_pattern(/search, "(?i)search.*for").
verb_pattern(/search, "(?i)find.*all").
verb_pattern(/search, "(?i)grep").

# =========================================================================
# MUTATION (Coder)
# =========================================================================

# /fix
verb_def(/fix, /mutation, /coder, 90).
verb_synonym(/fix, "fix").
verb_synonym(/fix, "repair").
verb_synonym(/fix, "patch").
verb_synonym(/fix, "resolve").
verb_synonym(/fix, "bug fix").
verb_pattern(/fix, "(?i)fix.*bug").
verb_pattern(/fix, "(?i)repair.*this").
verb_pattern(/fix, "(?i)resolve.*issue").

# /refactor
verb_def(/refactor, /mutation, /coder, 88).
verb_synonym(/refactor, "refactor").
verb_synonym(/refactor, "clean up").
verb_synonym(/refactor, "improve").
verb_synonym(/refactor, "optimize").
verb_synonym(/refactor, "simplify").
verb_pattern(/refactor, "(?i)refactor").
verb_pattern(/refactor, "(?i)clean.*up").
verb_pattern(/refactor, "(?i)improve.*code").

# /create
verb_def(/create, /mutation, /coder, 85).
verb_synonym(/create, "create").
verb_synonym(/create, "new").
verb_synonym(/create, "add").
verb_synonym(/create, "implement").
verb_synonym(/create, "generate").
verb_pattern(/create, "(?i)create.*new").
verb_pattern(/create, "(?i)add.*new").
verb_pattern(/create, "(?i)implement").

# /write
verb_def(/write, /mutation, /coder, 70).
verb_synonym(/write, "write").
verb_synonym(/write, "save").
verb_synonym(/write, "export").
verb_pattern(/write, "(?i)write.*to").
verb_pattern(/write, "(?i)save.*to").

# /delete
verb_def(/delete, /mutation, /coder, 85).
verb_synonym(/delete, "delete").
verb_synonym(/delete, "remove").
verb_synonym(/delete, "drop").
verb_pattern(/delete, "(?i)delete").
verb_pattern(/delete, "(?i)remove").

# =========================================================================
# DEBUGGING (Coder)
# =========================================================================

# /debug
verb_def(/debug, /query, /coder, 92).
verb_synonym(/debug, "debug").
verb_synonym(/debug, "troubleshoot").
verb_synonym(/debug, "diagnose").
verb_synonym(/debug, "root cause").
verb_pattern(/debug, "(?i)debug").
verb_pattern(/debug, "(?i)troubleshoot").
verb_pattern(/debug, "(?i)why.*fail").

# =========================================================================
# TESTING (Tester)
# =========================================================================

# /test
verb_def(/test, /mutation, /tester, 88).
verb_synonym(/test, "test").
verb_synonym(/test, "unit test").
verb_synonym(/test, "run tests").
verb_synonym(/test, "coverage").
verb_pattern(/test, "(?i)write.*test").
verb_pattern(/test, "(?i)run.*test").
verb_pattern(/test, "(?i)test.*coverage").

# =========================================================================
# RESEARCH (Researcher)
# =========================================================================

# /research
verb_def(/research, /query, /researcher, 75).
verb_synonym(/research, "research").
verb_synonym(/research, "learn").
verb_synonym(/research, "docs").
verb_synonym(/research, "documentation").
verb_pattern(/research, "(?i)research").
verb_pattern(/research, "(?i)learn.*about").
verb_pattern(/research, "(?i)find.*docs").

# =========================================================================
# SETUP & CONFIG
# =========================================================================

# /init
verb_def(/init, /mutation, /researcher, 70).
verb_synonym(/init, "init").
verb_synonym(/init, "setup").
verb_synonym(/init, "bootstrap").
verb_pattern(/init, "(?i)^init").
verb_pattern(/init, "(?i)set.*up").

# /configure
verb_def(/configure, /instruction, /none, 65).
verb_synonym(/configure, "configure").
verb_synonym(/configure, "config").
verb_synonym(/configure, "settings").
verb_pattern(/configure, "(?i)configure").
verb_pattern(/configure, "(?i)change.*setting").

# =========================================================================
# CAMPAIGN
# =========================================================================

# /campaign
verb_def(/campaign, /mutation, /coder, 95).
verb_synonym(/campaign, "campaign").
verb_synonym(/campaign, "epic").
verb_synonym(/campaign, "feature").
verb_pattern(/campaign, "(?i)start.*campaign").
verb_pattern(/campaign, "(?i)implement.*feature").

# =========================================================================
# AUTOPOIESIS (Tool Generation)
# =========================================================================

# /generate_tool
verb_def(/generate_tool, /mutation, /tool_generator, 95).
verb_synonym(/generate_tool, "generate tool").
verb_synonym(/generate_tool, "create tool").
verb_synonym(/generate_tool, "need a tool").
verb_pattern(/generate_tool, "(?i)create.*tool").
verb_pattern(/generate_tool, "(?i)need.*tool").
