# codeNERD Schema Template
# Mangle schemas for the Neuro-Symbolic Architecture
# Compatible with Cortex 1.5.0 and codeNERD kernel
#
# Mangle v0.4.0

# =============================================================================
# SECTION 1: Intent & Focus (Perception Transducer Output)
# =============================================================================

# User intent parsed from natural language
Decl user_intent(ID.Type<n>, Category.Type<n>, Verb.Type<n>, Target.Type<string>, Constraint.Type<string>)
  descr [doc("Parsed user intent from NL input")].

# Focus resolution - what the user is referring to
Decl focus_resolution(RawRef.Type<string>, Path.Type<string>, Symbol.Type<string>, Confidence.Type<float>)
  descr [doc("Resolved code references from user input")].

# Ambiguity markers for clarification
Decl ambiguity_flag(MissingParam.Type<string>, ContextClue.Type<string>, Hypothesis.Type<string>)
  descr [doc("Marks ambiguous user requests")].

# =============================================================================
# SECTION 2: World Model - File System
# =============================================================================

# File topology facts
Decl file_topology(Path.Type<string>, Hash.Type<string>, Language.Type<n>, LastModified.Type<int>, IsTestFile.Type<n>)
  descr [doc("File metadata from filesystem scan")].

# File content chunks (for large files)
Decl file_content(Path.Type<string>, StartLine.Type<int>, EndLine.Type<int>, Content.Type<string>)
  descr [doc("File content by line range")].

# File dependencies
Decl file_imports(SourcePath.Type<string>, ImportPath.Type<string>, ImportType.Type<n>)
  descr [doc("Import relationships between files")].

# =============================================================================
# SECTION 3: World Model - Symbol Graph
# =============================================================================

# Symbol definitions
Decl symbol_graph(SymbolID.Type<string>, Type.Type<n>, Visibility.Type<n>, DefinedAt.Type<string>, Signature.Type<string>)
  descr [doc("Code symbol definitions")].

# Symbol dependencies
Decl dependency_link(CallerID.Type<string>, CalleeID.Type<string>, ImportPath.Type<string>)
  descr [doc("Call/usage relationships between symbols")].

# Symbol references
Decl symbol_reference(SymbolID.Type<string>, FilePath.Type<string>, Line.Type<int>, Column.Type<int>)
  descr [doc("Where symbols are referenced")].

# =============================================================================
# SECTION 4: Diagnostics
# =============================================================================

# Compiler/linter diagnostics
Decl diagnostic(Severity.Type<n>, FilePath.Type<string>, Line.Type<int>, ErrorCode.Type<string>, Message.Type<string>)
  descr [doc("Compiler and linter errors/warnings")].

# Test results
Decl test_result(TestName.Type<string>, FilePath.Type<string>, Status.Type<n>, Duration.Type<int>, Message.Type<string>)
  descr [doc("Test execution results")].

# =============================================================================
# SECTION 5: TDD Loop State
# =============================================================================

# Current TDD state
Decl test_state(State.Type<n>)
  descr [doc("Current TDD loop state: /passing, /failing, /compiling, /unknown")].

# Next recommended action
Decl next_action(Action.Type<n>)
  descr [doc("Mangle-derived next action")].

# Action permission (constitutional safety)
Decl permitted(Action.Type<n>)
  descr [doc("Actions permitted by constitution")].

# =============================================================================
# SECTION 6: Shard Management
# =============================================================================

# Shard profiles
Decl shard_profile(AgentName.Type<n>, Description.Type<string>, Topics.Type<string>, Tools.Type<string>)
  descr [doc("Specialist shard capabilities")].

# Task delegation
Decl delegate_task(ShardType.Type<n>, Task.Type<string>, Priority.Type<int>)
  descr [doc("Tasks delegated to shards")].

# Shard execution status
Decl shard_status(AgentName.Type<n>, Status.Type<n>, LastActive.Type<int>)
  descr [doc("Current shard execution status")].

# =============================================================================
# SECTION 7: Research & Knowledge
# =============================================================================

# Knowledge atoms from research
Decl knowledge_atom(SourceURL.Type<string>, Concept.Type<string>, CodePattern.Type<string>, AntiPattern.Type<string>)
  descr [doc("Extracted knowledge from research")].

# Research topics
Decl research_topic(AgentName.Type<n>, Topic.Type<string>, Status.Type<n>)
  descr [doc("Active research topics")].

# Documentation references
Decl doc_reference(Library.Type<string>, URL.Type<string>, LastFetched.Type<int>)
  descr [doc("External documentation sources")].

# =============================================================================
# SECTION 8: Observations & Memory
# =============================================================================

# Session observations
Decl observation(Key.Type<n>, Value.Type<string>)
  descr [doc("Runtime observations")].

# User preferences (learned)
Decl preference(Key.Type<n>, Value.Type<string>)
  descr [doc("Learned user preferences")].

# Spreading activation context
Decl activation(Concept.Type<string>, Weight.Type<float>, Source.Type<n>)
  descr [doc("Spreading activation weights")].

# =============================================================================
# SECTION 9: Campaign & Goals
# =============================================================================

# Current campaign
Decl campaign(ID.Type<n>, Goal.Type<string>, Status.Type<n>)
  descr [doc("Multi-step campaign tracking")].

# Campaign phases
Decl campaign_phase(CampaignID.Type<n>, PhaseNum.Type<int>, Description.Type<string>, Status.Type<n>)
  descr [doc("Individual campaign phases")].

# Phase dependencies
Decl phase_depends(CampaignID.Type<n>, Phase.Type<int>, DependsOn.Type<int>)
  descr [doc("Phase ordering constraints")].

# =============================================================================
# SECTION 10: Autopoiesis (Tool Generation)
# =============================================================================

# Generated tools
Decl generated_tool(Name.Type<string>, Description.Type<string>, Schema.Type<string>, Status.Type<n>)
  descr [doc("Dynamically generated tools")].

# Tool invocation history
Decl tool_invocation(ToolName.Type<string>, Timestamp.Type<int>, Success.Type<n>)
  descr [doc("Tool usage history")].

# Tool quality metrics
Decl tool_quality(ToolName.Type<string>, SuccessRate.Type<float>, AvgDuration.Type<int>)
  descr [doc("Tool effectiveness metrics")].
