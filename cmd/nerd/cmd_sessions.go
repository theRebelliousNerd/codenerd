// Package main implements session management CLI commands for codeNERD.
// This file handles session listing, loading, and management.
package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	nerdinit "codenerd/internal/init"
	"codenerd/internal/sessionlog"

	"github.com/spf13/cobra"
)

// =============================================================================
// SESSION MANAGEMENT COMMANDS
// =============================================================================

// sessionsCmd manages CLI sessions
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage codeNERD sessions",
	Long: `List, search and load codeNERD sessions.

A session lives in .nerd/sessions/<session-id>/: session.json is the chat, each
shard, subagent and campaign phase has a folder with its transcript, and with
debug mode on every model call is a numbered file under its agent's folder.

Subcommands:
  list    - List sessions: id, start, agents, size
  search  - Find lines in what agents said, were told and ran
  load    - Load a specific session
  wire    - Print a captured model call's wire body exactly as sent`,
	RunE: runSessionsList,
}

// sessionsListCmd lists available sessions
var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions: id, start, agents, size",
	RunE:  runSessionsList,
}

// sessionsSearchCmd finds text in the session record.
var sessionsSearchCmd = &cobra.Command{
	Use:   "search <text>",
	Short: "Find lines in the session record",
	Long: `Print every line of every session file that contains <text> (case-insensitive),
with its session, agent, file and turn. Searches session.json, agent
transcripts and exchange files.

  nerd sessions search "undefined: foo"
  nerd sessions search "go test" --agent coder
  nerd sessions search timeout --session session-1793208939530632285`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsSearch,
}

var (
	sessionsSearchAgent   string
	sessionsSearchSession string
)

// sessionsLoadCmd loads a specific session
var sessionsLoadCmd = &cobra.Command{
	Use:   "load <session-id>",
	Short: "Load a specific session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsLoad,
}

// sessionsWireCmd prints a captured wire body from an exchange file.
var sessionsWireCmd = &cobra.Command{
	Use:   "wire <exchange-file>",
	Short: "Print a captured model call's wire body exactly as sent",
	Long: `Print the request body of one model call, as it went over the wire, from an
exchange file (NNNN-<purpose>.json under an agent's session folder, written in
debug mode). The file stores long strings as arrays of lines; this joins them
back, keeping the provider's key order, so the body can be diffed against
another call or sent again.

  nerd sessions wire .nerd/sessions/<id>/coder-1/0003-turn.json
  nerd sessions wire 0003-turn.json --attempt 1
  nerd sessions wire 0003-turn.json --response`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsWire,
}

var (
	sessionsWireAttempt  int
	sessionsWireResponse bool
)

func runSessionsWire(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("read exchange file: %w", err)
	}
	body, err := sessionlog.WireBody(data, sessionsWireAttempt, sessionsWireResponse)
	if err != nil {
		return fmt.Errorf("%s: %w", args[0], err)
	}
	out := cmd.OutOrStdout()
	if _, err := out.Write(body); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

// sessionsWorkspace is the workspace the sessions commands read. Flat session
// files from before the folder layout are moved into their folders first, so
// the commands see one layout.
func sessionsWorkspace() string {
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	if _, err := sessionlog.MigrateFlat(ws); err != nil {
		fmt.Fprintf(os.Stderr, "warning: migrating flat session files: %v\n", err)
	}
	return ws
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	infos, err := sessionlog.List(sessionsWorkspace())
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}
	if len(infos) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}
	return writeSessionList(cmd.OutOrStdout(), infos)
}

// writeSessionList prints the session table: id, start, agents, size.
func writeSessionList(w io.Writer, infos []sessionlog.SessionInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tSTART\tAGENTS\tSIZE")
	for _, s := range infos {
		start := "-"
		if !s.Start.IsZero() {
			start = s.Start.Local().Format("2006-01-02 15:04")
		}
		agents := "-"
		if len(s.Agents) > 0 {
			agents = strings.Join(s.Agents, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, start, agents, humanBytes(s.Bytes))
	}
	return tw.Flush()
}

func runSessionsSearch(cmd *cobra.Command, args []string) error {
	hits, err := sessionlog.Search(sessionsWorkspace(), args[0], sessionlog.SearchOptions{
		Agent: sessionsSearchAgent, Session: sessionsSearchSession,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	return writeSearchHits(cmd.OutOrStdout(), hits)
}

// writeSearchHits prints one line per hit:
// session  agent  file  turn N: matching line.
func writeSearchHits(w io.Writer, hits []sessionlog.Hit) error {
	for _, h := range hits {
		turn := "-"
		if h.Turn > 0 {
			turn = fmt.Sprintf("%d", h.Turn)
		}
		if _, err := fmt.Fprintf(w, "%s  %s  %s  turn %s: %s\n", h.Session, h.Agent, h.File, turn, h.Line); err != nil {
			return err
		}
	}
	return nil
}

func runSessionsLoad(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	ws := sessionsWorkspace()

	sessions, err := nerdinit.ListSessionHistories(ws)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	found := slices.Contains(sessions, sessionID)

	if !found {
		return fmt.Errorf("session '%s' not found. Use 'nerd sessions list' to see available sessions", sessionID)
	}

	fmt.Printf("✅ Session '%s' exists.\n", sessionID)
	// There is no --load-session flag on the root command; loading a saved
	// session is a TUI slash command. The old hint pointed at a flag that
	// fails with "unknown flag".
	fmt.Println("To load this session, start the TUI with `nerd` and run:")
	fmt.Printf("  /load-session %s\n", sessionID)

	return nil
}

func init() {
	sessionsSearchCmd.Flags().StringVar(&sessionsSearchAgent, "agent", "", "only this agent role (chat, jev, coder, ...)")
	sessionsSearchCmd.Flags().StringVar(&sessionsSearchSession, "session", "", "only this session id")
	sessionsWireCmd.Flags().IntVar(&sessionsWireAttempt, "attempt", 0, "the round trip's attempt number (0: the last, the one the call kept)")
	sessionsWireCmd.Flags().BoolVar(&sessionsWireResponse, "response", false, "print the response body instead of the request body")
	sessionsCmd.AddCommand(sessionsListCmd, sessionsSearchCmd, sessionsLoadCmd, sessionsWireCmd)
}
