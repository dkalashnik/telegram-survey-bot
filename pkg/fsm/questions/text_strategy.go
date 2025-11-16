package questions

import (
	"fmt"
	"strings"
	"telegramsurveylog/pkg/config"
)

type textStrategy struct{}

// NewTextStrategy returns a QuestionStrategy for "text" prompts.
func NewTextStrategy() QuestionStrategy {
	return &textStrategy{}
}

func (t *textStrategy) Name() string {
	return "text"
}

func (t *textStrategy) Validate(sectionID string, question config.QuestionConfig) error {
	if len(question.Options) > 0 {
		return fmt.Errorf("config validation failed: question '%s' in section '%s' is type 'text' but has options defined", question.ID, sectionID)
	}
	return nil
}

func (t *textStrategy) Render(ctx RenderContext) (PromptSpec, error) {
	return PromptSpec{
		Text:     ctx.Question.Prompt,
		Keyboard: nil,
	}, nil
}

func (t *textStrategy) HandleAnswer(ctx AnswerContext, input AnswerInput) (AnswerResult, error) {
	if input.Source != InputSourceText {
		return AnswerResult{
			Feedback: "Пожалуйста, отправьте текстовый ответ.",
			Repeat:   true,
		}, nil
	}

	value := strings.TrimSpace(input.Text)
	if value == "" {
		return AnswerResult{
			Feedback: "Текст не должен быть пустым, попробуйте ещё раз.",
			Repeat:   true,
		}, nil
	}

	record, err := ctx.ensureRecord()
	if err != nil {
		return AnswerResult{}, err
	}

	record.Data[ctx.Question.StoreKey] = value
	return AnswerResult{Advance: true}, nil
}
