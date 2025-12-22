# System Shard Coordination - Session Logic
# Domain: Session Management and Planning

Decl agenda_item_ready(ItemID.Type<string>).
Decl agenda_item(ItemID.Type<string>, Desc.Type<string>, Priority.Type<atom>, Status.Type<atom>, Meta.Type<string>).
Decl has_incomplete_dependency(ItemID.Type<string>).
Decl agenda_dependency(ItemID.Type<string>, DepID.Type<string>).
Decl next_agenda_item(ItemID.Type<string>).
Decl has_higher_priority_item(ItemID.Type<string>).
Decl priority_higher(High.Type<atom>, Low.Type<atom>).
Decl checkpoint_due().
Decl checkpoint_needed().
Decl next_action(Action.Type<atom>).
Decl agenda_item_escalate(ItemID.Type<string>, Reason.Type<string>).
Decl item_retry_count(ItemID.Type<string>, Count.Type<int64>).
Decl escalation_needed(Domain.Type<atom>, Entity.Type<string>, Reason.Type<string>).
Decl activate_shard(ShardName.Type<atom>).
Decl current_campaign(CampaignID.Type<string>).
Decl system_shard_healthy(ShardName.Type<atom>).
Decl user_intent(IntentID.Type<atom>, Goal.Type<string>, Verb.Type<atom>, Target.Type<string>, Args.Type<string>).

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

# Activate session_planner for campaigns or complex goals
activate_shard(/session_planner) :-
    current_campaign(_),
    !system_shard_healthy(/session_planner).

activate_shard(/session_planner) :-
    user_intent(/current_intent, _, /plan, _, _),
    !system_shard_healthy(/session_planner).
