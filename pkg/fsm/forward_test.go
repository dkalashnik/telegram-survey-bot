package fsm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"text/template"

	"github.com/dkalashnik/telegram-survey-bot/pkg/bot/fakeadapter"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
)

func TestBuildForwardPayloadUsesNoAnswer(t *testing.T) {
	rc := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"a": {
				Title: "Section A",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "P1", StoreKey: "k1"},
					{ID: "q2", Prompt: "P2", StoreKey: "k2"},
				},
			},
		},
	}
	record := &state.Record{Data: map[string]string{"k1": "answer 1"}}
	userState := &state.UserState{UserID: 42, UserName: "Tester"}

	payload := buildForwardPayload(rc, record, userState)

	if len(payload.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(payload.Sections))
	}
	got := payload.Sections[0].Questions
	if got[0].Answer != "answer 1" {
		t.Fatalf("expected first answer kept, got %q", got[0].Answer)
	}
	if got[1].Answer != noAnswerPlaceholder {
		t.Fatalf("expected placeholder for missing answer, got %q", got[1].Answer)
	}
}

func TestHandleForwardAnsweredSectionsSuccessClearsAnswers(t *testing.T) {
	config.SetTargetUserID(999)
	rc := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Main",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Name", StoreKey: "name"},
				},
			},
		},
	}
	rec := state.NewRecord()
	rec.Data["name"] = "Alice"
	rec.IsSaved = true

	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      1,
		UserName:    "User One",
		Records:     []*state.Record{rec, state.NewRecord()}, // ensure other saved records are preserved
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	userState.Records[1].IsSaved = true
	adapter := &fakeadapter.FakeAdapter{}

	handleForwardAnsweredSections(context.Background(), userState, adapter, rc, 1)

	if len(userState.Records) != 1 || userState.CurrentRecord != nil {
		t.Fatalf("expected other saved records preserved and forwarded removed, got records=%d current=%v", len(userState.Records), userState.CurrentRecord)
	}
	if len(adapter.Calls) < 2 {
		t.Fatalf("expected at least two sends (target + confirmation), got %d", len(adapter.Calls))
	}
	if adapter.Calls[0].ChatID != 999 {
		t.Fatalf("expected first send to target 999, got %+v", adapter.Calls[0])
	}
	targetCall := adapter.LastCall("send_message")
	if targetCall == nil || targetCall.ChatID != 1 {
		t.Fatalf("expected confirmation send to chat 1, got %+v", targetCall)
	}
	if targetCall == nil || !strings.Contains(targetCall.Text, "999") {
		t.Fatalf("expected confirmation text to mention target user id, got %+v", targetCall)
	}
}

func TestHandleForwardAnsweredSectionsFailureKeepsAnswers(t *testing.T) {
	config.SetTargetUserID(777)
	rc := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Main",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Field", StoreKey: "f1"},
				},
			},
		},
	}
	rec := state.NewRecord()
	rec.Data["f1"] = "Value"
	rec.IsSaved = true

	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      2,
		UserName:    "User Two",
		Records:     []*state.Record{rec},
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	adapter := &fakeadapter.FakeAdapter{}
	adapter.Fail("send_message", fakeadapter.RateLimited("send_message", 0))

	handleForwardAnsweredSections(context.Background(), userState, adapter, rc, 2)

	if len(userState.Records) == 0 {
		t.Fatalf("expected answers retained on failure")
	}
	confirm := adapter.LastCall("send_message")
	if confirm == nil || confirm.ChatID != 2 {
		t.Fatalf("expected failure notice to chat 2, got %+v", confirm)
	}
}

func TestHandleForwardToSelfDoesNotClearAnswers(t *testing.T) {
	rc := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Main",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Field", StoreKey: "f1"},
				},
			},
		},
	}
	rec := state.NewRecord()
	rec.Data["f1"] = "Self"
	rec.IsSaved = true

	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      10,
		UserName:    "Self",
		Records:     []*state.Record{rec},
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	adapter := &fakeadapter.FakeAdapter{}

	handleForwardToSelf(context.Background(), userState, adapter, rc, userState.UserID)

	if len(userState.Records) != 1 {
		t.Fatalf("expected records kept after sending to self, got %d", len(userState.Records))
	}
	if adapter.Calls == nil || len(adapter.Calls) < 2 {
		t.Fatalf("expected two sends (self target + confirmation), got %+v", adapter.Calls)
	}
	if adapter.Calls[0].ChatID != userState.UserID {
		t.Fatalf("expected first send to self %d, got %+v", userState.UserID, adapter.Calls[0])
	}
}

func TestHandleForwardAnsweredSectionsEmptyAnswers(t *testing.T) {
	config.SetTargetUserID(555)
	rc := &config.RecordConfig{Sections: map[string]config.SectionConfig{}}
	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      3,
		UserName:    "Empty User",
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	adapter := &fakeadapter.FakeAdapter{}

	handleForwardAnsweredSections(context.Background(), userState, adapter, rc, 3)

	call := adapter.LastCall("send_message")
	if call == nil || call.ChatID != 3 || call.Text == "" {
		t.Fatalf("expected notice to user chat for empty send, got %+v", call)
	}
	if len(userState.Records) != 0 {
		t.Fatalf("expected no state change, got %d records", len(userState.Records))
	}
}

func TestHandleForwardAnsweredSectionsMissingTarget(t *testing.T) {
	config.SetTargetUserID(0)
	rc := &config.RecordConfig{Sections: map[string]config.SectionConfig{}}
	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      4,
		UserName:    "NoTarget",
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	adapter := &fakeadapter.FakeAdapter{}

	handleForwardAnsweredSections(context.Background(), userState, adapter, rc, 4)

	call := adapter.LastCall("send_message")
	if call == nil || call.ChatID != 4 || call.Text == "" {
		t.Fatalf("expected warning to user about missing TARGET_USER_ID, got %+v", call)
	}
}

func TestHandleForwardAnsweredSectionsRenderError(t *testing.T) {
	config.SetTargetUserID(888)
	rc := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Main",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Field", StoreKey: "f1"},
				},
			},
		},
	}
	rec := state.NewRecord()
	rec.Data["f1"] = "Value"
	rec.IsSaved = true

	fsmCreator := NewFSMCreator()
	userState := &state.UserState{
		UserID:      5,
		UserName:    "RenderFail",
		Records:     []*state.Record{rec},
		MainMenuFSM: fsmCreator.NewMainMenuFSM(),
		RecordFSM:   fsmCreator.NewRecordFSM(),
	}
	adapter := &fakeadapter.FakeAdapter{}

	originalTpl := forwardTpl
	defer func() { forwardTpl = originalTpl }()
	forwardTpl = template.Must(template.New("forward").Funcs(template.FuncMap{
		"fail": func() (string, error) { return "", errors.New("boom") },
	}).Parse(`{{fail}}`))

	handleForwardAnsweredSections(context.Background(), userState, adapter, rc, 5)

	if len(userState.Records) == 0 {
		t.Fatalf("expected records retained on render error")
	}
	call := adapter.LastCall("send_message")
	if call == nil || call.ChatID != 5 {
		t.Fatalf("expected error notice to chat 5, got %+v", call)
	}
}
