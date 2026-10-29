# Measurements are replaced at each observation; no wall-clock expiry.
Decl oriented_head(Head) bound [/string].
Decl current_repo_head(Head) bound [/string].
Decl oriented_document(Path, Digest) bound [/string, /string].
Decl current_document(Path, Digest) bound [/string, /string].
Decl orient_document_present(Path) bound [/string].
Decl orient_document_known(Path) bound [/string].
Decl orient_stale(Why) bound [/name].
Decl orient_refresh_document(Path, Why) bound [/string, /name].
Decl orient_role_pending(Path) bound [/string].

orient_document_present(Path) :- current_document(Path, Digest).
orient_document_known(Path) :- oriented_document(Path, Digest).
orient_stale(/head_changed) :- oriented_head(Old), current_repo_head(New), Old != New.
orient_refresh_document(Path, /content_changed) :-
    oriented_document(Path, Old), current_document(Path, New), Old != New.
orient_refresh_document(Path, /added) :-
    current_document(Path, Digest), !orient_document_known(Path).
orient_refresh_document(Path, /deleted) :-
    oriented_document(Path, Digest), !orient_document_present(Path).
orient_stale(/documents_changed) :- orient_refresh_document(Path, Why).
