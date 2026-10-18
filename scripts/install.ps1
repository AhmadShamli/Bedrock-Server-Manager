<#
.SYNOPSIS
    Bedrock Server Manager (BSM) - Windows Automated Installer & Setup Script

.DESCRIPTION
    Automates the installation, service registration, firewall configuration,
    and host optimizations for Bedrock Server Manager on Windows.

.PARAMETER InstallDir
    Target directory for the BSM executable and binaries.
    Default: "$env:ProgramFiles\BedrockServerManager"

.PARAMETER DataDir
    Target directory for BSM databases, backups, and native BDS server files.
    Default: "$env:ProgramData\BedrockServerManager"

.PARAMETER Port
    Port for the BSM Web Management UI.
    Default: 8080

.PARAMETER StartupType
    Service startup type ('Delayed' or 'Auto').
    Default: 'Delayed' (Delayed Automatic)

.PARAMETER SkipService
    Do not register or start the Windows background service (portable mode).

.PARAMETER SkipFirewall
    Skip adding Windows Defender Firewall rules for the Web UI.

.PARAMETER SkipLoopback
    Skip granting UWP loopback exemption for local Minecraft client.

.PARAMETER SkipDefender
    Skip adding Windows Defender real-time scanning exclusion for the data directory.

.PARAMETER SkipVCRuntime
    Skip checking and installing the Microsoft Visual C++ 2015-2022 x64 Redistributable.

.PARAMETER Uninstall
    Stops and removes the BSM Windows service and firewall rule.

.PARAMETER PurgeData
    Only valid with -Uninstall: completely removes the data directory including all worlds and databases.
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:ProgramFiles\BedrockServerManager",
    [string]$DataDir = "$env:ProgramData\BedrockServerManager",
    [int]$Port = 8080,
    [ValidateSet('Delayed', 'Auto')]
    [string]$StartupType = 'Delayed',
    [switch]$SkipService,
    [switch]$SkipFirewall,
    [switch]$SkipLoopback,
    [switch]$SkipDefender,
    [switch]$SkipVCRuntime,
    [switch]$Uninstall,
    [switch]$PurgeData
)

$ErrorActionPreference = 'Stop'

# 1. Require Administrator elevation
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Warning "Bedrock Server Manager setup requires Administrator privileges."
    Write-Host "Re-launching PowerShell as Administrator..." -ForegroundColor Yellow
    $argsList = "-NoProfile -ExecutionPolicy Bypass -File `"$($MyInvocation.MyCommand.Path)`""
    if ($PSBoundParameters.Count -gt 0) {
        foreach ($param in $PSBoundParameters.GetEnumerator()) {
            if ($param.Value -is [switch] -and $param.Value) {
                $argsList += " -$($param.Key)"
            } elseif ($param.Value -isnot [switch]) {
                $argsList += " -$($param.Key) `"$($param.Value)`""
            }
        }
    }
    Start-Process powershell -Verb RunAs -ArgumentList $argsList
    exit 0
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceExe = $null

# Detect executable in current directory, parent directory, or build folder
$candidates = @(
    Join-Path $ScriptDir "bedrock-server-manager.exe",
    Join-Path $ScriptDir "bsm.exe",
    Join-Path (Split-Path -Parent $ScriptDir) "bedrock-server-manager.exe",
    Join-Path (Split-Path -Parent $ScriptDir) "bsm.exe",
    Join-Path $InstallDir "bedrock-server-manager.exe"
)

foreach ($c in $candidates) {
    if (Test-Path $c) {
        $SourceExe = $c
        break
    }
}

# --- UNINSTALL MODE ---
if ($Uninstall) {
    Write-Host "==========================================================" -ForegroundColor Cyan
    Write-Host "   Bedrock Server Manager - Windows Service Uninstallation" -ForegroundColor Cyan
    Write-Host "==========================================================" -ForegroundColor Cyan

    $InstalledExe = Join-Path $InstallDir "bedrock-server-manager.exe"
    if (-not (Test-Path $InstalledExe) -and $SourceExe) {
        $InstalledExe = $SourceExe
    }

    if (Test-Path $InstalledExe) {
        Write-Host "==> Stopping BSM service..." -ForegroundColor Yellow
        try {
            & "$InstalledExe" service stop | Out-Null
        } catch {
            # Service might already be stopped
        }

        Write-Host "==> Removing BSM service..." -ForegroundColor Yellow
        try {
            & "$InstalledExe" service uninstall | Out-Null
        } catch {
            Write-Warning "Could not uninstall service via binary: $_"
        }
    }

    # Clean up firewall rule
    Write-Host "==> Removing firewall rules..." -ForegroundColor Yellow
    try {
        netsh advfirewall firewall delete rule name="Bedrock Server Manager Web UI" | Out-Null
    } catch {
        # Rule might not exist
    }

    if ($PurgeData) {
        Write-Host "==> PURGING all data and server instances at $DataDir..." -ForegroundColor Red
        if (Test-Path $DataDir) {
            Remove-Item -Path $DataDir -Recurse -Force
        }
        Write-Host "Data directory purged." -ForegroundColor Green
    } else {
        Write-Host ""
        Write-Host "NOTICE: Data directory preserved at: $DataDir" -ForegroundColor Cyan
        Write-Host "Your server worlds, configs, and databases were NOT deleted." -ForegroundColor Cyan
        Write-Host "To permanently remove all data, run: .\install.ps1 -Uninstall -PurgeData" -ForegroundColor DarkGray
    }

    Write-Host ""
    Write-Host "[SUCCESS] Bedrock Server Manager uninstallation complete." -ForegroundColor Green
    exit 0
}

# --- INSTALL MODE ---
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "     Bedrock Server Manager - Windows Host Setup" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "Install Directory: $InstallDir"
Write-Host "Data Directory:    $DataDir"
Write-Host "Web UI Port:       $Port"
Write-Host "Startup Type:      $StartupType"
Write-Host ""

# Ensure directories exist
if (-not (Test-Path $InstallDir)) {
    Write-Host "==> Creating install directory: $InstallDir..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

if (-not (Test-Path $DataDir)) {
    Write-Host "==> Creating data directory: $DataDir..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
}

# Copy binary to InstallDir
$TargetExe = Join-Path $InstallDir "bedrock-server-manager.exe"
if ($SourceExe -and (Resolve-Path $SourceExe).Path -ne (Resolve-Path $TargetExe -ErrorAction SilentlyContinue).Path) {
    Write-Host "==> Copying executable to $TargetExe..." -ForegroundColor Yellow
    Copy-Item -Path $SourceExe -Destination $TargetExe -Force
} elseif (-not (Test-Path $TargetExe)) {
    Write-Error "Could not find bedrock-server-manager.exe in installation package or $InstallDir."
    exit 1
}

# 2. Check and install Visual C++ 2015-2022 Redistributable (x64)
if (-not $SkipVCRuntime) {
    Write-Host "==> Checking Microsoft Visual C++ 2015-2022 Redistributable (x64)..." -ForegroundColor Yellow
    $vcInstalled = $false
    $vcPaths = @(
        "HKLM:\SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\x64",
        "HKLM:\SOFTWARE\WOW6432Node\Microsoft\VisualStudio\14.0\VC\Runtimes\x64"
    )
    foreach ($p in $vcPaths) {
        if (Test-Path $p) {
            $val = Get-ItemProperty -Path $p -Name "Installed" -ErrorAction SilentlyContinue
            if ($val -and $val.Installed -eq 1) {
                $vcInstalled = $true
                break
            }
        }
    }

    if ($vcInstalled) {
        Write-Host "    Visual C++ 2015-2022 Redistributable is installed." -ForegroundColor Green
    } else {
        Write-Host "    VC++ runtime not detected. Official BDS bedrock_server.exe requires it." -ForegroundColor Yellow
        Write-Host "    Downloading vc_redist.x64.exe from Microsoft..." -ForegroundColor Yellow
        $installerPath = Join-Path $env:TEMP "vc_redist.x64.exe"
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -Uri "https://aka.ms/vs/17/release/vc_redist.x64.exe" -OutFile $installerPath -UseBasicParsing
            Write-Host "    Installing Visual C++ Redistributable silently..." -ForegroundColor Yellow
            $proc = Start-Process -FilePath $installerPath -ArgumentList "/install", "/quiet", "/norestart" -Wait -PassThru
            if ($proc.ExitCode -eq 0 -or $proc.ExitCode -eq 3010) {
                Write-Host "    Visual C++ Redistributable installed successfully." -ForegroundColor Green
            } else {
                Write-Warning "VC++ installer exited with code $($proc.ExitCode). You may need to manually install from https://aka.ms/vs/17/release/vc_redist.x64.exe"
            }
        } catch {
            Write-Warning "Could not automatically download/install VC++ Redistributable: $_"
        }
    }
}

# 3. Configure Windows Defender Firewall for Web UI
if (-not $SkipFirewall) {
    Write-Host "==> Configuring Windows Defender Firewall for Web UI (TCP $Port)..." -ForegroundColor Yellow
    try {
        # Remove old rule if present to avoid duplicates
        netsh advfirewall firewall delete rule name="Bedrock Server Manager Web UI" | Out-Null
    } catch {}
    
    $fwResult = netsh advfirewall firewall add rule name="Bedrock Server Manager Web UI" dir=in action=allow protocol=TCP localport=$Port
    Write-Host "    Firewall rule added: Bedrock Server Manager Web UI (TCP $Port)." -ForegroundColor Green
}

# 4. Configure UWP Loopback Exemption
if (-not $SkipLoopback) {
    Write-Host "==> Configuring UWP Loopback Exemption for local Minecraft Bedrock client..." -ForegroundColor Yellow
    try {
        $loopResult = CheckNetIsolation LoopbackExempt -a -n=Microsoft.MinecraftUWP_8wekyb3d8bbwe
        Write-Host "    $loopResult" -ForegroundColor Green
    } catch {
        Write-Warning "Could not set loopback exemption: $_"
    }
}

# 5. Add Windows Defender Real-time Exclusion for Data Directory
if (-not $SkipDefender) {
    Write-Host "==> Adding Windows Defender exclusion for data directory ($DataDir)..." -ForegroundColor Yellow
    try {
        Add-MpPreference -ExclusionPath $DataDir -ErrorAction SilentlyContinue
        Write-Host "    Windows Defender exclusion registered for $DataDir." -ForegroundColor Green
    } catch {
        Write-Warning "Could not register Windows Defender exclusion (requires Defender PowerShell module): $_"
    }
}

# 6. Service Installation & Startup
if (-not $SkipService) {
    Write-Host "==> Installing Bedrock Server Manager as a Windows Service..." -ForegroundColor Yellow
    
    # Check if already installed
    $existing = Get-Service -Name "BedrockServerManager" -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Host "    Service 'BedrockServerManager' already registered. Stopping for update..." -ForegroundColor Yellow
        try {
            & "$TargetExe" service stop | Out-Null
        } catch {}
        try {
            & "$TargetExe" service uninstall | Out-Null
        } catch {}
    }

    $svcInstall = & "$TargetExe" service install --data-dir "$DataDir" --port $Port
    Write-Host "    $svcInstall" -ForegroundColor Green

    # Configure delayed start if requested
    if ($StartupType -eq 'Delayed') {
        Write-Host "    Configuring delayed automatic startup..." -ForegroundColor Yellow
        sc.exe config BedrockServerManager start= delayed-auto | Out-Null
    }

    Write-Host "==> Starting Bedrock Server Manager service..." -ForegroundColor Yellow
    $svcStart = & "$TargetExe" service start
    Write-Host "    $svcStart" -ForegroundColor Green

    Start-Sleep -Seconds 2
    $svcCheck = Get-Service -Name "BedrockServerManager" -ErrorAction SilentlyContinue
    if ($svcCheck -and $svcCheck.Status -eq 'Running') {
        Write-Host "    Service status: RUNNING" -ForegroundColor Green
    } else {
        Write-Warning "Service status is: $($svcCheck.Status). Check Windows Event Viewer Application logs if issues arise."
    }
} else {
    Write-Host "==> Skipping Windows Service installation (-SkipService specified)." -ForegroundColor Cyan
    Write-Host "    To run manually: & '$TargetExe' --data-dir '$DataDir' --port $Port" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "==========================================================" -ForegroundColor Green
Write-Host "    Bedrock Server Manager Setup Successfully Finished!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Web Dashboard:     http://localhost:$Port" -ForegroundColor Cyan
Write-Host "  Default Admin:     admin / admin (please change on first login)" -ForegroundColor Yellow
Write-Host "  Data Directory:    $DataDir"
Write-Host "  Binary Location:   $TargetExe"
if (-not $SkipService) {
    Write-Host "  Windows Service:   BedrockServerManager (Automatic)"
    Write-Host ""
    Write-Host "  Useful Service Commands:" -ForegroundColor DarkGray
    Write-Host "    & '$TargetExe' service status" -ForegroundColor DarkGray
    Write-Host "    & '$TargetExe' service stop" -ForegroundColor DarkGray
    Write-Host "    & '$TargetExe' service start" -ForegroundColor DarkGray
}
Write-Host ""
Write-Host "Enjoy running your high-performance Minecraft Bedrock servers!" -ForegroundColor Green
Write-Host "==========================================================" -ForegroundColor Green
