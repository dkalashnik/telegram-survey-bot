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

	// Validate rating range if explicitly set
	minRating := question.RatingMin
	maxRating := question.RatingMax

	// Only validate if values are explicitly set (non-zero)
	if minRating != 0 {
		if minRating < 1 {
			return fmt.Errorf("rating_min must be at least 1, got %d", minRating)
		}
	}

	if maxRating != 0 {
		if maxRating > 20 {
			return fmt.Errorf("rating_max cannot exceed 20, got %d", maxRating)
		}
	}

	// If both are set, validate the relationship
	if minRating != 0 && maxRating != 0 {
		if minRating > maxRating {
			return fmt.Errorf("rating_min (%d) cannot be greater than rating_max (%d)", minRating, maxRating)
		}
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
	minRating, maxRating := s.getRatingRange(ctx.Question)
	text := fmt.Sprintf("Оцените от %d до %d:", minRating, maxRating)

	// Create buttons for the rating range
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, maxRating-minRating+1)
	for i := minRating; i <= maxRating; i++ {
		buttonText := fmt.Sprintf("%d", i)
		callbackData := fmt.Sprintf("%s%s:%d", ctx.CallbackPrefix, ctx.Question.ID, i)
		button := tgbotapi.NewInlineKeyboardButtonData(buttonText, callbackData)
		buttons = append(buttons, button)
	}

	// Split buttons into rows of 5
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(buttons); i += 5 {
		end := i + 5
		if end > len(buttons) {
			end = len(buttons)
		}
		rows = append(rows, buttons[i:end])
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	return PromptSpec{
		Text:     text,
		Keyboard: &keyboard,
	}, nil
}

func (s *TextRatingStrategy) renderNextFinishButtons(ctx RenderContext) (PromptSpec, error) {
	text := "Выберите действие:"

	nextLabel := s.getNextButtonLabel(ctx.Question)
	finishLabel := s.getFinishButtonLabel(ctx.Question)

	nextCallback := fmt.Sprintf("%s%s:next", ctx.CallbackPrefix, ctx.Question.ID)
	finishCallback := fmt.Sprintf("%s%s:finish", ctx.CallbackPrefix, ctx.Question.ID)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(nextLabel, nextCallback),
			tgbotapi.NewInlineKeyboardButtonData(finishLabel, finishCallback),
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
	if !s.isValidRating(ctx.Question, rating) {
		minRating, maxRating := s.getRatingRange(ctx.Question)
		return AnswerResult{
			Repeat:   true,
			Feedback: fmt.Sprintf("Пожалуйста, выберите оценку от %d до %d.", minRating, maxRating),
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

func (s *TextRatingStrategy) isValidRating(question config.QuestionConfig, rating string) bool {
	minRating, maxRating := s.getRatingRange(question)

	// Try to parse rating as integer
	var ratingInt int
	_, err := fmt.Sscanf(rating, "%d", &ratingInt)
	if err != nil {
		return false
	}

	return ratingInt >= minRating && ratingInt <= maxRating
}

func (s *TextRatingStrategy) getRatingRange(question config.QuestionConfig) (int, int) {
	minRating := question.RatingMin
	if minRating == 0 {
		minRating = 1 // Default minimum
	}

	maxRating := question.RatingMax
	if maxRating == 0 {
		maxRating = 10 // Default maximum
	}

	return minRating, maxRating
}

func (s *TextRatingStrategy) getNextButtonLabel(question config.QuestionConfig) string {
	if question.NextButtonLabel != "" {
		return question.NextButtonLabel
	}
	return "➡️ Следующий" // Default label
}

func (s *TextRatingStrategy) getFinishButtonLabel(question config.QuestionConfig) string {
	if question.FinishButtonLabel != "" {
		return question.FinishButtonLabel
	}
	return "✅ Завершить" // Default label
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
