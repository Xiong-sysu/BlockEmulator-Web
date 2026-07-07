#!/usr/bin/env bash
#
# start.sh — Launch all BlockEmulator-Web services
#
# Usage:
#   ./start.sh              # Start all three services
#   ./start.sh backend      # Start Go backend only
#   ./start.sh frontend     # Start React frontend only
#   ./start.sh charts       # Start Python chart server only
#   ./start.sh -h           # Show help
#
# Services:
#   Go backend      → http://localhost:8080
#   React frontend  → http://localhost:5173
#   Python charts   → http://localhost:5001

set -euo pipefail

# —— Paths ——
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$SCRIPT_DIR/backend"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
PYTHON_DIR="$SCRIPT_DIR/python-backend"

# —— Colors ——
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# —— PIDs for cleanup ——
declare -a PIDS=()
CLEANUP_DONE=false

cleanup() {
    if $CLEANUP_DONE; then return; fi
    CLEANUP_DONE=true
    echo ""
    echo -e "${YELLOW}[shutdown] Stopping all services...${NC}"
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            wait "$pid" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}[shutdown] All services stopped.${NC}"
}
trap cleanup EXIT INT TERM

# —— Helpers ——
check_command() {
    if ! command -v "$1" &>/dev/null; then
        echo -e "${RED}[error] '$1' not found. Please install it first.${NC}"
        return 1
    fi
    return 0
}

print_banner() {
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║       BlockEmulator-Web Console Startup          ║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════╝${NC}"
    echo ""
}

# —— Start functions ——
start_backend() {
    echo -e "${GREEN}[backend]${NC} Starting Go backend..."
    check_command go || return 1

    cd "$BACKEND_DIR"
    go run . &
    local pid=$!
    PIDS+=("$pid")
    echo -e "       PID: $pid  →  ${CYAN}http://localhost:8080${NC}"
    cd "$SCRIPT_DIR"
}

start_frontend() {
    echo -e "${GREEN}[frontend]${NC} Starting React + Vite dev server..."

    cd "$FRONTEND_DIR"

    # Install deps if node_modules is missing
    if [ ! -d "node_modules" ]; then
        echo -e "       ${YELLOW}node_modules not found, running npm install...${NC}"
        npm install
    fi

    npm run dev &
    local pid=$!
    PIDS+=("$pid")
    echo -e "       PID: $pid  →  ${CYAN}http://localhost:5173${NC}"
    cd "$SCRIPT_DIR"
}

start_charts() {
    echo -e "${GREEN}[charts]${NC} Starting Python chart server..."

    cd "$PYTHON_DIR"

    # Ensure venv exists
    if [ ! -d "venv" ]; then
        echo -e "       ${YELLOW}Creating Python virtual environment...${NC}"
        python3 -m venv venv
        source venv/bin/activate
        pip install -r requirements.txt
    else
        source venv/bin/activate
    fi

    python app.py &
    local pid=$!
    PIDS+=("$pid")
    echo -e "       PID: $pid  →  ${CYAN}http://localhost:5001${NC}"
    cd "$SCRIPT_DIR"
}

# —— Main ——
print_banner

case "${1:-all}" in
    backend)
        start_backend
        ;;
    frontend)
        start_frontend
        ;;
    charts)
        start_charts
        ;;
    all)
        start_backend
        start_frontend
        start_charts
        ;;
    -h|--help|help)
        echo "Usage: $0 [backend|frontend|charts|all]"
        echo ""
        echo "Start one or all BlockEmulator-Web services."
        echo ""
        echo "  backend   Go API server        → http://localhost:8080"
        echo "  frontend  React dev server     → http://localhost:5173"
        echo "  charts    Python chart server  → http://localhost:5001"
        echo "  all       Start all three (default)"
        exit 0
        ;;
    *)
        echo -e "${RED}[error] Unknown service: '$1'${NC}"
        echo "Usage: $0 [backend|frontend|charts|all]"
        exit 1
        ;;
esac

echo ""
echo -e "${YELLOW}─────────────────────────────────────────────────${NC}"
echo -e "${GREEN}All requested services are running.${NC}"
echo -e "Press ${YELLOW}Ctrl+C${NC} to stop all services."
echo -e "${YELLOW}─────────────────────────────────────────────────${NC}"

# Wait for all background processes
wait