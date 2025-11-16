name: "Base PRP Template v3 - Implementation-Focused with Precision Standards"
description: |

---

## Goal

**Feature Goal**: Add a main-menu action to compile all answered sections into a single go-template-rendered text block, send it to the env-configured `TARGET_USER_ID`, and clear stored answers only after a successful send; keep answers and log if forwarding fails.

**Deliverable**: Updated FSM/menu wiring, answer aggregation + rendering helper, forward-to-target logic leveraging BotPort/Telegram adapter, state cleanup on success, logging/notification on failure, configuration/env docs, and tests covering aggregation, rendering, success/failure paths.

**Success Definition**: Main menu shows the forward action; selecting it sends the compiled message to `TARGET_USER_ID` with `no_answer` for blanks; when send succeeds, respondent answers are cleared; when send fails, answers remain and an error is logged/surfaced; tests pass for aggregation/rendering/forward success/failure; Go fmt/vet/test succeed.

## User Persona (if applicable)

**Target User**: Operator/maintainer using the bot’s main menu to forward responses to a designated recipient (e.g., reviewer) without manual copy/paste.

**Use Case**: From main menu, trigger “Forward answered sections” to deliver all answered survey sections to the configured user ID in one message, then continue with a clean state upon success.

## Why

- Removes manual copy/paste of survey responses.
- Ensures delivery to a known reviewer via env-configured ID.
- Avoids data loss: only clear answers after confirmed send; retain on failure.
- Provides predictable formatting via Go template (`no_answer` placeholders).

## What

- Main-menu action that orchestrates aggregation → render → forward → conditional cleanup.
- Aggregator to gather answered sections/questions for respondent; substitute `no_answer` when blank/uncompleted.
- Go template renderer producing a structured text block (section headers, prompt + answer).
- BotPort send/forward to `TARGET_USER_ID`; success clears answers; failures log and keep data.
- Operator feedback: success/failure notice in-chat or log, per current BotPort patterns.

### Success Criteria

- [ ] Menu includes “Forward answered sections”.
- [ ] Rendered message contains all sections/questions with `no_answer` for blanks.
- [ ] Forward uses `TARGET_USER_ID` from env; fails gracefully when missing/invalid.
- [ ] On forward success, answers for the respondent are cleared.
- [ ] On forward failure, answers remain stored and error is logged/notified.
- [ ] Tests cover aggregation/render/rendered output snapshot, forward success cleanup, forward failure retention.

## All Needed Context

### Context Completeness Check

✅ PRD + API contract + external doc: forwardMessage reference; code references for FSM, state, BotPort, config, strategies. Guidance here should let an unfamiliar agent implement end-to-end.

### Documentation & References

```yaml
# Product & contract
- file: PRPs/PRP-006_forward-answered-sections/prd-forward-answered-sections.md
  why: Feature scope, user stories, flows, template rules (`no_answer`), success/failure behavior.
- file: PRPs/PRP-006_forward-answered-sections/api-contract-forward-answered-sections.md
  why: Integration points, data contracts, behavior contract, template expectations, error handling.

# External reference
- docfile: PRPs/ai_docs/telegram_forward_message.md
  why: Telegram forward/copy specifics when considering BotPort calls and failure modes.
- docfile: PRPs/ai_docs/botport_hex_adapter.md
  why: BotPort error codes (`message_not_modified`, `rate_limited`) and adapter semantics for send/edit/answer.

# Code references to follow
- file: pkg/fsm/fsm-main.go
  why: Main menu composition, handlers, and how to add a new action/button; patterns for sending menu messages.
- file: pkg/fsm/fsm.go
  why: Update dispatch, handlers (`handleMessage`, `handleCallbackQuery`), FSM integration points.
- file: pkg/fsm/fsm-record.go
  why: Answer handling, section completion, data persistence patterns; use for aggregation logic/lifecycle.
- file: pkg/fsm/utils.go
  why: Helpers for message formatting/keyboards; follow style.
- file: pkg/state/store.go
  why: User state lifecycle; place to add clear operation for answers/records if needed.
- file: pkg/state/models.go
  why: UserState/Record fields (`CurrentRecord`, `Records`, `LastPrompt`).
- file: pkg/bot/telegramadapter/adapter.go
  why: BotPort → Telegram send/edit/answer implementation; see how errors map to BotError.
- file: pkg/bot/fakeadapter/fakeadapter.go
  why: For tests; inspect call recording and failure injection patterns.
- file: pkg/fsm/questions/*.go
  why: Strategy registry patterns for rendering/answer persistence and tests style.
- file: pkg/fsm/fsm_test.go
  why: Headless FSM tests setup; reuse patterns for new tests.
- file: docs/fsm.md
  why: FSM states/events overview to slot new menu flows correctly.
```

### Current Codebase tree (gist)

```bash
.
├── main.go
├── pkg
│   ├── bot/{telegramadapter,fakeadapter}
│   ├── fsm/{fsm.go,fsm-main.go,fsm-record.go,questions/...}
│   ├── config/{config.go,loader.go}
│   └── state/{store.go,models.go}
├── docs/{fsm.md,question-strategy.md,system-overview.md}
└── PRPs/PRP-006_forward-answered-sections/{prd,api-contract}.md
```

### Desired Codebase tree additions/changes

```bash
pkg/fsm/fsm-main.go           # add menu action -> forward handler wiring
pkg/fsm/fsm.go                # route incoming updates if new command/callback used
pkg/fsm/utils.go (or new file)# aggregator + renderer helper for forward payload/template
pkg/state/store.go            # add clear-answers-for-respondent helper (if not present)
pkg/config/config.go          # expose TARGET_USER_ID from env (if not already)
pkg/bot/telegramadapter/adapter.go # no API change expected, but validate send usage

tests:
pkg/fsm/fsm_main_forward_test.go (new) # forward flow tests with fakeadapter
pkg/fsm/helpers_forward_test.go (new)  # aggregation/render tests
```

### Known Gotchas of our codebase & Library Quirks

```python
# BotPort errors use codes; use botport.IsCode for handling (message_not_modified, rate_limited).
# FSM main menu uses reply keyboards; record FSM uses inline; be careful to place new action in main menu stage.
# State store holds drafts and saved records; clearing must not delete saved records unless intended—scope cleanup to draft/answered data only.
# Telegram message length limits (~4096 chars); long aggregated text may need truncation/chunking; at least log/guard size.
# Tests should avoid network; use fakeadapter; do not break existing adapter interfaces.
# Go fmt and tabs; follow existing style/camelCase; keep comments minimal and purposeful.
```

## Implementation Blueprint

### Data models and structure

- Aggregation output: slice of sections with prompts/answers; substitute `no_answer` when empty/uncompleted.
- Forward request: `{TargetUserID int64, Sections []SectionAnswers, RespondentID string/UserID}`.
- Template: Go `text/template` string with section headers and question lines.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CONFIG expose TARGET_USER_ID
  - ADD: env var binding in pkg/config/config.go (int64) with validation (non-zero).
  - DOC: README or PRP note referencing new env.

Task 2: STATE cleanup helper
  - ADD: method on Store/UserState to clear answers for respondent (drafts and/or saved answers depending on scope); ensure saved records not lost unless intended; lock appropriately.
  - PATTERN: follow existing mutation patterns in store.go.

Task 3: AGGREGATOR + TEMPLATE
  - ADD: helper (pkg/fsm/utils.go or new file) to gather sections/questions/answers from UserState/Records; fill `no_answer` defaults.
  - ADD: Go template string/function to render aggregated text; include section title, question prompt, answer.
  - TEST: unit tests for aggregation + render snapshot (fake data).

Task 4: MAIN MENU ACTION
  - UPDATE: pkg/fsm/fsm-main.go to add button label (e.g., "Forward answered sections") in reply keyboard.
  - UPDATE: handler to respond by invoking aggregator -> render -> send via BotPort to TARGET_USER_ID.
  - EMPTY CASE: if no answers, send “Nothing to forward” message; skip forward/cleanup.
  - SUCCESS: on send success, call cleanup helper.
  - FAILURE: log error, keep answers; notify operator message.
  - TEST: pkg/fsm/fsm_main_forward_test.go using fakeadapter: success clears, failure retains.

Task 5: ROUTING (if needed)
  - UPDATE: pkg/fsm/fsm.go message/callback handling to recognize the new main-menu action.

Task 6: VALIDATION + DOCS
  - RUN: go fmt ./...
  - TEST: go test ./pkg/fsm/...
  - DOC: note `TARGET_USER_ID` env and forward behavior in README or docs/fsm.md (optional but recommended).
```

### Implementation Patterns & Key Details

```python
# Template rendering (pseudo):
tpl := `{{range .Sections}}## {{.Title}}
{{range .Questions}}- {{.Prompt}}: {{.Answer}}
{{end}}
{{end}}`
# Keep under Telegram limit; consider truncation per answer if needed.

# Send + cleanup:
msg, err := botPort.SendMessage(ctx, targetUserID, renderedText, nil)
if err != nil { log + notify; return }
clearAnswers(userState)  # ensure thread-safe; avoid deleting saved records unless required
```

### Integration Points

```yaml
ENV:
  - TARGET_USER_ID: required int64 target for forward

FSM:
  - Add main menu button + handler to trigger forward flow
  - Use BotPort.SendMessage for output (forwardMessage/copyMessage not required since we render new text)

STATE:
  - Read answers/records; clear answers on success

BOTPORT:
  - Use existing SendMessage; rely on BotError for failure handling
```

## Validation Loop

### Level 1: Syntax & Style

```bash
go fmt ./pkg/fsm ./pkg/state ./pkg/config
go vet ./pkg/fsm ./pkg/state ./pkg/config
```

### Level 2: Unit Tests

```bash
go test ./pkg/fsm -run Forward -v
go test ./pkg/fsm
```

### Level 3: Targeted Flow Tests

```bash
go test ./pkg/fsm -run Forward
```

### Level 4: Creative Validation

```bash
git grep "Forward answered" pkg/fsm   # ensure single entry point
```

## Final Validation Checklist

### Technical Validation

- [ ] `go fmt ./...`
- [ ] `go vet ./pkg/fsm ./pkg/state ./pkg/config`
- [ ] `go test ./pkg/fsm -run Forward`
- [ ] `go test ./...`

### Feature Validation

- [ ] Main menu shows forward action.
- [ ] No-answers path responds without clearing.
- [ ] Successful send clears answers.
- [ ] Failed send retains answers and logs/notifies.
- [ ] Rendered text matches template with `no_answer`.

---

Confidence Score: 8.5/10 (rich internal references, explicit tasks, env noted; confirm exact cleanup scope vs saved records during implementation).
