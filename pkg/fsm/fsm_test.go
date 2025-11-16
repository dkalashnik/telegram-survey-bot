package fsm

import (
	"context"
	"testing"

	"github.com/dkalashnik/telegram-survey-bot/pkg/bot/fakeadapter"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/fsm/questions"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
)

func TestAskCurrentQuestionStoresBotMessage(t *testing.T) {
	questions.RegisterBuiltins()
	userState := &state.UserState{
		UserID:          1,
		CurrentRecord:   state.NewRecord(),
		CurrentSection:  "sec",
		CurrentQuestion: 0,
	}
	recordConfig := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Section",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Hello?", Type: "text", StoreKey: "name"},
				},
			},
		},
	}
	adapter := &fakeadapter.FakeAdapter{NextMessageID: 5}

	askCurrentQuestion(context.Background(), userState, adapter, recordConfig, 0)

	if userState.LastMessageID != 5 {
		t.Fatalf("expected LastMessageID=5 got %d", userState.LastMessageID)
	}
	if userState.LastPrompt.MessageID != 5 || userState.LastPrompt.Transport != "telegram" || userState.LastPrompt.Payload != "Hello?" {
		t.Fatalf("unexpected LastPrompt: %+v", userState.LastPrompt)
	}
	call := adapter.LastCall("send_message")
	if call == nil || call.Op != "send_message" {
		t.Fatalf("expected send_message call recorded, got %+v", call)
	}
}

func TestAskCurrentQuestionHandlesMessageNotModified(t *testing.T) {
	questions.RegisterBuiltins()
	userState := &state.UserState{
		UserID:          2,
		LastMessageID:   10,
		CurrentRecord:   state.NewRecord(),
		CurrentSection:  "sec",
		CurrentQuestion: 0,
	}
	recordConfig := &config.RecordConfig{
		Sections: map[string]config.SectionConfig{
			"sec": {
				Title: "Section",
				Questions: []config.QuestionConfig{
					{ID: "q1", Prompt: "Hi", Type: "text", StoreKey: "name"},
				},
			},
		},
	}
	adapter := &fakeadapter.FakeAdapter{NextMessageID: 20}
	adapter.Fail("edit_message", fakeadapter.MessageNotModified("edit_message"))

	askCurrentQuestion(context.Background(), userState, adapter, recordConfig, 10)

	if userState.LastMessageID != 10 {
		t.Fatalf("expected LastMessageID to remain 10, got %d", userState.LastMessageID)
	}
	if userState.LastPrompt.MessageID != 10 || userState.LastPrompt.ChatID != 2 {
		t.Fatalf("expected LastPrompt message id 10, got %+v", userState.LastPrompt)
	}
}
