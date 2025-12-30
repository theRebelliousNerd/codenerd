# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: CODEDOM
# Sections: 34

# =============================================================================
# SECTION 34: CODE DOM (Interactive Code Elements)
# =============================================================================
# Analogous to Browser DOM, Code DOM projects code into semantic chunks
# (functions, structs, interfaces) with stable refs for querying and editing.
# Uses 1-hop dependency scope: active file + imports + files that import it.

# -----------------------------------------------------------------------------
# 34.1 File Scope Management
# -----------------------------------------------------------------------------

# active_file(Path) - the primary file being worked on
Decl active_file(Path).

# file_in_scope(Path, Hash, Language, LineCount) - files in current scope
# Language: /go, /python, /ts, /rust
Decl file_in_scope(Path, Hash, Language, LineCount).

# -----------------------------------------------------------------------------
# 34.2 Code Elements (Semantic Chunks)
# -----------------------------------------------------------------------------

# code_element(Ref, ElemType, File, StartLine, EndLine)
# Ref: stable reference like "fn:context.Compressor.Compress"
# ElemType: /function, /method, /struct, /interface, /type, /const, /var
Decl code_element(Ref, ElemType, File, StartLine, EndLine).

# element_signature(Ref, Signature) - declaration line
Decl element_signature(Ref, Signature).

# element_body(Ref, BodyText) - full text for display/editing
Decl element_body(Ref, BodyText).

# element_parent(Ref, ParentRef) - containment (method -> struct)
Decl element_parent(Ref, ParentRef).

# element_visibility(Ref, Visibility) - /public, /private
Decl element_visibility(Ref, Visibility).

# code_interactable(Ref, ActionType) - available actions per element
# ActionType: /view, /replace, /insert_before, /insert_after, /delete
Decl code_interactable(Ref, ActionType).

# -----------------------------------------------------------------------------
# 34.3 Edit Tracking
# -----------------------------------------------------------------------------

# element_modified(Ref, SessionID, Timestamp) - tracks element changes
Decl element_modified(Ref, SessionID, Timestamp).

# lines_edited(File, StartLine, EndLine, SessionID) - line-level tracking
Decl lines_edited(File, StartLine, EndLine, SessionID).

# lines_inserted(File, AfterLine, LineCount, SessionID) - insertions
Decl lines_inserted(File, AfterLine, LineCount, SessionID).

# lines_deleted(File, StartLine, EndLine, SessionID) - deletions
Decl lines_deleted(File, StartLine, EndLine, SessionID).

# file_read(Path, SessionID, Timestamp) - file access tracking
Decl file_read(Path, SessionID, Timestamp).

# file_written(Path, Hash, SessionID, Timestamp) - file write tracking
Decl file_written(Path, Hash, SessionID, Timestamp).

# -----------------------------------------------------------------------------
# 34.4 Code DOM Derived Predicates
# -----------------------------------------------------------------------------

# in_scope(File) - derived: file is in current scope
Decl in_scope(File).

# editable(Ref) - derived: element can be edited
Decl editable(Ref).

# function_in_scope(Ref, File, Sig) - derived: functions in scope
Decl function_in_scope(Ref, File, Sig).

# method_of(MethodRef, StructRef) - derived: method belongs to struct
Decl method_of(MethodRef, StructRef).

# code_contains(Parent, Child) - derived: transitive containment
Decl code_contains(Parent, Child).

# safe_to_modify(Ref) - derived: has tests, builds pass
Decl safe_to_modify(Ref).

# requires_campaign(Intent) - derived: complex refactor needs campaign
Decl requires_campaign(Intent).

# code_edit_outcome(Ref, EditType, Success, Timestamp) - edit result tracking
Decl code_edit_outcome(Ref, EditType, Success, Timestamp).

# proven_safe_edit(Ref, EditType) - derived: edit pattern is safe
Decl proven_safe_edit(Ref, EditType).

# method_in_scope(Ref, File, Sig) - derived: methods in scope
Decl method_in_scope(Ref, File, Sig).

# scope_refreshed(File) - helper: file scope has been refreshed
Decl scope_refreshed(File).

# successful_edit(Ref, EditType) - derived: edit succeeded
Decl successful_edit(Ref, EditType).

# failed_edit(Ref, EditType) - derived: edit failed
Decl failed_edit(Ref, EditType).

# element_count_high() - helper: many elements in scope (triggers campaign for complex refactors)
Decl element_count_high().

# -----------------------------------------------------------------------------
# 34.5 Error Handling & Edge Cases
# -----------------------------------------------------------------------------

# scope_open_failed(Path, Error) - file scope open failed
Decl scope_open_failed(Path, Error).

# scope_closed() - current scope was closed
Decl scope_closed().

# parse_error(File, Error, Timestamp) - Go AST parsing failed
Decl parse_error(File, Error, Timestamp).

# file_not_found(Path, Timestamp) - requested file doesn't exist
Decl file_not_found(Path, Timestamp).

# file_hash_mismatch(Path, ExpectedHash, ActualHash) - concurrent modification detected
Decl file_hash_mismatch(Path, ExpectedHash, ActualHash).

# element_stale(Ref, Reason) - element ref may be outdated
Decl element_stale(Ref, Reason).

# scope_refresh_failed(Path, Error) - re-parsing failed after edit
Decl scope_refresh_failed(Path, Error).

# encoding_issue(Path, IssueType) - file encoding problem detected
# IssueType: /bom_detected, /crlf_inconsistent, /non_utf8
Decl encoding_issue(Path, IssueType).

# large_file_warning(Path, LineCount, ByteSize) - file exceeds size thresholds
Decl large_file_warning(Path, LineCount, ByteSize).

# -----------------------------------------------------------------------------
# 34.6 Operation Tracking
# -----------------------------------------------------------------------------

# scope_operation(OpType, Path, Success, Timestamp) - scope operation audit
# OpType: /open, /refresh, /close
Decl scope_operation(OpType, Path, Success, Timestamp).

# edit_operation_event(OpType, Path, StartLine, EndLine, Success, Timestamp)
# OpType: /edit_lines, /insert_lines, /delete_lines, /replace_element
Decl edit_operation_event(OpType, Path, StartLine, EndLine, Success, Timestamp).

# undo_available(Path, OperationID) - undo is available for an operation
Decl undo_available(Path, OperationID).

# -----------------------------------------------------------------------------
# 34.7 Derived Predicates for Edge Cases
# -----------------------------------------------------------------------------

# file_modified_externally(Path) - derived: file changed outside of scope
Decl file_modified_externally(Path).

# needs_scope_refresh() - derived: scope is stale and needs refresh
Decl needs_scope_refresh().

# element_edit_blocked(Ref, Reason) - derived: edit is blocked
Decl element_edit_blocked(Ref, Reason).

# -----------------------------------------------------------------------------
# 34.8 Code Pattern Detection
# -----------------------------------------------------------------------------

# generated_code(File, Generator, Marker) - file is auto-generated
# Generator: /protobuf, /openapi, /swagger, /grpc, /wire, /ent, /sqlc, /gqlgen
# Marker: the comment/directive that indicates generation
Decl generated_code(File, Generator, Marker).

# api_client_function(Ref, Endpoint, Method) - function makes HTTP calls
# Method: /GET, /POST, /PUT, /DELETE, /PATCH
Decl api_client_function(Ref, Endpoint, Method).

# api_handler_function(Ref, Route, Method) - function handles HTTP requests
Decl api_handler_function(Ref, Route, Method).

# has_external_callers(Ref) - derived: function is called from outside package
Decl has_external_callers(Ref).

# breaking_change_risk(Ref, RiskLevel, Reason) - edit may break callers
# RiskLevel: /low, /medium, /high, /critical
Decl breaking_change_risk(Ref, RiskLevel, Reason).

# mock_file(TestFile, SourceFile) - test file mocks source file
Decl mock_file(TestFile, SourceFile).

# interface_impl(StructRef, InterfaceRef) - struct implements interface
Decl interface_impl(StructRef, InterfaceRef).

# cgo_code(File) - file contains CGo directives
Decl cgo_code(File).

# build_tag(File, Tag) - file has build constraints
Decl build_tag(File, Tag).

# embed_directive(File, EmbedPath) - file has go:embed
Decl embed_directive(File, EmbedPath).

# -----------------------------------------------------------------------------
# 34.9 Edit Safety Derived Predicates
# -----------------------------------------------------------------------------

# edit_unsafe(Ref, Reason) - derived: editing this element is risky
Decl edit_unsafe(Ref, Reason).

# suggest_update_mocks(Ref) - derived: mocks may need updating after edit
Decl suggest_update_mocks(Ref).

# signature_change_detected(Ref, OldSig, NewSig) - function signature changed
Decl signature_change_detected(Ref, OldSig, NewSig).

# requires_integration_test(Ref) - derived: API client needs integration test
Decl requires_integration_test(Ref).

# requires_contract_check(Ref) - derived: API handler contract validation needed
Decl requires_contract_check(Ref).

# api_edit_warning(Ref, Reason) - derived: warning when editing API code
Decl api_edit_warning(Ref, Reason).

