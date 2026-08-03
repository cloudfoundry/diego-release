@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2
set DEP_DIR=%3
set SUB_DIR=%4


echo SUPPLYING

if exist %CACHE_DIR%\old-supply (
  set /P contents=<%CACHE_DIR%\old-supply
) else (
  set contents=always-detects-buildpack
)

echo %contents% > %CACHE_DIR%\supplied
echo %contents% > %DEP_DIR%\%SUB_DIR%\supplied
