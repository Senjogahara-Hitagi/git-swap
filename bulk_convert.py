import os
import re
import subprocess
import argparse
from datetime import datetime

# --- 核心排除（只排除系统受限目录，提升根目录扫描成功率） ---
SYSTEM_EXCLUDE = {
    "System Volume Information", "$RECYCLE.BIN", "Config.Msi", "RECYCLE.BIN", "Recovery", "Windows", "AppData"
}

PLATFORMS = ['github.com', 'gitee.com', 'gitlab.com']
HTTPS_PATTERN = re.compile(
    r'^https:\/\/(?:.*@)?(' + '|'.join(re.escape(p) for p in PLATFORMS) + r')\/([\w-]+\/[\w.-]+?)(?:\.git)?\/?$'
)

def run_command(cmd, cwd):
    try:
        result = subprocess.run(
            cmd, cwd=cwd, capture_output=True, text=True, check=True, shell=True
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError:
        return None

def process_repo(repo_path):
    remotes_raw = run_command("git remote", repo_path)
    if not remotes_raw:
        return "NO_REMOTE", []

    results = []
    found_https = False
    remotes = remotes_raw.splitlines()

    for name in remotes:
        urls_raw = run_command(f"git remote get-url --all {name}", repo_path)
        if not urls_raw: continue
        
        urls = urls_raw.splitlines()
        for url in urls:
            match = HTTPS_PATTERN.match(url)
            if match:
                host = match.group(1)
                repo_id = match.group(2)
                ssh_url = f"git@{host}:{repo_id}.git"
                
                if run_command(f"git remote set-url {name} {ssh_url} {url}", repo_path) is not None:
                    results.append(f"  ✅ [{name}] {url} -> {ssh_url}")
                    found_https = True
            elif url.startswith("git@"):
                results.append(f"  ℹ️  [{name}] Already SSH: {url}")
    
    if found_https:
        return "SUCCESS", results
    elif results:
        return "ALREADY_SSH", results
    else:
        return "OTHER_PLATFORM", ["  ⚠️  Non-target platform."]

def fast_scan(root):
    repos = []
    print(f"🚀 Scanning root: {root}")
    
    # 检查根目录本身
    if os.path.exists(os.path.join(root, ".git")):
        repos.append(root)

    for dirpath, dirnames, filenames in os.walk(root, topdown=True):
        # 实时过滤 dirnames
        new_dirs = []
        for d in dirnames:
            if d in SYSTEM_EXCLUDE or d.startswith('$'):
                continue
            new_dirs.append(d)
        
        dirnames[:] = new_dirs
        
        if '.git' in dirnames:
            repos.append(dirpath)
            # 找到 .git 就不再递归进入此目录内部，大幅提速
            dirnames.remove('.git')
            
    return repos

def main():
    parser = argparse.ArgumentParser(description="Bulk convert Git HTTPS remotes to SSH.")
    parser.add_argument("--roots", nargs="+", default=["."], help="Root directories to scan")
    
    args = parser.parse_args()
    
    all_repos = []
    for root_path in args.roots:
        if not os.path.exists(root_path):
            print(f"⚠️  Path not found: {root_path}")
            continue
        all_repos.extend(fast_scan(root_path))

    report_lines = [
        f"Git HTTPS to SSH Conversion Report",
        f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
        f"Scanned Roots: {', '.join(args.roots)}",
        "=" * 60,
        ""
    ]
    
    stats = {"SUCCESS": 0, "ALREADY_SSH": 0, "OTHER_PLATFORM": 0, "NO_REMOTE": 0}
    
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
        f"✅ Successfully converted: {stats['SUCCESS']}",
        f"ℹ️  Already SSH: {stats['ALREADY_SSH']}",
        f"⚠️  Other/Skip: {stats['OTHER_PLATFORM'] + stats['NO_REMOTE']}",
        f"📁 Total Repositories Found: {len(all_repos)}",
        "-" * 60
    ]
    
    for s in summary:
        print(s)
        report_lines.append(s)

    with open("conversion_report.txt", "w", encoding="utf-8") as f:
        f.write("\n".join(report_lines))
    
    print(f"\n📄 Detailed report saved to: {os.path.abspath('conversion_report.txt')}")

if __name__ == "__main__":
    main()
