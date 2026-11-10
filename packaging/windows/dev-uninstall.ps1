# SPDX-License-Identifier: AGPL-3.0-or-later
#
# dev-uninstall.ps1: removes the development install of vltd (VLT-IS-003
# TASK 03-00-00-008). Run as Administrator. Stops and removes the service.
# Asks before it deletes the data directory (the data directory has the
# Store, the encrypted DEK and the logs).

#Requires -RunAsAdministrator
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'

$name = 'vltd'
$installDir = Join-Path $env:ProgramFiles 'VoltioVolt'
$dataDir = Join-Path $env:ProgramData 'VoltioVolt'

$svc = Get-Service -Name $name -ErrorAction SilentlyContinue
if ($svc) {
    if ($svc.Status -ne 'Stopped') {
        Stop-Service -Name $name
        $svc.WaitForStatus('Stopped', (New-TimeSpan -Seconds 30))
    }
    & sc.exe delete $name | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "sc.exe delete failed: $LASTEXITCODE" }
    Write-Host "Service $name removed."
} else {
    Write-Host "Service $name not found."
}

$exe = Join-Path $installDir 'vltd.exe'
if (Test-Path -LiteralPath $exe) { Remove-Item -LiteralPath $exe -Force }
if ((Test-Path -LiteralPath $installDir) -and -not (Get-ChildItem -LiteralPath $installDir)) {
    Remove-Item -LiteralPath $installDir -Force
}

if (Test-Path -LiteralPath $dataDir) {
    Write-Host "The data directory $dataDir contains the Store, the encrypted DEK and the logs."
    Write-Host 'If you delete it, all stored Secrets and Settings are lost.'
    $answer = Read-Host 'Type DELETE to delete the data directory, or press Enter to keep it'
    if ($answer -ceq 'DELETE') {
        Remove-Item -LiteralPath $dataDir -Recurse -Force
        Write-Host 'Data directory deleted.'
    } else {
        Write-Host 'Data directory kept.'
    }
}
