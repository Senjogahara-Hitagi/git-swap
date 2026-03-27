# install-completion.ps1

Write-Host "Installing git-swap autocomplete and aliases..." -ForegroundColor Cyan

# 1. Ensure Profile exists
if (!(Test-Path $PROFILE)) {
    New-Item -Type File -Path $PROFILE -Force | Out-Null
    Write-Host "Created new PowerShell profile at $PROFILE"
}

# 2. Write the completion script to the user's home directory
$destScript = Join-Path $HOME ".git-swap-completion.ps1"
$completionCode = @"
# Enhanced PowerShell completion script for git-swap
`$GitSwapCompleter = {
    param(`$wordToComplete, `$commandAst, `$cursorPosition)
    `$commandElements = `$commandAst.CommandElements
    `$nElements = `$commandElements.Count
    
    if (`$nElements -le 2) {
        # Added convert-ssh to the list of commands
        `$commands = @('list', 'status', 'add', 'edit', 'remove', 'rm', 'auto', 'setup-hook', 'convert-ssh', 'help')
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
Write-Host "Created completion script at $destScript"

# 3. Add source and alias to profile
$profileContent = ""
if (Test-Path $PROFILE) {
    $profileContent = Get-Content $PROFILE -Raw
}

$aliasLine = "Set-Alias -Name gsw -Value git-swap"
$sourceLine = ". '$destScript'"

$modified = $false

if ($profileContent -notmatch [regex]::Escape($aliasLine)) {
    Add-Content -Path $PROFILE -Value "`n$aliasLine"
    Write-Host "Added 'gsw' alias to profile."
    $modified = $true
}

if ($profileContent -notmatch [regex]::Escape($sourceLine)) {
    Add-Content -Path $PROFILE -Value "`n$sourceLine"
    Write-Host "Added completion script source to profile."
    $modified = $true
}

if ($modified) {
    Write-Host "`n✅ Installation complete! Please apply changes by running:" -ForegroundColor Green
    Write-Host "    . `$PROFILE" -ForegroundColor Yellow
} else {
    Write-Host "`n✅ Already installed. No changes made to profile." -ForegroundColor Green
}
