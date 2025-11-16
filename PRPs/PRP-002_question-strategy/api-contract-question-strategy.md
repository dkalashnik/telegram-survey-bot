# API & Integration Contract — Question-Type Strategy Registry

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `main.go` boot sequence | After `config.LoadConfig`, register built-in strategies and pass registry reference to FSM/state initializers. | Registry must be ready before `fsm.HandleUpdate` uses it; missing registrations panic at startup. |
| `pkg/config/loader.go` → `RecordConfig.Validate()` | Each `QuestionConfig` type triggers `registry.MustGet(question.Type).Validate(question)` to ensure YAML correctness. | Validation errors reference section/question IDs and prevent app start. |
| `pkg/fsm/fsm-record.go` → `askCurrentQuestion` | Replace `switch question.Type` with `registry.MustGet(question.Type).Render(ctx)` to obtain prompt/markup. | Strategy receives `RenderContext{Bot BotPort, ChatID, SectionID, QuestionID, RecordRef}`. |
| `pkg/fsm/fsm.go` → `processAnswer` & callback handlers | Route incoming text/callbacks to `strategy.HandleAnswer(ctx)` to persist data and return next-step instructions. | `HandleAnswer` writes to `ctx.Record.Data[storeKey]` and returns `Result{Advance bool, Complete bool, Error error}` consumed by FSM to fire events. |
| `pkg/state` | Provides `state.UserState`/`state.Record` references passed through contexts. | Strategies mutate records only via provided references, keeping persistence centralized. |
| `pkg/bot` | Transport abstraction exposed to strategies. | Provide `BotPort` interface (SendMessage, EditMessageText, AnswerCallback, etc.) injected via contexts. |

## Backend Implementation Notes
```golang
// Registry bootstrap
func init() {
    questions.MustRegister(NewTextStrategy())
    questions.MustRegister(NewButtonsStrategy())
}

// Context definitions
type RenderContext struct {
    Bot       BotPort
    UserState *state.UserState
    Record    *state.Record
    Section   config.SectionConfig
    Question  config.QuestionConfig
    ChatID    int64
    MessageID int
}

type AnswerContext struct {
    RenderContext
    Update   tgbotapi.Update
    Payload  string // parsed callback or text
}

// Strategy contract
type QuestionType interface {
    Name() string
    Validate(config.QuestionConfig) error
    Render(RenderContext) (text string, markup tgbotapi.InlineKeyboardMarkup, err error)
    HandleAnswer(AnswerContext) (Result, error)
}

type Result struct {
    AdvanceToNext bool
    RepeatCurrent bool
    CompleteSection bool
}

// FSM integration example
func askCurrentQuestion(...) {
    strat := registry.MustGet(question.Type)
    text, markup, err := strat.Render(ctx)
    // send or edit message using BotPort
}

func processAnswer(...) {
    strat := registry.MustGet(question.Type)
    result, err := strat.HandleAnswer(answerCtx)
    // trigger FSM events based on result
}
```
- Keep registry singleton under `pkg/fsm/questions`, but expose getter for tests.
- Unit test registry (duplicate registration, missing type) and each strategy (render/answer behavior).
- Ensure strategies log with section/question IDs for observability.
- Context structs avoid direct dependencies on Telegram types outside of provided `tgbotapi.Update` to keep adapters thin.
