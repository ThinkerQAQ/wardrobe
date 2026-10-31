@echo off
setlocal
cd /d "%~dp0"
echo Wardrobe - Local Installer and ChatGPT MCP Setup
echo.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\Setup-Wardrobe.ps1" %*
set "ret=%ERRORLEVEL%"
echo.
if not "%ret%"=="0" echo Installation was not fully completed. Check the message above.
echo Press any key to close this window.
pause >nul
exit /b %ret%
