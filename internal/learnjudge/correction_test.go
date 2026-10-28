package learnjudge

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The correction question sees the request, codeNERD's reply and the message;
// a long reply is bounded and the state says how much was left out.
func TestCorrectionRequest_StateAndBound(t *testing.T) {
	state, qs := CorrectionRequest(CorrectionInput{PreviousRequest: "fix Max", LastReply: "edited Min", Message: "no, Max"})
	if state["previous_request"] != "fix Max" || state["codenerd_reply"] != "edited Min" || state["message"] != "no, Max" {
		t.Fatalf("state = %v", state)
	}
	if _, ok := state["codenerd_reply_omitted_bytes"]; ok {
		t.Fatal("a short reply was marked as cut")
	}
	if _, ok := qs[CorrectsLastTurn]; !ok || len(qs) != 1 {
		t.Fatalf("questions = %v", qs)
	}
	long := strings.Repeat("a", correctionReplyLimit) + "THE END"
	state, _ = CorrectionRequest(CorrectionInput{LastReply: long, Message: "wrong"})
	if n, _ := state["codenerd_reply_omitted_bytes"].(int); n <= 0 || !strings.HasSuffix(state["codenerd_reply"].(string), "THE END") {
		t.Fatalf("long reply not bounded with its end kept: omitted=%v", state["codenerd_reply_omitted_bytes"])
	}
	if _, ok := state["previous_request"]; ok {
		t.Fatal("an empty previous request went into the state")
	}
}

func TestJudgeCorrection_AsksOnlyWithAReplyAndFailsLoudly(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{CorrectsLastTurn: 0.91}}
	p, asked, err := JudgeCorrection(context.Background(), a, CorrectionInput{LastReply: "done", Message: "that's the wrong file"})
	if err != nil || !asked || p != 0.91 {
		t.Fatalf("p=%v asked=%v err=%v", p, asked, err)
	}
	a = &autopoiesisAsker{answers: map[string]float64{CorrectsLastTurn: 0.91}}
	if _, asked, err := JudgeCorrection(context.Background(), a, CorrectionInput{Message: "wrong"}); asked || err != nil || a.questions != nil {
		t.Fatalf("asked with no reply to correct: asked=%v err=%v", asked, err)
	}
	failing := &autopoiesisAsker{err: errors.New("503")}
	if _, asked, err := JudgeCorrection(context.Background(), failing, CorrectionInput{LastReply: "done", Message: "wrong"}); err == nil || asked {
		t.Fatal("a failed call must be an error, not a no")
	}
	if got := CorrectionFact(0.876); got.Predicate != CorrectionJudgmentPredicate || got.Args[0] != int64(88) {
		t.Fatalf("fact = %+v", got)
	}
}
