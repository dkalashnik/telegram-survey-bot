# BotPort Hexagonal Adapter Notes

Use these guidelines when implementing the BotPort abstraction described in PRP-001. They distill standard Hexagonal Architecture/ports-and-adapters principles plus Telegram-specific constraints.

## 1. Interface Shape
- Ports always accept `context.Context` as the first argument after the interface (e.g., `SendMessage(ctx, chatID, ...)`). Honor cancellation/timeouts so FSM callers can stop long-running API calls.
- Keep parameters transport-agnostic: use primitive Go types (int64, string, bool) and opaque `interface{}` for markup payloads. Concrete Telegram types (`tgbotapi.InlineKeyboardMarkup`) should only live in the Telegram adapter package.
- Return lightweight value objects (`BotMessage`, `BotError`) that expose the data the FSM needs without leaking adapter-specific structs.

## 2. Error Semantics
- Wrap Telegram API errors into a typed struct:
  ```go
  type BotError struct {
      Op         string
      Code       string // e.g. "bad_request", "rate_limited", "message_not_modified"
      RetryAfter time.Duration
      Wrapped    error
  }
  ```
- Use `errors.Is(err, context.Canceled)` and `context.DeadlineExceeded` to short-circuit retries.
- Critical codes to surface:
  - `message_not_modified` when editing identical content (FSM should ignore).
  - `rate_limited` and `retry_after` when Telegram instructs to back off.
  - `bad_request` for invalid payloads (FSM should log + force-exit).

## 3. Message Tracking
- Always return a `BotMessage` struct containing `ChatID`, `MessageID`, and optional `Meta` map for adapter hints. This lets the FSM persist message references (`state.UserState.LastBotMessage`) even if transport IDs differ.
- Include a `Transport` string ("telegram", "fake", etc.) so tests can assert they are using the expected adapter.

## 4. Fake Adapter Design
- Record every call in slices (e.g., `[]Call`) capturing method name, args, and timestamp.
- Provide helper assertions for tests: `func (f *FakePort) ExpectSend(t *testing.T, text string)` or `func (f *FakePort) LastMessage() BotMessage`.
- Allow scripted failures via `FailNext(op string, err error)` so tests can simulate rate limits.

## 5. Adapter Responsibilities
- Only adapters know about Telegram SDK structs; convert them to/from the port contracts within the adapter boundary.
- Keep adapters cohesive by splitting responsibilities:
  - `telegram/adapter.go` for BotPort implementation.
  - `telegram/updates.go` (or reuse existing `GetUpdatesChan`) to expose the update stream separately from outbound port methods.
- Avoid logging sensitive payloads (answers, metadata). Log operation names, chat IDs, and error codes only.

## 6. Hexagonal Mindset Checklist
- ✅ Domain (`pkg/fsm`, `pkg/fsm/questions`, `pkg/state`) depends on the port interface, never on `tgbotapi`.
- ✅ Infrastructure adapters (`pkg/bot/telegram`, `pkg/bot/fake`) implement the port and are injected at the application boundary (`main.go`).
- ✅ Tests can swap adapters without editing domain code.
- ✅ New transports (e.g., Slack) only implement the port and register themselves in `main.go`.

Use this doc whenever you add methods to the port or implement adapters so we keep the architecture coherent.
