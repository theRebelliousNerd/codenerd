# Coder Policy - Language Detection & Conventions
# Version: 1.0.0
# Extracted from coder.mg Section 3

# =============================================================================
# SECTION 3: LANGUAGE DETECTION & CONVENTIONS
# =============================================================================
# Detect language and apply language-specific rules.

# -----------------------------------------------------------------------------
# 3.1 Language Detection
# -----------------------------------------------------------------------------
# This table is the only answer to "what language is this file". The session
# executor measures the extension of the file a turn or a planned step is aimed
# at and asks extension_language(Ext, Lang) with it bound; the answer becomes
# the compile's language, which is what selects (and excludes) the atom corpus's
# language-tagged knowledge. A file whose extension has no row keeps the
# project's language. Prose formats have rows so a turn on a document is not
# handed the project's code corpus.

# The extension table: which language a file extension names. A row is the
# whole decision. Readers join it with the extension they hold -- the session
# executor's languageOfFile asks it with the extension bound, the shard-expert
# selection joins its task-owned expert_task_extension, the coder rules below
# join file_extension -- so no reader asserts a shared temporary tuple to ask.
# A detected_language(File, Lang) relation stood in front of it, read in Go
# only by a probe that asserted file_extension, queried and retracted: one
# untagged tuple every reader of a path shared, so a concurrent reader's
# cleanup could remove it mid-question.
extension_language(".go", /go).
extension_language(".py", /python).
extension_language(".ts", /typescript).
extension_language(".tsx", /typescript).
extension_language(".js", /javascript).
extension_language(".jsx", /javascript).
extension_language(".mjs", /javascript).
extension_language(".cjs", /javascript).
extension_language(".rs", /rust).
extension_language(".java", /java).
extension_language(".cs", /csharp).
extension_language(".rb", /ruby).
extension_language(".php", /php).
extension_language(".cpp", /cpp).
extension_language(".cc", /cpp).
extension_language(".c", /c).
extension_language(".kt", /kotlin).
extension_language(".swift", /swift).
extension_language(".mg", /mangle).
extension_language(".gl", /mangle).
extension_language(".sql", /sql).
extension_language(".yaml", /yaml).
extension_language(".yml", /yaml).
extension_language(".json", /json).
extension_language(".md", /markdown).
extension_language(".markdown", /markdown).
extension_language(".rst", /restructuredtext).
extension_language(".adoc", /asciidoc).
extension_language(".txt", /text).
extension_language(".sh", /shell).
extension_language(".ps1", /powershell).

# -----------------------------------------------------------------------------
# 3.2 Language-Specific Conventions
# -----------------------------------------------------------------------------

# Go conventions
language_convention(/go, /error_handling, "return errors, don't panic").
language_convention(/go, /naming, "camelCase for private, PascalCase for exported").
language_convention(/go, /interfaces, "accept interfaces, return concrete types").
language_convention(/go, /context, "first parameter should be ctx context.Context").
language_convention(/go, /defer, "use defer for cleanup").
language_convention(/go, /channels, "close channels from sender side").

# Python conventions
language_convention(/python, /naming, "snake_case for functions and variables").
language_convention(/python, /docstrings, "use docstrings for public functions").
language_convention(/python, /typing, "use type hints for function signatures").
language_convention(/python, /exceptions, "use specific exception types").

# TypeScript conventions
language_convention(/typescript, /typing, "prefer explicit types over any").
language_convention(/typescript, /null, "use strict null checks").
language_convention(/typescript, /interfaces, "prefer interfaces over type aliases for objects").

# Rust conventions
language_convention(/rust, /errors, "use Result<T, E> for fallible operations").
language_convention(/rust, /ownership, "prefer borrowing over cloning").
language_convention(/rust, /lifetimes, "explicit lifetimes only when necessary").

# Java conventions
language_convention(/java, /naming, "PascalCase for classes, camelCase for methods").
language_convention(/java, /exceptions, "use checked exceptions for recoverable conditions").
language_convention(/java, /interfaces, "program to interfaces").

# -----------------------------------------------------------------------------
# 3.3 Convention Application
# -----------------------------------------------------------------------------

