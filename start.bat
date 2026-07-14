@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: ============================================================
::  start.bat — Launch all BlockEmulator-Web services (Windows)
::
::  Usage:
::    start.bat              Start all three services
::    start.bat backend      Start Go backend only
::    start.bat frontend     Start React frontend only
::    start.bat charts       Start Python chart server only
::    start.bat -h           Show help
::
::  Services:
::    Go backend      → http://localhost:9091
::    React frontend  → http://localhost:5173
::    Python charts   → http://localhost:5001
:: ============================================================

set SCRIPT_DIR=%~dp0
set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

set BACKEND_DIR=%SCRIPT_DIR%\backend
set FRONTEND_DIR=%SCRIPT_DIR%\frontend
set PYTHON_DIR=%SCRIPT_DIR%\python-backend

:: —— Parse argument ——
set MODE=%1
if "%MODE%"=="" set MODE=all

if /i "%MODE%"=="-h"  goto :help
if /i "%MODE%"=="--help" goto :help
if /i "%MODE%"=="help"    goto :help

echo.
echo ╔══════════════════════════════════════════════════╗
echo ║       BlockEmulator-Web Console Startup          ║
echo ╚══════════════════════════════════════════════════╝
echo.

:: —— Start Go backend ——
if /i not "%MODE%"=="backend" if /i not "%MODE%"=="all" goto :skip_backend
echo [backend] Starting Go backend...
where go >nul 2>&1
if errorlevel 1 (
    echo [error] 'go' not found. Please install Go first.
    goto :skip_backend
)
cd /d "%BACKEND_DIR%"
start "BlockEmulator-Go-Backend" cmd /c "go run . 2>&1"
echo           → http://localhost:9091
:skip_backend

:: —— Start React frontend ——
if /i not "%MODE%"=="frontend" if /i not "%MODE%"=="all" goto :skip_frontend
echo [frontend] Starting React + Vite dev server...
cd /d "%FRONTEND_DIR%"
if not exist "node_modules" (
    echo           node_modules not found, running npm install...
    call npm install
)
start "BlockEmulator-React-Frontend" cmd /c "npx vite --host 0.0.0.0"
echo           → http://localhost:5173
:skip_frontend

:: —— Start Python chart server ——
if /i not "%MODE%"=="charts" if /i not "%MODE%"=="all" goto :skip_charts
echo [charts] Starting Python chart server...
cd /d "%PYTHON_DIR%"
if not exist "venv" (
    echo           Creating Python virtual environment...
    python -m venv venv
    call venv\Scripts\activate.bat
    pip install -r requirements.txt
)
start "BlockEmulator-Python-Charts" cmd /c "venv\Scripts\python.exe app.py"
echo           → http://localhost:5001
:skip_charts

cd /d "%SCRIPT_DIR%"

echo.
echo ─────────────────────────────────────────────────
echo All requested services are running.
echo Close each window or press Ctrl+C in each to stop.
echo ─────────────────────────────────────────────────
goto :eof

:help
echo Usage: %~nx0 [backend^|frontend^|charts^|all]
echo.
echo Start one or all BlockEmulator-Web services.
echo.
echo   backend   Go API server        → http://localhost:9091
echo   frontend  React dev server     → http://localhost:5173
echo   charts    Python chart server  → http://localhost:5001
echo   all       Start all three (default)
goto :eof
