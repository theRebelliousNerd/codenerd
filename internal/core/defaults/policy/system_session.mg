# Session Planning (Session Planner)
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl agenda_item_ready(ItemID).
# Decl has_incomplete_dependency(ItemID).
# Decl agenda_item(ItemID, Description, Priority, Status, Timestamp).
# Decl agenda_dependency(ItemID, DepID).
# Decl next_agenda_item(ItemID).
# Decl has_higher_priority_item(ItemID).
# Decl priority_higher(PriorityA, PriorityB).
# Decl checkpoint_due().
# Decl checkpoint_needed().
# Decl next_action(Action).
# Decl agenda_item_escalate(ItemID, Reason).
# Decl item_retry_count(ItemID, Count).
# Decl escalation_needed(Target, Subject, Reason).

# Agenda item is ready when dependencies complete
agenda_item_ready(ItemID) :-
    agenda_item(ItemID, _, _, /pending, _),
    !has_incomplete_dependency(ItemID).

# Helper for dependency checking
has_incomplete_dependency(ItemID) :-
    agenda_dependency(ItemID, DepID),
    agenda_item(DepID, _, _, Status, _),
    /completed != Status.

# Next agenda item: highest priority ready item
next_agenda_item(ItemID) :-
    agenda_item_ready(ItemID),
    !has_higher_priority_item(ItemID).

# Helper for priority ordering
has_higher_priority_item(ItemID) :-
    agenda_item(ItemID, _, Priority, _, _),
    agenda_item_ready(OtherID),
    OtherID != ItemID,
    agenda_item(OtherID, _, OtherPriority, _, _),
    priority_higher(OtherPriority, Priority).

# Checkpoint needed based on time or completion (10 minutes = 600 seconds)
checkpoint_due() :-
    checkpoint_needed().

next_action(/create_checkpoint) :-
    checkpoint_due().

# Blocked item triggers escalation after retries
agenda_item_escalate(ItemID, "max_retries_exceeded") :-
    agenda_item(ItemID, _, _, /blocked, _),
    item_retry_count(ItemID, Count),
    Count >= 3.

escalation_needed(/session_planner, ItemID, Reason) :-
    agenda_item_escalate(ItemID, Reason).
