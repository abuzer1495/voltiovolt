# SPDX-License-Identifier: AGPL-3.0-or-later
#
# dev-install.ps1: development install of vltd on Windows (VLT-IS-004 TASK
# 03-00-00-002 step 4). This is not a package.
#
# The script only checks for Administrator rights and then starts
# vlt-setup.exe dev-install. vlt-setup.exe and vltd.exe must be in the same
# folder as this script. vlt-setup does all the steps (group, service, user
# right, required privileges, data directory ACL, recovery, start).

[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run this script in a PowerShell window that you opened as Administrator.'
}

$setup = Join-Path $PSScriptRoot 'vlt-setup.exe'
& $setup dev-install
exit $LASTEXITCODE
