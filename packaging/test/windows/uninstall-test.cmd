@echo off
rem SPDX-License-Identifier: AGPL-3.0-or-later
rem
rem uninstall-test.cmd: removes the test install of VoltioVolt (VLT-IS-007
rem TASK 03-00-00-001).
rem 1. Asks for Administrator rights (UAC).
rem 2. Runs "vlt-setup dev-uninstall --dir". vlt-setup asks before it
rem    deletes the data folder "%ProgramData%\VoltioVolt".
rem 3. Removes the folder "%ProgramFiles%\VoltioVolt-Test" and the shortcut.
setlocal EnableExtensions
set "VLT_SELF=%~f0"

net session >nul 2>&1
if errorlevel 1 (
    echo Asking for Administrator rights...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath $env:ComSpec -ArgumentList ('/c \"\"' + $env:VLT_SELF + '\"\"') -Verb RunAs"
    exit /b
)

set "DEST=%ProgramFiles%\VoltioVolt-Test"
set "LNK=%ProgramData%\Microsoft\Windows\Start Menu\Programs\VoltioVolt (test).lnk"
set "TMPSETUP=%TEMP%\vlt-setup-uninstall.exe"

echo Close the VLT GUI before you continue.
pause

rem vlt-setup runs from %TEMP%, so the install folder can be removed.
if exist "%DEST%\vlt-setup.exe" (
    copy /Y "%DEST%\vlt-setup.exe" "%TMPSETUP%" >nul || goto :fail
) else (
    if not exist "%~dp0vlt-setup.exe" (
        echo ERROR: vlt-setup.exe not found.
        goto :fail
    )
    copy /Y "%~dp0vlt-setup.exe" "%TMPSETUP%" >nul || goto :fail
)

echo Run vlt-setup dev-uninstall
"%TMPSETUP%" dev-uninstall --dir "%DEST%"
if errorlevel 1 (
    del /Q "%TMPSETUP%" >nul 2>&1
    goto :fail
)
del /Q "%TMPSETUP%" >nul 2>&1

if exist "%LNK%" del /Q "%LNK%" || goto :fail
if exist "%DEST%" rmdir /S /Q "%DEST%"
if exist "%DEST%" (
    echo ERROR: cannot remove "%DEST%". Close vlt-gui.exe and vlt-probe.exe, then try again.
    goto :fail
)

echo.
echo Done.
pause
exit /b 0

:fail
echo.
echo ERROR: the uninstall did not complete. Read the messages above.
pause
exit /b 1
