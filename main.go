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

	// 1. Try numeric swap first
	if idx, err := strconv.Atoi(command); err == nil {
		swapProfileByIndex(idx, config)
		return
	}

	// 2. Standard command dispatch
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
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	paths := []string{
		filepath.Join(exeDir, fileName),
		filepath.Join(filepath.Dir(exeDir), fileName),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil { return p }
	}
	usr, _ := user.Current()
	return filepath.Join(usr.HomeDir, fileName)
}

func loadConfig() Config {
	configFile, err := os.ReadFile(getConfigPath())
	if err != nil { return make(Config) }
	var config Config
	json.Unmarshal(configFile, &config)
	return config
}

func saveConfig(config Config) {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(getConfigPath(), data, 0644)
}

func expandPath(path string) string {
	if path == "" { return "" }
	re := regexp.MustCompile(`%([^%]+)%`)
	path = re.ReplaceAllStringFunc(path, func(m string) string {
		val := os.Getenv(strings.Trim(m, "%"))
		if val != "" { return val }
		return m
	})
	path = os.ExpandEnv(path)
	if strings.HasPrefix(path, "~") {
		usr, _ := user.Current()
		path = filepath.Join(usr.HomeDir, path[1:])
	}
	abs, _ := filepath.Abs(path)
	return filepath.ToSlash(abs)
}

func addProfile(key string, config Config) {
	if reservedCommands[strings.ToLower(key)] {
		fmt.Printf("%sError: Reserved command name.%s\n", ColorRed, ColorReset); os.Exit(1)
	}
	// Forbidden purely numeric names
	if _, err := strconv.Atoi(key); err == nil {
		fmt.Printf("%sError: Profile name cannot be a number.%s\n", ColorRed, ColorReset); os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Enter Name: "); name, _ := reader.ReadString('\n')
	fmt.Printf("Enter Email: "); email, _ := reader.ReadString('\n')
	fmt.Printf("Enter SSH Key Path (Optional): "); sshKey, _ := reader.ReadString('\n')
	fmt.Printf("Enter Signing Key (Optional): "); signingKey, _ := reader.ReadString('\n')

	config[key] = Profile{
		Name:       strings.TrimSpace(name),
		Email:      strings.TrimSpace(email),
		SSHKey:     strings.TrimSpace(sshKey),
		SigningKey: strings.TrimSpace(signingKey),
	}
	saveConfig(config); fmt.Printf("%s✅ Added!%s\n", ColorGreen, ColorReset)
}

func editProfile(key string, config Config) {
	p, ok := config[key]
	if !ok { fmt.Printf("%sProfile '%s' not found.%s\n", ColorRed, key, ColorReset); os.Exit(1) }
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Name [%s]: ", p.Name); n, _ := reader.ReadString('\n'); n = strings.TrimSpace(n)
	if n != "" { p.Name = n }
	fmt.Printf("Email [%s]: ", p.Email); e, _ := reader.ReadString('\n'); e = strings.TrimSpace(e)
	if e != "" { p.Email = e }
	fmt.Printf("SSH Key [%s]: ", p.SSHKey); s, _ := reader.ReadString('\n'); s = strings.TrimSpace(s)
	if s != "" { p.SSHKey = s }
	fmt.Printf("Signing Key [%s]: ", p.SigningKey); k, _ := reader.ReadString('\n'); k = strings.TrimSpace(k)
	if k != "" { p.SigningKey = k }
	config[key] = p; saveConfig(config); fmt.Printf("%s✅ Updated!%s\n", ColorGreen, ColorReset)
}

func removeProfile(key string, config Config) {
	if _, ok := config[key]; !ok { fmt.Printf("%sNot found.%s\n", ColorRed, ColorReset); os.Exit(1) }
	delete(config, key); saveConfig(config); fmt.Printf("%s✅ Removed!%s\n", ColorGreen, ColorReset)
}

func getSortedKeys(config Config) []string {
	keys := make([]string, 0, len(config))
	for k := range config { keys = append(keys, k) }
	sort.Strings(keys)
	return keys
}

func listProfiles(config Config) {
	if len(config) == 0 { fmt.Println("No profiles. Use 'add' to create one."); return }
	fmt.Println("Available Identities:")
	keys := getSortedKeys(config)
	for i, k := range keys {
		fmt.Printf(" %d. %s%s%s (%s)\n", i+1, ColorCyan, k, ColorReset, config[k].Email)
	}
}

func swapProfileByIndex(idx int, config Config) {
	keys := getSortedKeys(config)
	if idx < 1 || idx > len(keys) {
		fmt.Printf("%sError: Index %d is out of range (1-%d).%s\n", ColorRed, idx, len(keys), ColorReset)
		os.Exit(1)
	}
	swapProfile(keys[idx-1], config)
}

func autoDetectProfile(config Config) {
	if _, err := os.Stat(".git"); os.IsNotExist(err) { fmt.Printf("%sNot a git repo.%s\n", ColorRed, ColorReset); os.Exit(1) }
	k, s := detectByURL(config, true)
	if k == "" { k, s = detectByHistory(config) }
	if k == "" { k, s = detectByURL(config, false) }
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
	re := regexp.MustCompile(`[:/]([\w\.-]+)/[\w\.-]+(?:\.git)?\s+\((push|fetch)\)`)
	matches := re.FindAllStringSubmatch(string(out), -1)
	for _, m := range matches {
		if len(m) > 2 {
			if preferPush != (m[2] == "push") { continue }
			for key, p := range config {
				if strings.EqualFold(key, m[1]) || strings.EqualFold(strings.Split(p.Email, "@")[0], m[1]) || strings.EqualFold(p.Name, m[1]) {
					return key, "remote " + m[2] + " URL"
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
	if !ok { fmt.Printf("%sProfile '%s' not found.%s\n", ColorRed, profileName, ColorReset); os.Exit(1) }
	setGitConfig("user.name", p.Name)
	setGitConfig("user.email", p.Email)
	if p.SSHKey != "" {
		clean := filepath.ToSlash(expandPath(p.SSHKey))
		sshCmd := fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes -F /dev/null", clean)
		setGitConfig("core.sshCommand", sshCmd)
		fmt.Printf("🔑 SSH Key: %s\n", clean)
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

func setGitConfig(key, value string) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		if err := exec.Command("git", "config", "--local", key, value).Run(); err == nil { return nil } else { lastErr = err }
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}

func unsetGitConfig(key string) { exec.Command("git", "config", "--local", "--unset", key).Run() }

func printUsage() {
	fmt.Println("git-swap: Manage git identities.\nUsage: list, status, auto, add, edit, remove, <name/index>")
}
