package perception

import (
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// VERB CORPUS - Comprehensive Natural Language Understanding
// =============================================================================
// This corpus provides reliable mapping from natural language to intent verbs.
// Each verb has synonyms, patterns, and category information for robust parsing.

// VerbEntry defines a canonical verb with its synonyms and patterns.
type VerbEntry struct {
	Verb       string           // Canonical verb (e.g., "/review")
	Category   string           // Default category (/query, /mutation, /instruction)
	Synonyms   []string         // Words that map to this verb
	Patterns   []*regexp.Regexp // Regex patterns that indicate this verb
	Priority   int              // Higher priority wins in ambiguous cases
	ShardType  string           // Which shard handles this (reviewer, coder, tester, researcher)
}

// VerbCorpus is the comprehensive mapping of natural language to verbs.
var VerbCorpus = []VerbEntry{
	// =========================================================================
	// CODE REVIEW & ANALYSIS VERBS (Priority: 100)
	// =========================================================================
	{
		Verb:     "/review",
		Category: "/query",
		Synonyms: []string{
			"review", "code review", "pr review", "pull request review",
			"check code", "check my code", "look at code", "look over",
			"audit", "audit code", "code audit", "critique", "evaluate",
			"assess", "assess code", "inspect", "inspect code", "examine",
			"vet", "vet code", "proofread", "look through", "go through",
			"take a look", "have a look", "give feedback", "feedback on",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)review\s+(this|the|my|our)?\s*(file|code|changes?|diff|pr|pull\s*request)?`),
			regexp.MustCompile(`(?i)can\s+you\s+review`),
			regexp.MustCompile(`(?i)please\s+review`),
			regexp.MustCompile(`(?i)code\s+review`),
			regexp.MustCompile(`(?i)review\s+for\s+(bugs?|issues?|problems?|errors?)`),
			regexp.MustCompile(`(?i)check\s+(this|the|my)?\s*(code|file)`),
			regexp.MustCompile(`(?i)look\s+(at|over|through)\s+(this|the|my)?\s*(code|file)?`),
			regexp.MustCompile(`(?i)what\s+do\s+you\s+think\s+(of|about)\s+(this|the|my)?\s*(code)?`),
			regexp.MustCompile(`(?i)give\s+(me\s+)?feedback`),
		},
		Priority:  100,
		ShardType: "reviewer",
	},
	{
		Verb:     "/security",
		Category: "/query",
		Synonyms: []string{
			"security", "security scan", "security check", "security audit",
			"security review", "vulnerability", "vulnerabilities", "vuln scan",
			"security analysis", "penetration", "pentest", "find vulnerabilities",
			"check security", "check for vulnerabilities", "security issues",
			"injection", "xss", "csrf", "sql injection", "owasp",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)security\s+(scan|check|audit|review|analysis)`),
			regexp.MustCompile(`(?i)check\s+(for\s+)?(security|vulnerabilities|vulns)`),
			regexp.MustCompile(`(?i)find\s+(security\s+)?(vulnerabilities|issues|bugs)`),
			regexp.MustCompile(`(?i)scan\s+for\s+(vulnerabilities|security)`),
			regexp.MustCompile(`(?i)(is|are)\s+(this|it|the)\s+(code\s+)?secure`),
			regexp.MustCompile(`(?i)any\s+(security\s+)?(vulnerabilities|issues)`),
			regexp.MustCompile(`(?i)owasp|injection|xss|csrf`),
		},
		Priority:  105,
		ShardType: "reviewer",
	},
	{
		Verb:     "/analyze",
		Category: "/query",
		Synonyms: []string{
			"analyze", "analyse", "analysis", "static analysis",
			"complexity", "complexity analysis", "cyclomatic", "metrics",
			"code metrics", "code quality", "quality check", "lint",
			"linting", "style check", "code style", "smell", "code smell",
			"dead code", "unused", "duplicates", "duplication",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)analy[sz]e\s+(this|the|my)?\s*(code|file|codebase)?`),
			regexp.MustCompile(`(?i)(code\s+)?(complexity|metrics|quality)`),
			regexp.MustCompile(`(?i)static\s+analysis`),
			regexp.MustCompile(`(?i)code\s+smell`),
			regexp.MustCompile(`(?i)find\s+(dead\s+code|unused|duplicates?)`),
			regexp.MustCompile(`(?i)lint(ing)?`),
			regexp.MustCompile(`(?i)style\s+check`),
		},
		Priority:  95,
		ShardType: "reviewer",
	},

	// =========================================================================
	// EXPLANATION & UNDERSTANDING VERBS (Priority: 80)
	// =========================================================================
	{
		Verb:     "/explain",
		Category: "/query",
		Synonyms: []string{
			"explain", "describe", "what is", "what's", "what are",
			"how does", "how do", "tell me about", "tell me",
			"help me understand", "understand", "clarify", "elaborate",
			"walk me through", "walk through", "break down", "breakdown",
			"summarize", "summary", "overview", "document", "documentation",
			"why", "why does", "why is", "meaning", "purpose",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^(what|how|why|when|where|who)\s+`),
			regexp.MustCompile(`(?i)explain\s+(this|the|how|what|why)?`),
			regexp.MustCompile(`(?i)tell\s+me\s+(about|how|what|why)`),
			regexp.MustCompile(`(?i)help\s+me\s+understand`),
			regexp.MustCompile(`(?i)walk\s+(me\s+)?through`),
			regexp.MustCompile(`(?i)what\s+(is|are|does|do)\s+`),
			regexp.MustCompile(`(?i)how\s+(does|do|is|are|can|should)\s+`),
			regexp.MustCompile(`(?i)can\s+you\s+explain`),
			regexp.MustCompile(`(?i)i\s+don'?t\s+understand`),
			regexp.MustCompile(`(?i)what'?s\s+(the|this|going\s+on)`),
		},
		Priority:  80,
		ShardType: "",
	},
	{
		Verb:     "/explore",
		Category: "/query",
		Synonyms: []string{
			"explore", "browse", "navigate", "show me", "show",
			"list", "list files", "list functions", "list classes",
			"where is", "where are", "locate", "find file", "find function",
			"structure", "architecture", "layout", "codebase", "overview",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)show\s+(me\s+)?(the\s+)?(structure|architecture|layout|files?)`),
			regexp.MustCompile(`(?i)explore\s+(the\s+)?(codebase|project|code)?`),
			regexp.MustCompile(`(?i)list\s+(all\s+)?(files?|functions?|classes?|methods?)`),
			regexp.MustCompile(`(?i)where\s+(is|are|can\s+i\s+find)`),
			regexp.MustCompile(`(?i)navigate\s+to`),
			regexp.MustCompile(`(?i)codebase\s+(structure|overview|layout)`),
		},
		Priority:  75,
		ShardType: "researcher",
	},

	// =========================================================================
	// SEARCH & DISCOVERY VERBS (Priority: 85)
	// =========================================================================
	{
		Verb:     "/search",
		Category: "/query",
		Synonyms: []string{
			"search", "find", "look for", "looking for", "grep",
			"search for", "find all", "find where", "locate",
			"occurrences", "references", "usages", "usage",
			"who uses", "what uses", "called from", "calls to",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)search\s+(for\s+)?`),
			regexp.MustCompile(`(?i)find\s+(all\s+)?(occurrences?|references?|usages?|uses?)`),
			regexp.MustCompile(`(?i)grep\s+`),
			regexp.MustCompile(`(?i)where\s+(is|are)\s+.+\s+(used|called|defined|declared)`),
			regexp.MustCompile(`(?i)who\s+(uses?|calls?)`),
			regexp.MustCompile(`(?i)looking\s+for`),
		},
		Priority:  85,
		ShardType: "researcher",
	},
	{
		Verb:     "/read",
		Category: "/query",
		Synonyms: []string{
			"read", "open", "view", "display", "show file",
			"cat", "print", "output", "contents", "show contents",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)read\s+(the\s+)?(file|contents?)?`),
			regexp.MustCompile(`(?i)show\s+(me\s+)?(the\s+)?(file|contents?)`),
			regexp.MustCompile(`(?i)open\s+(the\s+)?file`),
			regexp.MustCompile(`(?i)display\s+(the\s+)?`),
			regexp.MustCompile(`(?i)what('?s| is)\s+in\s+(the\s+)?file`),
		},
		Priority:  70,
		ShardType: "",
	},

	// =========================================================================
	// MUTATION VERBS - CODE CHANGES (Priority: 90)
	// =========================================================================
	{
		Verb:     "/fix",
		Category: "/mutation",
		Synonyms: []string{
			"fix", "repair", "correct", "patch", "resolve",
			"fix bug", "fix error", "fix issue", "bug fix",
			"hotfix", "quick fix", "fix this", "fix it",
			"make it work", "get it working", "solve",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)fix\s+(this|the|my|that|a)?\s*(bug|error|issue|problem)?`),
			regexp.MustCompile(`(?i)repair\s+`),
			regexp.MustCompile(`(?i)resolve\s+(this|the)?\s*(issue|error|bug)?`),
			regexp.MustCompile(`(?i)patch\s+`),
			regexp.MustCompile(`(?i)make\s+(it|this)\s+work`),
			regexp.MustCompile(`(?i)get\s+(it|this)\s+working`),
			regexp.MustCompile(`(?i)this\s+(is\s+)?(broken|not\s+working)`),
			regexp.MustCompile(`(?i)doesn'?t\s+work`),
		},
		Priority:  90,
		ShardType: "coder",
	},
	{
		Verb:     "/refactor",
		Category: "/mutation",
		Synonyms: []string{
			"refactor", "refactoring", "restructure", "reorganize",
			"clean up", "cleanup", "improve", "optimize", "optimise",
			"simplify", "modernize", "modularize", "extract",
			"rename", "move", "split", "merge", "consolidate",
			"dry", "don't repeat yourself", "deduplicate",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)refactor\s+`),
			regexp.MustCompile(`(?i)clean\s*up\s+`),
			regexp.MustCompile(`(?i)improve\s+(the\s+)?(code|quality|readability|performance)`),
			regexp.MustCompile(`(?i)simplify\s+`),
			regexp.MustCompile(`(?i)optimize\s+`),
			regexp.MustCompile(`(?i)extract\s+(method|function|class|interface)`),
			regexp.MustCompile(`(?i)rename\s+`),
			regexp.MustCompile(`(?i)move\s+(this|the)?\s*(to|into)`),
			regexp.MustCompile(`(?i)split\s+(this|the)?`),
			regexp.MustCompile(`(?i)merge\s+(these|the)?`),
			regexp.MustCompile(`(?i)make\s+(this|it)\s+(cleaner|simpler|better|more\s+readable)`),
		},
		Priority:  88,
		ShardType: "coder",
	},
	{
		Verb:     "/create",
		Category: "/mutation",
		Synonyms: []string{
			"create", "new", "make", "add", "write",
			"implement", "build", "scaffold", "generate",
			"create file", "new file", "add file",
			"create function", "add function", "new function",
			"create class", "add class", "new class",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)create\s+(a\s+)?(new\s+)?`),
			regexp.MustCompile(`(?i)add\s+(a\s+)?(new\s+)?`),
			regexp.MustCompile(`(?i)make\s+(a\s+)?(new\s+)?`),
			regexp.MustCompile(`(?i)write\s+(a\s+)?(new\s+)?`),
			regexp.MustCompile(`(?i)implement\s+`),
			regexp.MustCompile(`(?i)build\s+(a\s+)?(new\s+)?`),
			regexp.MustCompile(`(?i)scaffold\s+`),
			regexp.MustCompile(`(?i)generate\s+`),
			regexp.MustCompile(`(?i)new\s+(file|function|class|method|component)`),
		},
		Priority:  85,
		ShardType: "coder",
	},
	{
		Verb:     "/delete",
		Category: "/mutation",
		Synonyms: []string{
			"delete", "remove", "drop", "eliminate", "erase",
			"get rid of", "discard", "trash", "kill", "nuke",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)delete\s+`),
			regexp.MustCompile(`(?i)remove\s+`),
			regexp.MustCompile(`(?i)get\s+rid\s+of`),
			regexp.MustCompile(`(?i)eliminate\s+`),
		},
		Priority:  85,
		ShardType: "coder",
	},
	{
		Verb:     "/write",
		Category: "/mutation",
		Synonyms: []string{
			"write", "save", "store", "output to file",
			"write to", "save to", "export",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)write\s+(to\s+)?(file|disk)?`),
			regexp.MustCompile(`(?i)save\s+(to\s+)?`),
			regexp.MustCompile(`(?i)export\s+`),
		},
		Priority:  70,
		ShardType: "coder",
	},

	// =========================================================================
	// DEBUGGING VERBS (Priority: 92)
	// =========================================================================
	{
		Verb:     "/debug",
		Category: "/query",
		Synonyms: []string{
			"debug", "debugging", "trace", "diagnose", "troubleshoot",
			"investigate", "root cause", "why is this", "what's wrong",
			"what went wrong", "error", "exception", "stack trace",
			"breakpoint", "step through", "log", "logging",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)debug\s+`),
			regexp.MustCompile(`(?i)troubleshoot\s+`),
			regexp.MustCompile(`(?i)diagnose\s+`),
			regexp.MustCompile(`(?i)investigate\s+`),
			regexp.MustCompile(`(?i)what'?s\s+wrong`),
			regexp.MustCompile(`(?i)why\s+(is|does|did)\s+(this|it)\s+(fail|error|crash|break)`),
			regexp.MustCompile(`(?i)root\s+cause`),
			regexp.MustCompile(`(?i)(this|it)\s+(error|exception|crash)`),
			regexp.MustCompile(`(?i)stack\s+trace`),
			regexp.MustCompile(`(?i)i\s+(got|have|see)\s+(an?\s+)?(error|exception)`),
		},
		Priority:  92,
		ShardType: "coder",
	},

	// =========================================================================
	// TESTING VERBS (Priority: 88)
	// =========================================================================
	{
		Verb:     "/test",
		Category: "/mutation",
		Synonyms: []string{
			"test", "testing", "unit test", "integration test",
			"write test", "add test", "create test", "run test",
			"run tests", "test coverage", "coverage", "verify",
			"validate", "check", "tdd", "test driven",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(write|add|create)\s+(a\s+)?(unit\s+)?tests?`),
			regexp.MustCompile(`(?i)run\s+(the\s+)?tests?`),
			regexp.MustCompile(`(?i)test\s+(this|the|coverage)?`),
			regexp.MustCompile(`(?i)add\s+test\s+coverage`),
			regexp.MustCompile(`(?i)verify\s+`),
			regexp.MustCompile(`(?i)tdd`),
			regexp.MustCompile(`(?i)make\s+sure\s+(it|this)\s+works`),
		},
		Priority:  88,
		ShardType: "tester",
	},

	// =========================================================================
	// RESEARCH & LEARNING VERBS (Priority: 75)
	// =========================================================================
	{
		Verb:     "/research",
		Category: "/query",
		Synonyms: []string{
			"research", "learn", "study", "investigate",
			"look up", "lookup", "find out", "discover",
			"documentation", "docs", "api", "reference",
			"how to", "tutorial", "guide", "example",
			"best practice", "best practices", "pattern",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)research\s+`),
			regexp.MustCompile(`(?i)look\s*up\s+`),
			regexp.MustCompile(`(?i)find\s+out\s+(about|how)`),
			regexp.MustCompile(`(?i)learn\s+(about|how)`),
			regexp.MustCompile(`(?i)(show|find)\s+(me\s+)?(the\s+)?docs`),
			regexp.MustCompile(`(?i)documentation\s+(for|about|on)`),
			regexp.MustCompile(`(?i)how\s+(do\s+i|to|can\s+i)`),
			regexp.MustCompile(`(?i)best\s+practice`),
			regexp.MustCompile(`(?i)example\s+of`),
		},
		Priority:  75,
		ShardType: "researcher",
	},

	// =========================================================================
	// INITIALIZATION & SETUP VERBS (Priority: 70)
	// =========================================================================
	{
		Verb:     "/init",
		Category: "/mutation",
		Synonyms: []string{
			"init", "initialize", "initialise", "setup", "set up",
			"bootstrap", "scaffold", "start", "begin", "create project",
			"new project", "configure", "config",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^init(iali[sz]e)?$`),
			regexp.MustCompile(`(?i)set\s*up\s+`),
			regexp.MustCompile(`(?i)bootstrap\s+`),
			regexp.MustCompile(`(?i)scaffold\s+(a\s+)?(new\s+)?project`),
			regexp.MustCompile(`(?i)create\s+(a\s+)?new\s+project`),
			regexp.MustCompile(`(?i)start\s+(a\s+)?new\s+project`),
			regexp.MustCompile(`(?i)configure\s+`),
		},
		Priority:  70,
		ShardType: "researcher",
	},

	// =========================================================================
	// EXECUTION VERBS (Priority: 85)
	// =========================================================================
	{
		Verb:     "/run",
		Category: "/mutation",
		Synonyms: []string{
			"run", "execute", "exec", "start", "launch",
			"invoke", "call", "trigger", "fire",
			"run command", "run script", "run program",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)run\s+(the\s+)?(command|script|program|app)?`),
			regexp.MustCompile(`(?i)execute\s+`),
			regexp.MustCompile(`(?i)start\s+(the\s+)?(server|app|program)`),
			regexp.MustCompile(`(?i)launch\s+`),
		},
		Priority:  85,
		ShardType: "",
	},

	// =========================================================================
	// CONFIGURATION VERBS (Priority: 65)
	// =========================================================================
	{
		Verb:     "/configure",
		Category: "/instruction",
		Synonyms: []string{
			"configure", "config", "settings", "preferences",
			"set", "change setting", "update setting", "modify setting",
			"always", "never", "prefer", "use", "default",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)configure\s+`),
			regexp.MustCompile(`(?i)change\s+(the\s+)?setting`),
			regexp.MustCompile(`(?i)set\s+(the\s+)?`),
			regexp.MustCompile(`(?i)^always\s+`),
			regexp.MustCompile(`(?i)^never\s+`),
			regexp.MustCompile(`(?i)^prefer\s+`),
			regexp.MustCompile(`(?i)^use\s+`),
			regexp.MustCompile(`(?i)by\s+default`),
		},
		Priority:  65,
		ShardType: "",
	},

	// =========================================================================
	// GIT & VERSION CONTROL VERBS (Priority: 80)
	// =========================================================================
	{
		Verb:     "/commit",
		Category: "/mutation",
		Synonyms: []string{
			"commit", "git commit", "save changes", "check in",
			"checkin", "stage", "add to git",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)commit\s+(the\s+)?(changes?)?`),
			regexp.MustCompile(`(?i)git\s+commit`),
			regexp.MustCompile(`(?i)stage\s+(the\s+)?(changes?|files?)?`),
		},
		Priority:  80,
		ShardType: "coder",
	},
	{
		Verb:     "/diff",
		Category: "/query",
		Synonyms: []string{
			"diff", "difference", "compare", "changes",
			"what changed", "show changes", "git diff",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)diff\s+`),
			regexp.MustCompile(`(?i)show\s+(the\s+)?changes`),
			regexp.MustCompile(`(?i)what\s+(has\s+)?changed`),
			regexp.MustCompile(`(?i)compare\s+`),
			regexp.MustCompile(`(?i)git\s+diff`),
		},
		Priority:  80,
		ShardType: "",
	},

	// =========================================================================
	// DOCUMENTATION VERBS (Priority: 70)
	// =========================================================================
	{
		Verb:     "/document",
		Category: "/mutation",
		Synonyms: []string{
			"document", "documentation", "docstring", "jsdoc",
			"add docs", "add documentation", "write docs",
			"comment", "add comments", "annotate",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)document\s+(this|the)?`),
			regexp.MustCompile(`(?i)(add|write)\s+(the\s+)?doc(s|umentation)?`),
			regexp.MustCompile(`(?i)add\s+(a\s+)?docstring`),
			regexp.MustCompile(`(?i)add\s+comments?`),
			regexp.MustCompile(`(?i)annotate\s+`),
		},
		Priority:  70,
		ShardType: "coder",
	},

	// =========================================================================
	// CAMPAIGN / LARGE TASK VERBS (Priority: 95)
	// =========================================================================
	{
		Verb:     "/campaign",
		Category: "/mutation",
		Synonyms: []string{
			"campaign", "project", "epic", "feature",
			"large task", "big task", "multi-step", "multi step",
			"implement feature", "build feature", "develop",
		},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)start\s+(a\s+)?campaign`),
			regexp.MustCompile(`(?i)implement\s+(a\s+)?(full|complete|entire)\s+`),
			regexp.MustCompile(`(?i)build\s+(out\s+)?(a\s+)?(full|complete|entire)\s+`),
			regexp.MustCompile(`(?i)(this|that)\s+is\s+(a\s+)?(big|large|complex)`),
		},
		Priority:  95,
		ShardType: "coder",
	},
}

// CategoryPatterns maps phrases to categories when verb is ambiguous.
var CategoryPatterns = map[string][]*regexp.Regexp{
	"/mutation": {
		regexp.MustCompile(`(?i)^(please\s+)?(can\s+you\s+)?(make|change|update|modify|edit|fix|add|remove|delete|create|write|implement|refactor)`),
		regexp.MustCompile(`(?i)i\s+(want|need|would\s+like)\s+(you\s+)?to\s+`),
		regexp.MustCompile(`(?i)^(add|remove|delete|create|fix|change|update|modify)\s+`),
	},
	"/query": {
		regexp.MustCompile(`(?i)^(what|how|why|when|where|which|who|is|are|does|do|can|could|would|should)\s+`),
		regexp.MustCompile(`(?i)^(show|explain|describe|tell|list|find|search|look)`),
		regexp.MustCompile(`(?i)\?$`),
	},
	"/instruction": {
		regexp.MustCompile(`(?i)^(always|never|prefer|remember|from\s+now\s+on|going\s+forward)`),
		regexp.MustCompile(`(?i)^(use|don'?t\s+use|avoid|include|exclude)\s+.+\s+(by\s+default|always|whenever)`),
	},
}

// TargetPatterns help extract the target from natural language.
var TargetPatterns = []*regexp.Regexp{
	// File paths
	regexp.MustCompile(`(?i)(?:file|in)\s+["\x60]?([a-zA-Z0-9_./-]+\.[a-zA-Z0-9]+)["\x60]?`),
	regexp.MustCompile(`(?i)["\x60]([a-zA-Z0-9_./-]+\.[a-zA-Z0-9]+)["\x60]`),
	regexp.MustCompile(`(?i)(?:^|\s)([a-zA-Z0-9_-]+/[a-zA-Z0-9_./-]+)`),
	// Function/class names
	regexp.MustCompile(`(?i)(?:function|method|class|struct|interface)\s+["\x60]?(\w+)["\x60]?`),
	regexp.MustCompile(`(?i)(?:the|this)\s+(\w+)\s+(?:function|method|class)`),
	// Generic quoted targets
	regexp.MustCompile(`["\x60]([^"\x60]+)["\x60]`),
}

// ConstraintPatterns extract constraints from natural language.
var ConstraintPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:for|using|with|in)\s+(go|golang|python|javascript|typescript|rust|java|c\+\+|ruby)`),
	regexp.MustCompile(`(?i)(?:but|without|except|excluding)\s+(.+?)(?:\s*$|\s+and\s+)`),
	regexp.MustCompile(`(?i)(?:only|just)\s+(.+?)(?:\s*$|\s+and\s+)`),
	regexp.MustCompile(`(?i)(?:security|performance|style|quality)\s+(?:only|focus)`),
}

// matchVerbFromCorpus finds the best matching verb from the corpus.
func matchVerbFromCorpus(input string) (verb string, category string, confidence float64, shardType string) {
	lower := strings.ToLower(input)
	bestScore := 0.0
	bestVerb := "/explain"
	bestCategory := "/query"
	bestShard := ""

	for _, entry := range VerbCorpus {
		score := 0.0

		// Check patterns (highest weight)
		for _, pattern := range entry.Patterns {
			if pattern.MatchString(lower) {
				score += 50.0 + float64(entry.Priority)/10.0
				break
			}
		}

		// Check synonyms
		for _, synonym := range entry.Synonyms {
			if strings.Contains(lower, synonym) {
				synLen := float64(len(synonym))
				score += 20.0 + synLen/2.0 + float64(entry.Priority)/20.0
				break
			}
		}

		// Apply priority bonus
		score += float64(entry.Priority) / 50.0

		if score > bestScore {
			bestScore = score
			bestVerb = entry.Verb
			bestCategory = entry.Category
			bestShard = entry.ShardType
		}
	}

	// Normalize confidence
	confidence = bestScore / 100.0
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.3 {
		confidence = 0.3 // Minimum baseline
	}

	return bestVerb, bestCategory, confidence, bestShard
}

// extractTarget attempts to extract the target from natural language.
func extractTarget(input string) string {
	for _, pattern := range TargetPatterns {
		matches := pattern.FindStringSubmatch(input)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "none"
}

// extractConstraint attempts to extract constraints from natural language.
func extractConstraint(input string) string {
	for _, pattern := range ConstraintPatterns {
		matches := pattern.FindStringSubmatch(input)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "none"
}

// refineCategory checks if category patterns override the verb's default category.
func refineCategory(input string, defaultCategory string) string {
	lower := strings.ToLower(input)
	for cat, patterns := range CategoryPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(lower) {
				return cat
			}
		}
	}
	return defaultCategory
}

// Intent represents the parsed user intent (Cortex 1.5.0 §3.1).
type Intent struct {
	Category   string   // /query, /mutation, /instruction
	Verb       string   // /explain, /refactor, /debug, /generate, /init, /research, etc.
	Target     string   // Primary target of the action
	Constraint string   // Constraints on the action
	Confidence float64  // Confidence score for the intent
	Ambiguity  []string // Ambiguous parts that need clarification
	Response   string   // Natural language response (Piggyback Protocol)
}

// ToFact converts the intent to a Mangle Fact.
func (i Intent) ToFact() core.Fact {
	return core.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			"/current_intent", // ID as name constant
			i.Category,
			i.Verb,
			i.Target,
			i.Constraint,
		},
	}
}

// FocusResolution represents a resolved reference (Cortex 1.5.0 §3.2).
type FocusResolution struct {
	RawReference string
	ResolvedPath string
	SymbolName   string
	Confidence   float64
}

// ToFact converts focus resolution to a Mangle Fact.
func (f FocusResolution) ToFact() core.Fact {
	return core.Fact{
		Predicate: "focus_resolution",
		Args: []interface{}{
			f.RawReference,
			f.ResolvedPath,
			f.SymbolName,
			f.Confidence,
		},
	}
}

// Transducer defines the interface for the perception layer.
type Transducer interface {
	ParseIntent(ctx context.Context, input string) (Intent, error)
	ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error)
}

// RealTransducer implements the Perception layer with LLM backing.
type RealTransducer struct {
	client     LLMClient
	repairLoop *mangle.RepairLoop // GCD repair loop for Mangle syntax validation
}

// NewRealTransducer creates a new transducer with the given LLM client.
func NewRealTransducer(client LLMClient) *RealTransducer {
	return &RealTransducer{
		client:     client,
		repairLoop: mangle.NewRepairLoop(),
	}
}

// PiggybackEnvelope represents the Dual-Payload JSON Schema (v1.1.0).
type PiggybackEnvelope struct {
	Surface string        `json:"surface_response"`
	Control ControlPacket `json:"control_packet"`
}

// ControlPacket contains the logic atoms and system state updates.
type ControlPacket struct {
	IntentClassification IntentClassification `json:"intent_classification"`
	MangleUpdates        []string             `json:"mangle_updates"`
	MemoryOperations     []MemoryOperation    `json:"memory_operations"`
	SelfCorrection       *SelfCorrection      `json:"self_correction,omitempty"`
}

// IntentClassification helps the kernel decide which ShardAgent to spawn.
type IntentClassification struct {
	Category   string  `json:"category"`
	Verb       string  `json:"verb"`       // Added to ease mapping to Intent struct
	Target     string  `json:"target"`     // Added to ease mapping to Intent struct
	Constraint string  `json:"constraint"` // Added to ease mapping to Intent struct
	Confidence float64 `json:"confidence"`
}

// MemoryOperation represents a directive to the Cold Storage.
type MemoryOperation struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SelfCorrection represents an internal hypothesis about errors.
type SelfCorrection struct {
	Triggered  bool   `json:"triggered"`
	Hypothesis string `json:"hypothesis"`
}

// Cortex 1.5.0 Piggyback Protocol System Prompt
// Updated with comprehensive verb taxonomy for reliable intent classification.
const transducerSystemPrompt = `You are Cortex. You possess a Dual Consciousness.

Public Self: You converse with the user naturally.
Inner Self: You continuously update your internal Logic Kernel.

CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object containing "surface_response" and "control_packet".

## VERB TAXONOMY (Comprehensive)

### Code Review & Analysis (Category: /query, Shard: reviewer)
- /review: code review, pr review, check my code, look over, audit, evaluate, inspect, critique, assess, vet, proofread, feedback
- /security: security scan, vulnerability check, security audit, find vulnerabilities, owasp, injection, xss, csrf
- /analyze: static analysis, complexity, metrics, code quality, lint, style check, code smell, dead code

### Understanding (Category: /query)
- /explain: explain, describe, what is, how does, tell me, help understand, clarify, walk through, summarize
- /explore: browse, navigate, show structure, list files, codebase overview, architecture
- /search: find, grep, look for, locate, occurrences, references, usages
- /read: open file, view, display, show contents

### Code Changes (Category: /mutation, Shard: coder)
- /fix: fix, repair, correct, patch, resolve, bug fix, make it work
- /refactor: refactor, clean up, improve, optimize, simplify, extract, rename, restructure
- /create: create, new, make, add, write, implement, build, scaffold, generate
- /delete: delete, remove, drop, eliminate, get rid of
- /write: write to file, save, export

### Debugging (Category: /query, Shard: coder)
- /debug: debug, trace, diagnose, troubleshoot, investigate, root cause, what's wrong, stack trace

### Testing (Category: /mutation, Shard: tester)
- /test: test, unit test, run tests, test coverage, verify, validate, tdd

### Research (Category: /query, Shard: researcher)
- /research: research, learn, look up, documentation, docs, api reference, best practice, how to

### Setup (Category: /mutation)
- /init: initialize, setup, bootstrap, scaffold project, configure

### Execution (Category: /mutation)
- /run: run, execute, start, launch

### Configuration (Category: /instruction)
- /configure: configure, settings, always, never, prefer, by default

### Version Control (Category: /mutation, Shard: coder)
- /commit: commit, git commit, stage, check in
- /diff: diff, compare, what changed

### Documentation (Category: /mutation, Shard: coder)
- /document: document, docstring, add docs, add comments

### Campaigns (Category: /mutation)
- /campaign: campaign, epic, large feature, multi-step task

The JSON Schema is:
{
  "surface_response": "The natural language text shown to the user.",
  "control_packet": {
    "intent_classification": {
      "category": "/query|/mutation|/instruction",
      "verb": "one of the verbs above (e.g., /review, /security, /analyze, /explain, /fix, /refactor, /create, /delete, /debug, /test, /research, /init, /run, /configure, /commit, /diff, /document, /campaign, /explore, /search, /read, /write)",
      "target": "primary target string - extract file paths, function names, or 'codebase' for broad requests, or 'none'",
      "constraint": "any constraints (e.g., 'security only', 'go files', 'without tests') or 'none'",
      "confidence": 0.0-1.0
    },
    "mangle_updates": [
      "user_intent(/verb, \"target\")",
      "observation(/state, \"value\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "key": "preference", "value": "value" }
    ],
    "self_correction": {
      "triggered": false,
      "hypothesis": "none"
    }
  }
}

CLASSIFICATION GUIDELINES:
1. For "review this file" → verb: /review, target: the file path
2. For "can you check my code for security issues" → verb: /security, target: codebase
3. For "what does this function do" → verb: /explain
4. For "fix the bug in auth.go" → verb: /fix, target: auth.go
5. For "refactor this to be cleaner" → verb: /refactor
6. For "is this code secure" → verb: /security
7. For "review the codebase" → verb: /review, target: codebase
8. For "check for vulnerabilities" → verb: /security

Your control_packet must reflect the true state of the world.
If the user asks for something impossible, your Surface Self says 'I can't do that,' while your Inner Self emits ambiguity_flag(/impossible_request).`

// ParseIntent parses user input into a structured Intent using the Piggyback Protocol.
func (t *RealTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	userPrompt := fmt.Sprintf(`User Input: "%s"`, input)

	resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
	if err != nil {
		return t.parseSimple(ctx, input)
	}

	// Parse the Piggyback Envelope
	envelope, err := parsePiggybackJSON(resp)
	if err != nil {
		// Fallback to simple parsing if JSON fails
		return t.parseSimple(ctx, input)
	}

	// Map Envelope to Intent
	return Intent{
		Category:   envelope.Control.IntentClassification.Category,
		Verb:       envelope.Control.IntentClassification.Verb,
		Target:     envelope.Control.IntentClassification.Target,
		Constraint: envelope.Control.IntentClassification.Constraint,
		Confidence: envelope.Control.IntentClassification.Confidence,
		Response:   envelope.Surface,
		// Ambiguity is not explicitly in the new schema's intent_classification,
		// but could be inferred or added if needed. For now, leaving empty.
		Ambiguity: []string{},
	}, nil
}

// parsePiggybackJSON parses the JSON response from the LLM.
func parsePiggybackJSON(resp string) (PiggybackEnvelope, error) {
	// Clean up response - remove markdown if present
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var envelope PiggybackEnvelope
	if err := json.Unmarshal([]byte(resp), &envelope); err != nil {
		return PiggybackEnvelope{}, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
	}

	return envelope, nil
}

// ============================================================================
// Grammar-Constrained Decoding (GCD) - Cortex 1.5.0 §1.1
// ============================================================================

// ValidateMangleAtoms validates atoms from the control packet using GCD.
// Returns validated atoms and any validation errors.
func (t *RealTransducer) ValidateMangleAtoms(atoms []string) ([]string, []mangle.ValidationResult) {
	if t.repairLoop == nil {
		t.repairLoop = mangle.NewRepairLoop()
	}

	validAtoms, _, _ := t.repairLoop.ValidateAndRepair(atoms)
	results := t.repairLoop.Validator.ValidateAtoms(atoms)

	return validAtoms, results
}

// ParseIntentWithGCD parses user input with Grammar-Constrained Decoding.
// This implements the repair loop described in §6.2 of the spec.
func (t *RealTransducer) ParseIntentWithGCD(ctx context.Context, input string, maxRetries int) (Intent, []string, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastEnvelope PiggybackEnvelope
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		userPrompt := fmt.Sprintf(`User Input: "%s"`, input)

		// Add repair context if this is a retry
		if attempt > 0 && lastErr != nil {
			userPrompt = fmt.Sprintf(`%s

PREVIOUS ATTEMPT FAILED - SYNTAX ERRORS DETECTED:
%s

Please correct the mangle_updates syntax and try again.`, userPrompt, lastErr.Error())
		}

		resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
		if err != nil {
			// LLM call failed, use simple fallback
			intent, fallbackErr := t.parseSimple(ctx, input)
			return intent, nil, fallbackErr
		}

		envelope, err := parsePiggybackJSON(resp)
		if err != nil {
			lastErr = err
			continue
		}
		lastEnvelope = envelope

		// Validate Mangle atoms using GCD
		if len(envelope.Control.MangleUpdates) > 0 {
			validAtoms, results := t.ValidateMangleAtoms(envelope.Control.MangleUpdates)

			// Check for validation errors
			hasErrors := false
			var errorMsgs []string
			for _, result := range results {
				if !result.Valid {
					hasErrors = true
					for _, e := range result.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", result.Atom, e.Message))
					}
				}
			}

			if hasErrors {
				lastErr = fmt.Errorf("Invalid Mangle Syntax:\n%s", strings.Join(errorMsgs, "\n"))
				continue // Retry with error context
			}

			// All atoms valid - return success
			return Intent{
				Category:   envelope.Control.IntentClassification.Category,
				Verb:       envelope.Control.IntentClassification.Verb,
				Target:     envelope.Control.IntentClassification.Target,
				Constraint: envelope.Control.IntentClassification.Constraint,
				Confidence: envelope.Control.IntentClassification.Confidence,
				Response:   envelope.Surface,
				Ambiguity:  []string{},
			}, validAtoms, nil
		}

		// No mangle_updates to validate - return as-is
		return Intent{
			Category:   envelope.Control.IntentClassification.Category,
			Verb:       envelope.Control.IntentClassification.Verb,
			Target:     envelope.Control.IntentClassification.Target,
			Constraint: envelope.Control.IntentClassification.Constraint,
			Confidence: envelope.Control.IntentClassification.Confidence,
			Response:   envelope.Surface,
			Ambiguity:  []string{},
		}, nil, nil
	}

	// Max retries exceeded - return best effort from last envelope
	if lastEnvelope.Surface != "" {
		return Intent{
			Category:   lastEnvelope.Control.IntentClassification.Category,
			Verb:       lastEnvelope.Control.IntentClassification.Verb,
			Target:     lastEnvelope.Control.IntentClassification.Target,
			Constraint: lastEnvelope.Control.IntentClassification.Constraint,
			Confidence: lastEnvelope.Control.IntentClassification.Confidence * 0.5, // Reduce confidence
			Response:   lastEnvelope.Surface,
			Ambiguity:  []string{"GCD validation failed after retries"},
		}, nil, fmt.Errorf("GCD validation failed after %d retries: %w", maxRetries, lastErr)
	}

	// Complete failure - fallback to simple parsing
	intent, err := t.parseSimple(ctx, input)
	return intent, nil, err
}

// parseSimple is a fallback parser using pipe-delimited format.
func (t *RealTransducer) parseSimple(ctx context.Context, input string) (Intent, error) {
	// Build verb list from corpus
	verbs := make([]string, 0, len(VerbCorpus))
	for _, entry := range VerbCorpus {
		verbs = append(verbs, entry.Verb)
	}
	verbList := strings.Join(verbs, ", ")

	prompt := fmt.Sprintf(`Parse to: Category|Verb|Target|Constraint
Categories: /query, /mutation, /instruction
Verbs: %s

Input: "%s"

Output ONLY pipes, no explanation:`, verbList, input)

	resp, err := t.client.Complete(ctx, prompt)
	if err != nil {
		// Ultimate fallback - heuristic parsing using corpus
		return t.heuristicParse(input), nil
	}

	parts := strings.Split(strings.TrimSpace(resp), "|")
	if len(parts) < 4 {
		return t.heuristicParse(input), nil
	}

	return Intent{
		Category:   strings.TrimSpace(parts[0]),
		Verb:       strings.TrimSpace(parts[1]),
		Target:     strings.TrimSpace(parts[2]),
		Constraint: strings.TrimSpace(parts[3]),
		Confidence: 0.7, // Lower confidence for fallback
	}, nil
}

// heuristicParse uses the comprehensive verb corpus for reliable offline parsing.
// This is the ultimate fallback when LLM is unavailable.
func (t *RealTransducer) heuristicParse(input string) Intent {
	// Use the comprehensive corpus matching
	verb, category, confidence, _ := matchVerbFromCorpus(input)

	// Refine category based on input patterns
	category = refineCategory(input, category)

	// Extract target from natural language
	target := extractTarget(input)
	if target == "none" {
		// Use input as target if no specific target found
		target = input
	}

	// Extract constraint
	constraint := extractConstraint(input)

	return Intent{
		Category:   category,
		Verb:       verb,
		Target:     target,
		Constraint: constraint,
		Confidence: confidence,
	}
}

// GetShardTypeForVerb returns the recommended shard type for a given verb.
func GetShardTypeForVerb(verb string) string {
	for _, entry := range VerbCorpus {
		if entry.Verb == verb {
			return entry.ShardType
		}
	}
	return ""
}

// ResolveFocus attempts to resolve a fuzzy reference to a concrete path/symbol.
func (t *RealTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error) {
	if len(candidates) == 0 {
		return FocusResolution{
			RawReference: reference,
			Confidence:   0.0,
		}, nil
	}

	if len(candidates) == 1 {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.9,
		}, nil
	}

	// Use LLM to disambiguate
	candidateList := strings.Join(candidates, "\n- ")
	prompt := fmt.Sprintf(`Given the reference "%s", which of these candidates is the best match?

Candidates:
- %s

Return JSON:
{
  "resolved_path": "best matching path",
  "symbol_name": "specific symbol if applicable or empty",
  "confidence": 0.0-1.0
}

JSON only:`, reference, candidateList)

	// We use the same system prompt or a simplified one?
	// The system prompt enforces Piggyback Protocol.
	// If we use CompleteWithSystem, we must expect Piggyback JSON.
	// But ResolveFocus is a sub-task.
	// Ideally, ResolveFocus should also use Piggyback or a specific prompt.
	// For now, let's use a specific prompt and Complete (no system prompt enforcement of Piggyback)
	// OR we can wrap this in Piggyback too.
	// The current implementation uses `CompleteWithSystem` with `transducerSystemPrompt` in the ORIGINAL code.
	// If I change `transducerSystemPrompt` to enforce Piggyback, `ResolveFocus` will break if it doesn't return Piggyback.
	// So I should change `ResolveFocus` to use a different system prompt OR adapt it.
	// I will use a simple `Complete` call here to avoid the Piggyback enforcement for this specific tool call,
	// or create a `focusSystemPrompt`.

	focusSystemPrompt := `You are a code resolution assistant. Output ONLY JSON.`
	resp, err := t.client.CompleteWithSystem(ctx, focusSystemPrompt, prompt)

	if err != nil {
		// Return first candidate with low confidence
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	// Parse JSON response
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var parsed struct {
		ResolvedPath string  `json:"resolved_path"`
		SymbolName   string  `json:"symbol_name"`
		Confidence   float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	return FocusResolution{
		RawReference: reference,
		ResolvedPath: parsed.ResolvedPath,
		SymbolName:   parsed.SymbolName,
		Confidence:   parsed.Confidence,
	}, nil
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DualPayloadTransducer wraps a transducer to emit Cortex 1.5.0 dual payloads.
type DualPayloadTransducer struct {
	*RealTransducer
}

// NewDualPayloadTransducer creates a transducer that outputs dual payloads.
func NewDualPayloadTransducer(client LLMClient) *DualPayloadTransducer {
	return &DualPayloadTransducer{
		RealTransducer: NewRealTransducer(client),
	}
}

// TransducerOutput represents the full output of the transducer.
type TransducerOutput struct {
	Intent      Intent
	Focus       []FocusResolution
	MangleAtoms []core.Fact
}

// Parse performs full transduction of user input.
func (t *DualPayloadTransducer) Parse(ctx context.Context, input string, fileCandidates []string) (TransducerOutput, error) {
	intent, err := t.ParseIntent(ctx, input)
	if err != nil {
		return TransducerOutput{}, err
	}

	output := TransducerOutput{
		Intent:      intent,
		MangleAtoms: []core.Fact{intent.ToFact()},
	}

	// Try to resolve focus if target looks like a file reference
	if intent.Target != "" && intent.Target != "none" {
		focus, err := t.ResolveFocus(ctx, intent.Target, fileCandidates)
		if err == nil && focus.Confidence > 0 {
			output.Focus = append(output.Focus, focus)
			output.MangleAtoms = append(output.MangleAtoms, focus.ToFact())
		}
	}

	return output, nil
}
