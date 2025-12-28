# codeNERD Schema Template
# Mangle schemas for the Neuro-Symbolic Architecture
# Compatible with Cortex 1.5.0 and codeNERD kernel
#
# Mangle v0.4.0 - Using correct syntax: descr before bound

# =============================================================================
# SECTION 1: Intent & Focus (Perception Transducer Output)
# =============================================================================

# User intent parsed from natural language
Decl user_intent(ID, Category, Verb, Target, Constraint)
    descr [doc("Parsed user intent from NL input")]
    bound [/name, /name, /name, /string, /string].

# Focus resolution - what the user is referring to
Decl focus_resolution(RawRef, Path, Symbol, Confidence)
    descr [doc("Resolved code references from user input")]
    bound [/string, /string, /string, /number].

# Ambiguity markers for clarification
Decl ambiguity_flag(MissingParam, ContextClue, Hypothesis)
    descr [doc("Marks ambiguous user requests")]
    bound [/string, /string, /string].

# =============================================================================
# SECTION 2: World Model - File System
# =============================================================================

# File topology facts
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile)
    descr [doc("File metadata from filesystem scan")]
    bound [/string, /string, /name, /number, /name].

# File content chunks (for large files)
Decl file_content(Path, StartLine, EndLine, Content)
    descr [doc("File content by line range")]
    bound [/string, /number, /number, /string].

# File dependencies
Decl file_imports(SourcePath, ImportPath, ImportType)
    descr [doc("Import relationships between files")]
    bound [/string, /string, /name].

# =============================================================================
# SECTION 3: World Model - Symbol Graph
# =============================================================================

# Symbol definitions
Decl symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
    descr [doc("Code symbol definitions")]
    bound [/string, /name, /name, /string, /string].

# Symbol dependencies
Decl dependency_link(CallerID, CalleeID, ImportPath)
    descr [doc("Call/usage relationships between symbols")]
    bound [/string, /string, /string].

# Symbol references
Decl symbol_reference(SymbolID, FilePath, Line, Column)
    descr [doc("Where symbols are referenced")]
    bound [/string, /string, /number, /number].

# =============================================================================
# SECTION 4: Diagnostics
# =============================================================================

# Compiler/linter diagnostics
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message)
    descr [doc("Compiler and linter errors/warnings")]
    bound [/name, /string, /number, /string, /string].

# Test results
Decl test_result(TestName, FilePath, Status, Duration, Message)
    descr [doc("Test execution results")]
    bound [/string, /string, /name, /number, /string].

# =============================================================================
# SECTION 5: TDD Loop State
# =============================================================================

# Current TDD state
Decl test_state(State)
    descr [doc("Current TDD loop state: /passing, /failing, /compiling, /unknown")]
    bound [/name].

# Next recommended action
Decl next_action(Action)
    descr [doc("Mangle-derived next action")]
    bound [/name].

# Action permission (constitutional safety)
Decl permitted(Action)
    descr [doc("Actions permitted by constitution")]
    bound [/name].

# =============================================================================
# SECTION 6: Shard Management
# =============================================================================

# Shard profiles
Decl shard_profile(AgentName, Description, Topics, Tools)
    descr [doc("Specialist shard capabilities")]
    bound [/name, /string, /string, /string].

# Task delegation
Decl delegate_task(ShardType, Task, Priority)
    descr [doc("Tasks delegated to shards")]
    bound [/name, /string, /number].

# Shard execution status
Decl shard_status(AgentName, Status, LastActive)
    descr [doc("Current shard execution status")]
    bound [/name, /name, /number].

# =============================================================================
# SECTION 7: Research & Knowledge
# =============================================================================

# Knowledge atoms from research
Decl knowledge_atom(SourceURL, Concept, CodePattern, AntiPattern)
    descr [doc("Extracted knowledge from research")]
    bound [/string, /string, /string, /string].

# Research topics
Decl research_topic(AgentName, Topic, Status)
    descr [doc("Active research topics")]
    bound [/name, /string, /name].

# Documentation references
Decl doc_reference(Library, URL, LastFetched)
    descr [doc("External documentation sources")]
    bound [/string, /string, /number].

# =============================================================================
# SECTION 8: Observations & Memory
# =============================================================================

# Session observations
Decl observation(Key, Value)
    descr [doc("Runtime observations")]
    bound [/name, /string].

# User preferences (learned)
Decl preference(Key, Value)
    descr [doc("Learned user preferences")]
    bound [/name, /string].

# Spreading activation context
Decl activation(Concept, Weight, Source)
    descr [doc("Spreading activation weights")]
    bound [/string, /number, /name].

# =============================================================================
# SECTION 9: Campaign & Goals
# =============================================================================

# Current campaign
Decl campaign(ID, Goal, Status)
    descr [doc("Multi-step campaign tracking")]
    bound [/name, /string, /name].

# Campaign phases
Decl campaign_phase(CampaignID, PhaseNum, Description, Status)
    descr [doc("Individual campaign phases")]
    bound [/name, /number, /string, /name].

# Phase dependencies
Decl phase_depends(CampaignID, Phase, DependsOn)
    descr [doc("Phase ordering constraints")]
    bound [/name, /number, /number].

# =============================================================================
# SECTION 10: Autopoiesis (Tool Generation)
# =============================================================================

# Generated tools
Decl generated_tool(Name, Description, Schema, Status)
    descr [doc("Dynamically generated tools")]
    bound [/string, /string, /string, /name].

# Tool invocation history
Decl tool_invocation(ToolName, Timestamp, Success)
    descr [doc("Tool usage history")]
    bound [/string, /number, /name].

# Tool quality metrics
Decl tool_quality(ToolName, SuccessRate, AvgDuration)
    descr [doc("Tool effectiveness metrics")]
    bound [/string, /number, /number].
