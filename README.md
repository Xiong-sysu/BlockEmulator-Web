# BlockEmulator-X Web Console

本项目是 `../block-emulator-x` 的本地网页控制台，提供配置、启动/停止实验、查看 CSV 结果和日志的第一版实现。

## 目录

- `backend/`：Go HTTP API，默认监听 `http://localhost:8080`
- `frontend/`：React + Vite 单页控制台，默认监听 `http://localhost:5173`
- `backend/workdir/`：保存网页生成的配置、IP 表、运行状态、运行日志和实验输出

## 启动后端

```sh
cd backend
go run .
```

后端默认会自动寻找旁边的 `../block-emulator-x`。如果你的路径不同：

```sh
BLOCK_EMULATOR_X_ROOT=/absolute/path/to/block-emulator-x go run .
```

## 启动前端

当前机器需要可用的 npm/pnpm/yarn 之一。以 npm 为例：

```sh
cd frontend
npm install
npm run dev
```

打开 `http://localhost:5173`。

### Python 统计图表后端（python-stats-charts 分支）

```sh
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python app.py
```

Python 后端默认监听 `http://localhost:5001`（macOS 上 5000 常被 AirPlay 占用），读取 Go 后端 `workdir/exp/results/` 中的 CSV 结果，用 matplotlib 生成折线图和柱状图。

**图表 API：**
- `GET /api/charts/tps?type=line|bar` — TPS 图表 (PNG)
- `GET /api/charts/ctx_ratio?type=line|bar` — CTX Ratio 图表 (PNG)
- `GET /api/charts/tcl?type=line|bar` — TCL 图表 (PNG)
- `GET /api/charts/combined` — 三合一组合图 (PNG)
- `GET /` — 独立 HTML 预览页面

## 已实现 API（Go 后端）

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

## 注意

启动实验不会覆盖 `../block-emulator-x/config.yaml` 或 `../block-emulator-x/ip_table.json`。

Web 后端会生成并使用：

```text
backend/workdir/generated_config.yaml
backend/workdir/generated_ip_table.json
backend/workdir/exp/
```

BlockEmulator-X 进程启动时会通过 `-config` 和 `-ip_table` 使用这些生成文件，实验结果也会写入 `backend/workdir/exp/results/`。
