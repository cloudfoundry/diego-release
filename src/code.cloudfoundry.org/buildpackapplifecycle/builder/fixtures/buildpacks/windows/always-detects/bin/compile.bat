@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2

echo WOO
cmd /C set
echo always-detects-buildpack > %BUILD_DIR%\compiled
echo always-detects-buildpack > %CACHE_DIR%\compiled

