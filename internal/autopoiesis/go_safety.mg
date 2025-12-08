# Ouroboros Go Safety Policy

Decl ast_import(F: /string, P: /string).
Decl ast_call(Fn: /string, Callee: /string).
Decl ast_goroutine_spawn(T: /string, L: /string).
Decl ast_uses_context_cancellation(L: /string).
Decl ast_assignment(V: /string, Val: /string).
Decl allowed_package(P: /string).
Decl violation(V: /string).

violation(P) :-
    ast_import(_, P),
    not allowed_package(P).

violation(Line) :-
    ast_goroutine_spawn(_, Line),
    not ast_uses_context_cancellation(Line).

violation(Func) :-
    ast_call(Func, "panic").
