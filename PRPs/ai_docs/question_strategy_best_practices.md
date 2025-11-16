# Question Strategy Registry Notes

This document captures implementation considerations for the strategy/registry refactor outlined in `PRPs/PRP-002_question-strategy/prd-question-strategy.md`. Use this when designing the package under `pkg/fsm/questions`.

## Registry Design
- Keep the registry package-scoped with `sync.RWMutex` protection so strategies can be registered from `init()` without data races. Expose `MustRegister` (panic on duplicates) and `Get`/`MustGet`.
- Accept interfaces instead of structs so tests can inject fakes. Provide `BotPort`/`PromptPort` interfaces mirroring the subset of `pkg/bot.Client` used by strategies, keeping the registry agnostic of Telegram SDK types.
- Store strategies by lowercase type key; normalize YAML inputs before lookups to avoid subtle mismatches.

## Strategy Interfaces
- Define a single `QuestionStrategy` interface with cohesive responsibilities:
  ```go
  type QuestionStrategy interface {
      Name() string
      Validate(cfg config.QuestionConfig) error
      Render(ctx RenderContext) (Prompt, error)
      HandleAnswer(ctx AnswerContext) (Result, error)
  }
  ```
- `Render` returns both prompt text and optional `tgbotapi.InlineKeyboardMarkup`; callers decide whether to edit or send a new message.
- `HandleAnswer` should be side-effect free except for writing via `ctx.Record.Data` and returning `Result` to inform the FSM on whether to advance, repeat, or abort.

## Context Structs
- Capture everything a strategy needs (bot port, chat/user IDs, section/question metadata, incoming Telegram update payload).
- Provide helper methods on the context to send validation errors ("⚠️ Пожалуйста, выберите вариант из списка") so each strategy produces consistent UX.

## Error Handling
- Bubble Go errors with section/question IDs, e.g., `fmt.Errorf("question %s/%s: %w", sectionID, question.ID, err)`; `processAnswer` already logs these identifiers.
- Keep Telegram-specific failures (message edits) outside of strategies; they should only describe desired UI, not transport operations.

## References
- Looplab FSM event guarantees: <https://pkg.go.dev/github.com/looplab/fsm#FSM.Event>
- Inline keyboard markup API (required when strategies need buttons): <https://pkg.go.dev/github.com/go-telegram-bot-api/telegram-bot-api/v5#InlineKeyboardMarkup>
- Strategy pattern refresher with Go sample: <https://refactoring.guru/design-patterns/strategy/go/example>
