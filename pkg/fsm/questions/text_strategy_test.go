package questions

import (
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"testing"
)

func TestTextStrategyHandleAnswer(t *testing.T) {
	strategy := NewTextStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Prompt:   "Enter text",
				Type:     "text",
				StoreKey: "name",
			},
		},
	}

	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   " Alice ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Advance {
		t.Fatalf("expected Advance=true")
	}
	if ctx.Record.Data["name"] != "Alice" {
		t.Fatalf("expected stored value 'Alice', got '%s'", ctx.Record.Data["name"])
	}
}

func TestTextStrategyRejectsEmptyInput(t *testing.T) {
	strategy := NewTextStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			Record:    record,
			UserState: &state.UserState{CurrentRecord: record},
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text",
				StoreKey: "name",
			},
		},
	}

	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   "   ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Advance {
		t.Fatalf("expected Advance=false")
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true to re-ask question")
	}
}
