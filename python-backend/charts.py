"""Generate statistical charts from BlockEmulator CSV results using matplotlib."""

import csv
import io
import os
from pathlib import Path

import matplotlib
matplotlib.use("Agg")  # non-interactive backend for server
import matplotlib.pyplot as plt
import matplotlib.ticker as ticker

# —— style settings ——
plt.rcParams.update(
    {
        "figure.facecolor": "#ffffff",
        "axes.facecolor": "#f8fafb",
        "axes.edgecolor": "#d6dee6",
        "axes.grid": True,
        "grid.alpha": 0.4,
        "grid.color": "#cbd5df",
        "font.family": "sans-serif",
        "font.size": 11,
        "axes.titlesize": 14,
        "axes.labelsize": 11,
        "axes.titleweight": "bold",
        "lines.linewidth": 2.2,
        "lines.marker": "o",
        "lines.markersize": 5,
    }
)

# —— chart colour palette ——
COLORS = ["#0f766e", "#ea580c", "#2563eb", "#7c3aed"]


def find_results_dir(workdir: str) -> str:
    """Locate the experiment results directory under workdir."""
    candidate = os.path.join(workdir, "exp", "results")
    if os.path.isdir(candidate):
        return candidate
    # fallback: walk under workdir looking for a results/ folder
    for root, dirs, _ in os.walk(workdir):
        if "results" in dirs:
            return os.path.join(root, "results")
    return candidate  # return default even if it doesn't exist yet


def find_brief_csv(results_dir: str) -> str | None:
    """Return path to the first brief CSV found in results_dir."""
    if not os.path.isdir(results_dir):
        return None
    preferred = [
        "relay_stats_brief_info.csv",
        "broker_stats_brief_info.csv",
    ]
    for name in preferred:
        path = os.path.join(results_dir, name)
        if os.path.isfile(path):
            return path
    # fallback: any csv with 'brief' in the name
    try:
        entries = sorted(os.listdir(results_dir))
    except FileNotFoundError:
        return None
    for name in entries:
        if name.lower().endswith(".csv") and "brief" in name.lower():
            return os.path.join(results_dir, name)
    return None


def read_csv_rows(path: str) -> tuple[list[str], list[dict[str, str]]]:
    """Read a CSV file and return (columns, rows)."""
    with open(path, newline="", encoding="utf-8") as fh:
        reader = csv.DictReader(fh)
        columns = reader.fieldnames or []
        rows = list(reader)
    return columns, rows


def _parse_numeric(rows: list[dict[str, str]], column: str) -> list[float]:
    """Extract numeric values for a named column from CSV rows."""
    values = []
    for row in rows:
        try:
            values.append(float(row[column]))
        except (KeyError, ValueError, TypeError):
            values.append(float("nan"))
    return values


# ——————————————————————————————————————————————————————
#  Individual chart builders (line + bar variants)
# ——————————————————————————————————————————————————————

METRICS = {
    "tps": {
        "column": "Avg. TPS of this epoch (txs per second)",
        "title": "TPS (Transactions Per Second)",
        "ylabel": "TPS",
    },
    "ctx_ratio": {
        "column": "CTX ratio of this epoch",
        "title": "CTX Ratio",
        "ylabel": "CTX Ratio",
    },
    "tcl": {
        "column": "Avg. TCL of this epoch (second)",
        "title": "TCL (Transaction Confirmation Latency)",
        "ylabel": "TCL (ns)",
    },
}


def build_single_chart(
    chart_type: str,
    metric_key: str,
    rows: list[dict[str, str]],
) -> io.BytesIO | None:
    """Build one chart (line or bar) and return a PNG buffer.

    Parameters
    ----------
    chart_type : "line" | "bar"
    metric_key : "tps" | "ctx_ratio" | "tcl"
    rows       : CSV rows from the brief CSV

    Returns
    -------
    io.BytesIO containing a PNG image, or None if there is no data.
    """
    if metric_key not in METRICS:
        raise ValueError(f"Unknown metric: {metric_key}")

    info = METRICS[metric_key]
    values = _parse_numeric(rows, info["column"])
    # drop NaN
    values = [v for v in values if not (v != v)]  # NaN != NaN → True
    if not values:
        return None

    epochs = list(range(1, len(values) + 1))

    fig, ax = plt.subplots(figsize=(8, 4.5))
    color = COLORS[0]

    if chart_type == "line":
        ax.plot(epochs, values, color=color, marker="o", linewidth=2.2, markersize=5, zorder=3)
        # fill area under curve
        ax.fill_between(epochs, values, alpha=0.08, color=color)
    elif chart_type == "bar":
        bars = ax.bar(epochs, values, color=color, alpha=0.85, edgecolor="white", linewidth=0.6, zorder=3)
        # highlight the tallest bar
        if values:
            max_val = max(values)
            for bar, val in zip(bars, values):
                if val == max_val:
                    bar.set_color("#ea580c")

    ax.set_title(info["title"], pad=12)
    ax.set_xlabel("Epoch")
    ax.set_ylabel(info["ylabel"])
    ax.xaxis.set_major_locator(ticker.MaxNLocator(integer=True, min_n_ticks=1, nbins=12))
    ax.yaxis.set_major_formatter(ticker.FuncFormatter(lambda x, _: f"{x:,.0f}" if abs(x) >= 100 else f"{x:,.2f}"))
    fig.tight_layout(pad=1.2)

    buf = io.BytesIO()
    fig.savefig(buf, format="png", dpi=120)
    plt.close(fig)
    buf.seek(0)
    return buf


def build_combined_chart(rows: list[dict[str, str]]) -> io.BytesIO | None:
    """Build a single figure with three subplots (TPS, CTX Ratio, TCL) using line charts."""
    if not rows:
        return None

    fig, axes = plt.subplots(1, 3, figsize=(18, 5))
    fig.subplots_adjust(wspace=0.28)

    for ax, (metric_key, info) in zip(axes, METRICS.items()):
        values = _parse_numeric(rows, info["column"])
        values = [v for v in values if not (v != v)]
        if not values:
            ax.text(0.5, 0.5, "no data", ha="center", va="center", transform=ax.transAxes, color="#999")
            ax.set_title(info["title"])
            continue

        epochs = list(range(1, len(values) + 1))
        ax.plot(epochs, values, color=COLORS[0], marker="o", linewidth=2.2, markersize=4, zorder=3)
        ax.fill_between(epochs, values, alpha=0.08, color=COLORS[0])
        ax.set_title(info["title"], pad=10)
        ax.set_xlabel("Epoch")
        ax.set_ylabel(info["ylabel"])
        ax.xaxis.set_major_locator(ticker.MaxNLocator(integer=True, min_n_ticks=1, nbins=8))
        ax.yaxis.set_major_formatter(
            ticker.FuncFormatter(lambda x, _: f"{x:,.0f}" if abs(x) >= 100 else f"{x:,.2f}")
        )

    fig.tight_layout(pad=1.5)
    buf = io.BytesIO()
    fig.savefig(buf, format="png", dpi=120)
    plt.close(fig)
    return buf