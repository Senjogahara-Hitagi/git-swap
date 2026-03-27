# Enhanced PowerShell completion script for git-swap
$GitSwapCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)
    $commandElements = $commandAst.CommandElements
    $nElements = $commandElements.Count
    if ($nElements -le 2) {
        $commands = @('list', 'status', 'add', 'edit', 'remove', 'rm', 'auto', 'setup-hook', 'help')
        $profiles = & git-swap _complete 2>$null
        $indices = @()
        if ($profiles) { for ($i = 1; $i -le $profiles.Count; $i++) { $indices += [string]$i } }
        $allChoices = $commands + $profiles + $indices
        return $allChoices | Where-Object { $_.StartsWith($wordToComplete) } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }
    if ($nElements -eq 3) {
        $firstArg = $commandElements[1].GetText()
        if ($firstArg -eq 'edit' -or $firstArg -eq 'remove' -or $firstArg -eq 'rm') {
            $profiles = & git-swap _complete 2>$null
            $indices = @()
            if ($profiles) { for ($i = 1; $i -le $profiles.Count; $i++) { $indices += [string]$i } }
            $choices = $profiles + $indices
            return $choices | Where-Object { $_.StartsWith($wordToComplete) } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
    return $null
}
Register-ArgumentCompleter -Native -CommandName 'git-swap' -ScriptBlock $GitSwapCompleter
