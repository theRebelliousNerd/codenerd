package learnjudge

import (
	"context"
	"errors"
	"testing"
)

func TestJudgeReport_OneRequestAsksBothQuestionsOverTaskAndReport(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{ReportAdmitsUnfinished: 0.99, ReportNamesBlocker: 0.03}}
	got, err := JudgeReport(context.Background(), a, ReportInput{Task: "fix the parser and update the docs", Report: "Fixed the parser. I did not get to the docs."})
	if err != nil || got != (ReportReading{Unfinished: 0.99, Blocked: 0.03}) {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if len(a.questions) != 2 {
		t.Fatalf("asked %d questions, want both in one request", len(a.questions))
	}
	state := a.state.(map[string]any)
	if state["task"] == nil || state["report"] == nil {
		t.Fatalf("state = %v", state)
	}
	if f := ReportReadingFact("/t1", got); f.Args[1] != int64(99) || f.Args[2] != int64(3) {
		t.Fatalf("fact = %v, want whole percents", f.Args)
	}
}

func TestJudgeReport_AFailedCallIsAnErrorNotAReading(t *testing.T) {
	if _, err := JudgeReport(context.Background(), &autopoiesisAsker{err: errors.New("503")}, ReportInput{Report: "done"}); err == nil {
		t.Fatal("a failed call must be an error")
	}
	if _, err := JudgeReport(context.Background(), nil, ReportInput{Report: "done"}); err == nil {
		t.Fatal("no client must be an error")
	}
	if _, err := JudgeReport(context.Background(), &autopoiesisAsker{}, ReportInput{Task: "x"}); err == nil {
		t.Fatal("no report must be an error")
	}
}
