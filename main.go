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
	"strings"
)

// ANSI Color Codes
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

type Profile struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	SSHKey     string `json:"ssh_key"`     // Optional, supports ~ and environment variables
	SigningKey string `json:"signing_key"` // Optional
}

type Config map[string]Profile

var reservedCommands = map[string]bool{
	"list":   true,
	"status": true,
	"add":    true,
	"edit":   true,
	"remove": true,
	"rm":     true,
	"auto":   true,
	"help":   true,
	"_complete": true,
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	config := loadConfig()

	switch command {
	case "list":
		listProfiles(config)
	case "status":
		showStatus(config)
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap add <profile-name>")
			os.Exit(1)
		}
		addProfile(os.Args[2], config)
	case "edit":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap edit <profile-name>")
			os.Exit(1)
		}
		editProfile(os.Args[2], config)
	case "remove", "rm":
		if len(os.Args) < 3 {
			fmt.Println("Usage: git-swap remove <profile-name>")
			os.Exit(1)
		}
		removeProfile(os.Args[2], config)
	case "auto":
		autoDetectProfile(config)
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

func getConfigPath() string {
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, ".git-swap-config.json")
}

func loadConfig() Config {
	configPath := getConfigPath()
	configFile, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return make(Config)
	}
	if err != nil {
		fmt.Printf("%sError reading config file: %s%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		fmt.Printf("%sJSON Format Error: %s%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	return config
}

func saveConfig(config Config) {
	configPath := getConfigPath()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		fmt.Printf("%sError processing data: %s%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		fmt.Printf("%sError saving file: %s%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}
}

func expandPath(path string) string {
	if path == "" { return "" }

	// 1. Expand Environment Variables (e.g., $HOME, %USERPROFILE%)
	path = os.ExpandEnv(path)

	// 2. Handle Tilde (~) Prefix
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = filepath.Join(usr.HomeDir, path[1:])
	}

	// 3. Normalize Slashes for Current OS
	return filepath.Clean(path)
}

func addProfile(key string, config Config) {
	if reservedCommands[strings.ToLower(key)] {
		fmt.Printf("%sError: '%s' is a reserved command name.%s\n", ColorRed, key, ColorReset)
		os.Exit(1)
	}
	if _, exists := config[key]; exists {
		fmt.Printf("%sProfile '%s' already exists.%s\n", ColorRed, key, ColorReset)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter Name: ")
	name, _ := reader.ReadString('\n')
	fmt.Printf("Enter Email: ")
	email, _ := reader.ReadString('\n')
	fmt.Printf("Enter SSH Key Path (Optional, supports ~ or $HOME): ")
	sshKey, _ := reader.ReadString('\n')
	fmt.Printf("Enter Signing Key (Optional): ")
	signingKey, _ := reader.ReadString('\n')

	config[key] = Profile{
		Name:       strings.TrimSpace(name),
		Email:      strings.TrimSpace(email),
		SSHKey:     strings.TrimSpace(sshKey),
		SigningKey: strings.TrimSpace(signingKey),
	}

	saveConfig(config)
	fmt.Printf("%s✅ Profile '%s' added!%s\n", ColorGreen, key, ColorReset)
}

func editProfile(key string, config Config) {
	profile, exists := config[key]
	if !exists {
		fmt.Printf("%sError: Profile '%s' does not exist.%s\n", ColorRed, key, ColorReset)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("💡 Tip: Press Enter to keep current value. Type '-' to clear.")

	fmt.Printf("Enter Name [%s]: ", profile.Name)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "-" { profile.Name = "" } else if name != "" { profile.Name = name }

	fmt.Printf("Enter Email [%s]: ", profile.Email)
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "-" { profile.Email = "" } else if email != "" { profile.Email = email }

	fmt.Printf("Enter SSH Key Path [%s]: ", profile.SSHKey)
	sshKey, _ := reader.ReadString('\n')
	sshKey = strings.TrimSpace(sshKey)
	if sshKey == "-" { profile.SSHKey = "" } else if sshKey != "" { profile.SSHKey = sshKey }

	fmt.Printf("Enter Signing Key [%s]: ", profile.SigningKey)
	signingKey, _ := reader.ReadString('\n')
	signingKey = strings.TrimSpace(signingKey)
	if signingKey == "-" { profile.SigningKey = "" } else if signingKey != "" { profile.SigningKey = signingKey }

	config[key] = profile
	saveConfig(config)
	fmt.Printf("%s✅ Profile '%s' updated!%s\n", ColorGreen, key, ColorReset)
}

func removeProfile(key string, config Config) {
	if _, exists := config[key]; !exists {
		fmt.Printf("%sError: Profile '%s' does not exist.%s\n", ColorRed, key, ColorReset)
		os.Exit(1)
	}
	delete(config, key)
	saveConfig(config)
	fmt.Printf("%s🗑️ Profile '%s' removed.%s\n", ColorGreen, key, ColorReset)
}

func listProfiles(config Config) {
	if len(config) == 0 {
		fmt.Println("No profiles found. Use 'git-swap add <name>' to create one.")
		return
	}
	fmt.Println("Available Identities:")
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		p := config[k]
		fmt.Printf(" 🔄 %s%s%s (%s)\n", ColorCyan, k, ColorReset, p.Email)
	}
}

func autoDetectProfile(config Config) {
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		fmt.Printf("%sError: Not a git repository.%s\n", ColorRed, ColorReset)
		os.Exit(1)
	}

	foundKey, foundSource := detectByHistory(config)
	if foundKey == "" {
		foundKey, foundSource = detectByURL(config)
	}

	if foundKey != "" {
		fmt.Printf("🔍 Detected via %s: %s%s%s\n", foundSource, ColorCyan, foundKey, ColorReset)
		swapProfile(foundKey, config)
	} else {
		fmt.Printf("%s⚠️  Could not auto-detect a profile.%s\n", ColorYellow, ColorReset)
		fmt.Println("Try: git-swap <name>")
	}
}

func detectByHistory(config Config) (string, string) {
	cmd := exec.Command("git", "log", "-n", "100", "--format=%ae")
	output, err := cmd.Output()
	if err != nil { return "", "" }
	emails := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if e == "" { continue }
		for key, p := range config {
			if strings.EqualFold(p.Email, e) {
				return key, "commit history"
			}
		}
	}
	return "", ""
}

func detectByURL(config Config) (string, string) {
	cmd := exec.Command("git", "remote", "-v")
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")
	re := regexp.MustCompile(`[:/]([\w\.-]+)/[\w\.-]+(?:\.git)?\s+`)
	usernames := make(map[string]bool)
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 { usernames[matches[1]] = true }
	}
	if len(usernames) == 0 { return "", "" }
	for user := range usernames {
		for key := range config {
			if strings.EqualFold(key, user) { return key, "remote URL" }
		}
		for key, p := range config {
			if strings.EqualFold(strings.Split(p.Email, "@")[0], user) { return key, "remote URL" }
			if strings.EqualFold(p.Name, user) { return key, "remote URL" }
		}
	}
	return "", ""
}

func showStatus(config Config) {
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		fmt.Printf("%sError: Not a git repository.%s\n", ColorRed, ColorReset)
		os.Exit(1)
	}
	cmdName := exec.Command("git", "config", "user.name")
	outName, _ := cmdName.Output()
	currentName := strings.TrimSpace(string(outName))
	cmdEmail := exec.Command("git", "config", "user.email")
	outEmail, _ := cmdEmail.Output()
	currentEmail := strings.TrimSpace(string(outEmail))

	fmt.Println("Current Git Config (Local):")
	fmt.Printf("  Name:  %s%s%s\n", ColorYellow, currentName, ColorReset)
	fmt.Printf("  Email: %s%s%s\n", ColorYellow, currentEmail, ColorReset)

	matchedProfile := ""
	for key, p := range config {
		if p.Name == currentName && p.Email == currentEmail {
			matchedProfile = key
			break
		}
	}
	if matchedProfile != "" {
		fmt.Printf("✅ This matches profile: %s%s%s\n", ColorGreen, matchedProfile, ColorReset)
	} else {
		fmt.Printf("%s⚠️  No matching profile found.%s\n", ColorYellow, ColorReset)
	}
}

func swapProfile(profileName string, config Config) {
	profile, ok := config[profileName]
	if !ok {
		fmt.Printf("%sError: Profile '%s' not found.%s\n", ColorRed, profileName, ColorReset)
		os.Exit(1)
	}
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		fmt.Printf("%sError: Not a git repository.%s\n", ColorRed, ColorReset)
		os.Exit(1)
	}
	
	setGitConfig("user.name", profile.Name)
	setGitConfig("user.email", profile.Email)

	if profile.SSHKey != "" {
		// Expand Environment Variables and Tilde
		expandedSSHPath := expandPath(profile.SSHKey)
		
		// Use forward slashes for Git config even on Windows for better compatibility
		cleanSSHPath := filepath.ToSlash(expandedSSHPath)
		
		sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -F /dev/null", cleanSSHPath)
		setGitConfig("core.sshCommand", sshCmd)
		fmt.Printf("🔑 SSH Key (Expanded): %s\n", cleanSSHPath)
	} else {
		unsetGitConfig("core.sshCommand")
	}

	if profile.SigningKey != "" {
		setGitConfig("user.signingkey", profile.SigningKey)
		setGitConfig("commit.gpgsign", "true")
		if strings.HasPrefix(profile.SigningKey, "ssh-") {
			setGitConfig("gpg.format", "ssh")
		} else {
			unsetGitConfig("gpg.format")
		}
	} else {
		unsetGitConfig("user.signingkey")
		setGitConfig("commit.gpgsign", "false")
		unsetGitConfig("gpg.format")
	}
	fmt.Printf("%s✅ Swapped to: %s%s\n", ColorGreen, profileName, ColorReset)
	fmt.Printf("👤 %s <%s>\n", profile.Name, profile.Email)
}

func setGitConfig(key, value string) {
	exec.Command("git", "config", "--local", key, value).Run()
}

func unsetGitConfig(key string) {
	exec.Command("git", "config", "--local", "--unset", key).Run()
}

func printUsage() {
	fmt.Println("git-swap: Manage git identities directly from CLI.")
	fmt.Println("\nUsage (Commands):")
	fmt.Println("  git-swap list              -> List all profiles")
	fmt.Println("  git-swap status            -> Show current repo identity")
	fmt.Println("  git-swap auto              -> Auto-detect from history or URL")
	fmt.Println("  git-swap add <name>        -> Add a new profile")
	fmt.Println("  git-swap edit <name>       -> Edit an existing profile")
	fmt.Println("  git-swap remove <name>     -> Delete a profile")
	fmt.Println("\nUsage (Swapping):")
	fmt.Println("  git-swap <profile-name>    -> Apply profile to current repo")
}
