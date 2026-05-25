@echo off
setlocal

cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo Go is not installed or not available in PATH.
    exit /b 1
)

if "%XROUTER_ADDR%"=="" set "XROUTER_ADDR=:1213"

echo Starting XRouter on %XROUTER_ADDR%
go run ./cmd/xrouter
