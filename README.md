# git-swap 🔄

<p align="center">
    <img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=500&pause=1000&color=FFE192&center=true&vCenter=true&width=435&lines=git-swap+add+username" alt="Typing SVG" />
</p>

> Stop committing with the wrong email! Switch Git identities instantly.

**git-swap** is a lightweight, zero-dependency CLI tool written in Go. It manages commit identities and independent GitHub/Gitee SSH accounts on a per-project basis.

Unlike a repository-wide `core.sshCommand`, provider-specific SSH host aliases let one repository use different keys for GitHub and Gitee at the same time.

## 🚀 Features

* **⚡️ Instant Switch:** Change identity locally for the current repository without affecting global settings.
* **🔑 Multi-provider SSH:** Independent GitHub and Gitee accounts, keys, aliases, and repository owners per profile.
* **🧭 Safe Remote Routing:** Rewrites each SSH fetch/push URL to the selected provider alias without changing its owner or repository path.
* **🔏 Commit Signing:** Supports GPG and SSH signing keys. Auto-enables signing per profile.
* **🤖 Auto Detection:** Improved `auto` command to detect and apply profiles based on git remote/history.
* **🔗 Hook System:** `setup-hook` allows automatic profile switching via pre-commit hooks.
* **🔄 HTTPS to SSH:** `convert-ssh` command to easily migrate remotes from HTTPS to SSH format.
* **👀 Status and Doctor:** Inspect the effective identity, repository binding, remote owners, and conflicting SSH overrides.
* **📦 Cross-Platform:** Works on macOS, Linux, and Windows with PowerShell completion support.

---

## 🍴 Why this Fork?

This fork of `git-swap` focuses on automation and robustness for developers managing many repositories.

| Feature | Official Repo | This Fork |
| :--- | :---: | :---: |
| SSH / GPG Management | ✅ | ✅ |
| Interactive Setup | ✅ | ✅ |
| **`git-swap auto`** | Basic | **Improved (Remote Priority)** |
| **`git-swap setup-hook`** | ❌ | **✅ (Automatic Switching)** |
| **`git-swap convert-ssh`** | ❌ | **✅ (HTTPS to SSH Migrate)** |
| **`git-swap current`** | ❌ | **✅ (Alias for Status)** |
| **PowerShell Completion** | ❌ | **✅ (Tab-to-complete)** |

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
go build -o bin/git-swap.exe .
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
- GitHub username and SSH private key (Optional),
- Signing Key (Optional: GPG Key ID or SSH Public Key path for verified commits),
- Gitee username and SSH private key (Optional)

Legacy top-level `ssh_key` and `github_user` fields remain compatible and are treated as a GitHub provider account.


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

Applying a profile binds the commit author with `git-swap.profile`. Its provider accounts are defaults only: remote owner/alias evidence can independently select another profile for GitHub or Gitee. The resulting bindings are stored as `git-swap.provider.github` and `git-swap.provider.gitee`.

*Output: ✅ Swapped to: work*

### 4. Check Status
Not sure which identity is active in the current folder?
```bash
git-swap current  # or 'status'
git-swap doctor
```

### 5. Configure Provider Accounts

Each profile can carry one GitHub account and one Gitee account. A repository may combine them across profiles—for example, commit profile `work`, GitHub provider profile `github-personal`, and Gitee provider profile `gitee-work`. The optional owners list supports organization/team repositories whose URL owner differs from the account username.

```bash
git-swap provider set work github
git-swap provider set work gitee
git-swap provider list work
git-swap provider remove work gitee
```

`provider set` prompts for username, private key, SSH host alias, and accepted repository owners. Enter `-` for the key, alias, or owners prompt to clear that field. A blank key with an explicit alias means the alias is managed externally in `~/.ssh/config`.

### 6. Automation (Hooks)
Tired of manually swapping? Install a pre-commit hook that warns you if your identity doesn't match the project.
```bash
git-swap setup-hook
```
*Note: The hook uses `git-swap auto` and requires the executable to be in your system `PATH`. If you have old hooks with absolute paths, running this command again will automatically upgrade them.*

### 7. Convert Remotes
Migrate HTTPS GitHub and Gitee remotes to SSH. If the repository is already bound, provider-specific aliases are used.
```bash
git-swap convert-ssh
```
This updates remote fetch URLs, explicit push URLs, and submodule URLs independently, preserving each URL's provider, owner, and repository path.

### 8. Edit or Remove
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

`git-swap` stores profiles in `~/.git-swap-config.json`. A profile owns the commit identity and a map of provider accounts:

```json
{
  "work": {
    "name": "Your Name",
    "email": "email@company.com",
    "providers": {
      "github": {
        "username": "github-user",
        "ssh_key": "~/.ssh/github-work",
        "owners": ["github-user", "company-org"]
      },
      "gitee": {
        "username": "gitee-user",
        "ssh_key": "~/.ssh/gitee-work",
        "owners": ["gitee-user", "company-team"]
      }
    }
  }
}
```

For managed keys, `git-swap` writes marked blocks near the top of `~/.ssh/config`:

```sshconfig
# BEGIN git-swap git-swap-github-work
Host git-swap-github-work
  HostName github.com
  User git
  IdentityFile ~/.ssh/github-work
  IdentitiesOnly yes
# END git-swap git-swap-github-work
```

The original SSH config is backed up to `~/.ssh/config.git-swap.bak` before replacement. Existing non-managed content is preserved. The alias is then stored in each matching remote URL, so GitHub and Gitee authentication remain independent even when both are push targets of the same remote.

`git-swap auto` first scans every local `remote.*.url` and `remote.*.pushurl`, then resolves each platform independently:

1. GitHub URLs select `git-swap.provider.github`.
2. Gitee URLs select `git-swap.provider.gitee`.
3. An existing valid `git-swap.profile` remains the commit-author binding.
4. Without one, a single shared provider profile, the effective Git identity, or recent commit-email history selects the commit author.

Different profiles across GitHub and Gitee are valid. Multiple fetch and push URLs are all inspected. Only conflicting profiles within the same platform fail closed, because one platform account cannot safely authenticate two account owners through one repository binding.

If GitHub CLI (`gh`) is installed, `git-swap` switches its active `github.com` account to the independently bound GitHub provider username. Gitee authentication remains SSH-based; no GitHub-specific behavior is applied to it.

## 🤝 Contributing

Pull requests are welcome! Feel free to open an issue for any bugs or feature requests.

## 📄 License

MIT
