# Intent Definitions - Code Review & Security
# SECTIONS 4-5: CODE REVIEW & SECURITY ANALYSIS - REVIEWER SHARD
# Requests for code review, quality checks, security audits, and vulnerability scanning.

# =============================================================================
# SECTION 4: CODE REVIEW (/review)
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
# SECTION 5: SECURITY ANALYSIS (/security)
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
