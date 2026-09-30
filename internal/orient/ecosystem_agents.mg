# Agent-ecosystem policy. Parsing asserts the measurements; this file decides
# which copy wins and which shard agents init creates.
#
# The orientation engine loads policy/config_params.mg and schema.mg before
# this file. Those already declare config_param/2, config_param_required/2,
# has_config_param/1, config_param_missing/2, repo_file_history/5,
# agent_source/6, agent_source_digest/2, agent_source_topic/2 and
# agent_source_scope/2. A second Decl is a hard error. Do not add one.
#
config_param_required(/orient, /orient_topic_overlap_min).
config_param_required(/orient, /orient_skill_cluster_min).
#
# Mangle compares numbers only, so the stable identity order is the numeric
# rank the driver asserts (agent_source_ord). The rule still decides.

Decl agent_source_norm(ID, Norm) bound [/string, /string].
Decl agent_source_bytes(ID, N) bound [/string, /number].
Decl agent_source_ord(ID, Ord) bound [/string, /number].
Decl agent_source_refers(FromID, ToID) bound [/string, /string].
Decl agent_source_tag(ID, Tag) bound [/string, /string].
Decl agent_source_declared_tool(ID, Tool) bound [/string, /string].
Decl agent_source_description(ID, Description) bound [/string, /string].

Decl profile_signal(Kind, Key) bound [/name, /string].
Decl profile_agent(Kind, Key, Name, Why) bound [/name, /string, /string, /string].
Decl profile_agent_topic(Name, Topic) bound [/string, /string].
Decl profile_agent_permission(Name, Perm) bound [/string, /string].
Decl profile_agent_description(Name, Description) bound [/string, /string].
Decl profile_agent_priority(Name, Priority) bound [/string, /number].
Decl fallback_agent(Name, Why) bound [/string, /string].
Decl fallback_agent_topic(Name, Topic) bound [/string, /string].
Decl fallback_agent_permission(Name, Perm) bound [/string, /string].
Decl fallback_agent_description(Name, Description) bound [/string, /string].
Decl fallback_agent_priority(Name, Priority) bound [/string, /number].

Decl imported_kind(Kind) bound [/name].
Decl knowledge_kind(Kind) bound [/name].
Decl imported_default_permission(Perm) bound [/string].
Decl imported_agent_priority(Priority) bound [/number].
Decl cluster_agent_priority(Priority) bound [/number].

Decl name_duplicate(A, B) bound [/string, /string].
Decl shared_topic(A, B, Topic) bound [/string, /string, /string].
Decl shared_topic_count(A, B, N) bound [/string, /string, /number].
Decl topic_duplicate(A, B) bound [/string, /string].
Decl duplicate_edge(A, B) bound [/string, /string].
Decl link(A, B) bound [/string, /string].
Decl peer(A, B) bound [/string, /string].
Decl in_duplicate(ID) bound [/string].
Decl in_group(ID, Other) bound [/string, /string].
Decl source_last_unix(ID, LastUnix) bound [/string, /number].
Decl has_history(ID) bound [/string].
Decl recency_beats(A, B) bound [/string, /string].
Decl recency_separates(A, B) bound [/string, /string].
Decl tracked_yes(ID) bound [/string].
Decl tracked_beats(A, B) bound [/string, /string].
Decl tracked_separates(A, B) bound [/string, /string].
Decl size_beats(A, B) bound [/string, /string].
Decl size_separates(A, B) bound [/string, /string].
Decl agent_source_topic_count(ID, N) bound [/string, /number].
Decl has_topic_count(ID) bound [/string].
Decl spec_beats(A, B) bound [/string, /string].
Decl spec_separates(A, B) bound [/string, /string].
Decl ref_count(ID, N) bound [/string, /number].
Decl has_ref_count(ID) bound [/string].
Decl ref_beats(A, B) bound [/string, /string].
Decl ref_separates(A, B) bound [/string, /string].
Decl id_beats(A, B) bound [/string, /string].
Decl why_recent(A, B) bound [/string, /string].
Decl why_tracked(A, B) bound [/string, /string].
Decl why_size(A, B) bound [/string, /string].
Decl why_specific(A, B) bound [/string, /string].
Decl why_referenced(A, B) bound [/string, /string].
Decl why_id(A, B) bound [/string, /string].
Decl why_recent_any(ID) bound [/string].
Decl why_tracked_any(ID) bound [/string].
Decl why_size_any(ID) bound [/string].
Decl why_specific_any(ID) bound [/string].
Decl why_referenced_any(ID) bound [/string].
Decl why_id_any(ID) bound [/string].
Decl better(A, B) bound [/string, /string].
Decl dominated(ID) bound [/string].
Decl undominated(ID) bound [/string].
Decl comp_id(ID, MinOrd) bound [/string, /number].
Decl comp_has_undominated(MinOrd) bound [/number].
Decl cycle_member(ID) bound [/string].
Decl agent_source_winner_any(ID) bound [/string].
Decl has_declared_tool(ID) bound [/string].
Decl profile_matched(Yes) bound [/name].
Decl winning_skill(ID) bound [/string].
Decl winning_skill_tag(ID, Tag) bound [/string, /string].
Decl skill_tag_count(Tag, N) bound [/string, /number].
Decl skill_cluster(Tag) bound [/string].
Decl topic_covered(Name, Topic) bound [/string, /string].

Decl agent_source_duplicate(A, B) bound [/string, /string].
Decl agent_source_winner(ID, Why) bound [/string, /name].
Decl agent_source_loser(Loser, Winner, Why) bound [/string, /string, /name].
Decl orient_agent(Name, Why) bound [/string, /string].
Decl orient_agent_topic(Name, Topic) bound [/string, /string].
Decl orient_agent_description(Name, Description) bound [/string, /string].
Decl orient_agent_permission(Name, Perm) bound [/string, /string].
Decl orient_agent_priority(Name, Priority) bound [/string, /number].
Decl orient_agent_knowledge(Name, SourceID) bound [/string, /string].
Decl orient_research_topic(Name, Topic) bound [/string, /string].
Decl orient_agent_prompt_candidate(Name, ID) bound [/string, /string].
Decl orient_agent_prompt_better(Name, ID) bound [/string, /string].
Decl orient_agent_prompt(Name, ID) bound [/string, /string].

orient_agent_prompt_candidate(Name, ID) :-
    agent_source_winner(ID, Why),
    agent_source(ID, Tool, Kind, Name, Path, Tracked),
    imported_kind(Kind).
orient_agent_prompt_better(Name, ID) :-
    orient_agent_prompt_candidate(Name, ID),
    orient_agent_prompt_candidate(Name, Other),
    agent_source_bytes(ID, Bytes),
    agent_source_bytes(Other, More),
    More > Bytes.
orient_agent_prompt_better(Name, ID) :-
    orient_agent_prompt_candidate(Name, ID),
    orient_agent_prompt_candidate(Name, Other),
    agent_source_bytes(ID, Bytes),
    agent_source_bytes(Other, Bytes),
    agent_source_ord(ID, Ord),
    agent_source_ord(Other, Earlier),
    Earlier < Ord.
orient_agent_prompt(Name, ID) :-
    orient_agent_prompt_candidate(Name, ID),
    !orient_agent_prompt_better(Name, ID).

# Catalog priorities are data, the same class as GoExpert's 100.
# Topic-overlap and skill-cluster minima are config_param rows, not literals.
# Both minima are required configuration facts.
imported_agent_priority(90).
cluster_agent_priority(70).

imported_kind(/subagent).
imported_kind(/mode).
knowledge_kind(/skill).
knowledge_kind(/memory).
knowledge_kind(/rule).

imported_default_permission("read_file").
imported_default_permission("code_graph").
imported_default_permission("exec_cmd").

# --- language / framework / dependency catalog (was the init Go switch) ---

profile_agent(/language, "go", "GoExpert", "Go project detected - expert knowledge improves code quality").
profile_agent(/language, "golang", "GoExpert", "Go project detected - expert knowledge improves code quality").
profile_agent_description("GoExpert", "Expert in Go idioms, concurrency patterns, and standard library").
profile_agent_priority("GoExpert", 100).
profile_agent_topic("GoExpert", "go concurrency").
profile_agent_topic("GoExpert", "go error handling").
profile_agent_topic("GoExpert", "go interfaces").
profile_agent_topic("GoExpert", "go testing").
profile_agent_permission("GoExpert", "read_file").
profile_agent_permission("GoExpert", "code_graph").
profile_agent_permission("GoExpert", "exec_cmd").

profile_agent(/language, "python", "PythonExpert", "Python project detected - expert knowledge improves code quality").
profile_agent_description("PythonExpert", "Expert in Python best practices, type hints, and async patterns").
profile_agent_priority("PythonExpert", 100).
profile_agent_topic("PythonExpert", "python typing").
profile_agent_topic("PythonExpert", "python async").
profile_agent_topic("PythonExpert", "python testing").
profile_agent_topic("PythonExpert", "python packaging").
profile_agent_permission("PythonExpert", "read_file").
profile_agent_permission("PythonExpert", "code_graph").
profile_agent_permission("PythonExpert", "exec_cmd").

profile_agent(/language, "typescript", "TSExpert", "TypeScript/JavaScript project detected").
profile_agent(/language, "javascript", "TSExpert", "TypeScript/JavaScript project detected").
profile_agent_description("TSExpert", "Expert in TypeScript/JavaScript patterns and modern ES features").
profile_agent_priority("TSExpert", 100).
profile_agent_topic("TSExpert", "typescript types").
profile_agent_topic("TSExpert", "javascript async").
profile_agent_topic("TSExpert", "react patterns").
profile_agent_topic("TSExpert", "node.js").
profile_agent_permission("TSExpert", "read_file").
profile_agent_permission("TSExpert", "code_graph").
profile_agent_permission("TSExpert", "exec_cmd").

profile_agent(/language, "rust", "RustExpert", "Rust project detected - ownership expertise critical").
profile_agent_description("RustExpert", "Expert in Rust ownership, lifetimes, and async patterns").
profile_agent_priority("RustExpert", 100).
profile_agent_topic("RustExpert", "rust ownership").
profile_agent_topic("RustExpert", "rust lifetimes").
profile_agent_topic("RustExpert", "rust async").
profile_agent_topic("RustExpert", "rust error handling").
profile_agent_permission("RustExpert", "read_file").
profile_agent_permission("RustExpert", "code_graph").
profile_agent_permission("RustExpert", "exec_cmd").

profile_agent(/language, "kotlin", "AndroidExpert", "Kotlin/Android project detected - mobile expertise critical").
profile_agent_description("AndroidExpert", "Expert in Kotlin Android development, Jetpack Compose, and mobile patterns").
profile_agent_priority("AndroidExpert", 100).
profile_agent_topic("AndroidExpert", "kotlin android").
profile_agent_topic("AndroidExpert", "jetpack compose").
profile_agent_topic("AndroidExpert", "android architecture").
profile_agent_topic("AndroidExpert", "coroutines").
profile_agent_topic("AndroidExpert", "room database").
profile_agent_topic("AndroidExpert", "hilt dependency injection").
profile_agent_permission("AndroidExpert", "read_file").
profile_agent_permission("AndroidExpert", "code_graph").
profile_agent_permission("AndroidExpert", "exec_cmd").

profile_agent(/framework, "gin", "WebAPIExpert", "gin framework detected - API expertise beneficial").
profile_agent(/framework, "echo", "WebAPIExpert", "echo framework detected - API expertise beneficial").
profile_agent(/framework, "fiber", "WebAPIExpert", "fiber framework detected - API expertise beneficial").
profile_agent_description("WebAPIExpert", "Expert in REST API design and HTTP middleware patterns").
profile_agent_priority("WebAPIExpert", 80).
profile_agent_topic("WebAPIExpert", "REST API design").
profile_agent_topic("WebAPIExpert", "HTTP middleware").
profile_agent_topic("WebAPIExpert", "API authentication").
profile_agent_topic("WebAPIExpert", "OpenAPI").
profile_agent_permission("WebAPIExpert", "read_file").
profile_agent_permission("WebAPIExpert", "network").

profile_agent(/framework, "react", "FrontendExpert", "react framework detected - frontend expertise beneficial").
profile_agent(/framework, "nextjs", "FrontendExpert", "nextjs framework detected - frontend expertise beneficial").
profile_agent(/framework, "vue", "FrontendExpert", "vue framework detected - frontend expertise beneficial").
profile_agent_description("FrontendExpert", "Expert in modern frontend patterns and state management").
profile_agent_priority("FrontendExpert", 80).
profile_agent_topic("FrontendExpert", "react hooks").
profile_agent_topic("FrontendExpert", "state management").
profile_agent_topic("FrontendExpert", "component patterns").
profile_agent_topic("FrontendExpert", "CSS-in-JS").
profile_agent_permission("FrontendExpert", "read_file").
profile_agent_permission("FrontendExpert", "browser").

profile_agent(/dependency, "rod", "RodExpert", "Rod browser automation detected - specialized expertise beneficial").
profile_agent_description("RodExpert", "Expert in Rod browser automation, selectors, and CDP protocol").
profile_agent_priority("RodExpert", 95).
profile_agent_topic("RodExpert", "rod browser automation").
profile_agent_topic("RodExpert", "CDP protocol").
profile_agent_topic("RodExpert", "web scraping").
profile_agent_topic("RodExpert", "headless chrome").
profile_agent_topic("RodExpert", "page selectors").
profile_agent_permission("RodExpert", "read_file").
profile_agent_permission("RodExpert", "browser").
profile_agent_permission("RodExpert", "exec_cmd").

profile_agent(/dependency, "chromedp", "BrowserAutomationExpert", "Browser automation library detected").
profile_agent(/dependency, "puppeteer", "BrowserAutomationExpert", "Browser automation library detected").
profile_agent(/dependency, "playwright", "BrowserAutomationExpert", "Browser automation library detected").
profile_agent_description("BrowserAutomationExpert", "Expert in browser automation patterns and CDP").
profile_agent_priority("BrowserAutomationExpert", 90).
profile_agent_topic("BrowserAutomationExpert", "browser automation").
profile_agent_topic("BrowserAutomationExpert", "CDP protocol").
profile_agent_topic("BrowserAutomationExpert", "page navigation").
profile_agent_topic("BrowserAutomationExpert", "element interaction").
profile_agent_permission("BrowserAutomationExpert", "read_file").
profile_agent_permission("BrowserAutomationExpert", "browser").

profile_agent(/dependency, "mangle", "MangleExpert", "Mangle/Datalog detected - logic programming expertise critical").
profile_agent_description("MangleExpert", "Expert in Google Mangle/Datalog, logic programming, and rule systems").
profile_agent_priority("MangleExpert", 95).
profile_agent_topic("MangleExpert", "datalog").
profile_agent_topic("MangleExpert", "mangle syntax").
profile_agent_topic("MangleExpert", "logic programming").
profile_agent_topic("MangleExpert", "horn clauses").
profile_agent_topic("MangleExpert", "fact derivation").
profile_agent_topic("MangleExpert", "negation as failure").
profile_agent_permission("MangleExpert", "read_file").
profile_agent_permission("MangleExpert", "code_graph").

profile_agent(/dependency, "openai", "LLMIntegrationExpert", "LLM API integration detected - expertise improves reliability").
profile_agent(/dependency, "anthropic", "LLMIntegrationExpert", "LLM API integration detected - expertise improves reliability").
profile_agent_description("LLMIntegrationExpert", "Expert in LLM API integration, prompt engineering, and token optimization").
profile_agent_priority("LLMIntegrationExpert", 90).
profile_agent_topic("LLMIntegrationExpert", "LLM APIs").
profile_agent_topic("LLMIntegrationExpert", "prompt engineering").
profile_agent_topic("LLMIntegrationExpert", "token optimization").
profile_agent_topic("LLMIntegrationExpert", "streaming responses").
profile_agent_topic("LLMIntegrationExpert", "function calling").
profile_agent_permission("LLMIntegrationExpert", "read_file").
profile_agent_permission("LLMIntegrationExpert", "network").

profile_agent(/dependency, "bubbletea", "BubbleTeaExpert", "Bubbletea TUI framework detected").
profile_agent_description("BubbleTeaExpert", "Expert in Bubbletea TUI framework, Elm architecture, and terminal rendering").
profile_agent_priority("BubbleTeaExpert", 85).
profile_agent_topic("BubbleTeaExpert", "bubbletea").
profile_agent_topic("BubbleTeaExpert", "elm architecture").
profile_agent_topic("BubbleTeaExpert", "terminal UI").
profile_agent_topic("BubbleTeaExpert", "lipgloss styling").
profile_agent_topic("BubbleTeaExpert", "bubbles components").
profile_agent_permission("BubbleTeaExpert", "read_file").
profile_agent_permission("BubbleTeaExpert", "code_graph").

profile_agent(/dependency, "cobra", "CobraExpert", "Cobra CLI framework detected").
profile_agent_description("CobraExpert", "Expert in Cobra CLI framework, command structure, and flag handling").
profile_agent_priority("CobraExpert", 75).
profile_agent_topic("CobraExpert", "cobra CLI").
profile_agent_topic("CobraExpert", "command patterns").
profile_agent_topic("CobraExpert", "flag handling").
profile_agent_topic("CobraExpert", "CLI best practices").
profile_agent_permission("CobraExpert", "read_file").

profile_agent(/dependency, "gorm", "DatabaseExpert", "Database ORM/driver detected").
profile_agent(/dependency, "sqlx", "DatabaseExpert", "Database ORM/driver detected").
profile_agent(/dependency, "sql", "DatabaseExpert", "Database ORM/driver detected").
profile_agent(/dependency, "prisma", "DatabaseExpert", "Database ORM/driver detected").
profile_agent(/dependency, "typeorm", "DatabaseExpert", "Database ORM/driver detected").
profile_agent_description("DatabaseExpert", "Expert in database patterns, ORM usage, and query optimization").
profile_agent_priority("DatabaseExpert", 80).
profile_agent_topic("DatabaseExpert", "database design").
profile_agent_topic("DatabaseExpert", "ORM patterns").
profile_agent_topic("DatabaseExpert", "SQL optimization").
profile_agent_topic("DatabaseExpert", "migrations").
profile_agent_topic("DatabaseExpert", "connection pooling").
profile_agent_permission("DatabaseExpert", "read_file").
profile_agent_permission("DatabaseExpert", "code_graph").

profile_agent(/dependency, "arangodb", "ArangoExpert", "ArangoDB detected - graph database expertise beneficial").
profile_agent_description("ArangoExpert", "Expert in ArangoDB graph database, AQL queries, and document/graph modeling").
profile_agent_priority("ArangoExpert", 85).
profile_agent_topic("ArangoExpert", "arangodb").
profile_agent_topic("ArangoExpert", "AQL queries").
profile_agent_topic("ArangoExpert", "graph traversal").
profile_agent_topic("ArangoExpert", "document modeling").
profile_agent_topic("ArangoExpert", "multi-model database").
profile_agent_topic("ArangoExpert", "graph database patterns").
profile_agent_permission("ArangoExpert", "read_file").
profile_agent_permission("ArangoExpert", "code_graph").
profile_agent_permission("ArangoExpert", "network").

profile_agent(/dependency, "adk", "ADKExpert", "Google ADK detected - agent orchestration expertise beneficial").
profile_agent_description("ADKExpert", "Expert in Google ADK for LLM agent orchestration and tool use").
profile_agent_priority("ADKExpert", 90).
profile_agent_topic("ADKExpert", "google adk").
profile_agent_topic("ADKExpert", "agent orchestration").
profile_agent_topic("ADKExpert", "llm tool use").
profile_agent_topic("ADKExpert", "multi-agent systems").
profile_agent_topic("ADKExpert", "agent workflows").
profile_agent_permission("ADKExpert", "read_file").
profile_agent_permission("ADKExpert", "code_graph").
profile_agent_permission("ADKExpert", "network").

profile_agent(/dependency, "a2a", "A2AExpert", "A2A protocol detected - agent interop expertise beneficial").
profile_agent_description("A2AExpert", "Expert in A2A card-driven and manifest-based agent patterns").
profile_agent_priority("A2AExpert", 85).
profile_agent_topic("A2AExpert", "a2a protocol").
profile_agent_topic("A2AExpert", "agent cards").
profile_agent_topic("A2AExpert", "manifest agents").
profile_agent_topic("A2AExpert", "agent interoperability").
profile_agent_topic("A2AExpert", "card-driven workflows").
profile_agent_permission("A2AExpert", "read_file").
profile_agent_permission("A2AExpert", "code_graph").

fallback_agent("SecurityAuditor", "Security analysis is critical for all projects").
fallback_agent_description("SecurityAuditor", "Security vulnerability detection and best practices").
fallback_agent_priority("SecurityAuditor", 90).
fallback_agent_topic("SecurityAuditor", "OWASP top 10").
fallback_agent_topic("SecurityAuditor", "secure coding").
fallback_agent_topic("SecurityAuditor", "vulnerability patterns").
fallback_agent_topic("SecurityAuditor", "code injection").
fallback_agent_permission("SecurityAuditor", "read_file").
fallback_agent_permission("SecurityAuditor", "code_graph").

fallback_agent("TestArchitect", "Test quality directly impacts code reliability").
fallback_agent_description("TestArchitect", "Test strategy, coverage analysis, and TDD patterns").
fallback_agent_priority("TestArchitect", 85).
fallback_agent_topic("TestArchitect", "unit testing").
fallback_agent_topic("TestArchitect", "integration testing").
fallback_agent_topic("TestArchitect", "test coverage").
fallback_agent_topic("TestArchitect", "mocking patterns").
fallback_agent_permission("TestArchitect", "read_file").
fallback_agent_permission("TestArchitect", "exec_cmd").

# --- duplicates and which copy wins ---

# A duplicate is a pair across tools, earlier ord first, different body. Same
# digest is the same text; same tool is two of one corpus, not a duplicate.
#
# Candidates come only through a key the two sources share (normalized name,
# topic), joined by equality, and the pair conditions are checked after. A
# pair relation built first from every two sources is |sources|^2 candidates:
# 3,112 sources on a large repository made 9.7M, kept 0.5%, and dominated
# orientation's evaluation (2026-09-29).
name_duplicate(A, B) :-
    agent_source_norm(A, Norm), Norm != "",
    agent_source_norm(B, Norm),
    agent_source_ord(A, OrdA), agent_source_ord(B, OrdB), OrdA < OrdB,
    agent_source(A, ToolA, _, _, _, _), agent_source(B, ToolB, _, _, _, _), ToolA != ToolB,
    agent_source_digest(A, DigA), agent_source_digest(B, DigB), DigA != DigB.

shared_topic(A, B, Topic) :-
    agent_source_topic(A, Topic),
    agent_source_topic(B, Topic),
    agent_source_ord(A, OrdA), agent_source_ord(B, OrdB), OrdA < OrdB,
    agent_source(A, ToolA, _, _, _, _), agent_source(B, ToolB, _, _, _, _), ToolA != ToolB,
    agent_source_digest(A, DigA), agent_source_digest(B, DigB), DigA != DigB.

shared_topic_count(A, B, N) :-
    shared_topic(A, B, Topic)
    |> do fn:group_by(A, B), let N = fn:count().

topic_duplicate(A, B) :-
    shared_topic_count(A, B, N),
    config_param(/orient_topic_overlap_min, Min),
    N >= Min.

duplicate_edge(A, B) :- name_duplicate(A, B).
duplicate_edge(A, B) :- topic_duplicate(A, B).

agent_source_duplicate(A, B) :- duplicate_edge(A, B).

link(A, B) :- duplicate_edge(A, B).
link(A, B) :- duplicate_edge(B, A).

peer(A, B) :- link(A, B).
peer(A, C) :- link(A, B), peer(B, C), A != C.

in_duplicate(ID) :- peer(ID, Other).

in_group(ID, ID) :- in_duplicate(ID).
in_group(ID, Other) :- peer(ID, Other).

# Recency counts only when both copies have history. A missing row is not
# an ancient file.
source_last_unix(ID, LastUnix) :-
    agent_source(ID, _, _, _, Path, _),
    repo_file_history(Path, _, LastUnix, _, _).

has_history(ID) :- source_last_unix(ID, LastUnix).

recency_beats(A, B) :-
    peer(A, B),
    source_last_unix(A, LastA),
    source_last_unix(B, LastB),
    LastA > LastB.

recency_separates(A, B) :- recency_beats(A, B).
recency_separates(A, B) :- recency_beats(B, A).

tracked_yes(ID) :- agent_source(ID, _, _, _, _, /yes).

tracked_beats(A, B) :-
    peer(A, B),
    tracked_yes(A),
    !tracked_yes(B).

tracked_separates(A, B) :- tracked_beats(A, B).
tracked_separates(A, B) :- tracked_beats(B, A).

size_beats(A, B) :-
    peer(A, B),
    agent_source_bytes(A, SizeA),
    agent_source_bytes(B, SizeB),
    SizeA > SizeB.

size_separates(A, B) :- size_beats(A, B).
size_separates(A, B) :- size_beats(B, A).

agent_source_topic_count(ID, N) :-
    agent_source_topic(ID, Topic)
    |> do fn:group_by(ID), let N = fn:count().

has_topic_count(ID) :- agent_source_topic_count(ID, N).

spec_beats(A, B) :-
    peer(A, B),
    agent_source_topic_count(A, CountA),
    agent_source_topic_count(B, CountB),
    CountA > CountB.

spec_beats(A, B) :-
    peer(A, B),
    has_topic_count(A),
    !has_topic_count(B).

spec_separates(A, B) :- spec_beats(A, B).
spec_separates(A, B) :- spec_beats(B, A).

ref_count(ID, N) :-
    agent_source_refers(FromID, ID)
    |> do fn:group_by(ID), let N = fn:count().

has_ref_count(ID) :- ref_count(ID, N).

ref_beats(A, B) :-
    peer(A, B),
    ref_count(A, CountA),
    ref_count(B, CountB),
    CountA > CountB.

ref_beats(A, B) :-
    peer(A, B),
    has_ref_count(A),
    !has_ref_count(B).

ref_separates(A, B) :- ref_beats(A, B).
ref_separates(A, B) :- ref_beats(B, A).

id_beats(A, B) :-
    peer(A, B),
    agent_source_ord(A, OrdA),
    agent_source_ord(B, OrdB),
    OrdA < OrdB.

# Each why fires only when every stronger criterion is silent, so a pair has
# one direction.
why_recent(A, B) :- recency_beats(A, B).

why_tracked(A, B) :-
    tracked_beats(A, B),
    !recency_separates(A, B).

why_size(A, B) :-
    size_beats(A, B),
    !recency_separates(A, B),
    !tracked_separates(A, B).

why_specific(A, B) :-
    spec_beats(A, B),
    !recency_separates(A, B),
    !tracked_separates(A, B),
    !size_separates(A, B).

why_referenced(A, B) :-
    ref_beats(A, B),
    !recency_separates(A, B),
    !tracked_separates(A, B),
    !size_separates(A, B),
    !spec_separates(A, B).

why_id(A, B) :-
    id_beats(A, B),
    !recency_separates(A, B),
    !tracked_separates(A, B),
    !size_separates(A, B),
    !spec_separates(A, B),
    !ref_separates(A, B).

why_recent_any(ID) :- why_recent(ID, Other).
why_tracked_any(ID) :- why_tracked(ID, Other).
why_size_any(ID) :- why_size(ID, Other).
why_specific_any(ID) :- why_specific(ID, Other).
why_referenced_any(ID) :- why_referenced(ID, Other).
why_id_any(ID) :- why_id(ID, Other).

better(A, B) :- why_recent(A, B).
better(A, B) :- why_tracked(A, B).
better(A, B) :- why_size(A, B).
better(A, B) :- why_specific(A, B).
better(A, B) :- why_referenced(A, B).
better(A, B) :- why_id(A, B).

dominated(ID) :- better(Other, ID).

undominated(ID) :-
    in_duplicate(ID),
    !dominated(ID).

# One Why: the strongest criterion on which the undominated copy beats anyone.
agent_source_winner(ID, /more_recent) :-
    undominated(ID),
    why_recent_any(ID).

agent_source_winner(ID, /tracked) :-
    undominated(ID),
    !why_recent_any(ID),
    why_tracked_any(ID).

agent_source_winner(ID, /larger_body) :-
    undominated(ID),
    !why_recent_any(ID),
    !why_tracked_any(ID),
    why_size_any(ID).

agent_source_winner(ID, /more_specific) :-
    undominated(ID),
    !why_recent_any(ID),
    !why_tracked_any(ID),
    !why_size_any(ID),
    why_specific_any(ID).

agent_source_winner(ID, /more_referenced) :-
    undominated(ID),
    !why_recent_any(ID),
    !why_tracked_any(ID),
    !why_size_any(ID),
    !why_specific_any(ID),
    why_referenced_any(ID).

agent_source_winner(ID, /stable_id) :-
    undominated(ID),
    !why_recent_any(ID),
    !why_tracked_any(ID),
    !why_size_any(ID),
    !why_specific_any(ID),
    !why_referenced_any(ID),
    why_id_any(ID).

# A component whose beat-graph cycles has no undominated member. The earliest
# id in that component wins, which is a total key, so the component still has
# one winner.
comp_id(ID, MinOrd) :-
    in_group(ID, Other),
    agent_source_ord(Other, Ord)
    |> do fn:group_by(ID), let MinOrd = fn:min(Ord).

comp_has_undominated(MinOrd) :-
    comp_id(ID, MinOrd),
    undominated(ID).

cycle_member(ID) :-
    comp_id(ID, MinOrd),
    !comp_has_undominated(MinOrd).

agent_source_winner(ID, /stable_id) :-
    cycle_member(ID),
    comp_id(ID, Ord),
    agent_source_ord(ID, Ord).

agent_source_winner(ID, /unique) :-
    agent_source(ID, _, _, _, _, _),
    !in_duplicate(ID).

agent_source_winner_any(ID) :- agent_source_winner(ID, Why).

# The loser's Why is the winner's Why: the criterion that decided the group.
agent_source_loser(Loser, Winner, Why) :-
    peer(Loser, Winner),
    agent_source_winner(Winner, Why),
    !agent_source_winner_any(Loser).

# --- which shard agents exist ---

profile_matched(/yes) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, _, _).

orient_agent(Name, Why) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, Name, Why).

orient_agent(Name, Why) :-
    !profile_matched(/yes),
    fallback_agent(Name, Why).

orient_agent(Name, "imported subagent") :-
    agent_source_winner_any(ID),
    agent_source(ID, _, /subagent, Name, _, _),
    Name != "".

orient_agent(Name, "imported mode") :-
    agent_source_winner_any(ID),
    agent_source(ID, _, /mode, Name, _, _),
    Name != "".

winning_skill(ID) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, /skill, _, _, _).

winning_skill_tag(ID, Tag) :-
    winning_skill(ID),
    agent_source_tag(ID, Tag).

skill_tag_count(Tag, N) :-
    winning_skill_tag(ID, Tag)
    |> do fn:group_by(Tag), let N = fn:count().

skill_cluster(Tag) :-
    skill_tag_count(Tag, N),
    config_param(/orient_skill_cluster_min, Min),
    N >= Min.

orient_agent(Tag, "skill cluster") :- skill_cluster(Tag).

orient_agent_topic(Name, Topic) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, Name, _),
    profile_agent_topic(Name, Topic).

orient_agent_topic(Name, Topic) :-
    !profile_matched(/yes),
    fallback_agent(Name, _),
    fallback_agent_topic(Name, Topic).

orient_agent_topic(Name, Topic) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind),
    agent_source_topic(ID, Topic).

orient_agent_topic(Tag, Tag) :- skill_cluster(Tag).

orient_agent_description(Name, Description) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, Name, _),
    profile_agent_description(Name, Description).

orient_agent_description(Name, Description) :-
    !profile_matched(/yes),
    fallback_agent(Name, _),
    fallback_agent_description(Name, Description).

orient_agent_description(Name, Description) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind),
    agent_source_description(ID, Description).

orient_agent_permission(Name, Perm) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, Name, _),
    profile_agent_permission(Name, Perm).

orient_agent_permission(Name, Perm) :-
    !profile_matched(/yes),
    fallback_agent(Name, _),
    fallback_agent_permission(Name, Perm).

has_declared_tool(ID) :- agent_source_declared_tool(ID, Tool).

orient_agent_permission(Name, Perm) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind),
    agent_source_declared_tool(ID, Perm).

orient_agent_permission(Name, Perm) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind),
    !has_declared_tool(ID),
    imported_default_permission(Perm).

orient_agent_permission(Name, Perm) :-
    skill_cluster(Name),
    imported_default_permission(Perm).

orient_agent_priority(Name, Priority) :-
    profile_signal(Kind, Key),
    profile_agent(Kind, Key, Name, _),
    profile_agent_priority(Name, Priority).

orient_agent_priority(Name, Priority) :-
    !profile_matched(/yes),
    fallback_agent(Name, _),
    fallback_agent_priority(Name, Priority).

orient_agent_priority(Name, Priority) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind),
    imported_agent_priority(Priority).

orient_agent_priority(Name, Priority) :-
    skill_cluster(Name),
    cluster_agent_priority(Priority).

# The defining subagent or mode, plus winning skills, memories and rules that
# share a topic with the agent. Instructions and commands are prompt atoms,
# not knowledge rows.
orient_agent_knowledge(Name, ID) :-
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, Name, _, _),
    imported_kind(Kind).

orient_agent_knowledge(Name, ID) :-
    orient_agent_topic(Name, Topic),
    agent_source_winner_any(ID),
    agent_source(ID, _, Kind, _, _, _),
    knowledge_kind(Kind),
    agent_source_topic(ID, Topic).

topic_covered(Name, Topic) :-
    orient_agent_knowledge(Name, ID),
    agent_source_topic(ID, Topic).

orient_research_topic(Name, Topic) :-
    orient_agent_topic(Name, Topic),
    !topic_covered(Name, Topic).
