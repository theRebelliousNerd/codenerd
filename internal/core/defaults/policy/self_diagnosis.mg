# "Debug yourself" is a chain the kernel lays out, not one verb (chat
# self_diagnosis.go): the problems log is projected as evidence
# (self_problems.mg), a read-only shard root-causes every hot problem in the
# background, and its findings return as a turn whose fix the work mode
# admits, proposes, or holds (routing_arbitration.mg).

Decl self_diagnosis_lane() bound [].

self_diagnosis_lane() :-
    route_background(),
    user_intent(/current_intent, _, /self_diagnose, _, _).
