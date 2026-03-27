# git-swap 🔄

<p align="center">
    <img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=500&pause=1000&color=FFE192&center=true&vCenter=true&width=435&lines=git-swap+add+username" alt="Typing SVG" />
</p>

> Stop committing with the wrong email! Switch Git identities instantly.

**git-swap** is a lightweight, zero-dependency CLI tool written in Go. It allows developers to manage multiple Git identities (Personal, Work, Freelance) and switch between them on a per-project basis with a single command.

It handles not just `user.name` and `user.email`, but also manages project-specific **SSH keys** (`core.sshCommand`), ensuring you never get "Permission denied" errors again.

## 🚀 Features

* **⚡️ Instant Switch:** Change identity locally for the current repository without affecting global settings.
* **🔑 SSH Key Management:** Automatically sets specific SSH keys for specific profiles.
* **🔏 Commit Signing:** Supports GPG and SSH signing keys. Auto-enables signing per profile.
* **🛠 CLI Managed:** No manual JSON editing needed. Add, edit, and remove profiles interactively.
* **👀 Status Check:** Verify which profile is currently active in your repo.
* **📦 Cross-Platform:** Works on macOS, Linux, and Windows.

---

## 📦 Installation

### macOS & Linux
Install via the automatic script:

```bash
curl -sL https://raw.githubusercontent.com/abdozkaya/git-swap/main/install.sh | bash
```



### Windows (PowerShell)
Run as Administrator:
```powershell
iwr -useb https://raw.githubusercontent.com/abdozkaya/git-swap/main/install.ps1 | iex
```


### Build from Source (Go required)
If you prefer to build it yourself:
```bash
git clone https://github.com/abdozkaya/git-swap.git
cd git-swap
go build -o bin/git-swap.exe main.go
```
---

## 🎮 Usage

### 1. Create a Profile
The tool is interactive. You can add a new identity (e.g., "work") easily.
```bash
git-swap add work
```
It will ask for:
- Name, 
- Email,
- SSH Key Path (Optional: e.g., `~/.ssh/id_work`),
- Signing Key (Optional: GPG Key ID or SSH Public Key path for verified commits)


### 2. List Profiles
See all your configured identities.
```bash
git-swap list
```
### 3. Swap Identity
Navigate to any git repository and apply a profile.
```bash
cd ~/my-company-project
git-swap work
```
*Output: ✅ Swapped to: work*

### 4. Check Status
Not sure which identity is active in the current folder?
```bash
git-swap status
```
### 5. Edit or Remove
Update an existing profile or delete one.

# Update details
```bash
git-swap edit work
```
# Delete profile
```bash
git-swap remove work
```
---

## ⚙️ How It Works

`git-swap` stores your profiles in a local configuration file (`~/.git-swap-config.json`).
When you run `git-swap <profile>`, it executes the following git commands locally in your project:

```bash
git config --local user.name "Your Name"
git config --local user.email "email@company.com"

# If SSH Key is provided:
git config --local core.sshCommand "ssh -i /path/to/private_key -F /dev/null"

# If Signing Key is provided:
git config --local user.signingkey "key_id_or_pub_key"
git config --local commit.gpgsign true
```
This ensures your global git configuration (`~/.gitconfig`) remains untouched and clean.

## 🤝 Contributing

Pull requests are welcome! Feel free to open an issue for any bugs or feature requests.

## 📄 License

MIT