# internal/ux - User Experience Management

This package provides user experience management for codeNERD, including user journey tracking, progressive disclosure, and contextual help.

**Related Packages:**
- [internal/config](../config/CLAUDE.md) - GuidanceLevel and UX configuration
- [internal/perception](../perception/CLAUDE.md) - Intent parsing (observed, not modified)
- [internal/init](../init/CLAUDE.md) - Onboarding during initialization

## Architecture

The UX layer operates as a parallel concern to the perception pipeline, observing without blocking:
- **User Journey Tracking**: Progression from New → Power user
- **Progressive Disclosure**: Commands revealed as experience grows
- **Preferences Management**: Extended schema with metrics and learnings
- **Migration**: Existing users skip onboarding and start as "productive"

## File Index

| File | Description |
|------|-------------|
| `doc.go` | Package documentation describing UX layer principles: non-blocking, opt-in, respectful, adaptive. Lists key capabilities: journey tracking, progressive disclosure, contextual help, preferences management, migration. |
| `user_state.go` | User journey state machine with transition logic. Exports `UserJourneyState` enum (New→Onboarding→Learning→Productive→Power), `UserMetrics` tracking sessions/commands/errors, and `ShouldTransition()` based on clarification rate. |
| `preferences.go` | Extended preferences schema for UX tracking with persistence. Exports `UserPreferences` (version 2.0) containing `JourneyPrefs`, `GuidancePrefs`, `TelemetryPrefs`, `UserMetrics`, `LearnedPatterns`, `AgentSelectionPrefs`. |
| `migration.go` | Preferences migration to latest schema for existing users. Exports `MigrationResult`, `MigratePreferences()` creating productive state for existing users (skip onboarding), and new user defaults. |

## Key Types

### UserJourneyState
```go
const (
    StateNew         UserJourneyState = "new"         // First-time user
    StateOnboarding  UserJourneyState = "onboarding"  // In welcome wizard
    StateLearning    UserJourneyState = "learning"    // First 10-20 sessions
    StateProductive  UserJourneyState = "productive"  // Comfortable with basics
    StatePower       UserJourneyState = "power"       // Minimal guidance needed
)
```

### UserMetrics
```go
type UserMetrics struct {
    SessionsCount        int
    CommandsExecuted     int
    ClarificationsNeeded int
    HelpRequests         int
    SuccessfulTasks      int
    ErrorsEncountered    int
    ClarificationRate    float64 // Computed
}
```

## State Transitions

| From | To | Trigger |
|------|-----|---------|
| New | Onboarding | First command executed |
| Onboarding | Learning | Setup complete |
| Learning | Productive | 15+ sessions, 20+ tasks, <15% clarification rate |
| Productive | Power | 50+ sessions, <5% clarification rate |

## Design Principles

1. **Non-blocking**: Observes but never modifies the 6-layer fallback chain
2. **Opt-in**: All features can be disabled via config
3. **Respectful**: Existing users skip onboarding and start as "productive"
4. **Adaptive**: Guidance decreases as user becomes more experienced

## Dependencies

- `internal/config` - TransparencyConfig, GuidanceLevel

## Testing

```bash
go test ./internal/ux/...
```

---

**Remember: Push to GitHub regularly!**
