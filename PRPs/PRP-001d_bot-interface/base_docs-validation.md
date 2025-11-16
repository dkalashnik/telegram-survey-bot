name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |

---

## Goal

**Feature Goal**: Finalize BotPort adoption by aligning documentation and validation guardrails with the new adapter/fake wiring, ensuring developers and CI enforce port-only domain code and context hydration expectations.

**Deliverable**: Updated docs (system overview, question strategy, README) describing BotPort/adapter/fake usage and context hydration, plus documented validation commands/targets (fmt/vet/test/grep) that guard against regressions to `pkg/bot.Client`.

**Success Definition**: Docs accurately reflect the current architecture (BotPort + telegram/fake adapters, LastPrompt/Message hydration), validation commands are documented (and optionally scripted) to catch port regressions, and onboarding steps include env/test guidance. Running the documented validations passes.

## User Persona (if applicable)

**Target User**: Contributors/maintainers extending the bot; CI engineers adding guardrails.

**Use Case**: Developer understands BotPort boundaries, runs documented validations locally/CI, and avoids reintroducing direct Telegram client references.

**User Journey**:
1. Read README/docs to learn BotPort/adapter/fake roles and env setup.
2. Run documented validation commands (fmt/vet/test/grep) before PRs.
3. Confidently extend strategies/FSM without touching Telegram-specific types.

**Pain Points Addressed**:
- Removes ambiguity about adapter vs. domain boundaries.
- Prevents regressions toward `pkg/bot.Client` in FSM/state.
- Documents how BotMessage hydration (LastPrompt/Message) works for future features/tests.

## Why

- Consolidates architectural shifts from slices 001a–001c into authoritative docs.
- Adds lightweight validation to enforce port purity and tested behavior.
- Simplifies onboarding by documenting env/test steps and fake adapter usage.

## What

- Update documentation (system-overview, question-strategy, README) to reflect BotPort + telegram/fake adapters, context hydration, and validation steps.
- Document/run validation commands: `go fmt ./...`, `go vet ./...`, `go test ./...`, `git grep "pkg/bot" pkg/fsm pkg/state`.
- Optionally add Makefile targets for the validation bundle if consistent with repo style.

### Success Criteria

- [ ] Docs include BotPort/adapter/fake flows and context hydration (LastPrompt/Message).
- [ ] Validation commands documented (and runnable) to catch `pkg/bot` usage in domain code.
- [ ] README covers env (`TELEGRAM_BOT_TOKEN`) and testing with fake adapter (network-free).
- [ ] Running documented validations succeeds.

## All Needed Context

### Context Completeness Check

✅ PRD/API contract, current code references, and validation commands are included so an agent without prior knowledge can execute this slice.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-001d_bot-interface/prd-docs-validation.md
  why: Defines the doc/validation polish scope, success metrics, and diagrams.

- file: PRPs/PRP-001d_bot-interface/api-contract-docs-validation.md
  why: Lists integration points (docs, README, Makefile/commands) and required validation commands.

- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: Reference for BotPort semantics when explaining adapter boundaries and message tracking.
  section: 1-3.

# Code references
- file: docs/system-overview.md
  why: Current architecture diagrams; update to show BotPort + telegram/fake adapters and hydration.
  pattern: Mermaid sequence/architecture diagrams.

- file: docs/question-strategy.md
  why: Explains contexts/strategies; ensure LastPrompt/Message hydration and adapter sources are documented.

- file: README.md
  why: Onboarding/env/test instructions; add BotPort/adapter/fake notes and validation commands.

- file: Makefile
  why: Potential place for validation targets; follow existing style if adding (or document commands instead).

- file: pkg/fsm/fsm.go
  why: Confirms FSM now uses BotPort; mention in docs and ensure grep checks reflect port-only domain code.

- file: pkg/fsm/fsm-record.go
  why: Hydrates LastPrompt/Message; cite in docs as the source of context data.

- file: pkg/bot/telegramadapter/adapter.go
  why: Real adapter implementation; referenced in docs when describing production path.

- file: pkg/bot/fakeadapter/fakeadapter.go
  why: Fake adapter for headless tests; document usage/testing.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── README.md
├── Makefile
├── docs
│   ├── system-overview.md
│   └── question-strategy.md
├── pkg
│   ├── bot
│   │   ├── bot.go
│   │   ├── telegramadapter
│   │   └── fakeadapter
│   ├── fsm
│   └── ports
└── PRPs/PRP-001d_bot-interface
    ├── prd-docs-validation.md
    └── api-contract-docs-validation.md
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
docs/
  system-overview.md      # Updated diagrams/text for BotPort + adapters + hydration.
  question-strategy.md    # Notes on contexts receiving BotMessage and adapter roles.
README.md                 # Onboarding/env/test and validation commands.
Makefile (optional)       # Validation target bundling fmt/vet/test/grep, if consistent with repo.
```

### Known Gotchas of our codebase & Library Quirks

```python
# Go tools may need elevated permissions for ~/Library cache (gofmt/go test).
# Mermaid diagrams must stay syntactically correct; avoid breaking fenced blocks.
# Validation grep must not fail on adapter packages where pkg/bot is expected; scope to pkg/fsm pkg/state only.
# Keep docs ASCII-friendly; avoid heavy formatting beyond needed mermaid/markdown.
```

## Implementation Blueprint

### Data models and structure

- Reuse existing BotPort concepts; no new data models. Emphasize `LastPrompt`/`Message` hydration paths in docs.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: UPDATE docs/system-overview.md
  - ADD: BotPort boundary with telegramadapter/fakeadapter; show context hydration (LastPrompt/Message) in diagrams.
  - FOLLOW: Existing Mermaid style; ensure sequences mention BotPort.

Task 2: UPDATE docs/question-strategy.md
  - DESCRIBE: RenderContext.LastPrompt + AnswerContext.Message populated from BotPort; list adapters (telegram/fake) as sources.
  - CLARIFY: Strategies remain transport-agnostic; BotMessage comes from FSM wiring.

Task 3: UPDATE README.md
  - INCLUDE: Env setup (TELEGRAM_BOT_TOKEN), how to run bot (`go run ./main.go`), how to run tests (`go test ./...`), mention fakeadapter for headless tests, and list validation commands.
  - KEEP: Existing tone/structure; add concise sections rather than overhauling.

Task 4: OPTIONAL Makefile target
  - IF appropriate: add `validate` target running fmt/vet/test/grep (port purity). If adding seems inconsistent, document commands instead.

Task 5: VALIDATION commands
  - RUN: go fmt ./...; go vet ./...; go test ./...
  - RUN/Document: git grep "pkg/bot" pkg/fsm pkg/state (expect no hits).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go fmt ./...
go vet ./...
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./...
```

### Level 3: Integration Testing (System Validation)

```bash
# No external integrations; ensure full test suite passes.
```

### Level 4: Creative & Domain-Specific Validation

```bash
git grep "pkg/bot" pkg/fsm pkg/state
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `git grep "pkg/bot" pkg/fsm pkg/state` has no hits

### Feature Validation

- [ ] Docs reflect BotPort/adapter/fake and hydration details.
- [ ] README documents env/test steps and validation commands.
- [ ] (If added) Makefile validate target runs without errors.

### Code Quality Validation

- [ ] Mermaid diagrams renderable; Markdown lint-friendly.
- [ ] Commands match project tooling (Go modules, no extra deps).

### Documentation & Deployment

- [ ] PRPs/plans.md updated after completion.
- [ ] No new env vars beyond TELEGRAM_BOT_TOKEN referenced.

---

## Anti-Patterns to Avoid

- ❌ Leaving outdated references to `pkg/bot.Client` in docs outside adapter contexts.
- ❌ Adding heavyweight tooling; keep validation minimal and documented.
- ❌ Breaking existing doc formatting/diagrams.

## Success Indicators

- ✅ Contributors can follow docs to understand BotPort boundary and run validations.
- ✅ Validation commands prevent reintroduction of direct bot.Client usage in domain code.
- ✅ Diagrams/text match the current code after slices 001a–001c.

## Confidence Score

8/10 — Scope is documentation/validation only; main risk is missing minor references or diagram syntax issues, mitigated by explicit file targets and grep checks.
