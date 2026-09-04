@echo off
setlocal
cd /d "%~dp0\engine"

set "PATH=C:\Users\UPCOMING\go\bin;C:\Program Files\Go\bin;%PATH%"

where go >nul 2>nul
if %errorlevel% neq 0 (
    echo [ERROR] Go compiler not found in PATH or C:\Users\UPCOMING\go\bin
    pause
    exit /b 1
)

echo ==================================================================
echo   ⚡ COMPILING AND LAUNCHING GO HIGH-CONCURRENCY TWITCH SWARM
echo ==================================================================
go build -o ../twitch-engine.exe main.go
if %errorlevel% neq 0 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)

cd ..
twitch-engine.exe -channel vinco_vibeslive -viewers 50 -proxies data/proxies.json
pause
