# System Session Logic
# Section 21 of Cortex Executive Policy

# Session Planning (Session Planner)

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
