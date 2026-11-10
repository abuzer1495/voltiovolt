# SPDX-License-Identifier: AGPL-3.0-or-later
#
# dev-uninstall.ps1: removes the development install of vltd (VLT-IS-004
# TASK 03-00-00-002 step 4).
#
# The script only checks for Administrator rights and then starts
# vlt-setup.exe dev-uninstall. vlt-setup.exe must be in the same folder as
# this script. vlt-setup asks before it deletes the data directory.

[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run this script in a PowerShell window that you opened as Administrator.'
}

$setup = Join-Path $PSScriptRoot 'vlt-setup.exe'
& $setup dev-uninstall
exit $LASTEXITCODE
