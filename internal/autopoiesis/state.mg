# MANDATE FOR AI CODING TOOLS:
# This file contains critical product requirements and architectural mandates.
# DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
# They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
# This File is best written for Gemini 2.5 Pro.
# YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

# Symbiogen Product Requirements Document (PRD) for internal/autopoiesis/state.mg
#
# File: internal/autopoiesis/state.mg
# Author: Gemini
# Date: 2025-12-08
#
# Recommended Model: 2.5 Pro
#
# Overview:
# Defines the Mangle schema and rules for the Ouroboros Loop "Transactional State Machine".
# It governs self-modifiction stability and termination.
#
# Key Features & Business Value:
# - Stability transition rules (monotonic improvement).
# - Stagnation detection (loop prevention).
#
# Architectural Context:
# - Component Type: Mangle Schema
# - Used By: OuroborosLoop (ouroboros.go) via Mangle Engine.
#
# --- END OF PRD HEADER ---

Decl state(StepID: /name, Stability: /float, Loc: /number).
Decl proposed(StepID: /name).
Decl history(StepID: /name, Hash: /string).

# Transition Logic
# A transition is valid if the proposed next state has equal or greater stability than the current state.
# We assume 'state' facts are populated for both 'Curr' (from context) and 'Next' (simulated/proposed).
valid_transition(Next) :-
  state(Curr, CurrStability, _),
  proposed(Next),
  state(Next, NextStability, _),
  NextStability >= CurrStability.

# Halting Oracle
# Detects stagnation if two history entries refer to the same hash but have different step IDs.
# This implies the agent is circling back to a previous code state.
stagnation_detected() :-
  history(StepA, Hash),
  history(StepB, Hash),
  StepA != StepB.

# Panic Event
# Used to penalize stability.
Decl error_event(Type: /name).
