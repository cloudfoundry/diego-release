@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2

echo COMPILE should never run

set contents=has-finalize-buildpack

echo %contents% > %BUILD_DIR%\compiled
echo %contents% > %CACHE_DIR%\compiled
