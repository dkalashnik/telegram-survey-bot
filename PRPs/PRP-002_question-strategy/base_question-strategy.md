name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |
  Implementation playbook for PRP-002 (question-type strategy registry). Guides restructuring of question handling,
  registry bootstrap, config validation wiring, and the migration of built-in types (text/buttons) with exhaustive validation.

---

## Goal

**Feature Goal**: Decouple question rendering/answer processing from the FSM by introducing a pluggable strategy registry so adding a new `QuestionConfig.Type` never requires editing `pkg/fsm` or `pkg/config`.

**Deliverable**: A `pkg/fsm/questions` package (interfaces, registry, contexts, built-in strategies, tests) plus updated FSM/config/docs wiring that resolve strategies at runtime while preserving existing Telegram UX.

**Success Definition**: `text`/`buttons` strategies live entirely under the new package, config validation fails fast for unknown types, FSM functions (`askCurrentQuestion`, `processAnswer`, callbacks) only interact through strategy interfaces, and `go test ./pkg/fsm/... ./pkg/config/... ./pkg/state/...` passes once the Go toolchain is available.

## User Persona (if applicable)

**Target User**: Operator/maintainer configuring surveys and occasionally adding new question behaviors via Go code. Respondents remain Telegram-only users.

**Use Case**: Operator adds a `multi_select` question in YAML and a corresponding Go strategy without touching FSM orchestration code.

**User Journey**:
1. Operator writes YAML referencing the new `type`.
2. Strategy registers during startup.
3. Config validation invokes the strategy to ensure options/schema correctness.
4. FSM resolves and executes strategy methods when prompting / processing answers.

**Pain Points Addressed**: Eliminates switch statements sprinkled across FSM/config, removes risk of forgetting to update answer handling, and enables fast experimentation with new question types through isolated strategies + tests.

## Why

- Aligns with SOLID by isolating question behavior into dedicated strategies and keeping `pkg/fsm` open for extension but closed for modification.
- Restores YAML-only authoring promise—operators extend surveys without risky FSM edits.
- Unlocks targeted unit tests for each question type, reducing regressions from manual telegraph flows.

## What

Implement a strategy registry keyed by `QuestionConfig.Type` that exposes validation, rendering, and answer-processing hooks. The FSM becomes an orchestrator that:
- resolves the active strategy for the current question,
- invokes `Render` to build prompt + markup,
- routes text/callback payloads to `HandleAnswer`,
- uses returned `Result` metadata to decide whether to advance, repeat, or exit sections.

`pkg/config.RecordConfig.Validate` must use the registry for type-specific checks so YAML errors surface before `main.go` starts polling Telegram. Built-in `text`/`buttons` strategies move out of FSM, and a new `BotPort` abstraction makes strategies unit-testable without the Telegram client.

### Success Criteria

- [ ] `askCurrentQuestion` + `processAnswer` contain no `switch question.Type` checks; they call strategies exclusively.
- [ ] Registry prevents duplicate registrations and panics/logs when resolving unknown types.
- [ ] New Go tests cover registry behavior and both built-in strategies (render + answer flows).
- [ ] Config validation errors cite section/question IDs via strategy-returned errors.

## All Needed Context

### Context Completeness Check

✅ Use the references below plus this PRP to pass the "No Prior Knowledge" rule—an implementing agent only needs repository access to follow these steps.

### Documentation & References

```yaml
# Product requirements
- file: PRPs/PRP-002_question-strategy/prd-question-strategy.md
  why: Captures the driver for registry refactor and success metrics (zero FSM edits per new type).
  pattern: Align terminology (registry, strategies, context structs) and metrics cited by stakeholders.
  gotcha: Visual diagrams reference ActionResult semantics—mirror them when naming result structs.

- file: PRPs/PRP-002_question-strategy/api-contract-question-strategy.md
  why: Defines integration contracts per touchpoint (config loader, FSM, bot/state).
  pattern: Reuse signature ideas for `RenderContext`, `AnswerContext`, and registry API.
  gotcha: Registry must be initialized before FSM handles updates—ensure bootstrap ordering in `main.go`.

# Internal architecture docs
- file: docs/system-overview.md
  why: Shows runtime flow (bot → FSM → state/config) and clarifies where the strategy registry slots in.
  pattern: Keep RecordConfig as single source of section/question metadata.
  gotcha: Mentioned promise that new question types should avoid Go changes—ensure PRP meets it.

- file: docs/fsm.md
  why: Details main/record FSM states/events that strategies must honor.
  pattern: Reference transitions triggered after `Result` is returned (EventAnswerQuestion vs EventSectionComplete).
  gotcha: `EventForceExit` is fail-safe; strategies must bubble fatal errors so FSM can trigger it.

# New AI doc (critical)
- docfile: PRPs/ai_docs/question_strategy_best_practices.md
  why: Summarizes registry/strategy design constraints, interface sketch, and external links for Looplab/Telegram APIs.
  section: Registry Design + Strategy Interfaces + Error Handling.

# Source files to inspect/follow
- file: pkg/fsm/fsm.go
  why: Current `handleMessage`, `handleCallbackQuery`, `processAnswer`, and `startOrResumeRecordCreation` functions show where question-type branching exists.
  pattern: Maintain logging structure when swapping in strategy calls.
  gotcha: `userState.Mu` is locked before entering these handlers—strategies must not block long-running operations.

- file: pkg/fsm/fsm-record.go
  why: `askCurrentQuestion` and record FSM callbacks currently build inline keyboards manually.
  pattern: Keep `LastMessageID` semantics when strategies request edits; centralize cancel button creation.
  gotcha: Telegram edit errors with "message is not modified" must still propagate message IDs—strategies should supply data, not perform edits themselves.

- file: pkg/fsm/const.go
  why: Houses callback prefixes/states that remain shared with strategies.
  pattern: Reuse prefixes for button payload encoding to avoid breaking callback handlers.

- file: pkg/config/config.go
  why: `RecordConfig.Validate` currently enforces `question.Type` via switch.
  pattern: Replace switches with `questions.Registry.MustGet(question.Type).Validate`.
  gotcha: Validation loops require section/question indices for error context—pass them into strategy validators.

- file: pkg/state/models.go
  why: `UserState` fields (CurrentSection, CurrentQuestion, CurrentRecord) feed into render/answer contexts.
  pattern: Provide these references when crafting `RenderContext`/`AnswerContext`.

- file: pkg/bot/bot.go
  why: Defines available Telegram helpers; strategies should target a `BotPort` interface exposing the subset they need.
  pattern: Keep edit/send semantics identical to avoid regressions.

- file: record_config.yaml
  why: Baseline dataset for manual validation; ensures text/buttons strategies are feature-parity after migration.
  pattern: Use existing sections when writing tests or manual runbooks.

- file: main.go
  why: Entry point to register built-in strategies before polling updates.
  pattern: After `config.LoadConfig`, call `questions.MustRegisterBuiltin()` to avoid nil registry usage.

# External references (retain URLs for context window)
- url: https://pkg.go.dev/github.com/looplab/fsm#FSM.Event
  why: Shows how FSM events accept variadic args; strategies returning `Result` must align with existing transition triggers.
  critical: Misaligned args cause panics—use this signature to pass contexts.

- url: https://pkg.go.dev/github.com/go-telegram-bot-api/telegram-bot-api/v5#InlineKeyboardMarkup
  why: Strategies render inline keyboards; this API reference ensures markup structs are populated correctly.
  critical: Mistmatching button payloads break callbacks, so double-check fields.

- url: https://refactoring.guru/design-patterns/strategy/go/example
  why: Provides Go-specific strategy implementations with registries.
  critical: Mirrors how to keep strategies cohesive and unit-testable.

- url: https://pkg.go.dev/sync#RWMutex
  why: Registry must guard concurrent reads/writes.
  critical: Use read locks for lookups to avoid contention when concurrent updates arrive from Telegram.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── AGENTS.md
├── Makefile
├── PRPs
│── ├── PRP-002_question-strategy
│── ├── plans.md
│── └── tempates
├── README.md
├── docker-compose.yml
├── docs
│   ├── fsm.md
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
  fsm/
    questions/
      registry.go              # thread-safe registration + lookup helpers (MustRegister, Get, MustGet, ResetForTests)
      strategy.go              # QuestionStrategy interface, Result struct, BotPort abstraction, factory helpers
      context.go               # RenderContext & AnswerContext structs wrapping bot/state/config references
      prompt.go                # PromptSpec struct (text, markup, forceNew bool) consumed by askCurrentQuestion
      text_strategy.go         # Implements QuestionStrategy for `type: text`
      buttons_strategy.go      # Implements QuestionStrategy for `type: buttons`, builds inline keyboard payloads
      strategy_test.go         # Table-driven tests for text/buttons strategies and registry behaviors
  docs/
    fsm.md                    # Updated diagrams referencing strategy invocation steps
    system-overview.md        # Updated runtime flow referencing questions registry
main.go                       # Calls questions.RegisterBuiltins() before FSM wiring
pkg/config/config.go          # Uses registry validators instead of switch statements
pkg/fsm/fsm.go                # handleMessage/callback/answer functions delegate to strategies
pkg/fsm/fsm-record.go         # askCurrentQuestion uses PromptSpec from strategies
pkg/fsm/const.go              # (Optional) relocate callback ID helpers to share with strategy package
record_config.yaml            # (Optional) add commented example for new type to validate config path
```

### Known Gotchas of our codebase & Library Quirks

```python
# CRITICAL: state.UserState.Mu is locked before handlers run; never call into long-running goroutines from strategies.
# CRITICAL: Telegram edit errors often return "message is not modified" — keep existing fallback that reuses message IDs.
# Looplab FSM requires that every Event(...) call receives the arguments each transition expects; missing args panic.
# record_config.yaml is loaded once at startup; registry must be initialized before config.Validate executes.
# go version 1.24 is declared, but current dev container lacks stdlib headers—document this when prescribing go test commands.
```

## Implementation Blueprint

### Data models and structure

```python
# Core interfaces & structs under pkg/fsm/questions:
- type BotPort interface { SendMessage(...); EditMessageText(...); AnswerCallback(...); } // subset of pkg/bot.Client
- type QuestionStrategy interface {
      Name() string
      Validate(question config.QuestionConfig, sectionID string) error
      Render(ctx RenderContext) (PromptSpec, error)
      HandleAnswer(ctx AnswerContext, incoming string) (Result, error)
  }
- type RenderContext struct { Bot BotPort; ChatID int64; MessageID int; UserState *state.UserState; Section config.SectionConfig; Question config.QuestionConfig }
- type AnswerContext struct { RenderContext; CallbackID string; Update *tgbotapi.Update }
- type Result struct { Advance bool; Repeat bool; CompleteSection bool; Telemetry string; ErrMsg string }
- type PromptSpec struct { Text string; Keyboard *tgbotapi.InlineKeyboardMarkup; ForceNew bool }
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/fsm/questions/registry.go
  - IMPLEMENT: thread-safe map + `MustRegister`, `Get`, `MustGet`, `ResetForTests`.
  - FOLLOW pattern: use sync.RWMutex (see https://pkg.go.dev/sync#RWMutex) and panic on duplicate registrations to fail fast.
  - NAMING: exported `RegisterBuiltinStrategies(botPort BotPort)` helper invoked from `main.go`.

Task 2: CREATE pkg/fsm/questions/strategy.go & context.go
  - IMPLEMENT: `QuestionStrategy`, `PromptSpec`, `Result`, `RenderContext`, `AnswerContext`, `BotPort`.
  - FOLLOW pattern: see PRPs/ai_docs/question_strategy_best_practices.md for exact method suggestions.
  - DEPENDENCIES: import `pkg/bot`, `pkg/config`, `pkg/state`, `tgbotapi`.

Task 3: CREATE pkg/fsm/questions/text_strategy.go
  - IMPLEMENT: text strategy translating prompts to `PromptSpec`, storing answers in `ctx.Record.Data`, validating `store_key`.
  - FOLLOW pattern: replicate logic from `handleMessage`/`processAnswer` for text questions and add dedicated unit tests.
  - PLACEMENT: register via `init()` or `RegisterBuiltinStrategies`.

Task 4: CREATE pkg/fsm/questions/buttons_strategy.go
  - IMPLEMENT: build inline keyboard using `question.Options`, encode callback data using `CallbackAnswerPrefix`, and parse callback payloads.
  - FOLLOW pattern: former logic in `askCurrentQuestion` for button rows; ensure validation checks `options`.
  - GOTCHA: need helper to verify callback question ID matches `userState.CurrentQuestion`.

Task 5: MODIFY pkg/fsm/fsm-record.go::askCurrentQuestion
  - INTEGRATE: instantiate `RenderContext` (chatID, section config, question, last message ID) and call `strategy.Render`.
  - FOLLOW pattern: re-use existing send/edit branching but base decisions on returned `PromptSpec.ForceNew`.
  - ADD: fallback log when `registry.Get` returns nil -> trigger `EventForceExit`.

Task 6: MODIFY pkg/fsm/fsm.go::handleMessage / handleCallbackQuery / processAnswer
  - INTEGRATE: route answers to strategies via new helper `questions.HandleAnswer(ctx, payload string)`; only FSM decides which event to fire based on `Result`.
  - FOLLOW pattern: reuse logging + `EventForceExit` on fatal errors; auto-handle misaligned question IDs.
  - NOTE: text answers still require ignoring input when strategy expects buttons—strategy should emit `Result.Repeat`.

Task 7: MODIFY pkg/config/config.go::(*RecordConfig).Validate
  - INTEGRATE: fetch strategy by `question.Type`, call its `Validate` with section/question context.
  - PRESERVE: duplicate `store_key` detection + base validations before delegating to strategies.

Task 8: MODIFY main.go (bootstrap)
  - ADD: import `pkg/fsm/questions` and register built-in strategies after loading config but before processing updates.
  - FOLLOW pattern: keep log statements so startup output shows registry readiness.

Task 9: ADD TESTS under pkg/fsm/questions
  - FILES: `registry_test.go`, `text_strategy_test.go`, `buttons_strategy_test.go`.
  - COVER: duplicate registration panic, validation error cases, rendering markup and answer routing for success/error flows.
  - USE: fake BotPort / fake user state to avoid Telegram calls.

Task 10: UPDATE docs + sample config
  - MODIFY: docs/fsm.md + docs/system-overview.md to mention strategy registry; README extensibility section referencing new workflow.
  - OPTIONAL: add commented `multi_select` example in record_config.yaml to show how third strategy would be added.
```

## Validation Gates

### Level 1: Static Analysis & Formatting

```bash
go fmt ./...
go vet ./...
golangci-lint run ./...    # if installed locally; otherwise document why skipped
```

> Notes:
> 1. The repository does not yet contain any `_test.go` files, so `go test ./...` is a no-op today. Part of this PRP is to introduce unit tests under `pkg/fsm/questions`, making the command meaningful.
> 2. Inside this dev container the Go stdlib headers are missing (`package context is not in std`), so once tests exist, rerun `go test ./...` on a workstation with a full Go toolchain.

### Level 2: Unit Tests (Component Validation)

```bash
go test ./pkg/fsm/questions/... ./pkg/config ./pkg/fsm
gotest -run 'Strategy|Registry' ./pkg/fsm/questions   # optional focused loop
```

### Level 3: Integration Testing (System Validation)

```bash
TELEGRAM_BOT_TOKEN=dummy go run ./main.go &
sleep 3
# Manually send /start via Telegram sandbox bot or mock updates using go-telegram-bot-api's Test helpers.
# Verify text + button flows still work end-to-end by walking through record creation.
```

### Level 4: Domain/UX Validation

```bash
# Run `make logs` (if docker-compose stack is up) to observe FSM/state logging for at least one successful record.
# Validate new README section by onboarding another developer—confirm they can add a mock strategy following docs.
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./...`, `go vet ./...`, and `go test ./pkg/fsm/questions ./pkg/fsm ./pkg/config` pass on a workstation with Go stdlib installed.
- [ ] Registry panics only during startup—runtime lookups return descriptive errors instead.
- [ ] No lingering `switch question.Type` statements outside `pkg/fsm/questions`.

### Feature Validation

- [ ] Manual Telegram runthrough: text and button questions behave exactly as before.
- [ ] Config validation errors cite section/question IDs when type-specific validation fails.
- [ ] Operator documentation includes steps for adding a new strategy.

### Code Quality Validation

- [ ] New package follows Go naming conventions and keeps responsibilities focused per SOLID/KISS instructions.
- [ ] Tests cover both success and failure flows for strategies/registry.
- [ ] Docs (docs/fsm.md, docs/system-overview.md, README) updated to reflect new architecture.

### Documentation & Deployment

- [ ] `PRPs/ai_docs/question_strategy_best_practices.md` remains referenced for future contributors.
- [ ] README adds short "Extending question types" subsection (YAML + Go steps).
- [ ] Commit/PR body mentions commands run (`go test ./...`, etc.) and highlights strategy registry introduction.

---

**Confidence Score**: 8/10 — All touchpoints and docs are mapped, but successful validation depends on restoring a working Go toolchain so `go test ./...` can actually run.
