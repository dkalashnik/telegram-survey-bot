## Purpose
Contract for “Forward answered sections” feature: main-menu action compiles all answered sections, renders a text block via Go template with `no_answer` placeholders, forwards to env-configured user ID, and clears answers only after successful send.

## Integration Points
- **Env config**: `TARGET_USER_ID` (int64 string) required. Loaded via config layer alongside existing bot/env vars.
- **FSM main menu** (`pkg/fsm`): add action `Forward answered sections` that triggers aggregation → render → forward → conditional cleanup. Must surface success/failure back to operator (at least via BotPort message and log).
- **Answer store** (`pkg/state`): read all sections/questions for the current respondent. Provide clear operation scoped to respondent. No mutation on forward failure.
- **BotPort adapter** (`pkg/ports/botport` + Telegram adapter): reuse send/forward method to specified `TARGET_USER_ID`. Must return success/failure with enough detail for logging and cleanup decision.
- **Template rendering** (`pkg/fsm` or helper pkg): Go `text/template` that receives aggregated payload and formats sections/questions; unanswered → `no_answer`.

## Data Contracts
### ForwardRequest (FSM → Renderer → Forwarder)
```go
type ForwardRequest struct {
    RespondentID string
    TargetUserID int64   // from TARGET_USER_ID env
    Sections     []SectionAnswers
}

type SectionAnswers struct {
    ID       string
    Title    string
    Questions []QuestionAnswer
}

type QuestionAnswer struct {
    ID       string
    Prompt   string
    Answer   string // already defaulted to "no_answer" for missing/blank
    Completed bool
}
```

### RenderedForward (Renderer → Forwarder)
```go
type RenderedForward struct {
    Text string // final message per template
}
```

## Behavior Contract
1. **Selection**: Operator taps main-menu item `Forward answered sections`.
2. **Aggregation**: FSM calls store to load all sections/questions for respondent. Substitute `no_answer` when `Answer` empty or `Completed` is false.
3. **Empty case**: If no answers present, short-circuit with BotPort message: “Nothing to forward.” No forwarding call, no cleanup.
4. **Render**: Renderer applies Go template to `ForwardRequest`, producing `RenderedForward.Text`.
5. **Forward**: BotPort sends text to `TargetUserID`. If adapter provides forward vs send distinction, either is acceptable; result must indicate success/failure.
6. **Cleanup**:
   - On success: clear respondent answers in store.
   - On failure: keep answers; log error; notify operator of failure.
7. **Logging**: Log includes respondent ID, target user ID, and error code/message from adapter on failure.

## Template Contract
- **Engine**: Go `text/template`.
- **Shape**: Receives `ForwardRequest`.
- **Output rules**:
  - Include section title headers.
  - Include each question prompt and resolved `Answer`.
  - Use `no_answer` literal when data missing/uncompleted.
  - Keep output under Telegram message limit; if chunking is later needed, preserve section boundaries.

## Error Handling
- Missing/invalid `TARGET_USER_ID`: block action, send BotPort message to operator, log, do not mutate store.
- BotPort failure (network/Telegram error, rate limit, permission): do not clear answers; send failure notice to operator; log details.
- Template/render error: do not call BotPort; notify operator; log error.

## Validation Notes
- Unit tests for aggregation (answer → `no_answer` substitution), render output snapshot, forward success triggers cleanup, forward failure keeps data.
- Integration test (headless) driving menu action to verify BotPort call and store mutation behavior.
