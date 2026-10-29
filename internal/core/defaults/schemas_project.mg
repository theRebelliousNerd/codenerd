# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: PROJECT
# Sections: 18, 19, 20

# =============================================================================
# SECTION 18: PROJECT PROFILE (nerd init)
# =============================================================================

# project_profile(ProjectID, Name, Description)
Decl project_profile(ProjectID, Name, Description) bound [/string, /string, /string].

# project_language(Language)
# Language: /go, /rust, /python, /javascript, /typescript, /java, etc.
Decl project_language(Language) bound [/name].

# project_framework(Framework)
# Framework: /gin, /echo, /nextjs, /react, /django, etc.
Decl project_framework(Framework) bound [/name].

# project_architecture(Architecture)
# Architecture: /monolith, /microservices, /clean_architecture, /serverless
Decl project_architecture(Architecture) bound [/name].

# build_system(BuildSystem)
# BuildSystem: /go, /npm, /cargo, /pip, /maven, /gradle
Decl build_system(BuildSystem) bound [/name].

# architectural_pattern(Pattern)
# Pattern: /standard_go_layout, /repository_pattern, /service_layer, /domain_driven
Decl architectural_pattern(Pattern) bound [/name].

# entry_point(FilePath)
Decl entry_point(FilePath) bound [/string].

# =============================================================================
# SECTION 19: USER PREFERENCES (Autopoiesis / Learning)
# =============================================================================

# user_preference(Key, Value)
# Keys: /test_style, /error_handling, /commit_style, /verbosity, /explanation_level
Decl user_preference(Key, Value) bound [/name, /string].

# preference_learned(Key, Value, Timestamp, Confidence)
Decl preference_learned(Key, Value, Timestamp, Confidence) bound [/name, /string, /number, /number].

# =============================================================================
# SECTION 20: SESSION STATE (Pause/Resume Protocol)
# =============================================================================

# session_state(SessionID, State, SerializedContext)
# State: /active, /suspended, /completed
Decl session_state(SessionID, State, SerializedContext) bound [/string, /name, /string].

# pending_clarification(Question, Options, DefaultValue)
Decl pending_clarification(Question, Options, DefaultValue) bound [/string, /string, /string].

# awaiting_clarification(Question)
Decl awaiting_clarification(Question) bound [/string].

# any_awaiting_clarification(Status)
Decl any_awaiting_clarification(Status) bound [/name].

# awaiting_user_input(RequestID)
Decl awaiting_user_input(RequestID) bound [/string].

# campaign_awaiting_clarification(CampaignID)
Decl campaign_awaiting_clarification(CampaignID) bound [/string].

# focus_clarification(Response) - user's clarification response
Decl focus_clarification(Response) bound [/string].

# turn_context(TurnNumber, IntentID, ActionsTaken)
Decl turn_context(TurnNumber, IntentID, ActionsTaken) bound [/number, /string, /number].
