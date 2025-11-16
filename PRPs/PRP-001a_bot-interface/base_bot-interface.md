name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |
  Implementation playbook for PRP-001a (BotPort foundation). Guides creation of
  the shared `pkg/ports/botport` package plus the strategy refactor so the
  codebase depends on a single transport abstraction before adapters are added
  in later slices.

---

## Goal

**Feature Goal**: Consolidate the BotPort abstraction into `pkg/ports/botport` and refactor `pkg/fsm/questions` to depend on it exclusively, eliminating duplicate interfaces and preparing downstream packages for adapter work.

**Deliverable**: New `pkg/ports/botport` package (BotPort interface, BotMessage struct, BotError scaffolding) plus updated strategy code/tests that import the shared types without pulling in Telegram structs.

**Success Definition**: `pkg/fsm/questions` no longer defines a local BotPort, strategy contexts/tests compile using `pkg/ports/botport`, and `go test ./pkg/fsm/questions` passes with zero `pkg/bot` imports.

## User Persona (if applicable)

**Target User**: Maintainer/operator evolving survey logic while keeping the codebase consistent and simple.

**Use Case**: Contributor updates question strategies or adds new question types and expects a single BotPort contract without digging into Telegram-specific types.

**User Journey**:
1. Maintainer implements new strategy features referencing `botport.BotPort`.
2. Strategy tests stub BotPort without touching `pkg/bot`.
3. Later PRPs plug adapters/fakes into the same interface, reusing this groundwork.

**Pain Points Addressed**: Removes duplicated interfaces, keeps dependencies clear (strategies only depend on ports/state/config), and unlocks future adapters/tests without additional churn.

## Why

- Duplicated BotPort definitions create divergence and hinder future adapter work.
- Consistency reduces dependency sprawl (strategies should not import `pkg/bot`).
- Establishing BotMessage/BotError now lets later slices reuse the same value objects.

## What

Introduce a canonical `pkg/ports/botport` package with interface + value objects, update `pkg/fsm/questions` to import it (via type alias), and ensure tests rely on the shared port. Keep Telegram wiring untouched for now; later PRPs expand the interface/adapters.

### Success Criteria

- [ ] `pkg/fsm/questions/strategy.go` imports `pkg/ports/botport` and removes its local interface definition.
- [ ] `RenderContext`/`AnswerContext` gain `botport.BotMessage` fields (even if zero-values initially).
- [ ] Strategy tests compile without referencing `pkg/bot`.
- [ ] `pkg/ports/botport` builds and is the only place defining `BotPort`.

## All Needed Context

### Context Completeness Check

✅ Use the references below plus this PRP to pass the "No Prior Knowledge" rule—an implementing agent only needs repository access to follow these steps.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-001a_bot-interface/prd-bot-interface.md
  why: Defines scope, success metrics, and user stories for slice 1a.
  pattern: Mirrors diagrams showing ports <-> strategies.
  gotcha: Keep slice focused on shared interface only (send/edit).

- file: PRPs/PRP-001a_bot-interface/api-contract-bot-interface.md
  why: Enumerates integration points (strategy files/tests) and backend notes (type alias guidance).
  pattern: Follow the example interface signature + context updates.

# Internal references
- file: pkg/fsm/questions/strategy.go
  why: Current BotPort interface lives here; refactor target.
  pattern: Observe Render/Answer context fields that need BotMessage.
  gotcha: Ensure import changes remove `pkg/bot` where possible.

- file: pkg/fsm/questions/text_strategy.go
  why: Simple example of Render/HandleAnswer that should continue to compile with the shared port.
  pattern: Use to verify no Telegram structs leak post-refactor.

- file: pkg/fsm/questions/buttons_strategy.go
  why: Uses `tgbotapi.InlineKeyboardMarkup`; ensure new alias doesn’t break keyboard creation.
  gotcha: Only BotPort interface moves; markup usage stays localized.

- file: pkg/fsm/questions/text_strategy_test.go
  why: Representative test needing updates if contexts gain new fields.
  pattern: Reuse struct initialization patterns when adding BotMessage zero-values.

- file: docs/question-strategy.md
  why: Describes strategy architecture; update wording once BotPort lives in `pkg/ports`.
  gotcha: Keep documentation synchronized with new package path.

- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: Captures naming/error-code guidance for BotPort/BotMessage; cite when commenting code.
  section: 1-3 (interface shape, error semantics, message tracking).

# External documentation
- url: https://go.dev/doc/effective_go#interfaces
  why: Reinforces Go interface embedding and alias patterns (useful when aliasing BotPort).
  critical: Maintain idiomatic Go style when defining shared interfaces.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── PRPs
│   ├── PRP-001a_bot-interface
│   ├── PRP-001b_bot-interface
│   ├── PRP-002_question-strategy
│   └── ...
├── docs
│   ├── question-strategy.md
│   └── ...
├── pkg
│   ├── bot
│   ├── config
│   ├── fsm
│   │   └── questions
│   ├── state
│   └── ...
└── record_config.yaml
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
pkg/
  ports/
    botport/
      botport.go        # Defines BotPort interface, BotMessage, BotError scaffolding, helper constructors.

pkg/fsm/
  questions/
    strategy.go        # Imports botport package; contexts embed botport.BotMessage.
    *_strategy.go      # Continue referencing ctx.Bot (alias) without Telegram imports.
    *_test.go          # Updated to construct contexts with zero-value BotMessage.

docs/
  question-strategy.md # Mentions shared package path for BotPort.
```

### Known Gotchas of our codebase & Library Quirks

```python
# Strategies currently import tgbotapi for keyboards—do not remove; just ensure BotPort interface no longer depends on Telegram types.
# Keep gofmt formatting (tabs) and standard Go style; run `go fmt ./...` before final validation.
# Tests rely on state.NewRecord() + strategy contexts; ensure added fields have sensible zero-values to avoid nil derefs.
# Avoid introducing unused parameters/methods; future phases will extend the interface, so keep slice 1a minimal (SendMessage/EditMessage + BotMessage/BotError scaffolding).
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

// BotError scaffolding (RetryAfter/time.Duration, Wrapped error) for later slices.

type BotPort interface {
    SendMessage(ctx context.Context, chatID int64, text string, markup interface{}) (BotMessage, error)
    EditMessage(ctx context.Context, chatID int64, messageID int, text string, markup interface{}) (BotMessage, error)
}
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/ports/botport/botport.go
  - IMPLEMENT: BotPort interface (SendMessage, EditMessage), BotMessage struct, BotError placeholder (Op, Code, RetryAfter, Wrapped), helpers `NewBotError` + `IsCode` for future use.
  - FOLLOW: PRPs/PRP-001a_bot-interface/api-contract-bot-interface.md (interface signature) and PRPs/ai_docs/botport_hex_adapter.md (naming/error semantics).
  - ADD: Package-level comments explaining purpose + link to AI doc for future contributors.

Task 2: UPDATE pkg/fsm/questions/strategy.go
  - IMPORT: `telegramsurveylog/pkg/ports/botport` and alias `type BotPort = botport.BotPort` to minimize downstream diffs.
  - ADD: `LastPrompt botport.BotMessage` (RenderContext) and `Message botport.BotMessage` (AnswerContext) placeholders with comments noting future FSM wiring.
  - REMOVE: Local BotPort interface + `var _ BotPort = (*bot.Client)(nil)` assertion.

Task 3: TOUCH strategy implementations/tests
  - FILES: `pkg/fsm/questions/text_strategy.go`, `buttons_strategy.go`, `*_test.go`.
  - ENSURE: They compile with the new context fields (zero-value BotMessage acceptable) and no longer import `pkg/bot`.
  - UPDATE: Tests to construct contexts by referencing `RenderContext{ Bot: nil, LastPrompt: botport.BotMessage{} }` if needed.

Task 4: UPDATE documentation
  - FILE: `docs/question-strategy.md` (BotPort description section) referencing `pkg/ports/botport` as the canonical location.
  - NOTE: Mention new data fields (`LastPrompt`) to keep docs consistent.
```

### Implementation Patterns & Key Details

```go
// Type alias keeps existing code referencing ctx.Bot unchanged
import "telegramsurveylog/pkg/ports/botport"

type BotPort = botport.BotPort

// Context additions stay optional until FSM wiring occurs
type RenderContext struct {
    Bot       BotPort
    LastPrompt botport.BotMessage
    ...
}

type AnswerContext struct {
    RenderContext
    Message botport.BotMessage
}

// Tests can ignore new fields if not needed
ctx := RenderContext{ Record: state.NewRecord(), LastPrompt: botport.BotMessage{} }
```

### Integration Points

```yaml
STRATEGIES:
  - Ensure `ctx.Bot` implements the shared interface; no additional behavior changes.

DOCS:
  - Update `docs/question-strategy.md` to mention new package path + context fields.

TESTS:
  - `go test ./pkg/fsm/questions` must pass after refactor.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go fmt ./pkg/ports/... ./pkg/fsm/questions
go vet ./pkg/fsm/questions
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./pkg/fsm/questions -v
```

### Level 3: Integration Testing (System Validation)

```bash
# Not required for slice 1a; no runtime behavior changes beyond strategy compilation.
```

### Level 4: Creative & Domain-Specific Validation

```bash
git grep "type BotPort" -n | grep -v pkg/ports/botport  # Ensure no duplicate definitions
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./pkg/ports/... ./pkg/fsm/questions`
- [ ] `go vet ./pkg/fsm/questions`
- [ ] `go test ./pkg/fsm/questions`
- [ ] `git grep "type BotPort"` shows only the shared definition

### Feature Validation

- [ ] Success criteria from "What" section satisfied
- [ ] Strategies/tests compile without `pkg/bot`
- [ ] Documentation references updated

### Code Quality Validation

- [ ] New package follows Go style/Naming conventions
- [ ] Context additions documented to prevent confusion
- [ ] No unused imports or dead code

### Documentation & Deployment

- [ ] `docs/question-strategy.md` mentions new package path
- [ ] Comments reference `PRPs/ai_docs/botport_hex_adapter.md`

---

## Confidence Score

8/10 — Scope is narrowly defined (new package + strategy refactor), references list precise files, and validation steps ensure no duplicate interfaces remain. Main residual risk: forgetting to update every test/context field, mitigated by explicit task instructions.
