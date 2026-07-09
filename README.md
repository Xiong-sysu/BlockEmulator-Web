# BlockEmulator-X Web Console

BlockEmulator-X Web Console is a local web console for running experiments with
`block-emulator-x`. It provides a browser-based configuration panel, experiment
start/stop controls, runtime status, logs, CSV result previews, and lightweight
charts.

The project is intentionally simple: the web app does not modify the
BlockEmulator-X source code, and generated experiment files are written under
this project's `backend/workdir/` directory.

## Features

- Edit common BlockEmulator-X experiment parameters in a web form.
- Generate a dedicated runtime `config.yaml` and `ip_table.json`.
- Start and stop BlockEmulator-X consensus nodes and supervisor processes.
- View runtime status, process count, backend logs, and result file count.
- Read BlockEmulator-X CSV results and display them as tables.
- Show quick frontend SVG charts for TPS, CTX ratio, and TCL.
- Download the generated brief CSV result file.
- Optionally run a Python/Flask chart server for matplotlib PNG charts.

## Project Layout

```text
BlockEmulator-web/
├── backend/                 # Go HTTP API server
│   ├── internal/emulator/    # BlockEmulator-X config, process, and result logic
│   └── workdir/              # Generated runtime files and experiment outputs
├── frontend/                 # React + Vite single-page app
├── python-backend/           # Optional Flask + matplotlib chart server
├── docs/                     # Notes and guide drafts
├── start.sh                  # macOS/Linux startup helper
└── start.bat                 # Windows startup helper
```

## Requirements

- Go 1.22 or newer
- Node.js and npm
- Python 3.10 or newer, only if using `python-backend`
- A local BlockEmulator-X checkout next to this project by default:

```text
BlockEmulator/
├── BlockEmulator-web/
└── block-emulator-x/
```

If BlockEmulator-X is somewhere else, set `BLOCK_EMULATOR_X_ROOT`.

## Quick Start

From the project root:

```sh
./start.sh
```

This starts:

- Go backend: `http://localhost:8080`
- React frontend: `http://localhost:5173`
- Python chart server: `http://localhost:5001`

Start only one service:

```sh
./start.sh backend
./start.sh frontend
./start.sh charts
```

On Windows:

```bat
start.bat
start.bat backend
start.bat frontend
start.bat charts
```

## Manual Startup

### 1. Start the Go backend

```sh
cd backend
go run .
```

The backend automatically searches for `../block-emulator-x` or
`../../block-emulator-x`, depending on where it is started from.

Use a custom BlockEmulator-X path:

```sh
BLOCK_EMULATOR_X_ROOT=/absolute/path/to/block-emulator-x go run .
```

Use a custom backend address:

```sh
BLOCK_EMULATOR_WEB_ADDR=:8090 go run .
```

### 2. Start the React frontend

```sh
cd frontend
npm install
npm run dev
```

Open:

```text
http://localhost:5173
```

### 3. Start the optional Python chart server

```sh
cd python-backend
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python app.py
```

Open:

```text
http://localhost:5001
```

The Python chart server reads CSV files from `backend/workdir/exp/results/` by
default. Override that location with:

```sh
BLOCKEMULATOR_WORKDIR=/absolute/path/to/backend/workdir python app.py
```

## Runtime Files

This project avoids overwriting the original BlockEmulator-X configuration.

The Go backend reads the original config as a template:

```text
../block-emulator-x/config.yaml
```

It writes generated runtime files here:

```text
backend/workdir/generated_config.yaml
backend/workdir/generated_ip_table.json
backend/workdir/last_config.json
backend/workdir/status.json
backend/workdir/experiment.log
backend/workdir/exp/
```

When launching BlockEmulator-X, the backend passes these generated files with:

```sh
-config backend/workdir/generated_config.yaml
-ip_table backend/workdir/generated_ip_table.json
```

Experiment results are read from:

```text
backend/workdir/exp/results/
```

## Configurable Fields

The web form currently covers these fields:

### System

- `system.shard_num`
- `system.node_num`
- `system.limit`
- `system.consensus_type`

Supported consensus types:

- `static_relay`
- `static_broker`
- `clpa_relay`
- `clpa_broker`

### Consensus Node

- `consensus_node.block_interval`

### Supervisor

- `supervisor.tx_number`
- `supervisor.tx_injection_speed`
- `supervisor.epoch_duration`
- `supervisor.tx_source.tx_source_type`
- `supervisor.tx_source.tx_source_file`
- `supervisor.tx_source.exclude_contract_txs`

`exclude_contract_txs` controls whether CSV transaction sources should filter
smart-contract-related transactions.

### Network

- `network.bandwidth`
- `network.latency`

The backend forces `network.communication_mode` to `direct` for the first local
version of the console.

## Go Backend API

All backend responses use:

```json
{
  "ok": true,
  "data": {},
  "error": ""
}
```

Implemented endpoints:

- `GET /api/config`
- `POST /api/config/validate`
- `POST /api/config`
- `POST /api/ip-table`
- `POST /api/experiments/start`
- `POST /api/experiments/stop`
- `GET /api/experiments/status`
- `GET /api/experiments/logs`
- `GET /api/results`
- `GET /api/results/download/{file}.csv`

## Python Chart API

The optional chart server exposes:

- `GET /api/health`
- `GET /api/charts/tps?type=line|bar`
- `GET /api/charts/ctx_ratio?type=line|bar`
- `GET /api/charts/tcl?type=line|bar`
- `GET /api/charts/combined`
- `GET /`

The React frontend currently uses its own lightweight SVG charts. The Python
chart server is available as a standalone preview and for richer PNG charts.

## Typical Workflow

1. Start the Go backend and React frontend.
2. Open `http://localhost:5173`.
3. Adjust experiment parameters in the left panel.
4. Click `Save Config` to write generated config and IP table files.
5. Click `Start` to launch BlockEmulator-X processes.
6. Watch status cards and runtime logs.
7. After the experiment finishes, inspect charts and result tables.
8. Download the brief CSV if needed.

## Development Checks

Backend:

```sh
cd backend
go test ./...
```

Frontend:

```sh
cd frontend
npm run build
```

Python chart server:

```sh
cd python-backend
python app.py
```

## Troubleshooting

### BlockEmulator-X root not found

Set:

```sh
BLOCK_EMULATOR_X_ROOT=/absolute/path/to/block-emulator-x
```

### Frontend cannot reach backend

Make sure the Go backend is running on `http://localhost:8080`. If using a
different API base URL, start Vite with:

```sh
VITE_API_BASE=http://localhost:8090 npm run dev
```

### Ports are already in use

Default ports:

- Go backend: `8080`
- React frontend: `5173`
- Python charts: `5001`

Stop the conflicting process or use the environment variables above where
supported.

### No result data is displayed

Run an experiment first, then check:

```text
backend/workdir/exp/results/
```

The result reader expects a brief CSV such as:

- `relay_stats_brief_info.csv`
- `broker_stats_brief_info.csv`

### npm certificate errors

If `npm install` fails with a local issuer certificate error in a local lab
environment, retry once with:

```sh
npm install --strict-ssl=false
```

Use this only when you understand the network environment.
