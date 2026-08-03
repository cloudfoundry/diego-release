@echo off
REM Args: BUILD_DIR CACHE_DIR DEPS_DIR DEPS_IDX
set DEPS_DIR=%3
set DEPS_IDX=%4

mkdir "%DEPS_DIR%\%DEPS_IDX%" 2>nul
(
echo name: "failing-buildpack"
echo version: "1.2.3"
) > "%DEPS_DIR%\%DEPS_IDX%\config.yml"

exit 0
