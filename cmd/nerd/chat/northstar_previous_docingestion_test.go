package chat

import "testing"

func TestPreviousNorthstarPhaseDocIngestionToWelcome(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.northstarWizard.Phase = NorthstarDocIngestion
	m.northstarWizard.SubStep = 3
	got, _ := m.previousNorthstarPhase()
	gm, ok := got.(Model)
	if !ok {
		t.Fatalf("expected Model, got %T", got)
	}
	if gm.northstarWizard == nil {
		t.Fatalf("expected wizard to exist")
	}
	if gm.northstarWizard.Phase != NorthstarWelcome {
		t.Fatalf("DocIngestion back should go to Welcome, got %v", gm.northstarWizard.Phase)
	}
	if gm.northstarWizard.SubStep != 0 {
		t.Fatalf("SubStep should reset to 0, got %d", gm.northstarWizard.SubStep)
	}
}

func TestPreviousNorthstarPhaseDocIngestionPrompt(t *testing.T) {
	m := InitChat(Config{})
	m.northstarWizard = NewNorthstarWizard()
	m.northstarWizard.Phase = NorthstarDocIngestion
	got, _ := m.previousNorthstarPhase()
	gm := got.(Model)
	msg := lastAssistantMessage(gm)
	if msg == "" {
		t.Fatalf("expected prompt message after back from DocIngestion")
	}
}
