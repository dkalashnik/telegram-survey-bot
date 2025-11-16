# API & Integration Contract — PRP-001c FSM Port Wiring & Fake Adapter

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `main.go` | Constructs Telegram client and FSM creator. | Switch to constructing a `botport.BotPort` via `telegramadapter.New(client, logger)` and pass it to FSM entrypoints. Remove remaining direct `bot.Client` usages after wiring. |
| `pkg/fsm/fsm.go` | Functions `HandleUpdate`, `handleMessage`, `handleCallbackQuery`, `processAnswer`, `handleAnswerResult`, `startOrResumeRecordCreation` currently accept `*bot.Client`. | Update signatures to accept `botport.BotPort`. All send/edit calls should use the injected port. Use returned `botport.BotMessage` to set `AnswerContext.Message` where applicable. |
| `pkg/fsm/fsm-record.go` | `askCurrentQuestion` builds `RenderContext` and calls strategies; currently uses `*bot.Client` and tgbotapi markup. | Swap Bot parameter to `botport.BotPort`. After `SendMessage`/`EditMessage`, assign `RenderContext.LastPrompt` with returned `BotMessage`. Preserve edit-vs-send behavior. |
| `pkg/fsm/questions` | Context structs already include `LastPrompt`/`Message` fields. | Ensure FSM populates those fields; strategies/tests remain unchanged. |
| `pkg/state` | Stores user state; currently tracks `LastMessageID`. | Optionally extend to store `BotMessage` (serialized) if needed; at minimum, ensure callers use `BotMessage.MessageID` instead of raw ints when persisting. |
| `pkg/bot/telegramadapter` | Implements `botport.BotPort` for production. | Compile-time assertion remains; ensure it is injected via `main.go` and any future factory. |
| Tests | Existing FSM tests are minimal; strategy tests use local contexts. | Add new headless FSM tests using the fake adapter; avoid network. |

## Backend Implementation Notes
```go
// FSM signature updates (example)
func HandleUpdate(ctx context.Context, update tgbotapi.Update, botPort botport.BotPort, recordConfig *config.RecordConfig, store *state.Store)
```
- Propagate `botPort` through all helper functions invoked by `HandleUpdate`.

### Render Context Hydration
```go
promptMsg, err := botPort.SendMessage(ctx, userState.UserID, prompt.Text, keyboard)
renderCtx.LastPrompt = promptMsg
userState.LastMessageID = promptMsg.MessageID // optional for legacy fields
```

### Answer Context Hydration
```go
answerCtx := buildAnswerContext(...)
answerCtx.Message = lastPromptBotMessage
// when repeating/advancing, use BotMessage from edit/send result
```

### Fake Adapter API (tests)
```go
type Call struct {
    Op        string
    ChatID    int64
    MessageID int
    Text      string
    Markup    interface{}
}

type FakeAdapter struct {
    Calls        []Call
    NextMessageID int
    FailNext     map[string]error
}

func (f *FakeAdapter) SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (botport.BotMessage, error)
func (f *FakeAdapter) EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (botport.BotMessage, error)
// Helpers
func (f *FakeAdapter) Fail(op string, err error)
func (f *FakeAdapter) LastCall(op string) *Call
```
- `NextMessageID` should auto-increment when zero; allow scripting explicit MessageID per op if needed.
- `Fail` should cause the next call for `op` to return the scripted error (wrap in `botport.BotError` codes like `message_not_modified`, `rate_limited`).

### Error Handling Expectations
- Use `botport.BotError.Code` to branch on `message_not_modified` (safe to ignore) and `rate_limited` (bubble up or log for retry in later slices).
- Context cancellation must short-circuit sends/edits via `context.Canceled` or `context.DeadlineExceeded` mapping (already supported in adapter; preserve in fake).

### Message Metadata
- Always set `BotMessage.Transport = "telegram"` in both real and fake adapters.
- Preserve `MessageID` from adapter responses; use it to update `userState.LastMessageID` when applicable.
- Include optional `Meta` (e.g., markup hints) when available; tests can assert presence but not contents.

### Testing Guidance
- Add `pkg/fsm/fsm_test.go` (or multiple files) to cover:
  - Initial question render uses SendMessage and sets `LastPrompt`.
  - Repeat flow triggers EditMessage and updates `LastPrompt`/`Message`.
  - Error from adapter with `message_not_modified` does not crash flow.
  - Rate limit surfaces BotError code.
- Use `FakeAdapter` to script responses without network calls.

### Documentation Updates
- `docs/system-overview.md`: diagrams show FSM → BotPort (adapter/fake) → Telegram.
- `docs/question-strategy.md`: note contexts now populated from BotPort responses.
- Keep `PRPs/plans.md` in sync with completed slice status.
