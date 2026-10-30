package chat

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Requests and their answers. Every user message is a request with a session
// id (Message.Request, given when it enters history); the message that
// settles it carries that id as Answers. The SQLite session projection
// (sessionTurns) pairs a request with the messages that answer it, so a
// queue notice, a side reply or another request's answer printed between
// them is never recorded as its answer.
//
// The projection used to pair each user message with the first assistant
// message before the next user message: with two requests typed before the
// first was answered, the first got no row and the second got its queue
// notice as its answer (audit queued-history-projection).
//
// The id travels with the work: the queue keeps it (queuedMessage.request),
// processInput is given it, and every message its turn ends with is wrapped
// in an answerMsg. Update stamps what that message puts in history; nothing
// is matched by position or by text.

// answerMsg is a turn's ending message, for the request it answers.
type answerMsg struct {
	Request uint64
	Msg     tea.Msg
}

// answerTo wraps a turn's ending message for its request. A stream's start is
// not an ending: its result is wrapped when it arrives. A turn with no
// request (an automatic step) is passed through.
func answerTo(request uint64, msg tea.Msg) tea.Msg {
	if request == 0 || msg == nil {
		return msg
	}
	if _, stream := msg.(streamStartMsg); stream {
		return msg
	}
	return answerMsg{Request: request, Msg: msg}
}

// applyAnswer runs the ending message with m.answering set, so every
// assistant message it adds answers the request (addMessage). A turn that
// hands its work to the background has not answered yet: its reply is an
// acknowledgement, and the job carries the request to the result it ends
// with (backgroundJob.RequestID).
func (m Model) applyAnswer(msg answerMsg) (tea.Model, tea.Cmd) {
	inner := msg.Msg
	answering := msg.Request
	if start, ok := inner.(backgroundStartMsg); ok {
		start.Job.RequestID = msg.Request
		inner, answering = start, 0
	}
	prev := m.answering
	m.answering = answering
	next, cmd := m.updateCore(inner)
	if nm, ok := next.(Model); ok {
		nm.answering = prev
		return nm, cmd
	}
	return next, cmd
}

// attribute gives a user message its request id and stamps an assistant
// message added while a request is being answered.
func (m Model) attribute(msg Message) Message {
	switch msg.Role {
	case "user":
		if msg.Request == 0 {
			msg.Request = m.turn.newRequest()
		}
	case "assistant":
		if msg.Answers == 0 {
			msg.Answers = m.answering
		}
	}
	return msg
}

// lastRequest is the request id of the newest user message; 0 when there is
// none.
func (m Model) lastRequest() uint64 {
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Role == "user" {
			return m.history[i].Request
		}
	}
	return 0
}

// maxRequest is the highest request id in history, so ids handed out after a
// session is loaded never repeat one of its own.
func maxRequest(history []Message) uint64 {
	var top uint64
	for _, h := range history {
		if h.Request > top {
			top = h.Request
		}
	}
	return top
}
