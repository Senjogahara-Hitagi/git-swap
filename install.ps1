$ErrorActionPreference = 'Stop'

try {
    $Repo = "abdozkaya/git-swap"
    $FileName = "git-swap-windows-amd64.exe"
    $Url = "https://github.com/$Repo/releases/latest/download/$FileName"
    $InstallDir = "$env:LOCALAPPDATA\Programs\git-swap"

    # Ensure UTF-8 for emojis in terminal
    if ($PSVersionTable.PSVersion.Major -ge 5) {
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    }

    Write-Host "⬇️  Downloading git-swap..." -ForegroundColor Cyan

    # Create Directory
    if (!(Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    # Download
    Invoke-WebRequest -Uri $Url -OutFile "$InstallDir\git-swap.exe"

    # Add to PATH if not exists
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Host "⚙️  Adding to PATH..." -ForegroundColor Yellow
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        $env:Path += ";$InstallDir"
        Write-Host "Path updated. You might need to restart your terminal." -ForegroundColor Yellow
    }

    Write-Host "✅ Installed successfully!" -ForegroundColor Green
    Write-Host "Run 'git-swap help' to get started."

    Write-Host "`n✨ Fork Features:" -ForegroundColor Cyan
    Write-Host " - 'git-swap auto': Improved profile detection (remote-priority)"
    Write-Host " - 'git-swap setup-hook': Auto-switch profiles via pre-commit hook"
    Write-Host " - 'git-swap convert-ssh': Easily migrate remotes from HTTPS to SSH"
    Write-Host " - 'git-swap current': Useful alias for checking current status"
} catch {
    Write-Host "`n❌ Installation failed: $($_.Exception.Message)" -ForegroundColor Red
} finally {
    Write-Host "`nPress any key to exit..." -ForegroundColor Gray
    $null = [Console]::ReadKey($true)
}
