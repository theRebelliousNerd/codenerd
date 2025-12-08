# Intent Definition Schema
# Maps Canonical Sentences to constrained Mangle Actions.

Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

# Canonical Examples (Must match transducerSystemPrompt)
intent_definition("Review this file for bugs.", /review, "context_file").
intent_category("Review this file for bugs.", /query).

intent_definition("Check my code for security issues.", /security, "codebase").
intent_category("Check my code for security issues.", /query).

intent_definition("Fix the compilation error.", /fix, "compiler_error").
intent_category("Fix the compilation error.", /mutation).

intent_definition("Refactor this function to be cleaner.", /refactor, "focused_symbol").
intent_category("Refactor this function to be cleaner.", /mutation).

intent_definition("What does this function do?", /explain, "focused_symbol").
intent_category("What does this function do?", /query).

intent_definition("Run the tests.", /test, "context_file").
intent_category("Run the tests.", /mutation).

intent_definition("Generate unit tests for this.", /test, "context_file").
intent_category("Generate unit tests for this.", /mutation).

intent_definition("Deploy to production.", /deploy, "production").
intent_category("Deploy to production.", /mutation).

intent_definition("Research how to use X.", /research, "X").
intent_category("Research how to use X.", /query).

intent_definition("Create a new file called main.go.", /create, "main.go").
intent_category("Create a new file called main.go.", /mutation).

intent_definition("Delete the database.", /delete, "database").
intent_category("Delete the database.", /mutation).

intent_definition("Start a campaign to rewrite auth.", /campaign, "rewrite auth").
intent_category("Start a campaign to rewrite auth.", /mutation).

intent_definition("Configure the agent to be verbose.", /configure, "verbosity").
intent_category("Configure the agent to be verbose.", /instruction).
