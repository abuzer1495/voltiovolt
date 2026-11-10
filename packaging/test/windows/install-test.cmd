@echo off
rem SPDX-License-Identifier: AGPL-3.0-or-later
rem
rem install-test.cmd: test install of VoltioVolt on Windows (VLT-IS-007
rem TASK 03-00-00-001). This is not a package. Use a test computer.
rem 1. Asks for Administrator rights (UAC).
rem 2. Copies the binaries to "%ProgramFiles%\VoltioVolt-Test\".
rem 3. Runs "vlt-setup dev-install --dir" with that folder.
rem 4. Makes the Start menu shortcut "VoltioVolt (test)" for vlt-gui.exe.
setlocal EnableExtensions
set "VLT_SELF=%~f0"

net session >nul 2>&1
if errorlevel 1 (
    echo Asking for Administrator rights...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath $env:ComSpec -ArgumentList ('/c \"\"' + $env:VLT_SELF + '\"\"') -Verb RunAs"
    exit /b
)

set "SRC=%~dp0"
set "DEST=%ProgramFiles%\VoltioVolt-Test"
set "LNK=%ProgramData%\Microsoft\Windows\Start Menu\Programs\VoltioVolt (test).lnk"

for %%F in (vltd.exe vlt-gui.exe vlt-setup.exe vlt-probe.exe) do (
    if not exist "%SRC%%%F" (
        echo ERROR: %%F is not in "%SRC%". Extract all files of the zip file first.
        goto :fail
    )
)

sc query vltd >nul 2>&1
if not errorlevel 1 (
    echo ERROR: the service vltd exists. Run uninstall-test.cmd first.
    goto :fail
)

echo Copy the binaries to "%DEST%"
if not exist "%DEST%" mkdir "%DEST%" || goto :fail
for %%F in (vltd.exe vlt-gui.exe vlt-setup.exe vlt-probe.exe) do (
    copy /Y "%SRC%%%F" "%DEST%\%%F" >nul || goto :fail
)

echo Run vlt-setup dev-install
"%DEST%\vlt-setup.exe" dev-install --dir "%DEST%" || goto :fail

echo Make the Start menu shortcut "VoltioVolt (test)"
powershell -NoProfile -ExecutionPolicy Bypass -Command "$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:LNK); $s.TargetPath = (Join-Path $env:DEST 'vlt-gui.exe'); $s.WorkingDirectory = $env:DEST; $s.Description = 'VoltioVolt (test)'; $s.Save()" || goto :fail

echo.
echo Done. Sign out and sign in again, then start "VoltioVolt (test)" from the Start menu.
pause
exit /b 0

:fail
echo.
echo ERROR: the install did not complete. Read the messages above.
pause
exit /b 1
