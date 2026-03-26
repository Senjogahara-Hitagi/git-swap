# PowerShell completion script for git-swap

$GitSwapCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commandElements = $commandAst.CommandElements
    $commandName = $commandElements[0].GetText()
    
    # Check if we are completing the first argument (the command or profile name)
    if ($commandElements.Count -eq 2 -and $cursorPosition -gt $commandElements[0].Extent.EndOffset) {
        # Commands to suggest
        $commands = @('list', 'status', 'add', 'edit', 'remove', 'rm', 'auto', 'help')
        
        # Profile names from git-swap _complete
        $profiles = & git-swap _complete 2>$null
        
        $allSuggestions = $commands + $profiles
        return $allSuggestions | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }

    # Check if we are completing the second argument for commands like edit/remove
    if ($commandElements.Count -eq 3 -and $cursorPosition -gt $commandElements[1].Extent.EndOffset) {
        $prevCommand = $commandElements[1].GetText()
        if ($prevCommand -eq 'edit' -or $prevCommand -eq 'remove' -or $prevCommand -eq 'rm') {
            $profiles = & git-swap _complete 2>$null
            return $profiles | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
}

Register-ArgumentCompleter -Native -CommandName 'git-swap' -ScriptBlock $GitSwapCompleter
