# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: PROJECT
# Sections: 18, 19, 20

# =============================================================================
# SECTION 18: PROJECT PROFILE (nerd init)
# =============================================================================

# project_profile(ProjectID, Name, Description)
Decl project_profile(ProjectID, Name, Description).

# project_language(Language)
# Language: /go, /rust, /python, /javascript, /typescript, /java, etc.
Decl project_language(Language).

# project_framework(Framework)
# Framework: /gin, /echo, /nextjs, /react, /django, etc.
Decl project_framework(Framework).

# project_architecture(Architecture)
# Architecture: /monolith, /microservices, /clean_architecture, /serverless
Decl project_architecture(Architecture).

# build_system(BuildSystem)
# BuildSystem: /go, /npm, /cargo, /pip, /maven, /gradle
Decl build_system(BuildSystem).

# architectural_pattern(Pattern)
# Pattern: /standard_go_layout, /repository_pattern, /service_layer, /domain_driven
Decl architectural_pattern(Pattern).

# entry_point(FilePath)
Decl entry_point(FilePath).

# =============================================================================
# SECTION 19: USER PREFERENCES (Autopoiesis / Learning)
# =============================================================================

# user_preference(Key, Value)
# Keys: /test_style, /error_handling, /commit_style, /verbosity, /explanation_level
Decl user_preference(Key, Value).

# preference_learned(Key, Value, Timestamp, Confidence)
Decl preference_learned(Key, Value, Timestamp, Confidence).

# =============================================================================
# SECTION 20: SESSION STATE (Pause/Resume Protocol)
# =============================================================================

# session_state(SessionID, State, SerializedContext)
# State: /active, /suspended, /completed
Decl session_state(SessionID, State, SerializedContext).

# pending_clarification(Question, Options, DefaultValue)
Decl pending_clarification(Question, Options, DefaultValue).

# awaiting_clarification(Question)
Decl awaiting_clarification(Question).

# awaiting_user_input(RequestID)
Decl awaiting_user_input(RequestID).

# campaign_awaiting_clarification(CampaignID)
Decl campaign_awaiting_clarification(CampaignID).

# focus_clarification(Response) - user's clarification response
Decl focus_clarification(Response).

# turn_context(TurnNumber, IntentID, ActionsTaken)
Decl turn_context(TurnNumber, IntentID, ActionsTaken).

