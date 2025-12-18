// Package ux provides user experience management for codeNERD.
//
// The UX layer operates as a parallel concern to the existing perception
// transducer pipeline, observing without blocking. It provides:
//
//   - User journey state tracking (New -> Onboarding -> Learning -> Productive -> Power)
//   - Progressive disclosure of commands based on experience level
//   - Contextual help triggers when users struggle
//   - Preferences management with extended schema
//   - Migration for existing users
//
// Key Design Principles:
//
//  1. Non-blocking: UX observes but never modifies the 6-layer fallback chain
//  2. Opt-in: All features can be disabled via config
//  3. Respectful: Existing users skip onboarding and start as "productive"
//  4. Adaptive: Guidance decreases as user becomes more experienced
//
// See the noble-sprouting-emerson.md plan file for full architecture details.
package ux
