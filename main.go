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
	SSHKey     string `json:"ssh_key"`
	SigningKey string `json:"signing_key"`
}

type Config map[string]Profile

var reservedCommands = map[string]bool{
	"list": true, "status": true, "add": true, "edit": true, "remove": true, "rm": true, "auto": true, "help": true, "_complete": true,
}

func main() {
	if len(os.Args) < 2 { printUsage(); os.Exit(1) }
	command := os.Args[1]
	config := loadConfig()
	switch command {
	case "list": listProfiles(config)
	case "status": showStatus(config)
	case "add":
		if len(os.Args) < 3 { fmt.Println("Usage: git-swap add <name>"); os.Exit(1) }
		addProfile(os.Args[2], config)
	case "edit":
		if len(os.Args) < 3 { fmt.Println("Usage: git-swap edit <name>"); os.Exit(1) }
		editProfile(os.Args[2], config)
	case "remove", "rm":
		if len(os.Args) < 3 { fmt.Println("Usage: git-swap remove <name>"); os.Exit(1) }
		removeProfile(os.Args[2], config)
	case "auto": autoDetectProfile(config)
	case "_complete":
		for k := range config { fmt.Println(k) }
	case "help": printUsage()
	default: swapProfile(command, config)
	}
}

func getConfigPath() string {
	fileName := ".git-swap-config.json"
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		localPath := filepath.Join(exeDir, fileName)
		if _, err := os.Stat(localPath); err == nil { return localPath }
		parentPath := filepath.Join(filepath.Dir(exeDir), fileName)
		if _, err := os.Stat(parentPath); err == nil { return parentPath }
	}
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, fileName)
}

func loadConfig() Config {
	configPath := getConfigPath()
	configFile, err := os.ReadFile(configPath)
	if os.IsNotExist(err) { return make(Config) }
	var config Config
	json.Unmarshal(configFile, &config)
	return config
}

func saveConfig(config Config) {
	configPath := getConfigPath()
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func expandPath(path string) string {
	if path == "" { return "" }
	re := regexp.MustCompile(`%([^%]+)%`)
	path = re.ReplaceAllStringFunc(path, func(m string) string {
		parts := re.FindStringSubmatch(m)
		if len(parts) > 1 {
			val := os.Getenv(parts[1])
			if val != "" { return val }
		}
		return m
	})
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = filepath.Join(usr.HomeDir, path[1:])
	}
	absPath, err := filepath.Abs(path)
	if err == nil { return absPath }
	return filepath.Clean(path)
}

func addProfile(key string, config Config) {
	if reservedCommands[strings.ToLower(key)] {
		fmt.Printf("%sError: Reserved command.%s\n", ColorRed, ColorReset); os.Exit(1)
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter Name: "); n, _ := reader.ReadString('\n')
	fmt.Printf("Enter Email: "); e, _ := reader.ReadString('\n')
	fmt.Printf("Enter SSH Key Path (Optional): "); s, _ := reader.ReadString('\n')
	fmt.Printf("Enter Signing Key (Optional): "); k, _ := reader.ReadString('\n')
	config[key] = Profile{strings.TrimSpace(n), strings.TrimSpace(e), strings.TrimSpace(s), strings.TrimSpace(k)}
	saveConfig(config); fmt.Printf("%s✅ Added! (%s)%s\n", ColorGreen, getConfigPath(), ColorReset)
}

func editProfile(key string, config Config) {
	p, ok := config[key]
	if !ok { fmt.Printf("%sNot found.%s\n", ColorRed, ColorReset); os.Exit(1) }
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter Name [%s]: ", p.Name); n, _ := reader.ReadString('\n'); n = strings.TrimSpace(n)
	if n != "" { p.Name = n }
	fmt.Printf("Enter Email [%s]: ", p.Email); e, _ := reader.ReadString('\n'); e = strings.TrimSpace(e)
	if e != "" { p.Email = e }
	fmt.Printf("Enter SSH Key Path [%s]: ", p.SSHKey); s, _ := reader.ReadString('\n'); s = strings.TrimSpace(s)
	if s != "" { p.SSHKey = s }
	fmt.Printf("Enter Signing Key [%s]: ", p.SigningKey); k, _ := reader.ReadString('\n'); k = strings.TrimSpace(k)
	if k != "" { p.SigningKey = k }
	config[key] = p; saveConfig(config); fmt.Printf("%s✅ Updated!%s\n", ColorGreen, ColorReset)
}

func removeProfile(key string, config Config) {
	delete(config, key); saveConfig(config); fmt.Printf("%s✅ Removed!%s\n", ColorGreen, ColorReset)
}

func listProfiles(config Config) {
	keys := make([]string, 0, len(config))
	for k := range config { keys = append(keys, k) }
	sort.Strings(keys)
	for _, k := range keys { fmt.Printf(" 🔄 %s%s%s (%s)\n", ColorCyan, k, ColorReset, config[k].Email) }
}

func autoDetectProfile(config Config) {
	if _, err := os.Stat(".git"); os.IsNotExist(err) { fmt.Printf("%sNot a repo.%s\n", ColorRed, ColorReset); os.Exit(1) }
	
	// Priority 1: Remote PUSH URL (Strongest signal)
	k, s := detectByURL(config, true)
	
	// Priority 2: History (Habit signal)
	if k == "" {
		k, s = detectByHistory(config)
	}
	
	// Priority 3: Remote FETCH URL (Fallback)
	if k == "" {
		k, s = detectByURL(config, false)
	}

	if k != "" {
		fmt.Printf("🔍 Detected via %s: %s%s%s\n", s, ColorCyan, k, ColorReset)
		swapProfile(k, config)
	} else {
		fmt.Printf("%s⚠️  No match found.%s\n", ColorYellow, ColorReset)
	}
}

func detectByHistory(config Config) (string, string) {
	out, err := exec.Command("git", "log", "-n", "100", "--format=%ae").Output()
	if err != nil { return "", "" }
	emails := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, e := range emails {
		for key, p := range config { if strings.EqualFold(p.Email, e) { return key, "history" } }
	}
	return "", ""
}

func detectByURL(config Config, preferPush bool) (string, string) {
	out, _ := exec.Command("git", "remote", "-v").Output()
	lines := strings.Split(string(out), "\n")
	
	re := regexp.MustCompile(`[:/]([\w\.-]+)/[\w\.-]+(?:\.git)?\s+\((push|fetch)\)`)
	
	sourceLabel := "remote URL"
	if preferPush { sourceLabel = "remote PUSH URL" }

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 2 {
			isPush := matches[2] == "push"
			if preferPush != isPush { continue }
			
			user := matches[1]
			for key, p := range config {
				if strings.EqualFold(key, user) || strings.EqualFold(strings.Split(p.Email, "@")[0], user) || strings.EqualFold(p.Name, user) {
					return key, sourceLabel
				}
			}
		}
	}
	return "", ""
}

func showStatus(config Config) {
	n, _ := exec.Command("git", "config", "user.name").Output()
	e, _ := exec.Command("git", "config", "user.email").Output()
	cn, ce := strings.TrimSpace(string(n)), strings.TrimSpace(string(e))
	fmt.Printf("Current: %s <%s>\n", cn, ce)
	for k, p := range config { if p.Name == cn && p.Email == ce { fmt.Printf("✅ Match: %s\n", k); return } }
	fmt.Printf("%s⚠️  No match found.%s\n", ColorYellow, ColorReset)
}

func swapProfile(profileName string, config Config) {
	p, ok := config[profileName]
	if !ok { fmt.Printf("%sNot found.%s\n", ColorRed, ColorReset); os.Exit(1) }
	setGitConfig("user.name", p.Name)
	setGitConfig("user.email", p.Email)
	if p.SSHKey != "" {
		expanded := expandPath(p.SSHKey)
		clean := filepath.ToSlash(expanded)
		sshCmd := fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes -F /dev/null", clean)
		setGitConfig("core.sshCommand", sshCmd)
		fmt.Printf("🔑 SSH Key (Expanded): %s\n", clean)
	} else {
		unsetGitConfig("core.sshCommand")
	}
	if p.SigningKey != "" {
		setGitConfig("user.signingkey", p.SigningKey); setGitConfig("commit.gpgsign", "true")
		if strings.HasPrefix(p.SigningKey, "ssh-") { setGitConfig("gpg.format", "ssh") } else { unsetGitConfig("gpg.format") }
	} else {
		unsetGitConfig("user.signingkey"); setGitConfig("commit.gpgsign", "false"); unsetGitConfig("gpg.format")
	}
	fmt.Printf("%s✅ Swapped to: %s%s\n", ColorGreen, profileName, ColorReset)
}

func setGitConfig(key, value string) { exec.Command("git", "config", "--local", key, value).Run() }
func unsetGitConfig(key string) { exec.Command("git", "config", "--local", "--unset", key).Run() }

func printUsage() {
	fmt.Println("git-swap: Manage git identities directly from CLI.")
	fmt.Println("\nUsage:\n  list, status, auto, add, edit, remove, <profile-name>")
}
