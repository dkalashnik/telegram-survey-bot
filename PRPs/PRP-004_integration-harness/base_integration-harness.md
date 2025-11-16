name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |

---

## Goal

**Feature Goal**: Build an integration test harness that drives the FSM with scripted updates using BotPort (fake adapter by default, optional Telegram test environment), covering start → answer → save/list flows and catching regressions without hitting production Telegram.

**Deliverable**: New `integration/` test package with harness helpers + tests (offline), optional live test gated by env vars for the Telegram test environment, Makefile target(s) to run the suite, and documentation of commands.

**Success Definition**: `go test ./integration/...` passes offline using fakeadapter; harness asserts BotPort call order and state changes for key flows; optional `make live-test` uses the Telegram test environment when env vars are set and is skipped otherwise; commands are documented.

## User Persona (if applicable)

**Target User**: Maintainer/CI engineer wanting automated, end-to-end coverage of bot flows without relying on real Telegram in default runs.

**Use Case**: Run scripted flows (start, answer, save, list) via the FSM with fake updates/BotPort; optionally run a live smoke test against Telegram’s test environment when credentials are provided.

**Pain Points Addressed**:
- Lack of E2E coverage beyond unit/headless FSM tests.
- Inability to catch regressions in markup/edit flows, BotMessage hydration, or state transitions.
- Need for CI-friendly, offline test runs with an optional live check.

## Why

- Ensures FSM + adapters work together through real update shapes.
- Offline harness avoids network flakiness and secrets in CI.
- Optional live test validates adapter behavior against Telegram test environment in a gated, non-prod context.

## What

- Add an `integration/` package with scripted update runner + fixtures.
- Write offline integration tests using `pkg/bot/fakeadapter` and a small RecordConfig fixture.
- Add optional live test gated by `TELEGRAM_TEST_TOKEN`/`TELEGRAM_TEST_CHAT_ID` using Telegram test environment endpoints.
- Add Makefile targets and documentation to run offline and live tests.

### Success Criteria

- [ ] Offline integration tests (`go test ./integration/...`) cover start/answer/save/list flows and error cases (message_not_modified, rate_limited).
- [ ] BotPort call sequences and state/record assertions validated in tests.
- [ ] Optional live test skipped unless env vars are set; uses Telegram test environment.
- [ ] Makefile target(s) or documented commands exist (`make int-test`, `make live-test`).

## All Needed Context

### Context Completeness Check

✅ PRD + API contract included; code references identify FSM, state, BotPort adapters, fake adapter, and Makefile. Commands are provided so an agent without prior knowledge can implement the harness.

### Documentation & References

```yaml
# Product + contract docs
- file: PRPs/PRP-004_integration-harness/prd-integration-harness.md
  why: Defines integration harness scope, user stories, diagrams, success metrics.

- file: PRPs/PRP-004_integration-harness/api-contract-integration-harness.md
  why: Lists integration points, harness helper specs, offline/live test guidance.

- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: BotPort error codes (message_not_modified, rate_limited) for assertions.

# Code references
- file: pkg/fsm/fsm.go
  why: Entrypoint for handling updates; harness will call HandleUpdate per script.

- file: pkg/fsm/fsm-record.go
  why: Contains askCurrentQuestion/processAnswer; informs expected BotMessage hydration/order.

- file: pkg/fsm/fsm-main.go
  why: Main menu flows, list navigation logic to script callbacks.

- file: pkg/bot/fakeadapter/fakeadapter.go
  why: BotPort fake used for offline harness; call recording fields to assert.

- file: pkg/config/config.go
  why: RecordConfig structure; build small fixtures for scripts.

- file: pkg/state/models.go
  why: UserState/Record structures to inspect after scripts.

- file: Makefile
  why: Add integration test targets (`int-test`, optional `live-test`).

- file: main.go
  why: Shows production wiring; integration harness will construct FSM/store directly instead.
```

### Current Codebase tree (run `tree` in the root of the project) to get an overview of the codebase

```bash
.
├── Makefile
├── main.go
├── pkg
│   ├── bot
│   │   ├── bot.go
│   │   ├── fakeadapter
│   │   └── telegramadapter
│   ├── fsm
│   ├── config
│   ├── ports
│   └── state
├── docs
└── PRPs/PRP-004_integration-harness
    ├── prd-integration-harness.md
    └── api-contract-integration-harness.md
```

### Desired Codebase tree with files to be added and responsibility of file

```bash
integration/
  harness.go          # RunScript helper, ScriptedUpdate type, HarnessResult (calls, records, errors).
  harness_test.go     # Offline flows (start/answer/save/list, error paths) using fakeadapter.
  live_test.go        # Optional; gated by TELEGRAM_TEST_TOKEN/TELEGRAM_TEST_CHAT_ID, uses Telegram test env.
```

### Known Gotchas of our codebase & Library Quirks

```python
# Go tools may need elevated permissions for ~/Library cache (fmt/test).
# FSM handlers spawn goroutines in main; harness should call HandleUpdate synchronously in tests.
# Avoid Telegram network in default tests; live test must be opt-in and use test environment endpoint (/test/).
# BotPort errors are normalized; use IsCode for message_not_modified/rate_limited checks.
```

## Implementation Blueprint

### Data models and structure

- `ScriptedUpdate` (name + tgbotapi.Update).
- `HarnessResult` (calls from fakeadapter, records from state.Store, errors encountered).
- Small `RecordConfig` fixture (1–2 sections/questions).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE integration/harness.go
  - DEFINE: ScriptedUpdate, HarnessResult, RunScript(ctx, script, botPort, cfg, store).
  - IMPLEMENT: Synchronous iteration over updates calling fsm.HandleUpdate; collect fakeadapter calls, final records, errors.
  - ACCEPT: BotPort to remain flexible (fakeadapter for offline, telegramadapter for live).

Task 2: CREATE integration/harness_test.go (offline suite)
  - USE: fakeadapter.FakeAdapter and a small RecordConfig fixture.
  - COVER: Happy path (/start -> select -> answer -> save/list), list navigation, message_not_modified + rate_limited scenarios (scripted BotError via fakeadapter.Fail).
  - ASSERT: BotPort call order, UserState/Record contents, LastPrompt/Message IDs.

Task 3: OPTIONAL integration/live_test.go
  - GATE: Skip unless TELEGRAM_TEST_TOKEN and TELEGRAM_TEST_CHAT_ID are set.
  - FLOW: Minimal /start + one answer; use Telegram test environment endpoint (/test/). Generous timeouts; skip by default.

Task 4: UPDATE Makefile
  - ADD: int-test target running `go test ./integration/...`.
  - ADD: live-test target running `go test ./integration -run Live -v` (skips when env vars absent).

Task 5: DOCUMENT commands (README or docs note)
  - NOTE: Offline harness: `go test ./integration/...`
  - NOTE: Live (opt-in): set TELEGRAM_TEST_TOKEN/TELEGRAM_TEST_CHAT_ID and run `make live-test`.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go fmt ./integration
go vet ./integration
```

### Level 2: Unit Tests (Component Validation)

```bash
go test ./integration -v
```

### Level 3: Integration Testing (System Validation)

```bash
# Optional live: only when env vars are set
go test ./integration -run Live -v
```

### Level 4: Creative & Domain-Specific Validation

```bash
git grep "pkg/bot" integration | grep -v fakeadapter  # Ensure harness doesn't hardwire prod client
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./integration`
- [ ] `go vet ./integration`
- [ ] `go test ./integration -v`
- [ ] (Optional) `go test ./integration -run Live -v`

### Feature Validation

- [ ] Offline harness covers start/answer/save/list flows + error codes.
- [ ] BotPort call assertions and state checks present.
- [ ] Live test is opt-in and uses Telegram test environment.
- [ ] Makefile targets documented/working.

### Code Quality Validation

- [ ] Tests are deterministic; no network in default run.
- [ ] Clear fixtures and assertions; minimal flakiness.

### Documentation & Deployment

- [ ] Commands to run integration tests documented (README/docs).
- [ ] Environment variables for live test documented; default skip when unset.

---

## Anti-Patterns to Avoid

- ❌ Relying on goroutines/timeouts in tests; keep synchronous where possible.
- ❌ Hardcoding production Telegram endpoints in tests; live test must use test environment and be gated.
- ❌ Adding bot.Client usage in harness beyond adapters.

## Success Indicators

- ✅ `go test ./integration/...` passes in CI without Telegram access.
- ✅ Optional `make live-test` provides a gated smoke test against Telegram test environment.
- ✅ Harness catches regressions in BotPort/FSM orchestration before deployment.

## Confidence Score

8/10 — Scope is well-defined; main risks are keeping scripts aligned with real update shapes and avoiding flakiness. Mitigated by fakeadapter, synchronous execution, and gated live test.
