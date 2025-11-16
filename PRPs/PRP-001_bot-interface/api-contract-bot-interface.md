# API & Integration Contract — Bot Interface Abstraction

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `main.go` boot sequence | Creates the concrete `bot.Client`, verifies the token, obtains `GetUpdatesChan`, and injects the client into `fsm.HandleUpdate` alongside config and the state store. | Replace the concrete pointer with `botport.BotPort` + `botport.Adapter` factories so the entrypoint decides whether to run the Telegram adapter or a fake port when wiring the FSM loop. |
| `pkg/fsm/fsm.go` (`HandleUpdate`, `handleMessage`, `handleCallbackQuery`, `startOrResumeRecordCreation`, `hideKeyboard`, etc.) | Reads updates, routes commands, acknowledges callbacks, and sends fallbacks or validation errors through `bot.Client`. | Every helper invoked from `HandleUpdate` must depend on the `BotPort` interface; errors bubble up as `botport.BotError` values so FSM transitions can decide whether to retry, fail fast, or downgrade UX (e.g., send simple text when keyboards fail). |
| `pkg/fsm/fsm-record.go` (`enterSelectingSection`, `enterAnsweringQuestion`, `askCurrentQuestion`, `handleAnswerResult`) | Uses `SendMessage`/`EditMessageText` to render menus/questions, stores `LastMessageID`, and emits follow-up notifications when a record is saved or cancelled. | Callbacks receive `botport.BotPort` plus `botport.MessageRef` values so transitions know exactly which chat/message to edit; question strategies remain agnostic to Telegram payloads. |
| `pkg/fsm/fsm-main.go` (`sendMainMenu`, `viewListHandler`, `viewLastRecordHandler`, `shareRecordHandler`) | Sends long-form summaries, keyboards, and share instructions from the main-menu FSM. | Functions emit prompts strictly via the port; share/export helpers may request keyboard removal/typing indicators without touching Telegram SDK types. |
| `pkg/fsm/questions/strategy.go` & strategies | Define a minimal `BotPort` for rendering/answering with Send/Edit. | Promote this interface into `pkg/ports/botport` and extend it with callbacks, typing, keyboard removal, and pin/unpin so both FSM core and strategies share one dependency. Update contexts to carry `botport.BotMessage` instead of Telegram structs. |
| `pkg/state/models.go` (`UserState`) | Persists `LastMessageID`, `CurrentSection`, and holds records mutated by FSM/question strategies. | Store the last sent `botport.BotMessage` (chat, message, transport metadata) instead of only `MessageID` so adapters can reconcile edits/deletes even when transport IDs diverge. |
| `pkg/bot/bot.go` | Telegram-specific client performing API requests. | Rebrand as the Telegram adapter that implements `botport.BotPort` and hides `tgbotapi` types. Fake adapters and future transports live next to it but never leak into FSM packages. |

## Backend Implementation Notes

### Interface & Types
```go
package botport

type BotMessage struct {
    ChatID    int64
    MessageID int
    Transport string            // e.g. "telegram"
    Payload   string            // raw JSON/text returned by adapter
    Meta      map[string]string // optional adapter-specific hints
}

// BotError wraps transport-level failures with retry hints.
type BotError struct {
    Op         string // SendMessage, EditMessage, etc.
    Code       string // adapter-defined error code
    RetryAfter time.Duration
    Wrapped    error
}

func (e BotError) Error() string { return fmt.Sprintf("botport %s: %s", e.Op, e.Wrapped) }

type BotPort interface {
    SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (BotMessage, error)
    EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (BotMessage, error)
    AnswerCallback(ctx context.Context, callbackID string, text string) error
    RemoveReplyKeyboard(ctx context.Context, chatID int64, text string) (BotMessage, error)
    SendTyping(ctx context.Context, chatID int64) error
    PinMessage(ctx context.Context, chatID int64, messageID int, silent bool) error
    UnpinMessage(ctx context.Context, chatID int64, messageID int) error
}
```
- Methods accept `context.Context` for deadlines/cancellation triggered by the FSM loop; adapters should honor `ctx.Done()` and return `context.Canceled` when appropriate.
- `markup interface{}` stays transport-agnostic; Telegram adapter accepts `tgbotapi.InlineKeyboardMarkup`, while future transports decode their own types.
- Errors returned by the adapter **must** either be `nil`, `botport.BotError`, or `context.Canceled`; FSM helpers treat unknown errors as fatal and fire `EventForceExit` to keep UX predictable.

### Wiring & Lifecycle
1. **Entry Point** (`main.go`)
   - Resolve `botport.AdapterFactory` based on env flags (e.g., `BOT_PORT=fake` during tests).
   - Build `botport.BotPort` plus a lightweight struct exposing the update stream (`telegram.AdapterUpdates()`), then pass only the port into `fsm.HandleUpdate` / FSM creators.
   - On shutdown, call `adapter.Close()` to drain the update channel gracefully.
2. **FSM Package (`pkg/fsm`)**
   - Update `NewFSMCreator`, `HandleUpdate`, and every helper signature to accept `botport.BotPort`.
   - Replace direct `bot.Client` calls with port methods. Example snippet:
     ```go
     func sendMainMenu(ctx context.Context, userState *state.UserState, bot botport.BotPort) {
         if err := bot.SendTyping(ctx, userState.UserID); err != nil { log.Printf("typing warn: %v", err) }
         msg, err := bot.SendMessage(ctx, userState.UserID, text, keyboard)
         if err != nil { handleBotError(ctx, err, userState) }
         userState.LastBotMessage = msg
     }
     ```
   - Add a tiny helper (`func handleBotError(err error)`) translating `BotError.Code` into FSM actions (retry, downgrade markup, fallback text alert).
3. **Question Strategies (`pkg/fsm/questions`)**
   - Import `pkg/ports/botport` and alias `type BotPort = botport.BotPort` so all strategy contexts automatically gain the richer port without further changes.
   - Ensure `RenderContext`/`AnswerContext` reference the shared `botport.BotMessage` for past prompts.
4. **State Layer (`pkg/state`)**
   - Extend `UserState` with `LastBotMessage botport.BotMessage` and store it whenever FSM/strategies send or edit prompts. This enables adapters to perform idempotent edits or restorations even if `MessageID` resets (e.g., Slack thread IDs).

### Adapter Implementations
- **Telegram Adapter (`pkg/bot/telegram_adapter.go`)**
  - Wrap the existing `Client` logic; convert Telegram responses into `botport.BotMessage` and wrap errors with meaningful `Code` values (e.g., `"message_not_modified"`, `"rate_limited"`).
  - Honor parse modes/keyboard structures by asserting allowed types (inline keyboard, remove keyboard) and returning `BotError{Code: "unsupported_markup"}` otherwise.
  - Provide `Updates(ctx)` helper returning `<-chan tgbotapi.Update` so the existing poll loop stays intact while the outbound API migrates.
- **Fake Adapter (`pkg/bot/fake/fake_port.go`)**
  - Implements `BotPort`, capturing invocations in-memory for assertions. Offer helpers like `ExpectSend(text)` and `LastMessage()` so integration tests (headless FSM runs) can assert flows defined in the PRD success metrics.
  - Simulate retryable failures by exposing knobs (e.g., `FailNext("SendMessage", botport.BotError{Code: "rate_limited", RetryAfter: time.Second})`).

### Testing & Observability
- Update existing unit tests under `pkg/fsm/questions` and add new ones for FSM transitions using the fake adapter to cover `/start`, section selection, answering, cancellation, and share actions without relying on Telegram tokens.
- Add structured logging decorators inside adapters to emit `op`, `chat_id`, and `error_code`. These logs feed into the success metrics (“zero manual Telegram sessions during CI”).
- Provide contract tests comparing fake vs. real adapter serialization (e.g., ensure keyboards rendered by strategies look identical whether using the fake recorder or Telegram adapter) to avoid drift, as highlighted in the PRD risk section.

