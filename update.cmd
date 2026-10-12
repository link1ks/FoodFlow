@echo off
setlocal
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\update.ps1" -Action update
set "foodflow_update_result=%errorlevel%"
echo.
pause
exit /b %foodflow_update_result%
