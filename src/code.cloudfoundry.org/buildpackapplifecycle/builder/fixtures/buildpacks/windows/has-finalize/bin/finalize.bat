@echo off

set BUILD_DIR=%1
set CACHE_DIR=%2
set DEP_DIR=%3
set SUB_DIR=%4


echo FINALIZING

set contents=has-finalize-buildpack

echo %contents% > %BUILD_DIR%\finalized
echo %contents% > %CACHE_DIR%\finalized
echo %contents% > %DEP_DIR%\%SUB_DIR%\finalized
