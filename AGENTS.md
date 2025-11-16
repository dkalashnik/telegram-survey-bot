# Repository Guidelines

## Project Structure & Module Organization
The bot entrypoint lives in `main.go`, wiring configuration and the Telegram FSM. Reusable code sits under `pkg/`: `pkg/bot` holds Telegram client orchestration, `pkg/fsm` implements survey states and transitions, `pkg/state` defines persistent models/stores, and `pkg/config` centralizes environment and YAML loading. Runtime configuration comes from `.env` (bot token & chat IDs) and `record_config.yaml` (survey prompts). Docker assets (`docker-compose.yml`) and task aliases (`Makefile`) stay at the root.

## Build, Test, and Development Commands
Use `go run ./main.go` for a quick local execution with your `.env`. Container workflows rely on Makefile targets: `make rebuild-up` rebuilds images from scratch and starts the stack while tailing logs; `make up` starts existing containers; `make down` stops them; `make logs` follows service output. When editing Go code, run `go fmt ./...` before committing to keep imports and indentation canonical.

## Coding Style & Naming Conventions
This project follows standard Go style: tabs for indentation, camelCase for locals, and PascalCase for exported types/functions. Keep packages narrowly focused (e.g., FSM helpers belong in `pkg/fsm/utils.go`). Name files by responsibility—`*_test.go` for tests, `const.go` for enumerations. Run `go vet ./...` if you introduce tricky logic. Comments should explain state transitions or Telegram interactions rather than restating code.
IMPORTANT: Always use SOLID and KISS principles. 
IMPORTANT: This project is not in prod, do not keep the backward compatibility.

## Testing Guidelines
Leverage Go's built-in `testing` package. Mirror production package names and place specs in `*_test.go` files next to the code (`pkg/fsm/fsm_test.go`, `pkg/state/store_test.go`). Favor table-driven tests when covering multiple survey paths. Ensure newly added behavior is covered by `go test ./...` and target at least the changed packages. Mock Telegram calls by faking the bot interface to keep tests deterministic.

## Commit & Pull Request Guidelines
Recent history (`Fix the bot`, `Cleanup`) shows short, capitalized imperatives; keep that convention and scope commits tightly. PRs should describe the survey scenario impacted, mention configs touched (`.env`, `record_config.yaml`), and, when UI-visible changes occur, attach screenshots of Telegram dialogs. Link issues or TODO references, list manual validation steps (e.g., `make up` + sample survey run), and highlight any new secrets required.

## Security & Configuration Notes
Never commit filled `.env` files—use `.env.example` snippets in the PR body instead. Validate that `record_config.yaml` contains only survey content; place secrets exclusively in environment variables injected via Docker Compose overrides.

## Planning & Documentation Canon
- Treat `PRPs/plans.md` as the single source of truth for work status. Whenever you complete a task, start a new effort, or identify a follow-up, update that file immediately so the plan reflects reality.
- Mirror every follow-up or scope note in both `PRPs/plans.md` and the relevant PRP/README so downstream slices inherit the context automatically.

## User Context
- **Operator Persona:** Single maintainer (the requester) configures surveys and manages code; respondents only answer via Telegram and do not operate the system.
- **Motivation:** No direct end-user pain today; refactors aim to keep the codebase clean, SOLID-aligned, and easier to extend.
- **Success Criteria:** Introducing new question types must require zero Go code changes, isolate logic per type, and improve separation between subsystems.
