# Enhanced PowerShell completion script for git-swap

$GitSwapCompleter = {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commandElements = $commandAst.CommandElements
    $nElements = $commandElements.Count
    
    # Identify the position we are completing
    # If we just typed 'git-swap ' (with a space), $nElements might be 1 or 2 depending on trailing space
    
    # 1. Completing the FIRST argument (Command or Profile)
    if ($nElements -le 2) {
        $commands = @('list', 'status', 'add', 'edit', 'remove', 'rm', 'auto', 'help')
        $profiles = & git-swap _complete 2>$null
        
        # Also suggest numbers based on profile count
        $indices = @()
        if ($profiles) {
            for ($i = 1; $i -le $profiles.Count; $i++) { $indices += [string]$i }
        }

        $allChoices = $commands + $profiles + $indices
        
        return $allChoices | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    }

    # 2. Completing the SECOND argument (for edit/remove/rm)
    if ($nElements -eq 3) {
        $firstArg = $commandElements[1].GetText()
        if ($firstArg -eq 'edit' -or $firstArg -eq 'remove' -or $firstArg -eq 'rm') {
            $profiles = & git-swap _complete 2>$null
            $indices = @()
            if ($profiles) {
                for ($i = 1; $i -le $profiles.Count; $i++) { $indices += [string]$i }
            }
            
            $choices = $profiles + $indices
            return $choices | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
    
    return $null
}

# Register the completer
# We use -Native to intercept before standard file completion kicks in
Register-ArgumentCompleter -Native -CommandName 'git-swap' -ScriptBlock $GitSwapCompleter
