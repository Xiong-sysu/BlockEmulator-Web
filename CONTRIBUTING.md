# Contributing to BlockEmulator-X Web Console

Thanks for improving this project. This repository is a learning-friendly local
console for BlockEmulator-X, so contributions should keep the code easy to read,
easy to run, and careful with the original BlockEmulator-X project files.

## Development Principles

- Do not overwrite `../block-emulator-x/config.yaml`.
- Do not overwrite `../block-emulator-x/ip_table.json`.
- Keep generated experiment files under `backend/workdir/`.
- Prefer small, focused changes.
- Keep user-facing UI text and code comments in English.
- Keep the first version local-only unless a change explicitly targets remote
  deployment.
- Use clear names instead of clever abstractions.

## Local Setup

Install:

- Go 1.22 or newer
- Node.js and npm
- Python 3.10 or newer, if working on `python-backend`

Recommended folder layout:

```text
BlockEmulator/
├── BlockEmulator-web/
└── block-emulator-x/
```

If your BlockEmulator-X checkout is elsewhere:

```sh
export BLOCK_EMULATOR_X_ROOT=/absolute/path/to/block-emulator-x
```

## Running the Project

Start all services:

```sh
./start.sh
```

Start services individually:

```sh
./start.sh backend
./start.sh frontend
./start.sh charts
```

Manual commands:

```sh
cd backend
go run .
```

```sh
cd frontend
npm install
npm run dev
```

```sh
cd python-backend
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python app.py
```

## Code Organization

### Go Backend

Important files:

- `backend/main.go`: HTTP routes and JSON response handling.
- `backend/internal/emulator/types.go`: API-facing data structures.
- `backend/internal/emulator/config.go`: config loading, validation, and
  generated config writing.
- `backend/internal/emulator/service.go`: process management, logs, status, and
  CSV result reading.

Guidelines:

- Keep route handlers thin.
- Put BlockEmulator-X-specific logic in `internal/emulator`.
- Return errors upward instead of logging and continuing silently.
- Use the shared JSON response shape:

```json
{
  "ok": true,
  "data": {},
  "error": ""
}
```

### React Frontend

Important files:

- `frontend/src/main.jsx`: UI layout, state, API calls, and SVG charts.
- `frontend/src/styles.css`: visual styling.

Guidelines:

- Keep form fields mapped clearly to backend JSON fields.
- Validate obvious input errors before sending requests.
- Keep controls accessible and predictable.
- Avoid adding heavy UI libraries unless they solve a real problem.

### Python Chart Server

Important files:

- `python-backend/app.py`: Flask routes.
- `python-backend/charts.py`: CSV parsing and matplotlib chart generation.

Guidelines:

- Keep chart endpoints read-only.
- Read result CSVs from `backend/workdir/exp/results/`.
- Return a clear JSON error when no experiment result exists.

## Adding a New Config Field

When adding a BlockEmulator-X config field, update all relevant layers:

1. Add the field to `backend/internal/emulator/types.go`.
2. Add a default value in `DefaultConfig`.
3. Read the field in `extractWebConfig`.
4. Write the field in `applyWebConfig`.
5. Add backend validation when needed.
6. Add the frontend control in `frontend/src/main.jsx`.
7. Add or adjust CSS in `frontend/src/styles.css`.
8. Update `README.md`.
9. Run backend and frontend checks.

Example mapping:

```text
frontend form field
  -> WebConfig JSON
  -> generated_config.yaml
  -> BlockEmulator-X process started with -config
```

## Testing and Verification

Run backend checks:

```sh
cd backend
go test ./...
```

If the system Go cache is not writable, use a project-local cache:

```sh
cd backend
mkdir -p .gocache
GOCACHE="$PWD/.gocache" go test ./...
```

Run frontend build:

```sh
cd frontend
npm run build
```

Run the optional chart server manually:

```sh
cd python-backend
python app.py
```

Smoke-test endpoints:

```sh
curl http://localhost:8080/api/config
curl http://localhost:8080/api/experiments/status
curl http://localhost:8080/api/results
curl http://localhost:5001/api/health
```

## Git Hygiene

Do not commit generated files or local runtime outputs:

- `backend/workdir/*`
- `backend/.gocache/`
- `frontend/node_modules/`
- `frontend/dist/`
- `python-backend/venv/`
- `__pycache__/`
- `*.log`

Before submitting a change, check:

```sh
git status --short
```

Only include files that are part of the intended change.

## Pull Request Checklist

- The change has a clear purpose.
- The original BlockEmulator-X config and IP table are not overwritten.
- Generated files are not committed.
- `go test ./...` passes for the backend.
- `npm run build` passes for the frontend when frontend files change.
- Python chart changes were smoke-tested when applicable.
- `README.md` or this guide was updated for user-visible behavior changes.

## Current Boundaries

This project currently targets local experiments. It does not include:

- User accounts or authentication.
- Multi-user experiment scheduling.
- Remote cluster deployment.
- Persistent experiment history beyond files in `backend/workdir/`.

Keep changes within these boundaries unless the task explicitly expands them.
