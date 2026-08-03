@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2

if NOT "%3" == "" (
  set DEPS_DIR=%3
) else (
  set DEPS_DIR=""
)


echo WOO

if exist %CACHE_DIR%\old-compile (
  set /p contents=<%CACHE_DIR%\old-compile
) else (
  set contents=also-always-detects-buildpack
)

if NOT %DEPS_DIR% == "" (
  set contents=%contents%-deps-provided
  echo %contents% > %DEPS_DIR%\compiled

)

echo %contents% > %BUILD_DIR%\compiled
echo %contents% > %CACHE_DIR%\compiled
