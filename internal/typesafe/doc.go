// Package typesafe is a client for TypeSafe's Jev (System One) API: typed
// questions about a state, answered with calibrated probabilities.
//
// Jev is evidence, not a verdict. Every failure surfaces as an error, and an
// answer that is missing or of the wrong type is an error too, so a caller can
// never read a failed call as a "no". Callers decide what a probability means.
package typesafe
