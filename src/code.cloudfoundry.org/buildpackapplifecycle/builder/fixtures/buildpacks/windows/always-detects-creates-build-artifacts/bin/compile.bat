@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2

echo WOO

echo "x" > %BUILD_DIR%\compiled
echo "x" > %CACHE_DIR%\build-artifact
