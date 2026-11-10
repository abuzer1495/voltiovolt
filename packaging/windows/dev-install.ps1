# SPDX-License-Identifier: AGPL-3.0-or-later
#
# dev-install.ps1: development install of vltd on Windows (VLT-IS-003 TASK
# 03-00-00-008). Run as Administrator. This is not a package.
#
# 1. Copies vltd.exe to %ProgramFiles%\VoltioVolt.
# 2. Makes the service vltd with the virtual account NT SERVICE\vltd.
# 3. Makes %ProgramData%\VoltioVolt with the ACL of TASK 03-00-00-003:
#    SYSTEM full control, Administrators full control, NT SERVICE\vltd
#    modify, no other entries, no inheritance from the parent.
# 4. Starts the service.

#Requires -RunAsAdministrator
[CmdletBinding()]
param(
    [string]$Binary = (Join-Path $PSScriptRoot 'vltd.exe')
)
$ErrorActionPreference = 'Stop'

$name = 'vltd'
$installDir = Join-Path $env:ProgramFiles 'VoltioVolt'
$dataDir = Join-Path $env:ProgramData 'VoltioVolt'

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw "vltd.exe not found: $Binary. Build it with: `$env:CGO_ENABLED='0'; go build -o vltd.exe ./cmd/vltd"
}
if (Get-Service -Name $name -ErrorAction SilentlyContinue) {
    throw "The service $name exists. Run dev-uninstall.ps1 first."
}

New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item -LiteralPath $Binary -Destination (Join-Path $installDir 'vltd.exe') -Force
$exe = Join-Path $installDir 'vltd.exe'

New-Service -Name $name -BinaryPathName "`"$exe`"" -DisplayName 'VoltioVolt service (development)' `
    -Description 'VoltioVolt UPS service, development install' -StartupType Manual | Out-Null
# Virtual account: no password.
& sc.exe config $name obj= "NT SERVICE\$name" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "sc.exe config failed: $LASTEXITCODE" }
& sc.exe sidtype $name unrestricted | Out-Null
if ($LASTEXITCODE -ne 0) { throw "sc.exe sidtype failed: $LASTEXITCODE" }

New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
# Remove the inherited entries, then set only the three entries.
& icacls.exe $dataDir /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "NT SERVICE\${name}:(OI)(CI)M" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "icacls failed: $LASTEXITCODE" }
# Remove each other explicit entry.
$acl = Get-Acl -LiteralPath $dataDir
$keep = @('S-1-5-18', 'S-1-5-32-544')
$svcSid = (New-Object System.Security.Principal.NTAccount("NT SERVICE\$name")).Translate([System.Security.Principal.SecurityIdentifier]).Value
$keep += $svcSid
foreach ($rule in @($acl.Access)) {
    $sid = $rule.IdentityReference.Translate([System.Security.Principal.SecurityIdentifier]).Value
    if ($keep -notcontains $sid) { [void]$acl.RemoveAccessRule($rule) }
}
Set-Acl -LiteralPath $dataDir -AclObject $acl
& icacls.exe $dataDir

Start-Service -Name $name
Get-Service -Name $name | Format-Table -AutoSize Name, Status, StartType
Write-Host "Log: $dataDir\logs\vltd.log"
