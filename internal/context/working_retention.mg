# Retention of working-context archives (.nerd/context/<scope digest>.db).
#
# An archive holds one working scope's observations so recall_context can
# serve them after they leave the window. The scope is named at random and held
# only in the memory of the executor that minted it, so once that executor is
# gone nothing can redeem a handle into its archive. What keeps an archive is
# that something can still read it -- never its age. Loaded into a private
# evaluation scope (working_retention.go); nothing here can authorize an action.
#
# Asserted by the survey, one row per archive on disk:
#   working_archive(A)               A is an archive (its digest).
#   working_archive_owner_live(A)    the process that minted A runs on this host.
#   working_archive_owner_foreign(A) A was minted on another host, whose
#                                    processes this host cannot see.
#   working_archive_retired(A)       its executor said its scope is over (a task
#                                    clone or a subagent finished its task).
#
# An archive with no owner record was minted before ownership was recorded;
# no process that minted one can be asked, so it has no owner fact and is
# prunable. When a scope can be resumed by a later run (audit N13), that
# resumption is one more reason for working_archive_redeemable.
Decl working_archive(Archive) bound [/string].
Decl working_archive_owner_live(Archive) bound [/string].
Decl working_archive_owner_foreign(Archive) bound [/string].
Decl working_archive_retired(Archive) bound [/string].
Decl working_archive_redeemable(Archive) descr [doc("Something can still redeem a handle into this archive: its owner is alive, or lives where this host cannot look, and has not retired the scope.")].
Decl working_archive_prunable(Archive) descr [doc("Nothing can redeem a handle into this archive any more; its files may be deleted.")].

working_archive_redeemable(A) :-
    working_archive(A),
    working_archive_owner_live(A),
    !working_archive_retired(A).

working_archive_redeemable(A) :-
    working_archive(A),
    working_archive_owner_foreign(A),
    !working_archive_retired(A).

working_archive_prunable(A) :-
    working_archive(A),
    !working_archive_redeemable(A).
