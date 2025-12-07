Decl verb_def(Verb.Type<n>, Category.Type<n>, Shard.Type<n>, Priority.Type<int>).
Decl verb_synonym(Verb.Type<n>, Synonym.Type<string>).
Decl verb_pattern(Verb.Type<n>, Regex.Type<string>).

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
verb_pattern(/review, "(?i)review\\s+(this|the|my|our)?\\s*(file|code|changes?|diff|pr|pull\\s*request)?").
verb_pattern(/review, "(?i)can\\s+you\\s+review").
verb_pattern(/review, "(?i)check\\s+(this|the|my)?\\s*(code|file)").

# /security
verb_def(/security, /query, /reviewer, 105).
verb_synonym(/security, "security").
verb_synonym(/security, "security scan").
verb_synonym(/security, "vulnerability").
verb_synonym(/security, "injection").
verb_synonym(/security, "xss").
verb_pattern(/security, "(?i)security\\s+(scan|check|audit|review|analysis)").
verb_pattern(/security, "(?i)check\\s+(for\\s+)?(security|vulnerabilities|vulns)").
verb_pattern(/security, "(?i)find\\s+(security\\s+)?(vulnerabilities|issues|bugs)").

# /analyze
verb_def(/analyze, /query, /reviewer, 95).
verb_synonym(/analyze, "analyze").
verb_synonym(/analyze, "complexity").
verb_synonym(/analyze, "metrics").
verb_synonym(/analyze, "lint").
verb_synonym(/analyze, "code smell").
verb_pattern(/analyze, "(?i)analy[sz]e\\s+(this|the|my)?\\s*(code|file|codebase)?").
verb_pattern(/analyze, "(?i)(code\\s+)?(complexity|metrics|quality)").
verb_pattern(/analyze, "(?i)static\\s+analysis").

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
verb_pattern(/explain, "(?i)explain\\s+(this|the|how|what|why)?").
verb_pattern(/explain, "(?i)tell\\s+me\\s+(about|how|what|why)").
verb_pattern(/explain, "(?i)help\\s+me\\s+understand").

# /explore
verb_def(/explore, /query, /researcher, 75).
verb_synonym(/explore, "explore").
verb_synonym(/explore, "browse").
verb_synonym(/explore, "show structure").
verb_synonym(/explore, "list files").
verb_pattern(/explore, "(?i)show\\s+(me\\s+)?(the\\s+)?(structure|architecture|layout|files?)").
verb_pattern(/explore, "(?i)explore\\s+(the\\s+)?(codebase|project|code)?").

# /search
verb_def(/search, /query, /researcher, 85).
verb_synonym(/search, "search").
verb_synonym(/search, "find").
verb_synonym(/search, "grep").
verb_synonym(/search, "occurrences").
verb_pattern(/search, "(?i)search\\s+(for\\s+)?").
verb_pattern(/search, "(?i)find\\s+(all\\s+)?(occurrences?|references?|usages?|uses?)").
verb_pattern(/search, "(?i)grep\\s+").

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
verb_pattern(/fix, "(?i)fix\\s+(this|the|my|that|a)?\\s*(bug|error|issue|problem)?").
verb_pattern(/fix, "(?i)repair\\s+").
verb_pattern(/fix, "(?i)resolve\\s+(this|the)?\\s*(issue|error|bug)?").

# /refactor
verb_def(/refactor, /mutation, /coder, 88).
verb_synonym(/refactor, "refactor").
verb_synonym(/refactor, "clean up").
verb_synonym(/refactor, "improve").
verb_synonym(/refactor, "optimize").
verb_synonym(/refactor, "simplify").
verb_pattern(/refactor, "(?i)refactor\\s+").
verb_pattern(/refactor, "(?i)clean\\s*up\\s+").
verb_pattern(/refactor, "(?i)improve\\s+(the\\s+)?(code|quality|readability|performance)").

# /create
verb_def(/create, /mutation, /coder, 85).
verb_synonym(/create, "create").
verb_synonym(/create, "new").
verb_synonym(/create, "add").
verb_synonym(/create, "implement").
verb_synonym(/create, "generate").
verb_pattern(/create, "(?i)create\\s+(a\\s+)?(new\\s+)?").
verb_pattern(/create, "(?i)add\\s+(a\\s+)?(new\\s+)?").
verb_pattern(/create, "(?i)implement\\s+").

# /write
verb_def(/write, /mutation, /coder, 70).
verb_synonym(/write, "write").
verb_synonym(/write, "save").
verb_synonym(/write, "export").
verb_pattern(/write, "(?i)write\\s+(to\\s+)?(file|disk)?").
verb_pattern(/write, "(?i)save\\s+(to\\s+)?").

# /delete
verb_def(/delete, /mutation, /coder, 85).
verb_synonym(/delete, "delete").
verb_synonym(/delete, "remove").
verb_synonym(/delete, "drop").
verb_pattern(/delete, "(?i)delete\\s+").
verb_pattern(/delete, "(?i)remove\\s+").

# =========================================================================
# DEBUGGING (Coder)
# =========================================================================

# /debug
verb_def(/debug, /query, /coder, 92).
verb_synonym(/debug, "debug").
verb_synonym(/debug, "troubleshoot").
verb_synonym(/debug, "diagnose").
verb_synonym(/debug, "root cause").
verb_pattern(/debug, "(?i)debug\\s+").
verb_pattern(/debug, "(?i)troubleshoot\\s+").
verb_pattern(/debug, "(?i)why\\s+(is|does|did)\\s+(this|it)\\s+(fail|error|crash|break)").

# =========================================================================
# TESTING (Tester)
# =========================================================================

# /test
verb_def(/test, /mutation, /tester, 88).
verb_synonym(/test, "test").
verb_synonym(/test, "unit test").
verb_synonym(/test, "run tests").
verb_synonym(/test, "coverage").
verb_pattern(/test, "(?i)(write|add|create)\\s+(a\\s+)?(unit\\s+)?tests?").
verb_pattern(/test, "(?i)run\\s+(the\\s+)?tests?").
verb_pattern(/test, "(?i)test\\s+(this|the|coverage)?").

# =========================================================================
# RESEARCH (Researcher)
# =========================================================================

# /research
verb_def(/research, /query, /researcher, 75).
verb_synonym(/research, "research").
verb_synonym(/research, "learn").
verb_synonym(/research, "docs").
verb_synonym(/research, "documentation").
verb_pattern(/research, "(?i)research\\s+").
verb_pattern(/research, "(?i)learn\\s+(about|how)").
verb_pattern(/research, "(?i)(show|find)\\s+(me\\s+)?(the\\s+)?docs").

# =========================================================================
# SETUP & CONFIG
# =========================================================================

# /init
verb_def(/init, /mutation, /researcher, 70).
verb_synonym(/init, "init").
verb_synonym(/init, "setup").
verb_synonym(/init, "bootstrap").
verb_pattern(/init, "(?i)^init(iali[sz]e)?$").
verb_pattern(/init, "(?i)set\\s*up\\s+").

# /configure
verb_def(/configure, /instruction, /none, 65).
verb_synonym(/configure, "configure").
verb_synonym(/configure, "config").
verb_synonym(/configure, "settings").
verb_pattern(/configure, "(?i)configure\\s+").
verb_pattern(/configure, "(?i)change\\s+(the\\s+)?setting").

# =========================================================================
# CAMPAIGN
# =========================================================================

# /campaign
verb_def(/campaign, /mutation, /coder, 95).
verb_synonym(/campaign, "campaign").
verb_synonym(/campaign, "epic").
verb_synonym(/campaign, "feature").
verb_pattern(/campaign, "(?i)start\\s+(a\\s+)?campaign").
verb_pattern(/campaign, "(?i)implement\\s+(a\\s+)?(full|complete|entire)\\s+").

# =========================================================================
# AUTOPOIESIS (Tool Generation)
# =========================================================================

# /generate_tool
verb_def(/generate_tool, /mutation, /tool_generator, 95).
verb_synonym(/generate_tool, "generate tool").
verb_synonym(/generate_tool, "create tool").
verb_synonym(/generate_tool, "need a tool").
verb_pattern(/generate_tool, "(?i)(create|make|generate|build)\\s+(a\\s+)?tool\\s+(for|to|that)").
verb_pattern(/generate_tool, "(?i)i\\s+need\\s+(a\\s+)?tool\\s+(for|to)").
