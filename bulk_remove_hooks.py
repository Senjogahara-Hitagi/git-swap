import os
import subprocess
import argparse
import sys
from datetime import datetime

SYSTEM_EXCLUDE = {
    "System Volume Information", "$RECYCLE.BIN", "Config.Msi", "RECYCLE.BIN", "Recovery", "Windows", "AppData"
}

def is_git_repo(path):
    return os.path.exists(os.path.join(path, ".git"))

def check_hook_exists(repo_path):
    hook_path = os.path.join(repo_path, ".git", "hooks", "pre-commit")
    if not os.path.exists(hook_path):
        return False
    try:
        with open(hook_path, "r", encoding="utf-8") as f:
            return "git-swap auto-swapper hook" in f.read()
    except Exception:
        return False

def process_repo(repo_path):
    if not check_hook_exists(repo_path):
        return "ALREADY_CLEAN", ["  ℹ️  No hook found to remove."]

    try:
        result = subprocess.run(
            "git-swap remove-hook",
            cwd=repo_path,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            shell=True
        )
        if result.returncode == 0:
            out = result.stdout.strip() if result.stdout else ""
            return "SUCCESS", [f"  ✅ {out}"]
        else:
            err = result.stderr.strip() if result.stderr else ""
            return "FAILED", [f"  ❌ {err}"]
    except Exception as e:
        return "FAILED", [f"  ⚠️ Error executing git-swap: {e}"]

def fast_scan(root):
    repos = []
    print(f"🚀 Scanning root: {root}")
    
    if os.path.exists(os.path.join(root, ".git")):
        repos.append(root)

    for dirpath, dirnames, filenames in os.walk(root, topdown=True):
        new_dirs = []
        for d in dirnames:
            if d.strip() in SYSTEM_EXCLUDE or d.startswith('$'):
                continue
            new_dirs.append(d)
        dirnames[:] = new_dirs
        
        if '.git' in dirnames:
            repos.append(dirpath)
            # 找到 .git 就不再递归进入此目录内部
            dirnames.remove('.git')
            
    return repos

def main():
    parser = argparse.ArgumentParser(description="Bulk remove git-swap auto hook from all git repos under specified paths.")
    parser.add_argument("paths", nargs="+", help="Paths to scan for Git repositories")
    args = parser.parse_args()
    
    try:
        subprocess.run(["git-swap", "help"], capture_output=True, shell=True)
    except FileNotFoundError:
        print("❌ Error: 'git-swap' command not found in PATH. Please make sure git-swap is in your PATH.")
        sys.exit(1)

    all_repos = []
    for root_path in args.paths:
        root_path = os.path.expandvars(os.path.expanduser(root_path))
        if not os.path.exists(root_path):
            print(f"⚠️  Path not found: {root_path}")
            continue
        all_repos.extend(fast_scan(root_path))

    report_lines = [
        f"Git-Swap Bulk Hook Removal Report",
        f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"Scanned Roots: {', '.join(args.paths)}",
        "=" * 60,
        ""
    ]
    
    stats = {"SUCCESS": 0, "ALREADY_CLEAN": 0, "FAILED": 0}
    
    print(f"\n📊 Found {len(all_repos)} repositories. Processing...\n")
    
    for repo in all_repos:
        status, details = process_repo(repo)
        stats[status] += 1
        
        line = f"📁 Repo: {repo} [{status}]"
        print(line)
        report_lines.append(line)
        for d in details:
            report_lines.append(d)
        report_lines.append("")

    summary = [
        "-" * 60,
        "🎉 Summary Report:",
        f"✅ Successfully removed: {stats['SUCCESS']}",
        f"ℹ️  Already clean: {stats['ALREADY_CLEAN']}",
        f"❌ Failed: {stats['FAILED']}",
        f"📁 Total Repositories Found: {len(all_repos)}",
        "-" * 60
    ]
    
    for s in summary:
        print(s)
        report_lines.append(s)

    with open("remove_hooks_report.txt", "w", encoding="utf-8") as f:
        f.write("\n".join(report_lines))
    
    print(f"\n📄 Detailed report saved to: {os.path.abspath('remove_hooks_report.txt')}")

if __name__ == "__main__":
    main()
