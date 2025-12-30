# Intent Instructions

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


