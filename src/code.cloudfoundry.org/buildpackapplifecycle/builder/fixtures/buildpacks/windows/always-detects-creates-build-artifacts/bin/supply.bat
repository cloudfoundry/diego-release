@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2
set DEP_DIR=%3
set SUB_DIR=%4

echo SUPPLYING

if exist "%CACHE_DIR%\old-supply" (
  set /p contents=<%CACHE_DIR%\old-supply
) else (
  set contents=always-detects-creates-buildpack-artifacts
)

echo %contents% > %CACHE_DIR%\supplied
echo %contents% > %DEP_DIR%\%SUB_DIR%\supplied

(
echo ---
echo name: Creates Buildpack Artifacts
echo version: 9.1.3
) > %DEP_DIR%\%SUB_DIR%\config.yml
