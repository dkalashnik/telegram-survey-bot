name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |

---

## Goal

**Feature Goal**: Introduce a reusable Telegram BotPort adapter that wraps `pkg/bot.Client`, converts Telegram responses/errors into `botport` value objects, and is wired at the application boundary for future FSM adoption.

**Deliverable**: New `pkg/bot/telegramadapter` package (adapter + helpers + tests) plus `main.go` wiring that instantiates the adapter alongside the existing client, ready for downstream injection.

**Success Definition**: Adapter implements `botport.BotPort`, returns populated `botport.BotMessage` values, wraps errors via `botport.BotError`, exposes logging hooks, and `main.go` provides a constructor path for swapping adapters without touching FSM/state yet.

## User Persona (if applicable)

**Target User**: Repository maintainer/operator evolving bot transports.

**Use Case**: Maintainer refactors outbound messaging to rely on a single BotPort interface, enabling future fake adapters and multi-transport support.

**User Journey**:
1. Maintainer instantiates the Telegram adapter in `main.go`.
2. Adapter converts Telegram SDK responses to `botport.BotMessage`.
3. Future slices inject the adapter into FSM/state, enabling BotMessage hydration.

**Pain Points Addressed**:
- Eliminates direct Telegram dependencies for packages that should use ports.
- Provides consistent error codes/logging for future retry logic.
- Prepares for fake adapters/tests without rewriting Telegram-specific code.

## Why

- Aligns with PRP-001 vision of hexagonal architecture by isolating Telegram concerns.
- Unlocks `RenderContext.LastPrompt` / `AnswerContext.Message` wiring in slice 001c.
- Simplifies observability and future adapter swaps by centralizing BotPort logic.

## What

- Create `pkg/bot/telegramadapter` implementing `botport.BotPort`.
- Map Telegram message data into `botport.BotMessage` (`ChatID`, `MessageID`, `Transport`, `Payload`, `Meta`).
- Wrap Telegram errors via `botport.BotError` with normalized codes (`rate_limited`, `message_not_modified`, etc.).
- Provide lightweight logging hooks and helper builders for send/edit configs.
- Update `main.go` to instantiate the adapter (without yet refactoring FSM signatures).
- Document adapter behavior/usage in docs and PRP references.

### Success Criteria

- [ ] `pkg/bot/telegramadapter` builds and exports `Adapter` satisfying `botport.BotPort`.
- [ ] `go test ./pkg/bot/...` covers adapter happy/error paths (table-driven).
- [ ] `main.go` constructs the adapter and exposes it for future wiring (e.g., storing with FSM creator or context).
- [ ] Documentation mentions adapter location and metadata expectations (links in docs/system-overview.md).

## All Needed Context

### Context Completeness Check

✅ This PRP includes PRD/API-contract references, code patterns (bot client, FSM usage), documentation (system overview, question strategy), and commands so an agent without prior knowledge can implement slice 001b end-to-end.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-001b_bot-interface/prd-telegram-adapter.md
  why: Defines adapter scope, success metrics, diagrams.
  pattern: Follow sequence/state diagrams for send/edit flows.

- file: PRPs/PRP-001b_bot-interface/api-contract-telegram-adapter.md
  why: Lists integration points (bot package, main.go, future FSM), constructor signatures, error mapping.
  gotcha: Adapter must return `botport.BotMessage` values with transport metadata.

- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: Provides BotPort naming/error guidance (codes, RetryAfter semantics, transport hints).
  section: 1-3 (Interface shape, error semantics, message tracking).

# Code references
- file: pkg/bot/bot.go
  why: Existing Telegram client methods to wrap; observe send/edit helpers and error handling.
  pattern: Reuse client creation, avoid duplicating token/config logic.
  gotcha: `EditMessageText` already handles "message not modified" cases; adapter must surface this via BotError code.

- file: main.go
  why: Application entrypoint that loads config, instantiates `bot.Client`, wires FSM.
  pattern: The adapter must be constructed here and kept ready for future FSM injection.
  gotcha: Signal handling loop must remain untouched; only extend initialization block.

- file: pkg/fsm/fsm.go
  why: Demonstrates current dependencies on `*bot.Client`.
  pattern: Identify future wiring points but do not refactor signatures in slice 001b.

- file: pkg/fsm/fsm-record.go
  why: Contains `askCurrentQuestion`, `handleAnswer`, etc., referencing `bot.Client`; reveals metadata needed for BotMessage (message IDs, markup).
  gotcha: Keep these untouched now; ensure adapter metadata captures necessary info for slice 001c.

- file: docs/system-overview.md
  why: Architecture diagrams referencing `pkg/bot.Client`; update to mention adapter boundary.
  gotcha: Maintain consistent Mermaid diagrams.

- file: docs/question-strategy.md
  why: Mentions `pkg/bot.Client`; ensure references shift to `botport` + adapter to prevent drift.
  pattern: Update textual description only; diagrams can be reused.

# Test references
- file: pkg/fsm/questions/text_strategy_test.go
  why: Shows Go testing conventions (table-driven, `state.NewRecord()` usage).
  pattern: Mirror style when writing adapter tests (use `testing` package, `t.Fatalf`).

- file: pkg/fsm/questions/buttons_strategy_test.go
  why: Additional testing style reference (setup contexts, assertions).
  gotcha: Keep tests deterministic; no network calls.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── main.go
├── pkg
│   ├── bot
│   │   └── bot.go
│   ├── fsm
│   │   ├── fsm.go
│   │   ├── fsm-record.go
│   │   └── fsm-main.go
│   ├── ports
│   │   └── botport
│   │       └── botport.go
│   └── state
├── docs
│   ├── system-overview.md
│   └── question-strategy.md
└── PRPs
    ├── PRP-001b_bot-interface
    │   ├── prd-telegram-adapter.md
    │   └── api-contract-telegram-adapter.md
    └── ai_docs
        └── botport_hex_adapter.md
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
pkg/
  bot/
    telegramadapter/
      adapter.go          # Adapter struct + BotPort implementation, logging hooks, helpers.
      adapter_test.go     # Unit tests covering send/edit + error mapping using fake client stubs.
docs/
  system-overview.md      # Update architecture diagram/text to mention adapter boundary.
  question-strategy.md    # Reference botport + adapter for outbound messaging.
main.go                   # Instantiate adapter, retain existing FSM wiring for now.
```

### Known Gotchas of our codebase & Library Quirks

```python
# Go modules: gofmt/go test require access to ~/Library cache; run with elevated permissions if sandbox blocks writes.
# pkg/bot.Client.SendMessage/EditMessageText currently ignore context; adapter should wrap existing methods but accept ctx for future cancellation.
# Telegram API returns localized error strings; error mapping must use substring matching cautiously to avoid brittle checks.
# Strategy/unit tests rely on zero network access—adapter tests must stub the client (do not hit Telegram).
```

## Implementation Blueprint

### Data models and structure

- `botport.BotMessage`: Already defined; adapter must populate `ChatID`, `MessageID`, `Transport ("telegram")`, `Payload` (text), `Meta` (e.g., markup info).
- `botport.BotError`: Use helper `botport.NewBotError` and wrap Telegram errors with operation names (`send_message`, `edit_message`), codes, optional `RetryAfter`.
- Logger interface: define minimal interface (`Printf(format string, ...any)`) to avoid coupling to stdlib logger; default to nil no-op.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/bot/telegramadapter/adapter.go
  - IMPLEMENT: `Adapter` struct with `client *bot.Client`, `logger Logger`.
  - DEFINE: `Logger` interface (Printf) + helper `log(op string, fields map[string]any)`.
  - ADD: Constructors `New(client *bot.Client, logger Logger) (*Adapter, error)` validating inputs.
  - INCLUDE: Helper functions `buildSendConfig`, `buildEditConfig`, `toBotMessage`, `wrapTelegramError`.
  - FOLLOW: Patterns from pkg/bot/bot.go for message/edit config; use `botport.BotMessage`/`BotError` semantics from PRPs/ai_docs/botport_hex_adapter.md.
  - ENSURE: `var _ botport.BotPort = (*Adapter)(nil)` compile-time assertion.

Task 2: TEST pkg/bot/telegramadapter/adapter_test.go
  - IMPLEMENT: Fake `bot.Client` stub struct capturing send/edit calls and configurable responses/errors.
  - COVER: Happy paths (send/edit returning expected BotMessage fields) and error mappings (`message_not_modified`, `rate_limited`, generic errors).
  - USE: Table-driven tests mirroring style from pkg/fsm/questions/*_test.go.

Task 3: UPDATE main.go
  - IMPORT: `telegramsurveylog/pkg/bot/telegramadapter`.
  - INSTANTIATE: Adapter via `telegramadapter.New(botClient, log.Default())` (or wrap `log.Printf`), store for future FSM wiring (e.g., `adapter := telegramadapter.New(...); _ = adapter` placeholder with TODO referencing PRP-001c).
  - COMMENT: Link to PRP follow-up about populating `LastPrompt`/`Message`.
  - DO NOT: Change FSM signatures yet; ensure linter doesn't complain about unused adapter (use `_ = adapter` or pass to future hooking struct).

Task 4: UPDATE docs/system-overview.md & docs/question-strategy.md
  - DESCRIBE: Telegram adapter as the new outbound boundary; mention `pkg/ports/botport`.
  - UPDATE: Mermaid diagrams to include adapter node between FSM and Telegram API.
  - NOTE: Adapter returns BotMessage metadata used in later slices.

Task 5: OPTIONAL scaffolding for future wiring
  - ADD: Comments/TODOs referencing PRP-001c near FSM creation or state store to remind follow-up (per AGENTS planning guidance).
  - ENSURE: `PRPs/plans.md` already tracks follow-up; cross-reference if needed.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go fmt ./pkg/bot/telegramadapter ./main.go ./docs
go vet ./pkg/bot/telegramadapter
```

*(Note: gofmt/go vet may require elevated permissions due to Go build cache under `~/Library`.)*

### Level 2: Unit Tests (Component Validation)

```bash
go test ./pkg/bot/telegramadapter -v
```

### Level 3: Integration Testing (System Validation)

```bash
go test ./pkg/fsm -run Test -c  # ensures FSM still compiles (no behavior change)
```

*(Running `go test ./pkg/fsm` verifies adapter additions didn't break existing compilation paths.)*

### Level 4: Creative & Domain-Specific Validation

```bash
git grep -n "pkg/bot" main.go docs/system-overview.md docs/question-strategy.md
```

Confirm documentation references mention the adapter boundary and that only intentional files reference `pkg/bot` directly (adapter, bot package, legacy FSM awaiting slice 001c).

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./pkg/bot/telegramadapter ./main.go`
- [ ] `go vet ./pkg/bot/telegramadapter`
- [ ] `go test ./pkg/bot/telegramadapter -v`
- [ ] `go test ./pkg/fsm`

### Feature Validation

- [ ] Adapter satisfies `botport.BotPort` with compile-time assertion.
- [ ] `main.go` constructs adapter without altering FSM behavior.
- [ ] Documentation updated to describe adapter boundary and BotMessage metadata expectations.
- [ ] Follow-up for `LastPrompt`/`Message` hydration noted (already tracked in `PRPs/plans.md`).

### Code Quality Validation

- [ ] Adapter logs use structured key/value fields and avoid noisy output.
- [ ] Error mapping covers known Telegram responses (message not modified, rate limit).
- [ ] Tests cover success + failure cases without hitting network.

### Documentation & Deployment

- [ ] `docs/system-overview.md` & `docs/question-strategy.md` mention `pkg/bot/telegramadapter`.
- [ ] Comments reference PRPs/ai_docs/botport_hex_adapter.md for future contributors.
- [ ] No new environment variables introduced.

---

## Anti-Patterns to Avoid

- ❌ Do not duplicate Telegram token/config handling outside `pkg/bot`.
- ❌ Do not call Telegram APIs directly from adapter tests (use fakes).
- ❌ Do not modify FSM/state signatures in this slice.
- ❌ Do not ignore `botport.BotMessage` metadata requirements (populate Transport/Payload/Meta).

## Success Indicators

- ✅ Adapter unit tests provide confidence for send/edit operations.
- ✅ Main wiring path exists, making slice 001c a matter of injecting the adapter.
- ✅ Documentation, PRD, and API contract stay consistent with implementation.

## Confidence Score

8/10 — Scope is well-defined (adapter package + wiring). Residual risk lies in error classification correctness; mitigated via thorough tests and adherence to `botport_hex_adapter.md`.
