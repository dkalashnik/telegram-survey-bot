# API & Integration Contract — PRP-004 Integration Harness

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `pkg/fsm` handlers (`HandleUpdate`, `askCurrentQuestion`, etc.) | Target of scripted updates and BotPort calls. | Harness invokes `HandleUpdate` with fake updates and a fake BotPort; assertions inspect BotPort call order and state changes. |
| `pkg/bot/fakeadapter` | Fake BotPort used for offline integration tests. | Harness reuses `FakeAdapter` to capture calls; may extend with helpers for call assertions if needed. |
| `pkg/state` (`Store`, `UserState`) | State mutated by FSM during flows. | Harness inspects `UserState` and records after script execution to validate behavior. |
| `pkg/config` (`RecordConfig`) | Supplies questions/sections for scripted flows. | Harness loads or constructs a small `RecordConfig` per test script. |
| `main.go` / wiring | Not directly exercised; harness builds FSM/store in-process. | Document commands/targets to run integration tests independently of production main. |
| `PRPs/ai_docs/botport_hex_adapter.md` | Error code semantics (message_not_modified, rate_limited). | Use for expected BotError code assertions in error-path scripts. |
| Optional live tests (`integration/live_test.go`) | Only when env vars are set (Telegram test environment). | Use `TELEGRAM_TEST_TOKEN` and `TELEGRAM_TEST_CHAT_ID`; skip otherwise to keep CI stable. |

## Backend Implementation Notes

### Harness Helpers
- Create an integration package (e.g., `integration/`):
  ```go
  type ScriptedUpdate struct {
      Name   string
      Update tgbotapi.Update
  }

  type HarnessResult struct {
      Calls   []fakeadapter.Call
      Records []*state.Record
      Errors  []error
  }

  func RunScript(ctx context.Context, script []ScriptedUpdate, botPort botport.BotPort, cfg *config.RecordConfig, store *state.Store) HarnessResult
  ```
- Keep FSM invocation synchronous: call `HandleUpdate` directly per scripted update (avoid goroutines or wait on a `sync.WaitGroup`).

### Test Structure (offline/default)
- Package: `integration` with tests like `harness_test.go`.
- Use `fakeadapter.FakeAdapter` and a small `RecordConfig` fixture.
- Scripts cover:
  - `/start` command flow (send main menu via BotPort).
  - Section selection callback -> question render -> answer -> save/list.
  - Edge cases: `message_not_modified` (scripted BotError), `rate_limited` (scripted BotError with RetryAfter).
- Assertions:
  - BotPort call sequence (`send_message`, `edit_message`, `answer_callback`).
  - `UserState` transitions and saved records content.
  - BotMessage hydration (message IDs non-zero when expected).

### Live Test (Telegram Test Environment)
- Optional test file `integration/live_test.go` gated by env vars `TELEGRAM_TEST_TOKEN` and `TELEGRAM_TEST_CHAT_ID`.
- Uses Telegram **test environment** (create test account/bot via @BotFather in test mode and send requests to `https://api.telegram.org/bot<TOKEN>/test/METHOD_NAME`).
- Flow: minimal `/start` + one answer, generous timeouts, skip if env vars absent. Mark target as opt-in (e.g., `make live-test`).

### Commands/Targets
- Add/document targets:
  - `make int-test` → `go test ./integration/...` (offline harness).
  - `make live-test` → `go test ./integration -run Live -v` (skips unless env vars set).
- Ensure harness imports `tgbotapi` only inside integration package; domain remains BotPort-only.

### Data/Fixtures
- Provide a tiny `RecordConfig` fixture (1–2 sections/questions) for repeatable scripts.
- Use deterministic chat/user IDs in scripted updates.

### Anti-flakiness
- Avoid sleeps/timeouts where possible; run FSM synchronously.
- For live test, wrap with context timeouts and skip on missing env vars.
