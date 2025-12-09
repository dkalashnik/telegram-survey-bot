package questions

import (
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	"testing"
)

func TestTextRatingStrategy_FullFlow(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Prompt:   "Как прошел ваш день?",
				Type:     "text_rating",
				StoreKey: "day_rating",
			},
			CallbackPrefix: "answer:",
		},
	}

	// Step 1: Submit text answer
	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   "Отличный день, все прошло хорошо",
	})
	if err != nil {
		t.Fatalf("step 1: unexpected error: %v", err)
	}
	if result.Advance {
		t.Fatalf("step 1: expected Advance=false")
	}
	if !result.Repeat {
		t.Fatalf("step 1: expected Repeat=true to show rating buttons")
	}

	// Step 2: Submit rating
	result, err = strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "8",
	})
	if err != nil {
		t.Fatalf("step 2: unexpected error: %v", err)
	}
	if result.Advance {
		t.Fatalf("step 2: expected Advance=false")
	}
	if !result.Repeat {
		t.Fatalf("step 2: expected Repeat=true to show next/finish buttons")
	}

	// Step 3: Choose "finish"
	result, err = strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "finish",
	})
	if err != nil {
		t.Fatalf("step 3: unexpected error: %v", err)
	}
	if !result.Advance {
		t.Fatalf("step 3: expected Advance=true to move to next question")
	}

	// Verify final stored value
	expected := "- Отличный день, все прошло хорошо\n  Рейтинг: 8"
	if ctx.Record.Data["day_rating"] != expected {
		t.Fatalf("unexpected stored value: %q", ctx.Record.Data["day_rating"])
	}

	// Verify temporary keys are cleaned up
	stepKey := strategy.getStepKey("q1")
	textKey := strategy.getTempTextKey("q1")
	ratingKey := strategy.getTempRatingKey("q1")

	if _, exists := ctx.Record.Data[stepKey]; exists {
		t.Fatalf("expected step key to be cleaned up")
	}
	if _, exists := ctx.Record.Data[textKey]; exists {
		t.Fatalf("expected temp text key to be cleaned up")
	}
	if _, exists := ctx.Record.Data[ratingKey]; exists {
		t.Fatalf("expected temp rating key to be cleaned up")
	}
}

func TestTextRatingStrategy_NextAction(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text_rating",
				StoreKey: "feedback",
			},
			CallbackPrefix: "answer:",
		},
	}

	// Step 1: Submit text
	strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   "Good service",
	})

	// Step 2: Submit rating
	strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "9",
	})

	// Step 3: Choose "next"
	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "next",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "- Good service\n  Рейтинг: 9"
	if ctx.Record.Data["feedback"] != expected {
		t.Fatalf("unexpected stored value after next: %q", ctx.Record.Data["feedback"])
	}
	if result.Advance {
		t.Fatalf("expected Advance=false when choosing 'next'")
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true to stay on question for next entry")
	}

	// Verify step is reset to text collection
	stepKey := strategy.getStepKey("q1")
	if ctx.Record.Data[stepKey] != stepCollectText {
		t.Fatalf("expected step to be reset to text collection, got: %s", ctx.Record.Data[stepKey])
	}
}

func TestTextRatingStrategy_MultipleEntries(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text_rating",
				StoreKey: "feedback",
			},
			CallbackPrefix: "answer:",
		},
	}

	// First entry
	strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceText, Text: "First"})
	strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceCallback, CallbackData: "7"})
	strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceCallback, CallbackData: "next"})

	// Second entry
	strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceText, Text: "Second"})
	strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceCallback, CallbackData: "5"})
	result, err := strategy.HandleAnswer(ctx, AnswerInput{Source: InputSourceCallback, CallbackData: "finish"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Advance {
		t.Fatalf("expected Advance=true on finish")
	}

	expected := "- First\n  Рейтинг: 7\n- Second\n  Рейтинг: 5"
	if record.Data["feedback"] != expected {
		t.Fatalf("unexpected aggregated value: %q", record.Data["feedback"])
	}
}

func TestTextRatingStrategy_RejectsEmptyText(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text_rating",
				StoreKey: "response",
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
		t.Fatalf("expected Repeat=true")
	}
	if result.Feedback == "" {
		t.Fatalf("expected feedback message")
	}
}

func TestTextRatingStrategy_RejectsInvalidRating(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text_rating",
				StoreKey: "response",
			},
		},
	}

	// First submit valid text
	strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   "Test response",
	})

	// Then try invalid rating
	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "15",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Advance {
		t.Fatalf("expected Advance=false")
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true")
	}
	if result.Feedback == "" {
		t.Fatalf("expected feedback message")
	}
}

func TestTextRatingStrategy_RejectsWrongInputType(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:       "q1",
				Type:     "text_rating",
				StoreKey: "response",
			},
		},
	}

	// Try to send callback when text is expected
	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "something",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true")
	}
	if result.Feedback == "" {
		t.Fatalf("expected feedback message")
	}
}

func TestTextRatingStrategy_Validate(t *testing.T) {
	strategy := NewTextRatingStrategy()

	// Should reject questions with options
	err := strategy.Validate("section1", config.QuestionConfig{
		ID:   "q1",
		Type: "text_rating",
		Options: []config.ButtonOption{
			{Text: "Option 1", Value: "opt1"},
		},
	})
	if err == nil {
		t.Fatalf("expected validation error for question with options")
	}

	// Should accept question without options
	err = strategy.Validate("section1", config.QuestionConfig{
		ID:   "q1",
		Type: "text_rating",
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestTextRatingStrategy_Name(t *testing.T) {
	strategy := NewTextRatingStrategy()
	if strategy.Name() != "text_rating" {
		t.Fatalf("expected name 'text_rating', got '%s'", strategy.Name())
	}
}

func TestTextRatingStrategy_CustomRatingRange(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := AnswerContext{
		RenderContext: RenderContext{
			UserState: &state.UserState{CurrentRecord: record},
			Record:    record,
			Question: config.QuestionConfig{
				ID:        "q1",
				Prompt:    "Rate from 1 to 5",
				Type:      "text_rating",
				StoreKey:  "rating",
				RatingMin: 1,
				RatingMax: 5,
			},
			CallbackPrefix: "answer:",
		},
	}

	// Submit text
	strategy.HandleAnswer(ctx, AnswerInput{
		Source: InputSourceText,
		Text:   "Good experience",
	})

	// Valid rating (3)
	result, err := strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true after valid rating")
	}

	// Reset for next test
	record.Data[strategy.getStepKey("q1")] = stepCollectRating

	// Invalid rating (10, out of range)
	result, err = strategy.HandleAnswer(ctx, AnswerInput{
		Source:       InputSourceCallback,
		CallbackData: "10",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Repeat {
		t.Fatalf("expected Repeat=true for invalid rating")
	}
	if result.Feedback == "" {
		t.Fatalf("expected feedback message for invalid rating")
	}
}

func TestTextRatingStrategy_CustomButtonLabels(t *testing.T) {
	strategy := NewTextRatingStrategy()
	record := state.NewRecord()
	ctx := RenderContext{
		UserState: &state.UserState{CurrentRecord: record},
		Record:    record,
		Question: config.QuestionConfig{
			ID:                "q1",
			Type:              "text_rating",
			StoreKey:          "rating",
			NextButtonLabel:   "🔄 Add Another",
			FinishButtonLabel: "🏁 Done",
		},
		CallbackPrefix: "answer:",
	}

	// Set state to next/finish step
	record.Data[strategy.getStepKey("q1")] = stepNextOrFinish
	record.Data[strategy.getTempTextKey("q1")] = "Test"
	record.Data[strategy.getTempRatingKey("q1")] = "8"

	// Render next/finish buttons
	prompt, err := strategy.Render(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that custom labels are used in the keyboard
	if prompt.Keyboard == nil {
		t.Fatalf("expected keyboard to be present")
	}

	// Verify the keyboard has the custom labels
	if len(prompt.Keyboard.InlineKeyboard) == 0 {
		t.Fatalf("expected at least one row in keyboard")
	}
	row := prompt.Keyboard.InlineKeyboard[0]
	if len(row) != 2 {
		t.Fatalf("expected 2 buttons in row, got %d", len(row))
	}

	if row[0].Text != "🔄 Add Another" {
		t.Fatalf("expected first button text '🔄 Add Another', got '%s'", row[0].Text)
	}
	if row[1].Text != "🏁 Done" {
		t.Fatalf("expected second button text '🏁 Done', got '%s'", row[1].Text)
	}
}

func TestTextRatingStrategy_ValidateRatingRange(t *testing.T) {
	strategy := NewTextRatingStrategy()

	// Invalid: min < 1 (negative value)
	err := strategy.Validate("section1", config.QuestionConfig{
		ID:        "q1",
		Type:      "text_rating",
		RatingMin: -1,
		RatingMax: 10,
	})
	if err == nil {
		t.Fatalf("expected validation error for rating_min < 1")
	}

	// Invalid: max > 20
	err = strategy.Validate("section1", config.QuestionConfig{
		ID:        "q1",
		Type:      "text_rating",
		RatingMin: 1,
		RatingMax: 25,
	})
	if err == nil {
		t.Fatalf("expected validation error for rating_max > 20")
	}

	// Invalid: min > max
	err = strategy.Validate("section1", config.QuestionConfig{
		ID:        "q1",
		Type:      "text_rating",
		RatingMin: 10,
		RatingMax: 5,
	})
	if err == nil {
		t.Fatalf("expected validation error for rating_min > rating_max")
	}

	// Valid: custom range
	err = strategy.Validate("section1", config.QuestionConfig{
		ID:        "q1",
		Type:      "text_rating",
		RatingMin: 1,
		RatingMax: 5,
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Valid: defaults (0 values will use defaults in runtime)
	err = strategy.Validate("section1", config.QuestionConfig{
		ID:   "q1",
		Type: "text_rating",
	})
	if err != nil {
		t.Fatalf("unexpected validation error for defaults: %v", err)
	}
}
