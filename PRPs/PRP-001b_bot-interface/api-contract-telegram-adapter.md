# API & Integration Contract — PRP-001b Telegram Adapter

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `pkg/bot` (`client.go`, constructors) | Provides the existing Telegram client with token/config loading. | Adapter must accept an instantiated `*bot.Client` so configuration stays centralized. No direct BotPort references should leak from this package. |
| `pkg/ports/botport` | Defines `BotPort`, `BotMessage`, `BotError`. | Adapter implements `botport.BotPort` exactly, returning `botport.BotMessage` populated with ChatID, MessageID, Transport (`"telegram"`), Payload, and Meta. Errors must be wrapped via `botport.BotError`. |
| `main.go` | Currently wires `pkg/bot.Client` directly into FSM/question logic. | Replace direct `bot.Client` usage with `telegramadapter.New(client, logger)` and pass the adapter wherever a BotPort is required (FSM, registries). No other package should instantiate the client directly after wiring. |
| `pkg/fsm` (`fsm.go`, `fsm-record.go`) | Accepts BotPort dependencies when sending/editing messages. | No structural changes in 001b, but ensure constructors accept a `botport.BotPort` so swapping adapters later is trivial. |
| `pkg/fsm/questions` | Strategies reference contexts with `BotPort` alias + BotMessage placeholders. | No direct changes, but adapter responses must include data needed for slice 001c to fill `LastPrompt`/`Message`. |
| Tests (`pkg/bot`, `pkg/fsm`) | Currently rely on concrete client or simple fakes. | Add adapter-level tests using fake `bot.Client`/HTTP responders to confirm BotMessage/ BotError mapping, without touching strategy tests yet. |

## Backend Implementation Notes
```go
package telegramadapter

// Adapter satisfies botport.BotPort using an existing bot.Client.
type Adapter struct {
    client *bot.Client
    logger Logger // minimal interface, e.g., Printf(format string, args ...any)
}

func New(client *bot.Client, logger Logger) *Adapter {
    if client == nil {
        panic("telegramadapter: client is nil")
    }
    return &Adapter{client: client, logger: logger}
}
```

### BotPort Methods
```go
func (a *Adapter) SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (botport.BotMessage, error) {
    msgCfg, err := buildMessageConfig(chatID, text, markup)
    if err != nil {
        return botport.BotMessage{}, botport.NewBotError("send_message", "bad_payload", err)
    }
    res, err := a.client.SendWithContext(ctx, msgCfg)
    if err != nil {
        return botport.BotMessage{}, wrapTelegramError("send_message", err)
    }
    return toBotMessage(res), nil
}

func (a *Adapter) EditMessage(ctx context.Context, chatID int64, msgID int, text string, markup interface{}) (botport.BotMessage, error) {
    editCfg, err := buildEditConfig(chatID, msgID, text, markup)
    if err != nil {
        return botport.BotMessage{}, botport.NewBotError("edit_message", "bad_payload", err)
    }
    res, err := a.client.EditMessageTextWithContext(ctx, editCfg)
    if err != nil {
        return botport.BotMessage{}, wrapTelegramError("edit_message", err)
    }
    return toBotMessage(res), nil
}
```
- `buildMessageConfig`/`buildEditConfig` cast `markup` (e.g., `*tgbotapi.InlineKeyboardMarkup`) and return descriptive errors when unsupported types are passed.
- `SendWithContext`/`EditMessageTextWithContext` represent the existing client methods (or equivalents) that accept `context.Context`. If not available, add wrapper methods in `pkg/bot.Client` for context awareness.

### BotMessage Construction
```go
func toBotMessage(msg tgbotapi.Message) botport.BotMessage {
    return botport.BotMessage{
        ChatID:    msg.Chat.ID,
        MessageID: msg.MessageID,
        Transport: "telegram",
        Payload:   msg.Text,
        Meta: map[string]string{
            "markup_type": detectMarkup(msg),
            "raw_markup":  serializeMarkup(msg.ReplyMarkup),
        },
    }
}
```
- `detectMarkup` inspects `ReplyMarkup` to distinguish inline keyboards vs. none.
- `serializeMarkup` JSON-encodes inline keyboards for debugging (size-limited as needed).

### Error Wrapping
```go
func wrapTelegramError(op string, err error) error {
    if err == nil {
        return nil
    }
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        return &botport.BotError{Op: op, Code: "context_canceled", Wrapped: err}
    }
    code, retry := classifyTelegramError(err)
    return &botport.BotError{Op: op, Code: code, RetryAfter: retry, Wrapped: err}
}
```
- `classifyTelegramError` inspects Telegram API responses/HTTP codes (`Too Many Requests`, `Bad Request`, "message is not modified") and returns mapped codes: `rate_limited`, `bad_request`, `message_not_modified`, `transport_failure`, or `unknown`.
- Log every wrapped error with `op`, `chat_id`, `message_id` (if known), and `code`.

### Logging Hook
```go
type Logger interface {
    Printf(format string, args ...any)
}

func (a *Adapter) log(op string, attrs map[string]any) {
    if a.logger == nil {
        return
    }
    a.logger.Printf("botport op=%s attrs=%v", op, attrs)
}
```
- Call `log` after every send/edit success/failure to prepare for observability metrics without enforcing a specific logging framework.

### Application Wiring
- `main.go`:
  ```go
  client := bot.NewClient(cfg.BotToken)
  adapter := telegramadapter.New(client, logger)
  fsm := fsm.NewFSM(adapter, ...)
  ```
- Ensure any other package needing outbound messaging accepts `botport.BotPort` interfaces only.

### Testing Notes
- Add adapter unit tests mocking `bot.Client` to assert `BotMessage` fields and error codes.
- Use table-driven tests covering send, edit, markup conversions, and known error responses.

### Follow-up Dependency
- Adapter must expose all metadata required by slice 001c to populate `RenderContext.LastPrompt` and `AnswerContext.Message`. Keep constructor/API stable so future slices only update FSM wiring, not adapter interfaces.
