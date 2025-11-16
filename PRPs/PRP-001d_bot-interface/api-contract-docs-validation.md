# API & Integration Contract — PRP-001d Docs & Validation Polish

## Integration Points (Current Codebase)
| Location | Interaction | Contract |
| --- | --- | --- |
| `docs/system-overview.md` | Architecture/sequence diagrams reference bot/adapter boundary. | Update to show `botport.BotPort` with `telegramadapter` (prod) and `fakeadapter` (tests). Include context hydration (LastPrompt/Message) in the flow. |
| `docs/question-strategy.md` | Strategy docs mention BotPort and contexts. | Ensure it explains that `RenderContext.LastPrompt`/`AnswerContext.Message` come from BotPort responses, and list adapter options (telegram/fake). |
| `README.md` (or root docs) | Onboarding instructions (env vars, running bot/tests). | Add concise steps: set `TELEGRAM_BOT_TOKEN`, run `go test ./...`, explain fake adapter/headless tests. Document validation commands. |
| `Makefile` (if present) | Task aliases for validation. | Optionally add targets for `fmt`, `vet`, `test`, and `port-check` (git grep) or document commands if not adding targets. |
| `pkg/fsm`, `pkg/state` | Must remain port-only. | Validation must check `git grep "pkg/bot" pkg/fsm pkg/state` to stay empty. |
| CI (future) | Adopt validation commands. | Provide ready-to-use command list for CI pipelines; no code change required in this slice beyond docs/targets. |

## Backend Implementation Notes
- No runtime API changes; focus is on documentation and validation commands.
- Validation command set to document (and optionally add to Makefile):
  ```bash
  go fmt ./...
  go vet ./...
  go test ./...
  git grep "pkg/bot" pkg/fsm pkg/state
  ```
- If adding a Makefile target, pattern:
  ```make
  validate:
  	go fmt ./...
  	go vet ./...
  	go test ./...
  	git grep "pkg/bot" pkg/fsm pkg/state || true # ensure no unexpected hits
  ```
- Diagrams: Keep Mermaid in docs updated to show BotPort boundary and hydration of LastPrompt/Message; ensure syntax remains valid (renderable).
- Onboarding: Spell out env var `TELEGRAM_BOT_TOKEN` for prod runs; tests use fake adapter and require no token.
- Anti-regression: Note explicitly in docs that all new domain code must depend on `botport.BotPort`, not `pkg/bot.Client`.
