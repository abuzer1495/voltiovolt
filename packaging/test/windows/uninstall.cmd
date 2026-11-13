@echo off
rem SPDX-License-Identifier: AGPL-3.0-or-later
rem
rem uninstall.cmd: removes the portable install of VoltioVolt (VLT-IS-008
rem TASK 03-00-00-006). No administrator rights. Asks first. Removes the
rem install folder, the Start menu shortcut and the startup shortcut of
rem the user. Asks separately before it deletes the data folder.
setlocal EnableExtensions EnableDelayedExpansion
set "DEST=%LOCALAPPDATA%\Programs\VoltioVolt"
set "LNK=%APPDATA%\Microsoft\Windows\Start Menu\Programs\VoltioVolt.lnk"
set "STARTUP=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\VoltioVolt.lnk"
set "DATA1=%DEST%\data"
set "DATA2=%LOCALAPPDATA%\VoltioVolt"

echo This removes the VoltioVolt files from "%DEST%".
echo Close VoltioVolt before you continue.
set "ANS="
set /p ANS=Continue? (Y/N) 
if /I not "%ANS%"=="Y" (
    echo Cancelled.
    pause
    exit /b 0
)

if exist "%LNK%" (
    del /Q "%LNK%" || goto :fail
    echo Removed the Start menu shortcut.
)
if exist "%STARTUP%" (
    del /Q "%STARTUP%" || goto :fail
    echo Removed the startup shortcut.
)

if exist "%DEST%\VoltioVolt.exe" del /Q "%DEST%\VoltioVolt.exe" || goto :fail
if exist "%DEST%\vlt-probe.exe" del /Q "%DEST%\vlt-probe.exe" || goto :fail

rem The data folder can be next to the binary or in the profile folder.
set "DATADIR="
if exist "%DATA1%" set "DATADIR=%DATA1%"
if not defined DATADIR if exist "%DATA2%" set "DATADIR=%DATA2%"
if defined DATADIR (
    echo.
    echo The data folder is "%DATADIR%".
    echo If you delete it, the stored Settings and Secrets are lost.
    set "ANS2="
    set /p ANS2=Delete the data folder? (Y/N) 
    if /I "!ANS2!"=="Y" (
        rmdir /S /Q "%DATADIR%"
        if exist "%DATADIR%" (
            echo ERROR: cannot remove "%DATADIR%". Close VoltioVolt, then try again.
            goto :fail
        )
        echo Data folder deleted.
    ) else (
        echo Data folder kept.
    )
)

if exist "%DEST%" rmdir /S /Q "%DEST%" 2>nul
if exist "%DEST%" (
    rem A kept data folder inside DEST can leave the folder.
    echo The folder "%DEST%" is not empty. It is kept.
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
