# API & Integration Contract — PRP-001a BotPort Foundation

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `pkg/fsm/questions/strategy.go` | Defines `QuestionStrategy`, `RenderContext`, `AnswerContext`, and currently hosts a local `BotPort` interface. | Replace local interface with `pkg/ports/botport.BotPort` (via direct import or type alias). No other code should define `BotPort`. |
| `pkg/fsm/questions/text_strategy.go` / `buttons_strategy.go` | Strategies call `ctx.Bot.SendMessage/EditMessageText` and rely on Telegram structs. | After refactor, strategies still call the same methods but depend on the shared interface; tests should stub via the new package. |
| `pkg/fsm/questions/registry.go` | Ensures `bot.Client` satisfies the local interface for compile-time safety. | Update the compile-time assertion to use `botport.BotPort` or remove once adapters implement interfaces in later slices. |
| `pkg/fsm/questions/*_test.go` | Imports `pkg/bot` only to satisfy the current BotPort type. | Tests should use lightweight fakes or interface implementations built off `botport.BotPort` without referencing `pkg/bot`. |

## Backend Implementation Notes
```go
// pkg/ports/botport/botport.go
package botport

type BotMessage struct {
    ChatID    int64
    MessageID int
    Transport string
    Payload   string
    Meta      map[string]string
}

type BotError struct { /* reserved for later phases */ }

type BotPort interface {
    SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (BotMessage, error)
    EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (BotMessage, error)
}
```
- Slice 1a focuses on defining the shared interface + value objects. Additional methods (AnswerCallback, typing, etc.) will be added in later PRPs.
- `pkg/fsm/questions/strategy.go` should import `botport` and declare `type BotPort = botport.BotPort` to minimize downstream diff.
- `RenderContext`/`AnswerContext` gain optional `botport.BotMessage` fields (e.g., `LastPrompt botport.BotMessage`). FSM code may populate these fields later, but strategies/tests should handle zero-values.
- Strategy unit tests mock `BotPort` using simple structs implementing the methods; they no longer depend on `pkg/bot.Client`.
- Document expectations via inline comments referencing `PRPs/ai_docs/botport_hex_adapter.md`.
```
