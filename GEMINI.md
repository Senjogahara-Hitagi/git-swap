# GEMINI.md - git-swap 🔄

## Project Overview
`git-swap` is a lightweight, zero-dependency CLI tool written in Go designed to manage and switch between multiple Git identities (e.g., Personal, Work, Freelance) on a per-project basis. It automates the configuration of `user.name`, `user.email`, SSH keys (`core.sshCommand`), and commit signing settings (`user.signingkey`) locally for a repository.

### Core Technologies
- **Language:** Go (Standard Library only)
- **Configuration Storage:** JSON file at `~/.git-swap-config.json`
- **Integration:** Directly invokes `git` commands via `os/exec`.

## Building and Running

### Build from Source
To build the binary locally, you need Go installed:
```bash
go build -o git-swap main.go
```

### Running the Tool
After building, you can run the binary directly:
```bash
./git-swap help
```

### Common Commands
- `git-swap list`: Show all configured profiles.
- `git-swap status`: Display the current Git identity active in the local repository.
- `git-swap add <name>`: Create a new profile interactively.
- `git-swap edit <name>`: Modify an existing profile.
- `git-swap remove <name>`: Delete a profile.
- `git-swap <name>`: Apply the specified profile to the current repository (requires `.git` directory).

## Development Conventions

### Code Structure
- **Single File:** The entire logic is currently contained within `main.go`.
- **ANSI Colors:** Terminal output is colorized using standard ANSI escape codes defined as constants.
- **Error Handling:** Errors are generally reported to `stdout`/`stderr` with color coding, followed by `os.Exit(1)`.

### Configuration Management
- Profiles are stored in a map-based structure (`Config map[string]Profile`).
- `loadConfig()` handles reading and unmarshaling the JSON file.
- `saveConfig()` handles marshaling and writing back to the user's home directory.

### Git Interaction
- The tool uses `git config --local` to ensure global settings remain untouched.
- SSH keys are managed via `core.sshCommand` with `-o IdentitiesOnly=yes` and `-F /dev/null` to ensure the specific key is used without interference from `ssh-agent`.
- Signing keys support both GPG and SSH formats.

### Testing
- Currently, there are no automated tests (no `_test.go` files).
- Manual verification involves creating profiles and checking `git config --local -l` in test repositories.
