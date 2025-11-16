name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |
  Implementation playbook for PRP-001 (Bot interface abstraction). Promotes a first-class
  BotPort contract, adapters (Telegram + fake), FSM/state rewiring, and documentation so
  future transports plug in without touching core logic.

---

## Goal

**Feature Goal**: Decouple every outbound Telegram interaction from concrete `pkg/bot.Client` usage by exposing a shared `botport.BotPort` interface consumed across FSM layers, question strategies, and tests.

**Deliverable**: `pkg/ports/botport` package (interface, message/value objects, typed errors), Telegram + fake adapters under `pkg/bot`, refactored FSM/question/state code that only depends on the port, and refreshed docs describing the contract.

**Success Definition**: `git grep "pkg/bot" pkg/fsm pkg/state` returns zero matches outside adapter wiring, FSM helper signatures accept `botport.BotPort`, state stores the last `BotMessage`, strategies alias the shared port, fake adapters unblock integration tests, and `go test ./pkg/...` passes with a fake BotPort harness.

## User Persona (if applicable)

**Target User**: Maintainer/operator who configures surveys, extends question logic, and needs deterministic tests; QA automation harness author; future adapter implementers (Slack/WebApp/etc.).

**Use Case**: Operator swaps between real Telegram and fake BotPort during CI to validate FSM flows; QA writes integration tests by injecting the fake port with scripted responses; adapter authors implement Slack transport without touching FSM code.

**User Journey**:
1. Maintainer adds env flag (`BOT_PORT=fake`) and runs `go test ./pkg/fsm/...`—tests inject the fake port and walk FSM transitions headlessly.
2. Production startup wires `telegram.Adapter` → FSM → state; all flows route through the interface.
3. When Telegram returns `message_not_modified`, FSM logging helper interprets the code and suppresses user-visible errors.
4. QA adds a Slack adapter later by implementing the same interface and registering it in `main.go`.

**Pain Points Addressed**: Eliminates direct Telegram SDK usage in FSM/state packages, removes duplicated send/edit logic, enables deterministic tests, simplifies future transport swaps, and makes error handling consistent.

## Why

- Aligns with SOLID/KISS mandates from AGENTS.md—core logic depends on abstractions, not concrete clients.
- Unlocks fake adapters so headless FSM/integration tests do not require network tokens.
- Makes future transports (Slack, CLI, HTTP) implementable without modifying FSM/question/state packages.
- Centralizes error semantics (rate limits, "message not modified", callback ACK requirements) to avoid regressions scattered across handlers.

## What

Introduce `pkg/ports/botport` with `BotPort`, `BotMessage`, and `BotError`. Refactor `pkg/fsm`, `pkg/state`, and `pkg/fsm/questions` to depend on this port. Implement a Telegram adapter that wraps the existing client plus a fake adapter for tests. Update wiring in `main.go`, update docs, and provide validation guidance so new adapters adhere to the contract.

### Success Criteria

- [ ] `pkg/fsm` compiles without importing `pkg/bot`; only adapters refer to `tgbotapi`.
- [ ] `state.UserState` stores the last `BotMessage`, enabling edits/pins without Telegram structs.
- [ ] Fake adapter records calls and is used in new FSM integration tests.
- [ ] Docs (`docs/system-overview.md`, `docs/question-strategy.md`) explain the BotPort boundary.
- [ ] Validation commands (`go test ./pkg/...`, `go vet ./pkg/...`, `go fmt ./...`) succeed.

## All Needed Context

### Context Completeness Check

✅ The references below plus this PRP satisfy the "No Prior Knowledge" rule—an implementing agent only needs repository access to proceed.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-001_bot-interface/prd-bot-interface.md
  why: Baseline problem statement, success metrics, diagrams for the BotPort initiative.
  pattern: Use listed phases (interface, adapters, refactor, tests, docs) as implementation roadmap.
  gotcha: Success metrics demand zero concrete `bot.Client` references outside adapter code—verify via `git grep`.

- file: PRPs/PRP-001_bot-interface/api-contract-bot-interface.md
  why: Defines integration points, interface fields, adapter expectations, and wiring lifecycle already aligned with stakeholders.
  pattern: Reuse the BotMessage/BotError structure and AdapterFactory guidance verbatim.
  gotcha: Contract mandates context-aware methods and standardized error codes.

# Internal system docs
- file: docs/system-overview.md
  why: Shows runtime flow (Telegram → bot.Client → FSM → state/config) and highlights where BotPort replaces direct client usage.
  pattern: Update diagrams to swap `pkg/bot.Client` with `botport.BotPort`.
  gotcha: Flow expects main.go to wire FSM after config load—respect ordering when injecting adapters.

- file: docs/fsm.md
  why: Enumerates main + record FSM states/events consumed by handlers.
  pattern: Ensure new BotPort-aware callbacks maintain event argument ordering documented here.
  gotcha: `EventForceExit` is the safety net; BotPort errors must bubble up so this path can fire.

- file: docs/question-strategy.md
  why: Captures current BotPort subset strategies rely on; needs update to mention shared port package.
  pattern: Keep `PromptSpec` semantics intact while swapping the underlying port alias.
  gotcha: Strategies must remain transport-agnostic—BotPort injection happens via contexts.

# Key source files
- file: main.go
  why: Entry point currently instantiates `bot.Client` and feeds updates to FSM.
  pattern: Replace direct client usage with adapter factory wiring + context cancellation.
  gotcha: Shutdown goroutine cancels `context.Context`; adapters must honor it when sending actions.

- file: pkg/bot/bot.go
  why: Current Telegram client wrapper; will host or delegate to the adapter implementation.
  pattern: Reuse Send/Edit/Typing logic while wrapping return values into `botport.BotMessage` and errors into `BotError`.
  gotcha: Keep "message is not modified" handling centralized to avoid duplicated string checks.

- file: pkg/fsm/fsm.go
  why: Handles updates, messages, callbacks, and answer routing.
  pattern: Update every function signature to accept `botport.BotPort`, propagate contexts, and centralize error handling.
  gotcha: Functions run with `UserState.Mu` locked; avoid blocking calls when adding typing indicators.

- file: pkg/fsm/fsm-record.go
  why: Contains `enterSelectingSection`, `askCurrentQuestion`, `enterRecordIdle`, etc. heavy on Telegram usage.
  pattern: Swap message send/edit logic with BotPort methods and store returned `BotMessage` for future edits.
  gotcha: Inline keyboard fallbacks currently check `strings.Contains(err, "message is not modified")`; convert this to `BotError.Code` comparisons.

- file: pkg/fsm/fsm-main.go
  why: Builds reply keyboard flows (sendMainMenu, list navigation, share record).
  pattern: Ensure reply keyboard removal/typing indicators use BotPort methods to keep UX parity.
  gotcha: viewListHandler edits messages conditionally—BotPort must support no-op markup removal.

- file: pkg/state/models.go
  why: Defines `UserState`/`Record` with only `LastMessageID` today.
  pattern: Add `LastBotMessage botport.BotMessage` (or pointer) and migrate code to use it for future edits/pins.
  gotcha: Update constructors/tests to initialize the struct; keep mutex semantics unchanged.

- file: pkg/config/config.go
  why: Question validation currently allows direct bot usage fallback when no validator exists.
  pattern: Ensure registry still registers validators before config load; BotPort introduction must not break startup ordering.
  gotcha: Config validation runs before Telegram wiring; keep dependencies minimal.

# AI doc (new)
- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: Summarizes Hexagonal Architecture best practices, error semantics, fake adapter guidance.
  section: All

# External references
- url: https://pkg.go.dev/github.com/go-telegram-bot-api/telegram-bot-api/v5#BotAPI.Send
  why: Confirms message send/edit configs (reply markup types, parse mode) that adapters must wrap.
  critical: Telegram returns `Bad Request: message is not modified`—map to BotError `Code: "message_not_modified"`.

- url: https://pkg.go.dev/context#Context
  why: BotPort methods must honor cancellation/deadlines when FSM cancels workflows.
  critical: Always pass the incoming `ctx` from FSM handlers; avoid `context.Background()` inside adapters.

- url: https://martinfowler.com/bliki/HexagonalArchitecture.html
  why: Reinforces ports-and-adapters patterns relevant to this refactor.
  critical: Domain (FSM/question/state) must not import adapter packages—validate via go list/go vet.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── AGENTS.md
├── Makefile
├── PRPs
│   ├── PRP-001_bot-interface
│   ├── PRP-002_question-strategy
│   ├── ai_docs
│   ├── plans.md
│   └── tempates
├── README.md
├── docker-compose.yml
├── docs
│   ├── fsm.md
│   ├── question-strategy.md
│   └── system-overview.md
├── go.mod
├── go.sum
├── main.go
├── pkg
│   ├── bot
│   ├── config
│   ├── fsm
│   └── state
└── record_config.yaml
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
pkg/
  ports/
    botport/
      botport.go            # BotPort interface, BotMessage struct, BotError type, helper utilities (NewBotError, IsCode).
  bot/
    client.go               # Telegram client creation + update polling (refactored from existing bot.go).
    telegram_adapter.go     # Implements botport.BotPort by wrapping client methods (send/edit/pin/typing/keyboard removal).
    fake/
      fake_port.go          # Deterministic BotPort implementation for tests (records calls, programmable failures).
  fsm/
    fsm.go                  # Updated to accept botport.BotPort everywhere.
    fsm-main.go             # Main-menu helpers rewritten to use the port.
    fsm-record.go           # Section/question callbacks and exit flows consume BotPort + BotMessage.
    const.go                # (Existing) callback prefixes remain shared.
  state/
    models.go               # Adds LastBotMessage, helper methods for storing message refs.
  fsm/questions/
    strategy.go             # Type alias (type BotPort = botport.BotPort), contexts use botport.BotMessage.
    registry.go             # No direct bot imports; ensures validator setup unchanged.
  docs/
    system-overview.md      # Updated diagrams referencing BotPort and adapters.
    question-strategy.md    # Describes new shared port + fake adapter testing guidance.
PRPs/ai_docs/
  botport_hex_adapter.md    # Already added—reference in PRP and future documentation.
```

### Known Gotchas of our codebase & Library Quirks

```python
# CRITICAL: FSM handlers execute while state.UserState.Mu is locked—BotPort calls must remain quick; avoid long retries.
# CRITICAL: Telegram edit errors return "Bad Request: message is not modified"; convert to BotError.Code="message_not_modified" so FSM can skip retries.
# Telegram requires AnswerCallback within ~1s; adapters must keep existing Ack behavior to avoid client-side "loading" spinners.
# Looplab FSM requires every Event(...) call to include the expected args (userState, botPort, config, chatID, messageID,...). Missing args panic.
# record_config.yaml loads once on startup; strategies must already be registered (RegisterBuiltins) before LoadConfig() runs.
# `state.UserState.LastMessageID` drives keyboard edits—migrate carefully so existing flows keep editing the same message when ForceNew=false.
```

## Implementation Blueprint

### Data models and structure

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

type BotError struct {
    Op         string
    Code       string
    RetryAfter time.Duration
    Wrapped    error
}

func (e BotError) Error() string { return fmt.Sprintf("botport %s: %s", e.Op, e.Wrapped) }
func IsCode(err error, code string) bool { /* unwrap + compare */ }

// BotPort abstracts send/edit/etc.
type BotPort interface {
    SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (BotMessage, error)
    EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (BotMessage, error)
    AnswerCallback(ctx context.Context, callbackID string, text string) error
    RemoveReplyKeyboard(ctx context.Context, chatID int64, text string) (BotMessage, error)
    SendTyping(ctx context.Context, chatID int64) error
    PinMessage(ctx context.Context, chatID int64, messageID int, silent bool) error
    UnpinMessage(ctx context.Context, chatID int64, messageID int) error
}

// state.UserState gains
 type UserState struct {
     ...
     LastBotMessage botport.BotMessage
 }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/ports/botport/botport.go
  - IMPLEMENT: BotMessage, BotError, helper constructors, and BotPort interface exactly as defined in the API contract.
  - FOLLOW pattern: PRPs/PRP-001_bot-interface/api-contract-bot-interface.md (Interface & Types section) for method signatures + comments.
  - NAMING: package botport; exported types BotPort, BotMessage, BotError, ErrCodeMessageNotModified constant for reuse.
  - NOTE: Provide helpers `func WrapError(op, code string, err error) error` and `func MessageFromTelegram(msg tgbotapi.Message)` stub for adapters.

Task 2: SPLIT pkg/bot/bot.go into client + adapter files
  - IMPLEMENT: `client.go` with `type Client struct` + `GetUpdatesChan`, `NewClient` (existing logic minus port interface concerns).
  - CREATE `telegram_adapter.go` implementing `botport.BotPort` using the client; include context usage, `BotMessage` conversions, and typed errors.
  - FOLLOW pattern: docs/system-overview.md (message lifecycle) to ensure Send/Edit fallbacks stay identical.
  - GOTCHA: Map "message is not modified" → `BotError{Code: ErrCodeMessageNotModified}` and reuse across all call sites.

Task 3: CREATE pkg/bot/fake/fake_port.go
  - IMPLEMENT: Struct storing slices of calls, fulfilling BotPort for tests.
  - ADD: Helpers `ExpectSend`, `PopCall`, `FailNext(op string, err error)` so tests can assert flows.
  - PATTERN: Mirror best practices from PRPs/ai_docs/botport_hex_adapter.md.
  - NOTE: Provide constructor `func NewFake() *FakePort` returning channels for unit tests.

Task 4: ADD adapter factory + wiring helpers
  - FILE: pkg/bot/adapter_factory.go (or similar) exposing `func NewPort(token string) (botport.BotPort, *Client, error)` and optional fake selection via env (`BOT_PORT=fake`).
  - MAIN: Update `main.go` to parse env flag, instantiate Telegram adapter + update channel, pass only BotPort into FSM handlers.
  - ENSURE: Shutdown path closes update channel and cancels context gracefully.

Task 5: REFRESH pkg/fsm/questions to use shared port
  - MODIFY: `strategy.go` to import `pkg/ports/botport` and set `type BotPort = botport.BotPort`; contexts carry `botport.BotMessage` instead of raw IDs.
  - UPDATE: Strategy render/answer code to store `ctx.Bot` typed references without referencing `pkg/bot`.
  - VERIFY: Tests compile without `tgbotapi` exposures (buttons strategy can keep markup building locally).

Task 6: REFRACTOR pkg/state models/store
  - MODIFY: `state.UserState` to include `LastBotMessage botport.BotMessage`, adjust getters/setters, ensure `NewStore` zero-values remain safe.
  - UPDATE: Everywhere `LastMessageID` was used (`askCurrentQuestion`, `enterRecordIdle`, `viewListHandler`) to leverage the new struct.
  - DOCUMENT: Comments clarifying that `LastBotMessage` is used for edits/pin/unpin.

Task 7: UPDATE pkg/fsm (fsm.go, fsm-record.go, fsm-main.go)
  - CHANGE signatures: `HandleUpdate(ctx context.Context, update tgbotapi.Update, bot botport.BotPort, ...)` and propagate through helper calls.
  - REPLACE direct `botClient.SendMessage/EditMessageText/AnswerCallback` with BotPort methods.
  - ADD helper `func handleBotError(err error) (skip bool)` mapping BotError codes for reuse.
  - STORE returned `BotMessage` in `userState.LastBotMessage` after prompts/menus.
  - ENSURE: `hideKeyboard`, `viewListHandler`, `enterRecordIdle` support ForceNew/edit fallback using message metadata.

Task 8: UPDATE docs + config references
  - UPDATE: `docs/system-overview.md` sequence/mermaid diagrams to show BotPort.
  - UPDATE: `docs/question-strategy.md` to mention shared port + fake adapter.
  - OPTIONAL: Add `docs/botport.md` summarizing adapter usage (if PRD expects).

Task 9: ADD/UPDATE tests
  - CREATE: `pkg/bot/fake/fake_port_test.go` verifying call recording + failure injection.
  - ADD: FSM-level tests under `pkg/fsm` using FakePort to simulate `/start`, section selection, button answers (mirror PRD success metrics of ≥5 flows).
  - RUN: `go test ./pkg/...` ensuring new tests cover BotPort error handling and message tracking.
```

### Implementation Patterns & Key Details

```go
// Centralized send helper to keep FSM concise
type botMessenger interface { botport.BotPort }

func sendPrompt(ctx context.Context, bot botMessenger, chatID int64, text string, markup interface{}, userState *state.UserState) {
    msg, err := bot.SendMessage(ctx, chatID, text, markup)
    if err != nil {
        if botport.IsCode(err, botport.ErrCodeMessageNotModified) {
            log.Printf("prompt already up-to-date (chat %d)", chatID)
            userState.LastBotMessage.MessageID = msg.MessageID // retain previous ID
            return
        }
        handleBotError(ctx, err, userState)
        return
    }
    userState.LastBotMessage = msg
}

// handleBotError translates bot errors into FSM actions
func handleBotError(ctx context.Context, err error, userState *state.UserState) {
    var bErr *botport.BotError
    if errors.As(err, &bErr) {
        switch bErr.Code {
        case botport.ErrCodeRateLimited:
            time.Sleep(bErr.RetryAfter)
        case botport.ErrCodeMessageNotModified:
            return
        default:
            log.Printf("bot error (%s): %v", bErr.Code, bErr.Wrapped)
        }
    } else if errors.Is(err, context.Canceled) {
        log.Printf("context canceled before sending to chat %d", userState.UserID)
    }
}

// Fake adapter pattern for tests
func TestRecordFlow(t *testing.T) {
    fake := fakebot.New()
    store := state.NewStore(fsm.NewFSMCreator())
    // inject fake into handleUpdate and assert fake.Calls slices.
}
```

### Integration Points

```yaml
CONFIG:
  - env: `BOT_PORT` (default "telegram", optional "fake"). Document in README + PRP.
  - env: existing `TELEGRAM_BOT_TOKEN` remains required for real adapter.

DOCS:
  - docs/system-overview.md: update diagrams + narrative to mention BotPort + adapters.
  - docs/question-strategy.md: mention BotPort shared package + fake testing guidance.
  - PRPs/ai_docs/botport_hex_adapter.md: already published; cite from docs if needed.

STATE:
  - Add `LastBotMessage` to `state.UserState` and ensure migrations update referencing code.

TESTS:
  - Introduce `pkg/fsm/tests/botport_integration_test.go` (or similar) using the fake port to assert `/start` → section selection → answer flows.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Format everything (required by repository guidelines)
go fmt ./...

# Lint for obvious issues (Go vet suffices in current repo)
go vet ./pkg/...
```

### Level 2: Unit Tests (Component Validation)

```bash
# Strategy + BotPort unit tests
go test ./pkg/fsm/questions ./pkg/bot/...

# FSM tests leveraging fake adapter
go test ./pkg/fsm -run BotPort -v
```

### Level 3: Integration Testing (System Validation)

```bash
# Full package tests (once stdlib/toolchain available)
go test ./pkg/... ./...

# Manual smoke: run bot with fake port to keep tests deterministic
env BOT_PORT=fake go run ./main.go &
sleep 2 && kill $!
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Verify no direct bot.Client usage remains outside adapters
git grep "pkg/bot" pkg/fsm pkg/state

# Ensure adapters honor context cancellation (optional race detector)
go test -race ./pkg/bot/...

# Rebuild docs if diagrams changed (manual check via markdown preview)
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./...` run with no diffs.
- [ ] `go vet ./pkg/...` passes.
- [ ] `go test ./pkg/...` succeeds (fake port tests included).
- [ ] `git grep "pkg/bot" pkg/fsm pkg/state` returns zero matches.

### Feature Validation

- [ ] Success criteria met (FSM uses BotPort, state stores BotMessage, fake tests exist, docs updated).
- [ ] Manual smoke with fake port completes `/start` → section menu → button answer.
- [ ] Telegram adapter logs include operation + error codes without leaking payloads.

### Code Quality Validation

- [ ] Naming + placement follow desired tree (ports in `pkg/ports`, adapters in `pkg/bot`).
- [ ] New files include succinct comments explaining non-obvious logic.
- [ ] `state.UserState` mutex usage unchanged (no new race conditions).

### Documentation & Deployment

- [ ] README references BotPort env toggle if necessary.
- [ ] Docs (system-overview, question-strategy) updated to mention BotPort + adapters.
- [ ] `PRPs/ai_docs/botport_hex_adapter.md` linked from docs/PRP for future contributors.

---

## Confidence Score

8/10 — The PRP references every touchpoint (code, docs, env) plus external patterns. Remaining risk lies in broad FSM refactor touching many files; mitigated by fake adapter tests and the validation plan.
