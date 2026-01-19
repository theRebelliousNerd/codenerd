# intent_qualifiers.mg
# =========================================================================
# INTENT QUALIFIERS: Interrogatives, Modals, Copular Patterns, Negation
# =========================================================================
# These predicates enhance intent classification beyond simple verb matching.
# They capture the grammatical structure of user requests to improve routing.

# =========================================================================
# SECTION 1: INTERROGATIVE TAXONOMY
# =========================================================================
# Interrogative words signal the TYPE of information the user seeks.
# Each maps to a semantic type and a default verb for fallback routing.


# --- WHAT: Definition/Explanation ---
interrogative_type("what", /definition, /explain, 80).
interrogative_type("what is", /definition, /explain, 85).
interrogative_type("what's", /definition, /explain, 85).
interrogative_type("what does", /mechanism, /explain, 85).
interrogative_type("what are", /enumeration, /explore, 82).
interrogative_type("what do", /mechanism, /explain, 82).
interrogative_type("what was", /history, /git, 78).
interrogative_type("what were", /history, /git, 78).
interrogative_type("what happened", /history, /debug, 85).
interrogative_type("what went wrong", /causation, /debug, 90).
interrogative_type("what causes", /causation, /debug, 88).
interrogative_type("what caused", /causation, /debug, 88).
interrogative_type("what if", /hypothetical, /dream, 85).

# --- WHY: Causation/Debugging ---
interrogative_type("why", /causation, /debug, 90).
interrogative_type("why is", /causation, /debug, 92).
interrogative_type("why are", /causation, /debug, 90).
interrogative_type("why does", /causation, /debug, 92).
interrogative_type("why do", /causation, /debug, 90).
interrogative_type("why did", /causation, /debug, 92).
interrogative_type("why was", /causation, /debug, 90).
interrogative_type("why were", /causation, /debug, 88).
interrogative_type("why isn't", /causation, /debug, 93).
interrogative_type("why doesn't", /causation, /debug, 93).
interrogative_type("why won't", /causation, /debug, 93).
interrogative_type("why can't", /causation, /debug, 93).

# --- HOW: Mechanism/Process ---
interrogative_type("how", /mechanism, /explain, 85).
interrogative_type("how does", /mechanism, /explain, 88).
interrogative_type("how do", /mechanism, /explain, 86).
interrogative_type("how did", /mechanism, /explain, 85).
interrogative_type("how is", /mechanism, /explain, 85).
interrogative_type("how are", /mechanism, /explain, 83).
interrogative_type("how to", /instruction, /explain, 88).
interrogative_type("how can i", /instruction, /explain, 90).
interrogative_type("how do i", /instruction, /explain, 90).
interrogative_type("how would i", /instruction, /explain, 88).
interrogative_type("how should i", /recommendation, /explain, 88).
interrogative_type("how come", /causation, /debug, 88).
interrogative_type("how many", /quantification, /stats, 85).
interrogative_type("how much", /quantification, /stats, 85).
interrogative_type("how often", /frequency, /analyze, 80).
interrogative_type("how long", /duration, /analyze, 80).

# --- WHERE: Location/Search ---
interrogative_type("where", /location, /search, 85).
interrogative_type("where is", /location, /search, 88).
interrogative_type("where's", /location, /search, 88).
interrogative_type("where are", /location, /search, 86).
interrogative_type("where do", /location, /search, 84).
interrogative_type("where does", /location, /search, 84).
interrogative_type("where can i", /location, /search, 85).
interrogative_type("where should", /recommendation, /search, 82).
interrogative_type("where was", /history, /git, 80).
interrogative_type("where were", /history, /git, 78).

# --- WHEN: Temporal/History ---
interrogative_type("when", /temporal, /git, 75).
interrogative_type("when was", /temporal, /git, 82).
interrogative_type("when were", /temporal, /git, 80).
interrogative_type("when did", /temporal, /git, 82).
interrogative_type("when does", /temporal, /explain, 78).
interrogative_type("when do", /temporal, /explain, 78).
interrogative_type("when is", /temporal, /explain, 76).
interrogative_type("when should", /recommendation, /explain, 80).
interrogative_type("when will", /prediction, /explain, 75).

# --- WHO: Attribution/Authorship ---
interrogative_type("who", /attribution, /git, 75).
interrogative_type("who is", /attribution, /explain, 78).
interrogative_type("who's", /attribution, /explain, 78).
interrogative_type("who are", /attribution, /explain, 76).
interrogative_type("who wrote", /attribution, /git, 88).
interrogative_type("who created", /attribution, /git, 88).
interrogative_type("who made", /attribution, /git, 86).
interrogative_type("who changed", /attribution, /git, 88).
interrogative_type("who modified", /attribution, /git, 88).
interrogative_type("who added", /attribution, /git, 86).
interrogative_type("who deleted", /attribution, /git, 86).
interrogative_type("who removed", /attribution, /git, 86).
interrogative_type("who owns", /ownership, /explain, 82).
interrogative_type("who maintains", /ownership, /explain, 82).

# --- WHOSE: Ownership/Possession ---
interrogative_type("whose", /ownership, /git, 75).
interrogative_type("whose code", /ownership, /git, 82).
interrogative_type("whose file", /ownership, /git, 82).
interrogative_type("whose responsibility", /ownership, /explain, 78).

# --- WHICH: Selection/Enumeration ---
interrogative_type("which", /selection, /explore, 80).
interrogative_type("which is", /selection, /explain, 82).
interrogative_type("which are", /selection, /explore, 80).
interrogative_type("which one", /selection, /explain, 84).
interrogative_type("which ones", /selection, /explore, 82).
interrogative_type("which file", /selection, /search, 86).
interrogative_type("which files", /selection, /search, 86).
interrogative_type("which function", /selection, /search, 86).
interrogative_type("which functions", /selection, /search, 86).
interrogative_type("which class", /selection, /search, 86).
interrogative_type("which method", /selection, /search, 86).
interrogative_type("which should", /recommendation, /explain, 85).
interrogative_type("which would", /recommendation, /explain, 83).

# =========================================================================
# SECTION 2: MODAL VERB TAXONOMY
# =========================================================================
# Modal verbs modify the intent - they can indicate politeness, hypotheticals,
# permissions, obligations, or possibilities.


# --- Polite Request Modals (Strip and process underlying intent) ---
modal_type("can", /polite_request, /strip, 90).
modal_type("can you", /polite_request, /strip, 95).
modal_type("can we", /polite_request, /strip, 93).
modal_type("could", /polite_request, /strip, 90).
modal_type("could you", /polite_request, /strip, 95).
modal_type("could we", /polite_request, /strip, 93).
modal_type("would you", /polite_request, /strip, 95).
modal_type("will you", /polite_request, /strip, 93).
modal_type("would you mind", /polite_request, /strip, 96).
modal_type("do you mind", /polite_request, /strip, 94).
modal_type("please", /polite_request, /strip, 92).
modal_type("please can you", /polite_request, /strip, 96).
modal_type("i need you to", /polite_request, /strip, 94).
modal_type("i want you to", /polite_request, /strip, 93).
modal_type("i'd like you to", /polite_request, /strip, 94).
modal_type("help me", /polite_request, /strip, 90).
modal_type("help me to", /polite_request, /strip, 91).

# --- Hypothetical/Conditional Modals (May trigger dream/shadow mode) ---
modal_type("would", /hypothetical, /dream, 80).
modal_type("what if", /hypothetical, /dream, 88).
modal_type("what would happen", /hypothetical, /dream, 90).
modal_type("imagine", /hypothetical, /dream, 85).
modal_type("suppose", /hypothetical, /dream, 85).
modal_type("hypothetically", /hypothetical, /dream, 92).
modal_type("in theory", /hypothetical, /dream, 82).
modal_type("theoretically", /hypothetical, /dream, 85).
modal_type("if i were to", /hypothetical, /dream, 88).
modal_type("if we were to", /hypothetical, /dream, 88).
modal_type("let's say", /hypothetical, /dream, 85).
modal_type("assuming", /hypothetical, /dream, 82).

# --- Possibility/Uncertainty Modals ---
modal_type("may", /possibility, /assess, 75).
modal_type("might", /possibility, /assess, 75).
modal_type("maybe", /possibility, /assess, 70).
modal_type("perhaps", /possibility, /assess, 70).
modal_type("possibly", /possibility, /assess, 72).
modal_type("potentially", /possibility, /assess, 74).
modal_type("is it possible", /possibility, /assess, 82).
modal_type("would it be possible", /possibility, /assess, 84).

# --- Recommendation/Advice Modals ---
modal_type("should", /recommendation, /advise, 85).
modal_type("should i", /recommendation, /advise, 88).
modal_type("should we", /recommendation, /advise, 86).
modal_type("ought to", /recommendation, /advise, 82).
modal_type("is it better to", /recommendation, /advise, 85).
modal_type("would it be better", /recommendation, /advise, 84).
modal_type("what's the best way", /recommendation, /advise, 88).
modal_type("what is the best way", /recommendation, /advise, 88).
modal_type("best practice", /recommendation, /advise, 85).
modal_type("recommended", /recommendation, /advise, 82).

# --- Obligation/Requirement Modals ---
modal_type("must", /obligation, /enforce, 90).
modal_type("have to", /obligation, /enforce, 88).
modal_type("need to", /obligation, /enforce, 86).
modal_type("required to", /obligation, /enforce, 88).
modal_type("necessary", /obligation, /enforce, 82).
modal_type("mandatory", /obligation, /enforce, 85).

# --- Capability/Ability Modals ---
modal_type("able to", /capability, /assess, 78).
modal_type("capable of", /capability, /assess, 78).
modal_type("is it able", /capability, /assess, 80).
modal_type("can it", /capability, /assess, 82).
modal_type("does it support", /capability, /assess, 84).

# =========================================================================
# SECTION 3: COPULAR (IS/ARE) + STATE PATTERNS
# =========================================================================
# Copular verbs (is, are, was, were) followed by adjectives indicate
# state queries - the user wants to know the current state of something.


# --- Security States ---
state_adjective("safe", /security, /security_state, 90).
state_adjective("secure", /security, /security_state, 92).
state_adjective("unsafe", /security, /security_state, 92).
state_adjective("insecure", /security, /security_state, 92).
state_adjective("vulnerable", /security, /security_state, 95).
state_adjective("protected", /security, /security_state, 85).
state_adjective("exposed", /security, /security_state, 90).
state_adjective("hardened", /security, /security_state, 82).
state_adjective("sanitized", /security, /security_state, 85).

# --- Test/Quality States ---
state_adjective("tested", /test, /test_state, 88).
state_adjective("untested", /test, /test_state, 88).
state_adjective("passing", /test, /test_state, 92).
state_adjective("failing", /debug, /test_state, 95).
state_adjective("broken", /debug, /error_state, 95).
state_adjective("working", /test, /test_state, 85).
state_adjective("green", /test, /test_state, 88).
state_adjective("red", /debug, /test_state, 88).
state_adjective("flaky", /test, /test_state, 85).

# --- Code Quality States ---
state_adjective("clean", /review, /quality_state, 82).
state_adjective("dirty", /review, /quality_state, 82).
state_adjective("messy", /review, /quality_state, 82).
state_adjective("readable", /review, /quality_state, 80).
state_adjective("unreadable", /review, /quality_state, 82).
state_adjective("maintainable", /review, /quality_state, 80).
state_adjective("unmaintainable", /review, /quality_state, 82).
state_adjective("documented", /review, /quality_state, 78).
state_adjective("undocumented", /review, /quality_state, 80).
state_adjective("commented", /review, /quality_state, 75).
state_adjective("uncommented", /review, /quality_state, 77).

# --- Performance States ---
state_adjective("optimized", /analyze, /performance_state, 85).
state_adjective("unoptimized", /analyze, /performance_state, 85).
state_adjective("efficient", /analyze, /performance_state, 82).
state_adjective("inefficient", /analyze, /performance_state, 84).
state_adjective("slow", /analyze, /performance_state, 88).
state_adjective("fast", /analyze, /performance_state, 80).
state_adjective("performant", /analyze, /performance_state, 82).
state_adjective("bloated", /analyze, /performance_state, 80).

# --- Lifecycle States ---
state_adjective("deprecated", /analyze, /lifecycle_state, 88).
state_adjective("obsolete", /analyze, /lifecycle_state, 88).
state_adjective("outdated", /analyze, /lifecycle_state, 86).
state_adjective("current", /analyze, /lifecycle_state, 75).
state_adjective("up-to-date", /analyze, /lifecycle_state, 78).
state_adjective("legacy", /analyze, /lifecycle_state, 82).
state_adjective("modern", /analyze, /lifecycle_state, 75).
state_adjective("stale", /analyze, /lifecycle_state, 82).

# --- Correctness States ---
state_adjective("correct", /review, /correctness_state, 85).
state_adjective("incorrect", /debug, /correctness_state, 88).
state_adjective("wrong", /debug, /correctness_state, 90).
state_adjective("right", /review, /correctness_state, 80).
state_adjective("valid", /review, /correctness_state, 82).
state_adjective("invalid", /debug, /correctness_state, 88).
state_adjective("buggy", /debug, /error_state, 92).
state_adjective("faulty", /debug, /error_state, 90).

# --- Completion States ---
state_adjective("complete", /review, /completion_state, 80).
state_adjective("incomplete", /review, /completion_state, 82).
state_adjective("finished", /review, /completion_state, 78).
state_adjective("unfinished", /review, /completion_state, 80).
state_adjective("done", /review, /completion_state, 75).
state_adjective("ready", /review, /completion_state, 78).
state_adjective("implemented", /review, /completion_state, 80).
state_adjective("unimplemented", /create, /completion_state, 85).
state_adjective("missing", /create, /completion_state, 88).

# --- Existence States (for "is there" patterns) ---
state_adjective("any", /search, /existence_state, 80).
state_adjective("existing", /search, /existence_state, 78).
state_adjective("available", /search, /existence_state, 80).
state_adjective("present", /search, /existence_state, 78).
state_adjective("absent", /search, /existence_state, 80).

# =========================================================================
# SECTION 4: NEGATION MARKERS
# =========================================================================
# Negation changes the intent - "don't delete" is NOT a delete intent.


# --- Direct Negation ---
negation_marker("don't", /prohibition, 95).
negation_marker("do not", /prohibition, 95).
negation_marker("dont", /prohibition, 94).
negation_marker("doesn't", /prohibition, 93).
negation_marker("does not", /prohibition, 93).
negation_marker("didn't", /prohibition, 92).
negation_marker("did not", /prohibition, 92).
negation_marker("won't", /prohibition, 93).
negation_marker("will not", /prohibition, 93).
negation_marker("wouldn't", /prohibition, 90).
negation_marker("would not", /prohibition, 90).
negation_marker("can't", /prohibition, 93).
negation_marker("cannot", /prohibition, 93).
negation_marker("can not", /prohibition, 93).
negation_marker("couldn't", /prohibition, 90).
negation_marker("could not", /prohibition, 90).
negation_marker("shouldn't", /advice_against, 88).
negation_marker("should not", /advice_against, 88).
negation_marker("mustn't", /prohibition, 92).
negation_marker("must not", /prohibition, 92).

# --- Imperative Negation ---
negation_marker("never", /prohibition, 95).
negation_marker("avoid", /avoidance, 88).
negation_marker("stop", /cessation, 90).
negation_marker("quit", /cessation, 88).
negation_marker("cease", /cessation, 85).
negation_marker("halt", /cessation, 85).
negation_marker("prevent", /prevention, 85).
negation_marker("block", /prevention, 82).

# --- Exclusion Negation ---
negation_marker("not", /exclusion, 85).
negation_marker("no", /exclusion, 82).
negation_marker("none", /exclusion, 80).
negation_marker("nothing", /exclusion, 80).
negation_marker("without", /exclusion, 82).
negation_marker("except", /exclusion, 80).
negation_marker("exclude", /exclusion, 82).
negation_marker("excluding", /exclusion, 82).
negation_marker("but not", /exclusion, 88).
negation_marker("other than", /exclusion, 78).

# --- Reversal/Undo ---
negation_marker("undo", /reversal, 90).
negation_marker("revert", /reversal, 92).
negation_marker("rollback", /reversal, 90).
negation_marker("restore", /reversal, 88).
negation_marker("undelete", /reversal, 85).
negation_marker("recover", /reversal, 85).

# =========================================================================
# SECTION 5: COPULAR VERB PATTERNS
# =========================================================================
# Patterns for copular verbs that introduce state queries.


copular_verb("is", /present, /singular).
copular_verb("are", /present, /plural).
copular_verb("was", /past, /singular).
copular_verb("were", /past, /plural).
copular_verb("be", /infinitive, /neutral).
copular_verb("been", /perfect, /neutral).
copular_verb("being", /progressive, /neutral).
copular_verb("isn't", /present_neg, /singular).
copular_verb("aren't", /present_neg, /plural).
copular_verb("wasn't", /past_neg, /singular).
copular_verb("weren't", /past_neg, /plural).

# =========================================================================
# SECTION 6: EXISTENCE QUERY PATTERNS
# =========================================================================
# "Is there a...", "Are there any...", "Do we have..."


existence_pattern("is there", /existence, /search, 88).
existence_pattern("is there a", /existence, /search, 90).
existence_pattern("is there any", /existence, /search, 88).
existence_pattern("are there", /existence, /search, 86).
existence_pattern("are there any", /existence, /search, 88).
existence_pattern("do we have", /existence, /search, 85).
existence_pattern("do we have a", /existence, /search, 87).
existence_pattern("do we have any", /existence, /search, 87).
existence_pattern("does it have", /existence, /search, 82).
existence_pattern("does this have", /existence, /search, 82).
existence_pattern("have we got", /existence, /search, 80).
existence_pattern("is there already", /existence, /search, 85).
existence_pattern("already have", /existence, /search, 82).
existence_pattern("already exists", /existence, /search, 85).

# =========================================================================
# SECTION 7: COMPARATIVE/EVALUATIVE PATTERNS
# =========================================================================
# "Is X better than Y?", "Which is faster?"


comparative_marker("better", /superiority, 85).
comparative_marker("worse", /inferiority, 85).
comparative_marker("faster", /performance, 88).
comparative_marker("slower", /performance, 88).
comparative_marker("simpler", /complexity, 82).
comparative_marker("more complex", /complexity, 84).
comparative_marker("cleaner", /quality, 82).
comparative_marker("messier", /quality, 82).
comparative_marker("safer", /security, 88).
comparative_marker("more secure", /security, 88).
comparative_marker("less secure", /security, 90).
comparative_marker("more efficient", /performance, 85).
comparative_marker("less efficient", /performance, 85).
comparative_marker("prefer", /preference, 80).
comparative_marker("preferred", /preference, 80).
comparative_marker("vs", /comparison, 85).
comparative_marker("versus", /comparison, 85).
comparative_marker("compared to", /comparison, 85).
comparative_marker("rather than", /preference, 82).
comparative_marker("instead of", /preference, 82).
comparative_marker("or", /alternative, 70).

# =========================================================================
# SECTION 8: INTENT SIGNAL COMBINATIONS
# =========================================================================
# Rules for combining signals to derive stronger intent classification.

# Interrogative + State Adjective combinations

# "Why is this failing?" → debug (causation + error_state)
interrogative_state_signal(/causation, /error_state, /debug, 98).
interrogative_state_signal(/causation, /test_state, /debug, 96).

# "How is this secure?" → security (mechanism + security_state)
interrogative_state_signal(/mechanism, /security_state, /security, 92).

# "What is broken?" → debug (definition + error_state)
interrogative_state_signal(/definition, /error_state, /debug, 94).

# "Where is this tested?" → test (location + test_state)
interrogative_state_signal(/location, /test_state, /test, 88).

# "Which is faster?" → analyze (selection + performance)
interrogative_state_signal(/selection, /performance_state, /analyze, 90).

# "Is this complete?" → review (existence + completion_state)
interrogative_state_signal(/existence, /completion_state, /review, 85).

# Modal + Verb combinations (for stripping)

# Polite request + mutation → still mutation
modal_verb_signal(/polite_request, /mutation, /mutation).
modal_verb_signal(/polite_request, /query, /query).

# Hypothetical + mutation → dream mode
modal_verb_signal(/hypothetical, /mutation, /dream_mutation).
modal_verb_signal(/hypothetical, /query, /dream_query).

# Recommendation + any → advisory response
modal_verb_signal(/recommendation, /mutation, /advisory).
modal_verb_signal(/recommendation, /query, /advisory).
