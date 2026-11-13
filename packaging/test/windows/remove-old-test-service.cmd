@echo off
rem SPDX-License-Identifier: AGPL-3.0-or-later
rem
rem remove-old-test-service.cmd: only for computers with the VLT-IS-007
rem test installation (VLT-IS-008 TASK 03-00-00-006). Asks for
rem administrator rights. Stops and removes the service vltd, removes the
rem user right of NT SERVICE\vltd, removes the group "VLT Operators" if it
rem has no other members, removes C:\Program Files\VoltioVolt-Test and the
rem old shortcut. Asks before it deletes C:\ProgramData\VoltioVolt.
setlocal EnableExtensions EnableDelayedExpansion
set "VLT_SELF=%~f0"

net session >nul 2>&1
if errorlevel 1 (
    echo Asking for Administrator rights...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Start-Process -FilePath $env:ComSpec -ArgumentList ('/c \"\"' + $env:VLT_SELF + '\"\"') -Verb RunAs"
    exit /b
)

set "DEST=%ProgramFiles%\VoltioVolt-Test"
if "%DEST%"=="\VoltioVolt-Test" set "DEST=C:\Program Files\VoltioVolt-Test"
set "LNK=%ProgramData%\Microsoft\Windows\Start Menu\Programs\VoltioVolt (test).lnk"
set "DATA=%ProgramData%\VoltioVolt"
if "%DATA%"=="\VoltioVolt" set "DATA=C:\ProgramData\VoltioVolt"

echo This removes the VLT-IS-007 test service and the old test files.
echo Close the old VLT GUI before you continue.
set "ANS="
set /p ANS=Continue? (Y/N) 
if /I not "%ANS%"=="Y" (
    echo Cancelled.
    pause
    exit /b 0
)

sc query vltd >nul 2>&1
if not errorlevel 1 (
    echo Stop the service vltd
    sc stop vltd >nul
    echo Remove the user right SeShutdownPrivilege of NT SERVICE\vltd
    powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $sid=(New-Object System.Security.Principal.NTAccount('NT SERVICE\vltd')).Translate([System.Security.Principal.SecurityIdentifier]); $tmp=Join-Path $env:TEMP 'vlt-old-right.inf'; secedit /export /cfg $tmp /quiet; Write-Host 'Exported the policy.'; } catch { Write-Host ('Warning: cannot read the account: '+$_.Exception.Message); exit 0 }; $c=Get-Content -Raw $tmp; $c=$c -replace '(?m)^(SeShutdownPrivilege\s*=\s*.*),\s*\*'+[regex]::Escape($sid.Value)+'(,|$)','$1$2' -replace '(?m)^(SeShutdownPrivilege\s*=\s*)\*'+[regex]::Escape($sid.Value)+'\s*$','$1'; Set-Content -Path $tmp -Value $c -Encoding Unicode; secedit /configure /db $env:TEMP\vlt-old-right.sdb /cfg $tmp /quiet; Write-Host 'User right removed.'; Remove-Item $tmp,$env:TEMP\vlt-old-right.sdb,$env:TEMP\vlt-old-right.jfm -ErrorAction SilentlyContinue"
    echo Remove the service vltd
    sc delete vltd
) else (
    echo The service vltd is not present.
)

powershell -NoProfile -ExecutionPolicy Bypass -Command "try { $g='VLT Operators'; $m=@(Get-LocalGroupMember -Group $g -ErrorAction Stop); if($m.Count -gt 0){ Write-Host ('Group has '+$m.Count+' members. It is not deleted.'); exit 0 }; Remove-LocalGroup -Name $g -ErrorAction Stop; Write-Host 'Group deleted (no members).' } catch [Microsoft.PowerShell.Commands.GroupNotFoundException] { Write-Host 'Group not found.' } catch { if($_.Exception.Message -match 'not found|cannot find'){ Write-Host 'Group not found.' } else { Write-Host ('Group: '+$_.Exception.Message) } }"

if exist "%LNK%" (
    del /Q "%LNK%" || goto :fail
    echo Removed the old Start menu shortcut.
)
if exist "%DEST%" (
    rmdir /S /Q "%DEST%"
    if exist "%DEST%" (
        echo ERROR: cannot remove "%DEST%". Close the old programs, then try again.
        goto :fail
    )
    echo Removed "%DEST%".
) else (
    echo "%DEST%" is not present.
)

if exist "%DATA%" (
    echo.
    echo The old data folder is "%DATA%".
    echo If you delete it, the stored Settings and Secrets of the old test are lost.
    set "ANS2="
    set /p ANS2=Delete C:\ProgramData\VoltioVolt? (Y/N) 
    if /I "!ANS2!"=="Y" (
        rmdir /S /Q "%DATA%"
        if exist "%DATA%" (
            echo ERROR: cannot remove "%DATA%".
            goto :fail
        )
        echo Data folder deleted.
    ) else (
        echo Data folder kept.
    )
)

echo.
echo Done.
pause
exit /b 0

:fail
echo.
echo ERROR: the removal did not complete. Read the messages above.
pause
exit /b 1
