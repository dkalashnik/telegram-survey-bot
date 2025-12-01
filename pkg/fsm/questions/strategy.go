package questions

import (
	"fmt"
	"github.com/dkalashnik/telegram-survey-bot/pkg/config"
	"github.com/dkalashnik/telegram-survey-bot/pkg/ports/botport"
	"github.com/dkalashnik/telegram-survey-bot/pkg/state"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// BotPort exposes outbound messaging helpers via the shared ports package.
type BotPort = botport.BotPort

// QuestionStrategy defines the lifecycle hooks for rendering and processing question answers.
type QuestionStrategy interface {
	Name() string
	Validate(sectionID string, question config.QuestionConfig) error
	Render(RenderContext) (PromptSpec, error)
	HandleAnswer(AnswerContext, AnswerInput) (AnswerResult, error)
}

// RenderContext captures dependencies for prompt generation.
type RenderContext struct {
	Bot            BotPort
	LastPrompt     botport.BotMessage // Populated by the FSM once adapters return BotMessage.
	ChatID         int64
	MessageID      int
	UserState      *state.UserState
	Record         *state.Record
	SectionID      string
	Section        config.SectionConfig
	Question       config.QuestionConfig
	CallbackPrefix string
}

// AnswerContext mirrors RenderContext and additionally carries callback metadata.
type AnswerContext struct {
	RenderContext
	Message    botport.BotMessage // Carries the latest adapter message for future FSM hooks.
	CallbackID string
}

// PromptSpec defines the text and markup returned by strategies.
type PromptSpec struct {
	Text     string
	Keyboard *tgbotapi.InlineKeyboardMarkup
	ForceNew bool
}

// AnswerInputSource differentiates between text and callback payloads.
type AnswerInputSource string

const (
	InputSourceText     AnswerInputSource = "text"
	InputSourceCallback AnswerInputSource = "callback"
)

const (
	TypeText    = "text"
	TypeButtons = "buttons"
)

// AnswerInput wraps user responses in a transport-agnostic struct.
type AnswerInput struct {
	Source       AnswerInputSource
	Text         string
	CallbackData string
	MessageID    int
}

// AnswerResult instructs the FSM how to proceed after a strategy processes an input.
type AnswerResult struct {
	Advance  bool
	Repeat   bool
	Feedback string
}

func (ctx RenderContext) ensureRecord() (*state.Record, error) {
	if ctx.Record == nil {
		return nil, fmt.Errorf("record is nil")
	}
	if ctx.Record.Data == nil {
		ctx.Record.Data = make(map[string]string)
	}
	return ctx.Record, nil
}
