@echo off
rem SPDX-License-Identifier: AGPL-3.0-or-later
rem
rem install.cmd: portable install of VoltioVolt (VLT-IS-008 TASK 03-00-00-006).
rem No administrator rights. Copies VoltioVolt.exe, vlt-probe.exe and
rem vlt-simulator.exe to "%LOCALAPPDATA%\Programs\VoltioVolt". Asks before
rem it makes a Start menu shortcut for VoltioVolt (not for the simulator).
rem Starts VoltioVolt.exe.
setlocal EnableExtensions EnableDelayedExpansion
set "SRC=%~dp0"
set "DEST=%LOCALAPPDATA%\Programs\VoltioVolt"
set "LNK=%APPDATA%\Microsoft\Windows\Start Menu\Programs\VoltioVolt.lnk"

for %%F in (VoltioVolt.exe vlt-probe.exe vlt-simulator.exe) do (
    if not exist "%SRC%%%F" (
        echo ERROR: %%F is not in "%SRC%". Extract all files of the zip file first.
        goto :fail
    )
)

echo Copy the files to "%DEST%"
if not exist "%DEST%" mkdir "%DEST%" || goto :fail
copy /Y "%SRC%VoltioVolt.exe" "%DEST%\VoltioVolt.exe" >nul || goto :fail
copy /Y "%SRC%vlt-probe.exe" "%DEST%\vlt-probe.exe" >nul || goto :fail
copy /Y "%SRC%vlt-simulator.exe" "%DEST%\vlt-simulator.exe" >nul || goto :fail

set "ANS="
set /p ANS=Make a Start menu shortcut? (Y/N) 
if /I "%ANS%"=="Y" (
    echo Make the Start menu shortcut of the user
    powershell -NoProfile -ExecutionPolicy Bypass -Command "$s = (New-Object -ComObject WScript.Shell).CreateShortcut($env:LNK); $s.TargetPath = (Join-Path $env:DEST 'VoltioVolt.exe'); $s.WorkingDirectory = $env:DEST; $s.Description = 'VoltioVolt'; $s.Save()" || goto :fail
    echo Shortcut: "%LNK%"
)

echo Start VoltioVolt.exe
start "" "%DEST%\VoltioVolt.exe"
echo.
echo Done.
pause
exit /b 0

:fail
echo.
echo ERROR: the install did not complete. Read the messages above.
pause
exit /b 1
