import os
import subprocess
import argparse
import sys

def is_git_repo(path):
    return os.path.exists(os.path.join(path, ".git"))

def setup_hook_in_repo(repo_path):
    print(f"📦 Processing: {repo_path}")
    try:
        # Run git-swap setup-hook inside the target directory
        result = subprocess.run(
            ["git-swap", "setup-hook"],
            cwd=repo_path,
            capture_output=True,
            text=True,
            shell=True
        )
        if result.returncode == 0:
            print(f"  ✅ Success: {result.stdout.strip()}")
        else:
            print(f"  ❌ Failed: {result.stderr.strip()}")
    except Exception as e:
        print(f"  ⚠️ Error executing git-swap: {e}")

SYSTEM_EXCLUDE = {
    "System Volume Information", "$RECYCLE.BIN", "Config.Msi", "RECYCLE.BIN", "Recovery", "Windows", "AppData"
}

def scan_and_setup(base_paths):
    found_repos = []
    for base_path in base_paths:
        base_path = os.path.expandvars(os.path.expanduser(base_path))
        if not os.path.exists(base_path):
            print(f"⚠️ Path not found: {base_path}")
            continue
        
        print(f"🔍 Scanning: {base_path}...")
        
        # Check root itself
        if os.path.exists(os.path.join(base_path, ".git")):
            found_repos.append(base_path)
            setup_hook_in_repo(base_path)

        for root, dirs, files in os.walk(base_path, topdown=True):
            # Fast filter
            new_dirs = []
            for d in dirs:
                if d in SYSTEM_EXCLUDE or d.startswith('$'):
                    continue
                new_dirs.append(d)
            dirs[:] = new_dirs

            if ".git" in dirs:
                repo_path = os.path.abspath(root)
                if repo_path not in found_repos:
                    found_repos.append(repo_path)
                    setup_hook_in_repo(repo_path)
                dirs.remove(".git")
    
    print("\n" + "="*30)
    print(f"🏁 Done! Total Git repos found and processed: {len(found_repos)}")
    print("="*30)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Bulk install git-swap setup-hook in all git repos under specified paths.")
    parser.add_argument("paths", nargs="+", help="Paths to scan for Git repositories")
    
    args = parser.parse_args()
    
    # Check if git-swap is available in PATH
    try:
        subprocess.run(["git-swap", "help"], capture_output=True, shell=True)
    except FileNotFoundError:
        print("❌ Error: 'git-swap' command not found in PATH. Please make sure bin\\git-swap.exe is in your PATH.")
        sys.exit(1)

    scan_and_setup(args.paths)
