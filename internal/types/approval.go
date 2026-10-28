package types

// HumanApprovalPredicate is the host-asserted consent fact: one row per exact
// approved request (ActionID, ActionType, Target, Payload, Digest), asserted by
// the session approval gate and the MCP remote authority, read by the
// constitution and the Dreamer. One name, so no copy can drift.
const HumanApprovalPredicate = "human_approval_granted"
