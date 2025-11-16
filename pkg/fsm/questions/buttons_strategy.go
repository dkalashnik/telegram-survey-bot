package questions

import (
	"fmt"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type buttonsStrategy struct{}

// NewButtonsStrategy returns a QuestionStrategy for inline button prompts.
func NewButtonsStrategy() QuestionStrategy {
	return &buttonsStrategy{}
}

func (b *buttonsStrategy) Name() string {
	return "buttons"
}

func (b *buttonsStrategy) Validate(sectionID string, question config.QuestionConfig) error {
	if len(question.Options) == 0 {
		return fmt.Errorf("config validation failed: question '%s' in section '%s' is type 'buttons' but has no options", question.ID, sectionID)
	}
	for idx, option := range question.Options {
		if option.Text == "" {
			return fmt.Errorf("config validation failed: option #%d for question '%s' in section '%s' has no text", idx+1, question.ID, sectionID)
		}
		if option.Value == "" {
			return fmt.Errorf("config validation failed: option #%d for question '%s' in section '%s' has no value", idx+1, question.ID, sectionID)
		}
	}
	return nil
}

func (b *buttonsStrategy) Render(ctx RenderContext) (PromptSpec, error) {
	markup := tgbotapi.NewInlineKeyboardMarkup()
	for _, option := range ctx.Question.Options {
		data := fmt.Sprintf("%s%s:%s", ctx.CallbackPrefix, ctx.Question.ID, option.Value)
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(option.Text, data),
		)
		markup.InlineKeyboard = append(markup.InlineKeyboard, row)
	}
	return PromptSpec{
		Text:     ctx.Question.Prompt,
		Keyboard: &markup,
	}, nil
}

func (b *buttonsStrategy) HandleAnswer(ctx AnswerContext, input AnswerInput) (AnswerResult, error) {
	if input.Source != InputSourceCallback {
		return AnswerResult{
			Feedback: "Пожалуйста, выберите ответ с помощью кнопок ниже.",
			Repeat:   true,
		}, nil
	}

	option := b.findOption(ctx.Question, input.CallbackData)
	if option == nil {
		return AnswerResult{
			Feedback: "Выбранный вариант больше недоступен. Попробуйте снова.",
			Repeat:   true,
		}, nil
	}

	record, err := ctx.ensureRecord()
	if err != nil {
		return AnswerResult{}, err
	}
	record.Data[ctx.Question.StoreKey] = option.Value
	return AnswerResult{Advance: true}, nil
}

func (b *buttonsStrategy) findOption(question config.QuestionConfig, value string) *config.ButtonOption {
	for _, opt := range question.Options {
		if opt.Value == value {
			return &opt
		}
	}
	return nil
}
