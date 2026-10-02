# GEMINI.md - git-swap 🔄

## Project Overview
`git-swap` is a lightweight, zero-dependency CLI tool written in Go designed to manage commit identities plus independent GitHub and Gitee accounts on a per-project basis. Provider SSH keys are routed through account-specific SSH host aliases rather than a repository-wide `core.sshCommand`.

### Core Technologies
- **Language:** Go (Standard Library only)
- **Configuration Storage:** JSON file at `~/.git-swap-config.json`
- **Integration:** Directly invokes `git` commands via `os/exec`.

## Building and Running

### Build from Source
To build the binary locally, you need Go installed:
```bash
go build -o bin/git-swap.exe .
```

### Running the Tool
After building, you can run the binary directly:
```bash
./git-swap help
```

### Common Commands
- `git-swap list`: Show all configured profiles.
- `git-swap status` (or `current`): Display the current Git identity active in the local repository.
- `git-swap add <name>`: Create a new profile interactively.
- `git-swap edit <name>`: Modify an existing profile.
- `git-swap remove <name>`: Delete a profile.
- `git-swap <name>`: Apply the specified profile to the current repository (requires `.git` directory).
- `git-swap setup-hook`: Install/Update 'auto' pre-commit hook in the current repository. Uses `git-swap auto` (requires PATH) and automatically upgrades old absolute-path hooks.
- `git-swap remove-hook`: Remove the pre-commit hook from the current repository.
- `bulk_setup_hooks.py`: Python script to scan directories and bulk install/update hooks in all discovered repositories (now supports overwriting).
- `git-swap auto`: Independently bind GitHub/Gitee accounts from all fetch/push URLs, then preserve or infer the commit profile.
- `git-swap provider <list|set|remove>`: Manage GitHub/Gitee accounts independently within a profile.
- `git-swap doctor`: Validate the repository binding, SSH override, and provider remotes.
- `git-swap convert-ssh`: Convert HTTPS GitHub/Gitee remotes to SSH format.
- `git-swap _complete`: Internal command used for PowerShell completion logic.

## Development Conventions

### Code Structure
- **Core CLI:** `main.go` contains profile, hook, signing, and GitHub CLI behavior.
- **Provider Layer:** `providers.go` contains GitHub/Gitee modeling, URL routing, SSH config management, provider detection, and doctor checks.
- **ANSI Colors:** Terminal output is colorized using standard ANSI escape codes defined as constants.
- **Error Handling:** Errors are generally reported to `stdout`/`stderr` with color coding, followed by `os.Exit(1)`.

### Configuration Management
- Profiles are stored in a map-based structure (`Config map[string]Profile`).
- `loadConfig()` handles reading and unmarshaling the JSON file.
- `saveConfig()` handles marshaling and writing back to the user's home directory.

### Git Interaction
- The tool uses `git config --local` to ensure global settings remain untouched.
- Each provider account uses a distinct SSH host alias with `IdentitiesOnly yes`; `core.sshCommand` is removed because it cannot select different keys for different remotes in one repository.
- Managed SSH config blocks are bounded by `# BEGIN/END git-swap <alias>`, written with a backup, and placed before wildcard `Host *` settings.
- Legacy top-level `ssh_key` and `github_user` fields normalize to a GitHub provider account.
- `git-swap.profile` is the commit-author binding; `git-swap.provider.github` and `git-swap.provider.gitee` are independent authentication bindings.
- Auto-detection accepts different profiles across providers and fails closed only when URLs for the same provider resolve to multiple profiles.
- Signing keys support both GPG and SSH formats.

### Testing
- Run `go test ./...` and `go vet ./...` before committing.
- Tests cover URL parsing/conversion, legacy migration, owner/alias matching, independent multi-provider push URLs, and managed SSH block safety.
