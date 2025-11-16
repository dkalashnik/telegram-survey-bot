name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |

---

## Goal

**Feature Goal**: Rewire FSM/state to depend on `botport.BotPort`, hydrate contexts with `BotMessage`, and introduce a deterministic fake adapter plus headless FSM tests to run without Telegram.

**Deliverable**: Refactored FSM/state signatures using `botport.BotPort`, new `pkg/bot/fakeadapter` with tests, hydrated `RenderContext.LastPrompt`/`AnswerContext.Message`, updated `main.go` wiring, and documentation describing the adapter boundary.

**Success Definition**: No direct `pkg/bot.Client` usage in FSM/state; adapter (real/fake) provides BotMessage data used in contexts; `go test ./pkg/fsm` and adapter/fake tests pass; docs reflect port-only wiring.

## User Persona (if applicable)

**Target User**: Maintainer/operator extending transports and testing flows without Telegram.

**Use Case**: Run FSM flows with a fake adapter for deterministic tests and rely on BotPort to swap transports without touching domain code.

**User Journey**:
1. Main constructs Telegram adapter and injects BotPort into FSM creation.
2. FSM uses BotPort for send/edit; captures BotMessage into contexts/userstate.
3. Tests swap in fake adapter to validate flows headlessly.

**Pain Points Addressed**:
- Removes Telegram coupling from FSM/state.
- Provides BotMessage data to strategies/FSM for retries/persistence.
- Enables fast, network-free tests.

## Why

- Aligns with hexagonal architecture: domain depends on ports, adapters vary.
- Unlocks LastPrompt/Message usage added in slice 001a/b.
- Reduces regression risk via fake-backed FSM tests.

## What

- Change FSM/state functions to accept `botport.BotPort` instead of `*bot.Client`.
- Capture adapter `BotMessage` responses to populate `RenderContext.LastPrompt` and `AnswerContext.Message`.
- Add `pkg/bot/fakeadapter` implementing BotPort with call recording and scripted responses/errors.
- Wire main.go to inject Telegram adapter BotPort (real), not passing raw client.
- Update docs to show adapter boundary and fake usage.

### Success Criteria

- [ ] No direct `pkg/bot.Client` usage within `pkg/fsm` or `pkg/state`.
- [ ] `RenderContext.LastPrompt` and `AnswerContext.Message` set from adapter responses in send/edit paths.
- [ ] Fake adapter tests and FSM headless tests pass via `go test ./pkg/bot/fakeadapter ./pkg/fsm`.
- [ ] Documentation updated to reflect BotPort wiring and fake adapter.

## All Needed Context

### Context Completeness Check

✅ Includes PRD, API contract, existing adapter tests/patterns, FSM/state code references, docs, and validation commands to guide an agent without prior knowledge.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-001c_bot-interface/prd-fsm-fake-adapter.md
  why: Defines scope, user stories, diagrams, success metrics for FSM BotPort wiring + fake adapter.
  pattern: Follow sequence and ER diagrams for BotMessage hydration.

- file: PRPs/PRP-001c_bot-interface/api-contract-fsm-fake-adapter.md
  why: Lists integration points (main, fsm.go, fsm-record.go), fake adapter API, hydration steps.
  gotcha: Ensure BotMessage.Transport/MessageID set; use BotError codes for message_not_modified/rate_limited.

- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: Guidance on BotPort semantics, error codes, message tracking.
  section: 1-3 (interface shape, error semantics, message tracking).

# Code references
- file: pkg/fsm/fsm.go
  why: Current handlers use *bot.Client; shows call graph and message handling paths.
  pattern: Update signatures to BotPort and propagate through helper calls.

- file: pkg/fsm/fsm-record.go
  why: askCurrentQuestion render flow; integrates SendMessage/EditMessage and tracks message IDs.
  pattern: Hydrate LastPrompt/Message from adapter responses while preserving edit-vs-send logic.
  gotcha: Handles "last message id" fallback; keep behavior when swapping to port.

- file: pkg/state/store.go
  why: UserState structure and storage; track any message metadata if needed.
  pattern: Preserve locking and state transitions while swapping types.

- file: pkg/bot/telegramadapter/adapter.go
  why: Real adapter implementing BotPort; ensure compatibility with FSM refactor.
  pattern: BotMessage mapping, BotError codes; reuse when adjusting FSM wiring.

- file: pkg/bot/telegramadapter/adapter_test.go
  why: Testing style for adapters; illustrates table-driven tests and metadata assertions.
  pattern: Mirror test approach for fake adapter and FSM tests (no network).

- file: main.go
  why: Bot/client/adapter initialization; modify to inject BotPort into FSM.
  gotcha: Keep signal handling loop intact; avoid unused variables after wire-up.

- file: docs/system-overview.md
  why: Sequence/architecture diagrams referencing bot/adapter boundary; update once FSM refactor done.

- file: docs/question-strategy.md
  why: Strategy docs mention BotPort contexts; ensure hydration details align.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── main.go
├── pkg
│   ├── bot
│   │   ├── bot.go
│   │   └── telegramadapter
│   │       ├── adapter.go
│   │       └── adapter_test.go
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
    ├── PRP-001c_bot-interface
    │   ├── prd-fsm-fake-adapter.md
    │   └── api-contract-fsm-fake-adapter.md
    └── ai_docs
        └── botport_hex_adapter.md
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
pkg/
  bot/
    fakeadapter/
      fakeadapter.go      # Fake BotPort implementation: call recording, scripted errors, BotMessage generation.
      fakeadapter_test.go # Unit tests for fake adapter behaviors (send/edit, error codes, retry_after).
  fsm/
    fsm_test.go           # Headless FSM tests using fake adapter to validate send/edit flows and hydration.
docs/
  system-overview.md      # Updated diagrams to show BotPort (adapter/fake) boundary.
  question-strategy.md    # Notes on contexts being hydrated from BotPort responses.
main.go                   # Inject BotPort (Telegram adapter) into FSM wiring.
```

### Known Gotchas of our codebase & Library Quirks

```python
# Go tooling writes to ~/Library cache; gofmt/go test may require elevated permissions in sandbox.
# FSM functions are invoked by goroutines; preserve context cancellation semantics when adding BotPort.
# fsm-record uses last message ID fallback logic; ensure BotMessage hydration keeps the same UX (edit vs send).
# Avoid network in tests: fake adapter should not import or call Telegram SDK.
# Error matching for Telegram uses substring checks; keep BotError codes stable across adapters.
```

## Implementation Blueprint

### Data models and structure

- `botport.BotMessage`: use Transport="telegram", ChatID, MessageID, Payload, Meta; store in contexts (and optionally state).
- `botport.BotError`: propagate codes `message_not_modified`, `rate_limited`, `bad_request`, etc.; fake adapter should wrap scripted errors accordingly.
- `state.UserState`: may keep legacy `LastMessageID`; when refactoring, ensure consistency with BotMessage.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE pkg/bot/fakeadapter/fakeadapter.go
  - IMPLEMENT: Fake BotPort with call recording (`Calls []Call`), auto-incrementing MessageIDs, ability to script failures per op.
  - FOLLOW: Testing patterns from pkg/bot/telegramadapter/adapter_test.go (table-driven, no network).
  - ENSURE: Returns BotMessage with Transport="telegram" and populated Meta where relevant.

Task 2: TEST pkg/bot/fakeadapter/fakeadapter_test.go
  - COVER: Send/edit success, message_not_modified codes, rate_limited with RetryAfter, call recording helpers.
  - GOTCHA: No Telegram imports; rely on standard library + botport.

Task 3: REFACTOR pkg/fsm to accept botport.BotPort
  - FILES: pkg/fsm/fsm.go, fsm-record.go, fsm-main.go (if needed).
  - CHANGE: function signatures and call sites from *bot.Client to botport.BotPort; remove pkg/bot imports in FSM.
  - HYDRATE: Assign RenderContext.Bot to injected port; set LastPrompt/Message from adapter responses; keep edit/send logic identical.
  - UPDATE: UserState message tracking to use BotMessage IDs where applicable.

Task 4: UPDATE main.go wiring
  - USE: telegramadapter.New(...) to produce BotPort; pass into FSM entrypoints/creator.
  - REMOVE: unused botClient references once BotPort is in use.
  - COMMENT: Note PRP-001d/ongoing follow-ups if any TODO remains.

Task 5: ADD headless FSM tests
  - FILE: pkg/fsm/fsm_test.go (or multiple) using fake adapter.
  - VALIDATE: Happy path (send + edit), repeat flow, message_not_modified handling, rate_limited propagation.
  - ENSURE: Tests avoid network and keep deterministic state.

Task 6: UPDATE docs
  - FILES: docs/system-overview.md, docs/question-strategy.md.
  - DESCRIBE: FSM uses BotPort (telegramadapter in prod, fakeadapter in tests); contexts now hydrated from adapter responses.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go fmt ./pkg/bot/fakeadapter ./pkg/fsm ./main.go
go vet ./pkg/bot/fakeadapter ./pkg/fsm
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./pkg/bot/fakeadapter -v
go test ./pkg/fsm -v
```

### Level 3: Integration Testing (System Validation)

```bash
# FSM headless tests already cover flows; no external integration needed.
go test ./... -run TestHandleUpdate -v  # optional targeted FSM tests if added
```

### Level 4: Creative & Domain-Specific Validation

```bash
git grep -n "pkg/bot" pkg/fsm pkg/state
```

Confirm no direct bot.Client references remain in FSM/state (only adapters/main should import pkg/bot).

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./pkg/bot/fakeadapter ./pkg/fsm ./main.go`
- [ ] `go vet ./pkg/bot/fakeadapter ./pkg/fsm`
- [ ] `go test ./pkg/bot/fakeadapter -v`
- [ ] `go test ./pkg/fsm -v`
- [ ] `git grep -n "pkg/bot" pkg/fsm pkg/state` returns empty

### Feature Validation

- [ ] FSM/state depend solely on BotPort.
- [ ] Context hydration sets `LastPrompt` and `Message` from BotMessage.
- [ ] Fake adapter supports deterministic tests; FSM tests pass with it.
- [ ] main.go wiring uses Telegram adapter BotPort.

### Code Quality Validation

- [ ] No network calls in tests; fakes used consistently.
- [ ] Error codes mapped consistently (message_not_modified, rate_limited, bad_request).
- [ ] Logging/metadata preserved through adapters.

### Documentation & Deployment

- [ ] Docs updated to show adapter boundary and fake usage.
- [ ] PRPs/plans.md updated with slice status upon completion.
- [ ] No new env vars introduced.

---

## Anti-Patterns to Avoid

- ❌ Leaving stray `pkg/bot.Client` references in FSM/state.
- ❌ Ignoring BotMessage hydration (contexts must be set from adapter responses).
- ❌ Writing tests that reach Telegram network.
- ❌ Diverging error codes between fake and real adapters.

## Success Indicators

- ✅ FSM compiles/runs with BotPort only.
- ✅ Deterministic headless tests validate send/edit flows.
- ✅ Documentation and PRPs stay aligned with implementation.

## Confidence Score

8/10 — Risks are contained to signature refactors and hydration plumbing; mitigated by fake adapter tests and grep checks ensuring bot.Client removal.
