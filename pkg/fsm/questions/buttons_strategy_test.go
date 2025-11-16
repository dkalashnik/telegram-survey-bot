package questions

import (
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"testing"
)

func TestButtonsStrategyRender(t *testing.T) {
	strategy := NewButtonsStrategy()
	record := state.NewRecord()
	ctx := RenderContext{
		UserState: &state.UserState{CurrentRecord: record},
		Record:    record,
		SectionID: "section",
		Question: config.QuestionConfig{
			ID:       "city",
			Type:     "buttons",
			Prompt:   "Выберите город",
			StoreKey: "city",
			Options: []config.ButtonOption{
				{Text: "A", Value: "a"},
			},
		},
		CallbackPrefix: "answer:",
	}

	prompt, err := strategy.Render(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt.Keyboard == nil || len(prompt.Keyboard.InlineKeyboard) != 1 {
		t.Fatalf("expected one keyboard row, got %+v", prompt.Keyboard)
	}
	dataPtr := prompt.Keyboard.InlineKeyboard[0][0].CallbackData
	if dataPtr == nil || *dataPtr != "answer:city:a" {
		t.Fatalf("unexpected callback payload: %v", dataPtr)
	}
}

func TestButtonsStrategyHandleAnswer(t *testing.T) {
	strategy := NewButtonsStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "city",
				Type:     "buttons",
				StoreKey: "city",
				Options: []config.ButtonOption{
					{Text: "A", Value: "a"},
					{Text: "B", Value: "b"},
				},
			},
		},
	}

	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "b",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Advance {
		t.Fatalf("expected Advance=true")
	}
	if record.Data["city"] != "b" {
		t.Fatalf("expected stored value 'b', got '%s'", record.Data["city"])
	}
}
