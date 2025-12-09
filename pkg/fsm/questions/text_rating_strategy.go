package questions

import (
	"fmt"
	"strings"

	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	stepCollectText   = "text"
	stepCollectRating = "rating"
	stepNextOrFinish  = "next_finish"
)

type TextRatingStrategy struct{}

func NewTextRatingStrategy() *TextRatingStrategy {
	return &TextRatingStrategy{}
}

func (s *TextRatingStrategy) Name() string {
	return "text_rating"
}

func (s *TextRatingStrategy) Validate(sectionID string, question config.QuestionConfig) error {
	if len(question.Options) > 0 {
		return fmt.Errorf("text_rating question should not have options")
	}
	return nil
}

func (s *TextRatingStrategy) Render(ctx RenderContext) (PromptSpec, error) {
	record, err := ctx.ensureRecord()
	if err != nil {
		return PromptSpec{}, err
	}

	// Get current step (default to text collection)
	stepKey := s.getStepKey(ctx.Question.ID)
	currentStep := record.Data[stepKey]
	if currentStep == "" {
		currentStep = stepCollectText
	}

	switch currentStep {
	case stepCollectText:
		return PromptSpec{
			Text:     ctx.Question.Prompt,
			Keyboard: nil, // No keyboard, expect text input
		}, nil

	case stepCollectRating:
		return s.renderRatingButtons(ctx)

	case stepNextOrFinish:
		return s.renderNextFinishButtons(ctx)

	default:
		return PromptSpec{}, fmt.Errorf("unknown step: %s", currentStep)
	}
}

func (s *TextRatingStrategy) renderRatingButtons(ctx RenderContext) (PromptSpec, error) {
	text := "Оцените от 1 до 10:"

	// Create 2 rows of 5 buttons each (1-5, 6-10)
	row1 := make([]tgbotapi.InlineKeyboardButton, 5)
	row2 := make([]tgbotapi.InlineKeyboardButton, 5)

	for i := 1; i <= 10; i++ {
		buttonText := fmt.Sprintf("%d", i)
		callbackData := fmt.Sprintf("%s%s:%d", ctx.CallbackPrefix, ctx.Question.ID, i)
		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData)

		if i <= 5 {
			row1[i-1] = button
		} else {
			row2[i-6] = button
		}
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(row1, row2)

	return PromptSpec{
		Text:     text,
		Keyboard: &keyboard,
	}, nil
}

func (s *TextRatingStrategy) renderNextFinishButtons(ctx RenderContext) (PromptSpec, error) {
	text := "Выберите действие:"

	nextCallback := fmt.Sprintf("%s%s:next", ctx.CallbackPrefix, ctx.Question.ID)
	finishCallback := fmt.Sprintf("%s%s:finish", ctx.CallbackPrefix, ctx.Question.ID)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➡️ Следующий", nextCallback),
			tgbotapi.NewInlineKeyboardButtonData("✅ Завершить", finishCallback),
		),
	)

	return PromptSpec{
		Text:     text,
		Keyboard: &keyboard,
	}, nil
}

func (s *TextRatingStrategy) HandleAnswer(ctx AnswerContext, input AnswerInput) (AnswerResult, error) {
	record, err := ctx.ensureRecord()
	if err != nil {
		return AnswerResult{}, err
	}

	// Get current step
	stepKey := s.getStepKey(ctx.Question.ID)
	currentStep := record.Data[stepKey]
	if currentStep == "" {
		currentStep = stepCollectText
	}

	switch currentStep {
	case stepCollectText:
		return s.handleTextInput(ctx, input, record, stepKey)

	case stepCollectRating:
		return s.handleRatingInput(ctx, input, record, stepKey)

	case stepNextOrFinish:
		return s.handleNextFinishInput(ctx, input, record, stepKey)

	default:
		return AnswerResult{}, fmt.Errorf("unknown step: %s", currentStep)
	}
}

func (s *TextRatingStrategy) handleTextInput(ctx AnswerContext, input AnswerInput, record *state.Record, stepKey string) (AnswerResult, error) {
	if input.Source != InputSourceText {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, отправьте текстовый ответ.",
		}, nil
	}

	text := strings.TrimSpace(input.Text)
	if text == "" {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, отправьте текстовый ответ.",
		}, nil
	}

	// Store text temporarily
	textKey := s.getTempTextKey(ctx.Question.ID)
	record.Data[textKey] = text

	// Move to rating step
	record.Data[stepKey] = stepCollectRating

	return AnswerResult{
		Repeat: true, // Re-render to show rating buttons
	}, nil
}

func (s *TextRatingStrategy) handleRatingInput(ctx AnswerContext, input AnswerInput, record *state.Record, stepKey string) (AnswerResult, error) {
	if input.Source != InputSourceCallback {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, используйте кнопки для выбора оценки.",
		}, nil
	}

	// Parse rating from callback data
	rating := input.CallbackData
	if !s.isValidRating(rating) {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, выберите оценку от 1 до 10.",
		}, nil
	}

	// Store rating temporarily
	ratingKey := s.getTempRatingKey(ctx.Question.ID)
	record.Data[ratingKey] = rating

	// Move to next/finish step
	record.Data[stepKey] = stepNextOrFinish

	return AnswerResult{
		Repeat: true, // Re-render to show next/finish buttons
	}, nil
}

func (s *TextRatingStrategy) handleNextFinishInput(ctx AnswerContext, input AnswerInput, record *state.Record, stepKey string) (AnswerResult, error) {
	if input.Source != InputSourceCallback {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, используйте кнопки для выбора действия.",
		}, nil
	}

	action := input.CallbackData
	if action != "next" && action != "finish" {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Пожалуйста, выберите 'Следующий' или 'Завершить'.",
		}, nil
	}

	// Retrieve temporary data
	textKey := s.getTempTextKey(ctx.Question.ID)
	ratingKey := s.getTempRatingKey(ctx.Question.ID)

	text := record.Data[textKey]
	rating := record.Data[ratingKey]
	if text == "" || rating == "" {
		return AnswerResult{
			Repeat:   true,
			Feedback: "Не удалось прочитать последний ответ, попробуйте снова.",
		}, nil
	}

	entry := s.formatEntry(text, rating)
	if existing := record.Data[ctx.Question.StoreKey]; existing != "" {
		record.Data[ctx.Question.StoreKey] = existing + "\n" + entry
	} else {
		record.Data[ctx.Question.StoreKey] = entry
	}

	// Clean up temporary keys
	delete(record.Data, stepKey)
	delete(record.Data, textKey)
	delete(record.Data, ratingKey)

	if action == "next" {
		// Reset step for next use
		record.Data[stepKey] = stepCollectText
		return AnswerResult{
			Repeat: true, // Stay on this question for next entry
		}, nil
	}

	// action == "finish"
	return AnswerResult{
		Advance: true, // Move to next question
	}, nil
}

func (s *TextRatingStrategy) formatEntry(text, rating string) string {
	return fmt.Sprintf("- %s\n  Рейтинг: %s", text, rating)
}

func (s *TextRatingStrategy) isValidRating(rating string) bool {
	validRatings := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
	for _, valid := range validRatings {
		if rating == valid {
			return true
		}
	}
	return false
}

func (s *TextRatingStrategy) getStepKey(questionID string) string {
	return fmt.Sprintf("_step_%s", questionID)
}

func (s *TextRatingStrategy) getTempTextKey(questionID string) string {
	return fmt.Sprintf("_text_%s", questionID)
}

func (s *TextRatingStrategy) getTempRatingKey(questionID string) string {
	return fmt.Sprintf("_rating_%s", questionID)
}
