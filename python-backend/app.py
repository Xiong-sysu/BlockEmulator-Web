"""Flask server that generates and serves statistical charts for BlockEmulator-X Web Console.

Usage
-----
    cd python-backend
    pip install -r requirements.txt
    python app.py

The server reads CSV results from the Go backend's workdir and exposes
chart images via /api/charts/<metric>.<format> endpoints.
"""

import os
from pathlib import Path

from flask import Flask, Response, jsonify, request, send_file

from charts import (
    METRICS,
    build_combined_chart,
    build_single_chart,
    find_brief_csv,
    find_results_dir,
    read_csv_rows,
)

app = Flask(__name__)

# —— CORS after_request ——
@app.after_request
def add_cors_headers(response: Response) -> Response:
    response.headers["Access-Control-Allow-Origin"] = "*"
    response.headers["Access-Control-Allow-Headers"] = "Content-Type"
    response.headers["Access-Control-Allow-Methods"] = "GET, OPTIONS"
    return response

# — config —
WORKDIR = os.environ.get(
    "BLOCKEMULATOR_WORKDIR",
    os.path.join(os.path.dirname(__file__), "..", "backend", "workdir"),
)
RESULTS_DIR = find_results_dir(WORKDIR)


def _get_rows():
    """Read the brief CSV and return rows; return empty list on failure."""
    csv_path = find_brief_csv(RESULTS_DIR)
    if csv_path is None:
        return []
    _, rows = read_csv_rows(csv_path)
    return rows


# ————————————————————————————————————————
#  Health / info
# ————————————————————————————————————————

@app.route("/api/health")
def health():
    """Lightweight health check — returns whether CSV data is available."""
    csv_path = find_brief_csv(RESULTS_DIR)
    rows = _get_rows()
    return jsonify({
        "ok": True,
        "data_available": len(rows) > 0,
        "csv_path": csv_path or None,
        "row_count": len(rows),
    })


# ————————————————————————————————————————
#  Chart image endpoints
# ————————————————————————————————————————

@app.route("/api/charts/<metric>")
def chart_metric(metric: str):
    """Return a single-metric chart as PNG.

    Query params
    -----------
    type : "line" (default) | "bar"
    """
    if metric not in METRICS:
        return jsonify({"ok": False, "error": f"Unknown metric: {metric}. Use one of: {', '.join(METRICS)}"}), 404

    rows = _get_rows()
    if not rows:
        return jsonify({"ok": False, "error": "No result data available. Run an experiment first."}), 404

    chart_type = request.args.get("type", "line")
    if chart_type not in ("line", "bar"):
        chart_type = "line"

    buf = build_single_chart(chart_type, metric, rows)
    if buf is None:
        return jsonify({"ok": False, "error": f"No valid data for metric: {metric}"}), 404

    return send_file(buf, mimetype="image/png")


@app.route("/api/charts/combined")
def chart_combined():
    """Return a combined chart (3 subplots) as PNG."""
    rows = _get_rows()
    if not rows:
        return jsonify({"ok": False, "error": "No result data available. Run an experiment first."}), 404

    buf = build_combined_chart(rows)
    if buf is None:
        return jsonify({"ok": False, "error": "No valid data for combined chart."}), 404

    buf.seek(0)
    return send_file(buf, mimetype="image/png")


# ————————————————————————————————————————
#  Simple HTML preview page
# ————————————————————————————————————————

@app.route("/")
def index():
    """A standalone HTML page showing all 3 charts."""
    metrics_html = ""
    for key in METRICS:
        for chart_type in ("line", "bar"):
            metrics_html += f"""
            <div class="chart-card">
              <h3>{METRICS[key]['title']} — {chart_type.title()} Chart</h3>
              <img src="/api/charts/{key}?type={chart_type}" alt="{METRICS[key]['title']} ({chart_type})" />
            </div>"""

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>BlockEmulator-X · Statistics</title>
<style>
  body {{
    font-family: Inter, system-ui, sans-serif;
    background: #eef2f5;
    margin: 0;
    padding: 32px 24px;
    color: #18202a;
  }}
  h1 {{ margin: 0 0 8px; font-size: 26px; }}
  .eyebrow {{ color: #667684; font-size: 12px; font-weight: 700; text-transform: uppercase; }}
  .grid {{
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
    gap: 20px;
    margin-top: 24px;
  }}
  .chart-card {{
    background: #ffffff;
    border: 1px solid #d9e0e7;
    border-radius: 10px;
    padding: 18px;
  }}
  .chart-card h3 {{
    margin: 0 0 12px;
    font-size: 16px;
    color: #405160;
  }}
  .chart-card img {{
    width: 100%;
    height: auto;
    border-radius: 6px;
  }}
  .combined {{
    margin-top: 24px;
    background: #ffffff;
    border: 1px solid #d9e0e7;
    border-radius: 10px;
    padding: 18px;
  }}
  .combined img {{
    width: 100%;
    height: auto;
    border-radius: 6px;
  }}
</style>
</head>
<body>
  <p class="eyebrow">BlockEmulator-X</p>
  <h1>Experiment Statistics</h1>
  <div class="grid">{metrics_html}</div>
  <div class="combined">
    <h3>Combined View</h3>
    <img src="/api/charts/combined" alt="Combined TPS / CTX Ratio / TCL chart" />
  </div>
</body>
</html>"""


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 5001))
    print(f"Chart server → http://localhost:{port}")
    app.run(host="0.0.0.0", port=port, debug=True)