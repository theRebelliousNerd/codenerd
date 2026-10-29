package turnjudge

import (
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Every question Jev is asked about a chat turn, in one place: the wording,
// the criteria and the names are what an audit of this layer reads and edits.
// Each asks one judgment a person could make at a glance from the state, and
// each has a rule that reads it (policy/jev_evidence.mg, side_turns.mg): a
// question no rule reads is cost without evidence, so it is not asked.
// What the answers decide is the kernel's, never these questions'.

// Judgment names. They are the kernel's names too (jev_judgment/2).
const (
	ChangesFiles     = "changes_files"
	NamesTarget      = "names_target"
	Hypothetical     = "hypothetical"
	AboutCodeNERD    = "about_codenerd"
	WantsToStop      = "wants_to_stop"
	RulesOutChanges  = "rules_out_changes"
	ApprovesHeldWork = "approves_held_work"
	AddsToHeldWork   = "adds_to_held_work"
	answeredPrefix   = "answered_"
)

// fixedQuestions are asked every turn.
func fixedQuestions() map[string]typesafe.Question {
	return map[string]typesafe.Question{
		ConversationAct: conversationActQuestion(),
		ChangesFiles: typesafe.Noul{
			Instructions: "Is the user asking for changes to files in their codebase?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The user wants code or files written, changed, fixed, refactored, renamed or removed.",
				False: "The user asks a question, wants something explained, found, reviewed or run, or is chatting.",
			},
		},
		NamesTarget: typesafe.Noul{
			Instructions: "Does the user's request say which file, function or feature they want to work on?",
			Criteria: &typesafe.NoulCriteria{
				True:  "A file, function, package or feature is named, or the conversation so far makes clear which one is meant.",
				False: "The request leaves open which code it is about, such as 'fix it' with nothing named before.",
			},
		},
		Hypothetical: typesafe.Noul{
			Instructions: "Is the user asking what would happen if something were done?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The user asks 'what if', 'what would happen', or how risky a change would be.",
				False: "The user asks for the change itself, or asks about the code as it is now.",
			},
		},
		AboutCodeNERD: typesafe.Noul{
			Instructions: "Is the user asking the assistant to investigate its own failures?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The user asks why the assistant failed, asks it to debug itself, or points it at its own logs.",
				False: "The user asks about their own code or project, even code that has bugs.",
			},
		},
		RulesOutChanges: typesafe.Noul{
			Instructions: "Has the user said not to change any files for this request?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The user says to only look, read, explain or report, and not to change, edit, write or touch anything.",
				False: "The user asks for changes, allows them, or says nothing about whether files may change.",
			},
		},
	}
}

// ConversationAct names the Choice over what the user is doing with the turn
// (conversationActQuestion). Its options are the kernel's vocabulary too:
// conversation_act_option(Option, Act), policy/conversation_acts.mg.
const ConversationAct = "conversation_act"

// The conversation acts, the options of ConversationAct. Each is named for
// what the user does, so the name itself tells Jev what the option means.
const (
	ActRequestForWork = "request_for_work"
	ActReply          = "reply_to_codenerd"
	ActQuestionAbout  = "question_about_codenerd"
	ActFeedback       = "feedback_on_codenerd"
	ActCorrection     = "correction_of_codenerd"
	ActSharedMaterial = "shared_material"
	ActStopRequest    = "stop_request"
	ActSmallTalk      = "small_talk"
)

// ConversationActs lists every option of ConversationAct.
var ConversationActs = []string{ActRequestForWork, ActReply, ActQuestionAbout, ActFeedback, ActCorrection, ActSharedMaterial, ActStopRequest, ActSmallTalk}

// conversationActQuestion asks what the user is doing in the conversation,
// beside which task perception reads in it: the task vocabulary has no word
// for "yes", "done", "that's bad", a pasted log or "what mode are you in?",
// and such turns were forced onto the nearest task verb (Jev read 50 of 70
// real messages as a weak fit for any of the 43 verbs, 2026-10-28). One
// Choice, not a Noul per act: routing takes one lane, so it needs the act the
// turn mainly is, and a turn whose top act is not clear reads as unsettled.
// No companion "does any fit" Noul: request_for_work and small_talk leave
// nothing uncovered, and the companion never read no on 78 real messages
// (lowest 0.45). What each act does to routing is the kernel's.
func conversationActQuestion() typesafe.Question {
	return typesafe.Choice{
		Instructions: "What is the user doing in `message`?",
		Criteria: map[string]any{
			ActRequestForWork: "The user asks codeNERD to do something: read, find, explain, review, change, fix, run or investigate something, in their code or in codeNERD itself.",
			ActReply:          "The user answers a question codeNERD just asked or picks from a choice it offered, such as 'yes', '1', 'skip' or 'done'.",
			ActQuestionAbout:  "The user asks about codeNERD's own state in this session: its mode or settings, what it can see or access, or what it did or was doing.",
			ActFeedback:       "The user says what they think of how codeNERD behaved or how it is built, and asks for nothing to be done.",
			ActCorrection:     "The user says codeNERD's last reply or action was wrong or not what they asked for.",
			ActSharedMaterial: "The user gives codeNERD text, logs or a file path to keep in mind, and asks for nothing to be done with it.",
			ActStopRequest:    "The user tells codeNERD to stop, cancel or wait.",
			ActSmallTalk:      "The user greets codeNERD, thanks it, or chats.",
		},
	}
}

// stopQuestion is asked only while work runs: `running_work` names it, and a
// stop means nothing without it.
func stopQuestion() typesafe.Question {
	return typesafe.Noul{
		Instructions: "Is the user telling the assistant to stop `running_work`?",
		Criteria: &typesafe.NoulCriteria{
			True:  "The user says stop, cancel, abort, hold on, or never mind about the running work.",
			False: "The user asks for something new, answers a question, or comments without asking for the work to stop.",
		},
	}
}

// heldWorkQuestion is asked only while work is held for a go-ahead.
func heldWorkQuestion() typesafe.Question {
	return typesafe.Noul{
		Instructions: "Is the user telling the assistant to start `held_work` now?",
		Criteria: &typesafe.NoulCriteria{
			True:  "The user says go, do it, yes, start, or otherwise approves starting the held work as it is.",
			False: "The user asks about the held work, changes it, declines it, or talks about something else.",
		},
	}
}

// addsToHeldWorkQuestion separates "go" from "go, and also ...": a go-ahead
// that asks for more is a new request, not the held one.
func addsToHeldWorkQuestion() typesafe.Question {
	return typesafe.Noul{
		Instructions: "Does `message` ask for anything besides starting `held_work` exactly as it is?",
		Criteria: &typesafe.NoulCriteria{
			True:  "The user adds, removes or changes something, such as 'go, and also add a test' or 'do it but skip the docs'.",
			False: "The user asks for nothing beyond starting it, such as 'go', 'yes do it' or 'ship it'.",
		},
	}
}

// openQuestion asks whether the latest message answers one of the brief's
// open questions. The question rides in its own question, as a field: a hop
// through an index into the brief is the indirection Jev handles worst.
func openQuestion(q string) typesafe.Question {
	return typesafe.Noul{
		Instructions: map[string]any{
			"question":      "Does `message` answer `open_question`?",
			"open_question": strings.TrimSpace(q),
		},
		Criteria: &typesafe.NoulCriteria{
			True:  "The message gives the information the question asks for, or settles it.",
			False: "The message is about something else, or leaves the question open.",
		},
	}
}

func answeredName(i int) string { return fmt.Sprintf("%s%d", answeredPrefix, i) }
