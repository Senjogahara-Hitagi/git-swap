package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ANSI Color Codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

func printError(format string, a ...interface{}) {
	fmt.Printf("%sError: %s%s\n", ColorRed, fmt.Sprintf(format, a...), ColorReset)
}
func printWarning(format string, a ...interface{}) {
	fmt.Printf("%s⚠️  %s%s\n", ColorYellow, fmt.Sprintf(format, a...), ColorReset)
}
func printSuccess(format string, a ...interface{}) {
	fmt.Printf("%s✅ %s%s\n", ColorGreen, fmt.Sprintf(format, a...), ColorReset)
}

type Profile struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	SSHKey     string `json:"ssh_key"`
	SigningKey string `json:"signing_key"`
}

type Config map[string]Profile

var reservedCommands = map[string]bool{
	"list": true, "status": true, "add": true, "edit": true, "remove": true, "rm": true,
	"auto": true, "help": true, "_complete": true, "setup-hook": true, "remove-hook": true, "convert-ssh": true, "current": true,
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	command := os.Args[1]
	config := loadConfig()

	if idx, err := strconv.Atoi(command); err == nil {
		swapProfileByIndex(idx, config)
		return
	}

	switch command {
	case "list":
		listProfiles(config)
	case "status", "current":
		showStatus(config)
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap add <name>")
			os.Exit(1)
		}
		addProfile(os.Args[2], config)
	case "edit":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap edit <name>")
			os.Exit(1)
		}
		editProfile(os.Args[2], config)
	case "remove", "rm":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap remove <name>")
			os.Exit(1)
		}
		removeProfile(os.Args[2], config)
	case "auto":
		autoDetectProfile(config)
	case "setup-hook":
		setupGitHook()
	case "remove-hook":
		removeGitHook()
	case "convert-ssh":
		convertSSH()
	case "_complete":
		for k := range config {
			fmt.Println(k)
		}
	case "help":
		printUsage()
	default:
		swapProfile(command, config)
	}
}

func printUsage() {
	fmt.Println("git-swap: Manage git identities locally per repository WITHOUT dependencies.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  git-swap list                  - Show all configured profiles")
	fmt.Println("  git-swap status|current        - Show current git identity in the local repository")
	fmt.Println("  git-swap add <name>            - Create a new profile interactively")
	fmt.Println("  git-swap edit <name>           - Edit an existing profile")
	fmt.Println("  git-swap remove <name>         - Delete a profile")
	fmt.Println("  git-swap <name|index>          - Apply a profile to the current repository")
	fmt.Println("  git-swap auto                  - Auto-detect and apply profile based on remote/history")
	fmt.Println("  git-swap setup-hook            - Install 'auto' pre-commit hook in current repo")
	fmt.Println("  git-swap remove-hook           - Remove 'auto' pre-commit hook from current repo")
	fmt.Println("  git-swap convert-ssh           - Convert HTTPS GitHub remotes to SSH format")
}

func getConfigPath() string {
	fileName := ".git-swap-config.json"
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, fileName)
}

func loadConfig() Config {
	configFile, err := os.ReadFile(getConfigPath())
	if err != nil {
		return make(Config)
	}
	var config Config
	json.Unmarshal(configFile, &config)
	return config
}

func saveConfig(config Config) {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(getConfigPath(), data, 0644)
}

func expandPath(path string) string {
	if path == "" {
		return ""
	}
	re := regexp.MustCompile(`%([^%]+)%`)
	path = re.ReplaceAllStringFunc(path, func(m string) string {
		val := os.Getenv(strings.Trim(m, "%"))
		if val != "" {
			return val
		}
		return m
	})
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = filepath.Join(usr.HomeDir, path[1:])
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func getSortedKeys(config Config) []string {
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func listProfiles(config Config) {
	if len(config) == 0 {
		fmt.Println("No profiles.")
		return
	}
	fmt.Println("Available Identities:")
	keys := getSortedKeys(config)
	for i, k := range keys {
		fmt.Printf(" %d. %s%s%s (%s)\n", i+1, ColorCyan, k, ColorReset, config[k].Email)
	}
}

func addProfile(key string, config Config) {
	if reservedCommands[strings.ToLower(key)] {
		printError("Reserved command.")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(key); err == nil {
		printError("Profile name cannot be a number.")
		os.Exit(1)
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter Name: ")
	n, _ := reader.ReadString('\n')
	fmt.Print("Enter Email: ")
	e, _ := reader.ReadString('\n')
	
	eTrimmed := strings.TrimSpace(e)
	if !strings.Contains(eTrimmed, "@") {
		printWarning("Email doesn't look valid (missing '@'). Saving anyway, but please verify.")
	}

	fmt.Print("Enter SSH Key Path: ")
	s, _ := reader.ReadString('\n')
	fmt.Print("Enter Signing Key: ")
	k, _ := reader.ReadString('\n')
	
	config[key] = Profile{
		Name:       strings.TrimSpace(n),
		Email:      eTrimmed,
		SSHKey:     strings.TrimSpace(s),
		SigningKey: strings.TrimSpace(k),
	}
	saveConfig(config)
	printSuccess("Added!")
}

func editProfile(key string, config Config) {
	p, ok := config[key]
	if !ok {
		printError("Profile '%s' not found.", key)
		os.Exit(1)
	}
	reader := bufio.NewReader(os.Stdin)
	
	fmt.Printf("Name [%s]: ", p.Name)
	if n, _ := reader.ReadString('\n'); strings.TrimSpace(n) != "" {
		p.Name = strings.TrimSpace(n)
	}
	
	fmt.Printf("Email [%s]: ", p.Email)
	if e, _ := reader.ReadString('\n'); strings.TrimSpace(e) != "" {
		p.Email = strings.TrimSpace(e)
		if !strings.Contains(p.Email, "@") {
			printWarning("Email doesn't look valid (missing '@'). Saving anyway, but please verify.")
		}
	}
	
	fmt.Printf("SSH Key [%s]: ", p.SSHKey)
	if s, _ := reader.ReadString('\n'); strings.TrimSpace(s) != "" {
		p.SSHKey = strings.TrimSpace(s)
	}
	
	fmt.Printf("Signing Key [%s]: ", p.SigningKey)
	if k, _ := reader.ReadString('\n'); strings.TrimSpace(k) != "" {
		p.SigningKey = strings.TrimSpace(k)
	}
	
	config[key] = p
	saveConfig(config)
	printSuccess("Updated!")
}

func removeProfile(key string, config Config) {
	if _, exists := config[key]; !exists {
		printError("Profile '%s' not found.", key)
		os.Exit(1)
	}
	delete(config, key)
	saveConfig(config)
	printSuccess("Removed!")
}

func swapProfileByIndex(idx int, config Config) {
	keys := getSortedKeys(config)
	if idx < 1 || idx > len(keys) {
		printError("Index out of range.")
		os.Exit(1)
	}
	swapProfile(keys[idx-1], config)
}

func setGitConfig(key, value string) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		if err := exec.Command("git", "config", "--local", key, value).Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}

func unsetGitConfig(keys ...string) {
	for _, key := range keys {
		exec.Command("git", "config", "--local", "--unset", key).Run()
	}
}

func swapProfile(profileName string, config Config) {
	p, ok := config[profileName]
	if !ok {
		printError("Profile '%s' not found.", profileName)
		os.Exit(1)
	}
	setGitConfig("user.name", p.Name)
	setGitConfig("user.email", p.Email)
	
	if p.SSHKey != "" {
		clean := expandPath(p.SSHKey)
		sshCmd := fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes -F /dev/null", clean)
		setGitConfig("core.sshCommand", sshCmd)
		fmt.Printf("🔑 SSH Key: %s\n", clean)
	} else {
		unsetGitConfig("core.sshCommand")
	}
	
	if p.SigningKey != "" {
		setGitConfig("user.signingkey", p.SigningKey)
		setGitConfig("commit.gpgsign", "true")
		if strings.HasPrefix(p.SigningKey, "ssh-") {
			setGitConfig("gpg.format", "ssh")
		} else {
			unsetGitConfig("gpg.format")
		}
	} else {
		unsetGitConfig("user.signingkey", "commit.gpgsign", "gpg.format")
	}
	
	printSuccess("Swapped to: %s", profileName)
}

func showStatus(config Config) {
	n, _ := exec.Command("git", "config", "user.name").Output()
	e, _ := exec.Command("git", "config", "user.email").Output()
	cn, ce := strings.TrimSpace(string(n)), strings.TrimSpace(string(e))
	
	fmt.Printf("Current: %s <%s>\n", cn, ce)
	for k, p := range config {
		if p.Name == cn && p.Email == ce {
			printSuccess("Match: %s", k)
			if p.SSHKey != "" {
				clean := expandPath(p.SSHKey)
				if _, err := os.Stat(clean); os.IsNotExist(err) {
					printWarning("SSH Key file not found at %s", clean)
				}
				cmdOut, _ := exec.Command("git", "config", "--local", "core.sshCommand").Output()
				currSSHCmd := strings.TrimSpace(string(cmdOut))
				expectedCmd := fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes -F /dev/null", clean)
				if currSSHCmd != expectedCmd && currSSHCmd != "" {
					printWarning("core.sshCommand is polluted or incorrect.")
					fmt.Printf("      Expected: %s\n", expectedCmd)
					fmt.Printf("      Found:    %s\n", currSSHCmd)
				}
			}
			return
		}
	}
	printWarning("No match found.")
}

func isGitRepo() bool {
	_, err := os.Stat(".git")
	return !os.IsNotExist(err)
}

func autoDetectProfile(config Config) {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}
	
	k, s := detectByRemotePriority(config)
	if k == "" {
		k, s = detectByHistory(config)
	}
	
	if k != "" {
		fmt.Printf("🔍 Detected via %s: %s%s%s\n", s, ColorCyan, k, ColorReset)
		swapProfile(k, config)
	} else {
		printWarning("No match found.")
	}
}

func detectByHistory(config Config) (string, string) {
	out, err := exec.Command("git", "log", "-n", "100", "--format=%ae").Output()
	if err != nil {
		return "", ""
	}
	emails := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, e := range emails {
		for key, p := range config {
			if strings.EqualFold(p.Email, e) {
				return key, "history"
			}
		}
	}
	return "", ""
}

type remoteMatch struct {
	remote string
	rtype  string
	score  int
	method string
}

func detectByRemotePriority(config Config) (string, string) {
	out, _ := exec.Command("git", "remote", "-v").Output()
	lines := strings.Split(string(out), "\n")
	re := regexp.MustCompile(`^(\S+)\s+.*[:/]([\w\.-]+)/[\w\.-]+(?:\.git)?\s+\((push|fetch)\)`)
	
	var remotes []remoteMatch
	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) > 3 {
			remoteName := m[1]
			score := 40
			if remoteName == "origin" {
				score = 100
			} else if remoteName == "upstream" {
				score = 80
			} else if strings.Contains(line, "github.com") {
				score = 60
			}
			if m[3] == "push" {
				score += 10
			}
			remotes = append(remotes, remoteMatch{
				remote: m[2], 
				rtype:  m[3], 
				score:  score, 
				method: remoteName + " " + m[3] + " URL",
			})
		}
	}
	
	sort.SliceStable(remotes, func(i, j int) bool {
		return remotes[i].score > remotes[j].score
	})
	
	for _, r := range remotes {
		for key, p := range config {
			if strings.EqualFold(key, r.remote) || 
			   strings.EqualFold(strings.Split(p.Email, "@")[0], r.remote) || 
			   strings.EqualFold(p.Name, r.remote) {
				return key, "remote " + r.method
			}
		}
	}
	return "", ""
}

func setupGitHook() {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}
	hooksDir := filepath.Join(".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		printError("Error creating hooks directory: %v", err)
		os.Exit(1)
	}
	
	hookPath := filepath.Join(hooksDir, "pre-commit")
	hookMarker := "# git-swap auto-swapper hook"
	newHookCommand := "\n" + hookMarker + "\ngit-swap auto\n"

	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		content := "#!/bin/sh\n" + newHookCommand
		if err = os.WriteFile(hookPath, []byte(content), 0755); err != nil {
			printError("Error creating hook: %v", err)
			os.Exit(1)
		}
	} else {
		existing, err := os.ReadFile(hookPath)
		if err != nil {
			printError("Error reading existing hook: %v", err)
			os.Exit(1)
		}
		
		contentStr := string(existing)
		if !strings.Contains(contentStr, hookMarker) {
			// Marker not found, append
			f, err := os.OpenFile(hookPath, os.O_APPEND|os.O_WRONLY, 0755)
			if err != nil {
				printError("Error modifying hook: %v", err)
				os.Exit(1)
			}
			defer f.Close()
			if _, err := f.WriteString(newHookCommand); err != nil {
				printError("Error writing to hook: %v", err)
				os.Exit(1)
			}
		} else {
			// Marker exists, check if it needs upgrading (old absolute paths vs new simple command)
			// Match the block starting with marker and the following line containing git-swap
			re := regexp.MustCompile("(?m)^" + regexp.QuoteMeta(hookMarker) + "\\s*\\n.*git-swap.*auto")
			currentMatch := re.FindString(contentStr)
			
			if currentMatch != "" && !strings.Contains(currentMatch, "git-swap auto") || strings.Contains(currentMatch, ".exe") || strings.Contains(currentMatch, ":/") {
				// Needs upgrade: replaces the old block with the new one
				newContent := re.ReplaceAllString(contentStr, strings.TrimSpace(newHookCommand))
				if err := os.WriteFile(hookPath, []byte(newContent), 0755); err != nil {
					printError("Error upgrading hook: %v", err)
					os.Exit(1)
				}
				printSuccess("Upgraded existing hook to use environment PATH.")
				return
			}
			// Already up to date
			printWarning("Hook already up to date.")
			return
		}
	}
	printSuccess("Git pre-commit hook installed successfully!")
	fmt.Println("Now 'git-swap auto' will run automatically before every commit.")
}

func removeGitHook() {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}
	hookPath := filepath.Join(".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		printWarning("No pre-commit hook found. Nothing to remove.")
		return
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		printError("Error reading pre-commit hook: %v", err)
		return
	}

	contentStr := string(content)
	startMarker := "# git-swap auto-swapper hook"
	
	if !strings.Contains(contentStr, startMarker) {
		printWarning("git-swap auto-swapper hook not found in pre-commit.")
		return
	}

	lines := strings.Split(contentStr, "\n")
	var newLines []string
	skipRegex := regexp.MustCompile(`(git-swap\s+auto|".*git-swap(\.exe)?"\s+auto)`)

	inHookBlock := false
	for _, line := range lines {
		if strings.TrimSpace(line) == startMarker {
			inHookBlock = true
			continue
		}
		if inHookBlock {
			if strings.TrimSpace(line) == "" || skipRegex.MatchString(line) {
				continue
			}
			// It's not a known line of the hook block, meaning the block is over
			inHookBlock = false
		}
		newLines = append(newLines, line)
	}

	// Clean up trailing/leading newlines slightly
	finalContent := strings.TrimSpace(strings.Join(newLines, "\n"))
	
	if finalContent == "#!/bin/sh" || finalContent == "" {
		// Just remove the hook entirely if it's practically empty
		os.Remove(hookPath)
		printSuccess("Removed pre-commit hook entirely as it was empty.")
	} else {
		finalContent += "\n" // Add trailing newline
		os.WriteFile(hookPath, []byte(finalContent), 0755)
		printSuccess("Removed git-swap hook block from pre-commit hook.")
	}
}

func convertSSH() {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}
	out, _ := exec.Command("git", "remote", "-v").Output()
	lines := strings.Split(string(out), "\n")
	
	processed := make(map[string]bool)
	convertedAny := false
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			urlStr := parts[1]
			
			if processed[name] {
				continue
			}
			
			if strings.HasPrefix(urlStr, "https://github.com/") {
				repoPath := strings.TrimPrefix(urlStr, "https://github.com/")
				if !strings.HasSuffix(repoPath, ".git") {
					repoPath += ".git"
				}
				newURL := "git@github.com:" + repoPath
				
				if err := exec.Command("git", "remote", "set-url", name, newURL).Run(); err == nil {
					printSuccess("Converted remote '%s' to SSH: %s", name, newURL)
					convertedAny = true
				} else {
					printError("Failed to convert remote '%s'", name)
				}
			}
			processed[name] = true
		}
	}
	if !convertedAny {
		printWarning("No HTTPS GitHub remotes found to convert.")
	}
}
