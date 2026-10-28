// Package sessionlog owns the on-disk record of a codeNERD session:
//
//	.nerd/sessions/<session-id>/
//	  session.json               the main chat conversation
//	  chat/                      the main agent's model calls (debug only)
//	  <role>-<id>/               one folder per shard, subagent, campaign phase
//	    transcript.json          the agent's turns, tool calls and results (always)
//	    NNNN-<purpose>.json      one file per model API exchange (debug only)
//	  jev/                       Jev / TypeSafe calls (debug only)
//
// Three record kinds with three lifetimes:
//
//   - session.json is the chat history. It is always written.
//   - transcript.json is the harness's own record of an agent's conversation:
//     input, assistant output with its tool calls, tool results, final answer.
//     It is written by the harness at the points every agent already passes
//     through, never by the agent, and flushed per event, so a killed agent's
//     transcript is complete up to the kill. It is always written.
//   - exchange files hold the wire request and response of one model call plus
//     the decisions that shaped it. They are written only while debug mode or
//     debug-self mode is on (logging.IsDebugMode).
//
// Every long text field in every file is stored as an array of its lines;
// joining with "\n" restores it exactly. Wire bodies are stored as ordered JSON
// with each multi-line string split the same way and its JSON pointer listed in
// lines_paths, so RestoreTree reproduces the wire body.
//
// This package is a leaf: it imports no codeNERD package but logging and
// atomicfile. The broker, the typesafe client and the session executor call
// into it; it calls into none of them.
package sessionlog
