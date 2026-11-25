# SPDX-License-Identifier: AGPL-3.0-or-later
# Capture VoltioVolt demo screens on a Windows runner with
# System.Drawing.Graphics.CopyFromScreen (in-box). No extra package.
param(
    [Parameter(Mandatory = $true)][string]$Bin,
    [Parameter(Mandatory = $true)][string]$Out,
    [string]$Screens = "",
    [int]$Wait = 60
)

$ErrorActionPreference = "Stop"
if (-not $Screens) {
    $Screens = Join-Path $PSScriptRoot "screenshot-screens.txt"
}
New-Item -ItemType Directory -Force -Path $Out | Out-Null

Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class VltWin {
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
  [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
  public struct RECT { public int Left, Top, Right, Bottom; }
}
"@
Add-Type -AssemblyName System.Drawing

function Wait-Ready([string]$Path, [int]$Seconds) {
    for ($i = 0; $i -lt $Seconds; $i++) {
        if (Test-Path $Path) { return }
        Start-Sleep -Seconds 1
    }
    throw "timeout waiting for $Path"
}

function Find-VltWindow {
    $candidates = Get-Process | Where-Object { $_.MainWindowTitle -like "*VoltioVolt*" -and $_.MainWindowHandle -ne [IntPtr]::Zero }
    if ($candidates) { return $candidates[0].MainWindowHandle }
    return [IntPtr]::Zero
}

function Capture-Screen([string]$Lang, [string]$Screen) {
    $name = "{0}-{1}" -f $Lang, ($Screen -replace '[/ ]', '-')
    $data = Join-Path $env:TEMP ("vlt-shot-" + [guid]::NewGuid().ToString("n"))
    New-Item -ItemType Directory -Force -Path $data | Out-Null
    $ready = Join-Path $data "ready"
    Write-Host "capture $name"
    $p = Start-Process -FilePath $Bin -ArgumentList @("--data-dir", $data, "--demo", "--lang", $Lang, "--demo-screen", $Screen, "--demo-ready-file", $ready) -PassThru -RedirectStandardOutput (Join-Path $data "out.log") -RedirectStandardError (Join-Path $data "err.log")
    try {
        Wait-Ready $ready $Wait
        Start-Sleep -Seconds 2
        $hwnd = [IntPtr]::Zero
        for ($i = 0; $i -lt 20; $i++) {
            $hwnd = Find-VltWindow
            if ($hwnd -ne [IntPtr]::Zero) { break }
            Start-Sleep -Milliseconds 500
        }
        if ($hwnd -eq [IntPtr]::Zero) { throw "no window for $name" }
        [void][VltWin]::ShowWindow($hwnd, 9)
        [void][VltWin]::SetForegroundWindow($hwnd)
        Start-Sleep -Milliseconds 400
        $rect = New-Object VltWin+RECT
        if (-not [VltWin]::GetWindowRect($hwnd, [ref]$rect)) { throw "GetWindowRect failed" }
        $w = [Math]::Max(1, $rect.Right - $rect.Left)
        $h = [Math]::Max(1, $rect.Bottom - $rect.Top)
        $bmp = New-Object System.Drawing.Bitmap $w, $h
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $g.CopyFromScreen($rect.Left, $rect.Top, 0, 0, $bmp.Size)
        $g.Dispose()
        $dest = Join-Path $Out ($name + ".png")
        $bmp.Save($dest, [System.Drawing.Imaging.ImageFormat]::Png)
        $bmp.Dispose()
    }
    finally {
        if ($p -and -not $p.HasExited) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
        Start-Sleep -Milliseconds 200
        if (Test-Path $data) { Remove-Item -Recurse -Force $data -ErrorAction SilentlyContinue }
    }
}

Get-Content $Screens | ForEach-Object {
    $screen = $_.Trim()
    if (-not $screen -or $screen.StartsWith("#")) { return }
    foreach ($lang in @("en", "tr")) {
        Capture-Screen $lang $screen
    }
}
Get-ChildItem $Out | Format-Table Name, Length
