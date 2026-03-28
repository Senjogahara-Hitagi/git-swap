# install-completion.ps1
# Robust installation script for git-swap autocomplete and aliases

$ErrorActionPreference = "Stop"

try {
    # 1. Ensure UTF-8 for emojis in terminal
    if ($PSVersionTable.PSVersion.Major -ge 5) {
        [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    }

    # 1. Check Execution Policy
    $policy = Get-ExecutionPolicy
    if ($policy -eq "Restricted" -or $policy -eq "AllSigned") {
        Write-Host "`n⚠️  Warning: Your PowerShell execution policy is '$policy'." -ForegroundColor Yellow
        Write-Host "This may prevent your profile from loading correctly."
        Write-Host "To fix this, run PowerShell as Administrator and execute:"
        Write-Host "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser" -ForegroundColor Cyan
    }

    # 2. Check if git-swap is in PATH
    $gitSwapInPath = Get-Command git-swap -ErrorAction SilentlyContinue
    if (!$gitSwapInPath) {
        $currentBin = Join-Path $PSScriptRoot "bin"
        if (Test-Path (Join-Path $currentBin "git-swap.exe")) {
            Write-Host "`n⚠️  Warning: 'git-swap' not found in your PATH." -ForegroundColor Yellow
            Write-Host "We detected it in: $currentBin"
            Write-Host "To make it permanent, add this directory to your User PATH environment variable."
        } else {
            Write-Host "`n⚠️  Warning: 'git-swap' command not found in PATH." -ForegroundColor Yellow
            Write-Host "Please ensure you have built the project: go build -o bin/git-swap.exe main.go"
        }
    }

    # 3. Determine Profile Paths
    # We target both Windows PowerShell and PowerShell Core profiles if they exist
    $profilesToUpdate = @($PROFILE)
    $psCoreProfile = Join-Path $HOME "Documents\PowerShell\Microsoft.PowerShell_profile.ps1"
    $winPSProfile = Join-Path $HOME "Documents\WindowsPowerShell\Microsoft.PowerShell_profile.ps1"

    # Add them if they exist and are different from current $PROFILE
    if ((Test-Path $psCoreProfile) -and ($psCoreProfile -ne $PROFILE)) { $profilesToUpdate += $psCoreProfile }
    if ((Test-Path $winPSProfile) -and ($winPSProfile -ne $PROFILE)) { $profilesToUpdate += $winPSProfile }

    $profilesToUpdate = $profilesToUpdate | Select-Object -Unique

    # 4. Prepare completion script
    $srcScript = Join-Path $PSScriptRoot ".git-swap-completion.ps1"
    $destScript = Join-Path $HOME ".git-swap-completion.ps1"

    if (Test-Path $srcScript) {
        Copy-Item -Path $srcScript -Destination $destScript -Force
        Write-Host "Copied completion script from $srcScript to $destScript"
    } else {
        Write-Host "`n⚠️ Warning: Source completion script not found at $srcScript." -ForegroundColor Yellow
        # Fallback only if the source file is somehow missing
        $completionCode = @"
# Enhanced PowerShell completion script for git-swap
`$GitSwapCompleter = {
    param(`$wordToComplete, `$commandAst, `$cursorPosition)
    `$commandElements = `$commandAst.CommandElements
    `$nElements = `$commandElements.Count
    
    if (`$nElements -le 2) {
        `$commands = @('list', 'status', 'current', 'add', 'edit', 'remove', 'rm', 'auto', 'setup-hook', 'remove-hook', 'convert-ssh', 'help')
        `$profiles = & git-swap _complete 2>`$null
        `$indices = @()
        if (`$profiles) { for (`$i = 1; `$i -le `$profiles.Count; `$i++) { `$indices += [string]`$i } }
        `$allChoices = `$commands + `$profiles + `$indices
        return `$allChoices | Where-Object { `$_.StartsWith(`$wordToComplete) } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new(`$_, `$_, 'ParameterValue', `$_)
        }
    }
    if (`$nElements -eq 3) {
        `$firstArg = `$commandElements[1].GetText()
        if (`$firstArg -eq 'edit' -or `$firstArg -eq 'remove' -or `$firstArg -eq 'rm') {
            `$profiles = & git-swap _complete 2>`$null
            `$indices = @()
            if (`$profiles) { for (`$i = 1; `$i -le `$profiles.Count; `$i++) { `$indices += [string]`$i } }
            `$choices = `$profiles + `$indices
            return `$choices | Where-Object { `$_.StartsWith(`$wordToComplete) } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new(`$_, `$_, 'ParameterValue', `$_)
            }
        }
    }
    return `$null
}

# Register completions for both git-swap and the gsw alias
Register-ArgumentCompleter -Native -CommandName 'git-swap' -ScriptBlock `$GitSwapCompleter
Register-ArgumentCompleter -Native -CommandName 'gsw' -ScriptBlock `$GitSwapCompleter
"@
        Set-Content -Path $destScript -Value $completionCode
        Write-Host "Created completion script fallback at $destScript"
    }

    # 5. Update profiles
    $aliasLine = "Set-Alias -Name gsw -Value git-swap"
    $sourceLine = ". '$destScript'"

    foreach ($prof in $profilesToUpdate) {
        if (!(Test-Path $prof)) {
            $parent = Split-Path $prof -Parent
            if (!(Test-Path $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
            New-Item -ItemType File -Path $prof -Force | Out-Null
            Write-Host "Created new profile at $prof"
        }

        $content = Get-Content $prof -Raw -ErrorAction SilentlyContinue
        if ($null -eq $content) { $content = "" }

        $modified = $false
        if ($content -notmatch [regex]::Escape($aliasLine)) {
            Add-Content -Path $prof -Value "`n$aliasLine"
            $modified = $true
        }
        if ($content -notmatch [regex]::Escape($sourceLine)) {
            Add-Content -Path $prof -Value "`n$sourceLine"
            $modified = $true
        }

        if ($modified) {
            Write-Host "Updated profile: $prof" -ForegroundColor Green
        }
    }

    Write-Host "`n✅ Installation complete!" -ForegroundColor Green
    Write-Host "To apply changes in this session, run:" -ForegroundColor Gray
    Write-Host "    . `$PROFILE" -ForegroundColor Yellow
    Write-Host "Or restart your terminal." -ForegroundColor Gray
} catch {
    Write-Host "`n❌ Installation failed: $($_.Exception.Message)" -ForegroundColor Red
} finally {
    Write-Host "`nPress any key to exit..." -ForegroundColor Gray
    $null = [Console]::ReadKey($true)
}
