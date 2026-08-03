@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2
set DEP_DIR=%3
set SUB_DIR=%4

echo ALSO-SUPPLYING

echo also-always-detects-buildpack > %CACHE_DIR%\supplied
echo also-always-detects-buildpack > %DEP_DIR%\%SUB_DIR%\supplied
