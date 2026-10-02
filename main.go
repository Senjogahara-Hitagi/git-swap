package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
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
	Name       string                     `json:"name"`
	Email      string                     `json:"email"`
	SigningKey string                     `json:"signing_key"`
	Providers  map[string]ProviderAccount `json:"providers,omitempty"`

	// Legacy fields are read for backward compatibility and migrated to the
	// GitHub provider model when a profile is next edited.
	SSHKey     string `json:"ssh_key,omitempty"`
	GitHubUser string `json:"github_user,omitempty"`
}

type Config map[string]Profile

var reservedCommands = map[string]bool{
	"list": true, "status": true, "add": true, "edit": true, "remove": true, "rm": true,
	"auto": true, "help": true, "_complete": true, "setup-hook": true, "remove-hook": true, "convert-ssh": true, "current": true,
	"provider": true, "doctor": true,
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
	case "provider":
		handleProviderCommand(os.Args[2:], config)
	case "doctor":
		if !doctor(config) {
			os.Exit(1)
		}
	case "setup-hook":
		setupGitHook()
	case "remove-hook":
		removeGitHook()
	case "convert-ssh":
		convertSSH(config)
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
	fmt.Println("  git-swap provider list <profile>")
	fmt.Println("  git-swap provider set <profile> <github|gitee>")
	fmt.Println("  git-swap provider remove <profile> <github|gitee>")
	fmt.Println("  git-swap doctor                - Validate repository identity and remote authentication")
	fmt.Println("  git-swap setup-hook            - Install 'auto' pre-commit hook in current repo")
	fmt.Println("  git-swap remove-hook           - Remove 'auto' pre-commit hook from current repo")
	fmt.Println("  git-swap convert-ssh           - Convert HTTPS GitHub/Gitee remotes to SSH format")
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

func validateSSHKeyPath(path string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	clean := expandPath(trimmed)
	if strings.HasSuffix(strings.ToLower(clean), ".pub") {
		return fmt.Errorf("SSH key path points to a public key: %s. Use the private key file instead", clean)
	}
	if _, err := os.Stat(clean); err != nil {
		return fmt.Errorf("SSH key file not found: %s", clean)
	}
	return nil
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

	fmt.Print("Enter Signing Key: ")
	k, _ := reader.ReadString('\n')
	fmt.Print("Enter GitHub Username (optional): ")
	g, _ := reader.ReadString('\n')
	fmt.Print("Enter GitHub SSH Private Key (optional): ")
	gs, _ := reader.ReadString('\n')
	fmt.Print("Enter Gitee Username (optional): ")
	ge, _ := reader.ReadString('\n')
	fmt.Print("Enter Gitee SSH Private Key (optional): ")
	ges, _ := reader.ReadString('\n')

	providers := make(map[string]ProviderAccount)
	githubUser, githubKey := strings.TrimSpace(g), strings.TrimSpace(gs)
	if githubUser != "" || githubKey != "" {
		if githubUser == "" {
			githubUser = key
		}
		if err := validateSSHKeyPath(githubKey); err != nil {
			printError("%s", err.Error())
			os.Exit(1)
		}
		providers[providerGitHub] = ProviderAccount{Username: githubUser, SSHKey: githubKey, Owners: []string{githubUser}}
	}
	giteeUser, giteeKey := strings.TrimSpace(ge), strings.TrimSpace(ges)
	if giteeUser != "" || giteeKey != "" {
		if giteeUser == "" {
			giteeUser = key
		}
		if err := validateSSHKeyPath(giteeKey); err != nil {
			printError("%s", err.Error())
			os.Exit(1)
		}
		providers[providerGitee] = ProviderAccount{Username: giteeUser, SSHKey: giteeKey, Owners: []string{giteeUser}}
	}

	config[key] = Profile{
		Name:       strings.TrimSpace(n),
		Email:      eTrimmed,
		SigningKey: strings.TrimSpace(k),
		Providers:  providers,
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

	fmt.Printf("Signing Key [%s]: ", p.SigningKey)
	if k, _ := reader.ReadString('\n'); strings.TrimSpace(k) != "" {
		p.SigningKey = strings.TrimSpace(k)
	}

	accounts := normalizedProviderAccounts(key, p)
	if len(accounts) > 0 {
		p.Providers = accounts
		p.SSHKey = ""
		p.GitHubUser = ""
	}

	config[key] = p
	saveConfig(config)
	printSuccess("Updated!")
	fmt.Println("Use 'git-swap provider set <profile> <github|gitee>' to edit provider accounts.")
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
	applyCommitProfile(profileName, p)
	if isGitRepo() {
		setGitConfig("git-swap.profile", profileName)
	}
	bindings := profileProviderBindings(profileName, p)
	// An explicit profile chooses the commit author and provides provider
	// defaults. Remote URL evidence remains authoritative per provider, so a
	// GitHub account and a Gitee account may come from different profiles.
	detection := detectProviderBindings(config)
	if len(detection.Conflicts) > 0 {
		for provider, profiles := range detection.Conflicts {
			printError("%s remotes match multiple accounts: %s", supportedProviders[provider].Name, strings.Join(profiles, ", "))
		}
		os.Exit(1)
	}
	for provider, providerProfile := range detection.Bindings {
		bindings[provider] = providerProfile
	}
	for provider := range supportedProviders {
		unsetGitConfig(providerBindingKey(provider))
	}
	if err := setRepositoryProviderBindings(bindings); err != nil {
		printError("Provider binding failed: %v", err)
		os.Exit(1)
	}
	if err := configureProviderAuthentication(bindings, config); err != nil {
		printError("Provider authentication setup failed: %v", err)
		os.Exit(1)
	}

	if githubProfile, ok := bindings[providerGitHub]; ok {
		syncGitHubCLIAccount(githubProfile, config[githubProfile])
	}

	printSuccess("Swapped to: %s", profileName)
}

func applyCommitProfile(profileName string, p Profile) {
	setGitConfig("user.name", p.Name)
	setGitConfig("user.email", p.Email)
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
}

type ghAuthStatus struct {
	Hosts map[string][]ghAccount `json:"hosts"`
}

type ghAccount struct {
	Login  string `json:"login"`
	Active bool   `json:"active"`
	State  string `json:"state"`
}

func getTargetGitHubUser(profileName string, p Profile) string {
	if account, ok := normalizedProviderAccounts(profileName, p)[providerGitHub]; ok {
		return strings.TrimSpace(account.Username)
	}
	return ""
}

func syncGitHubCLIAccount(profileName string, p Profile) {
	targetUser := getTargetGitHubUser(profileName, p)
	if targetUser == "" {
		return
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return
	}

	statusOut, err := exec.Command("gh", "auth", "status", "--json", "hosts").Output()
	if err != nil {
		printWarning("Unable to inspect gh auth status: %v", err)
		return
	}

	var status ghAuthStatus
	if err := json.Unmarshal(statusOut, &status); err != nil {
		printWarning("Unable to parse gh auth status output.")
		return
	}

	accounts := status.Hosts["github.com"]
	if len(accounts) == 0 {
		printWarning("gh has no authenticated accounts for github.com.")
		return
	}

	currentUser := ""
	targetExists := false
	for _, account := range accounts {
		if account.Active {
			currentUser = account.Login
		}
		if strings.EqualFold(account.Login, targetUser) {
			targetExists = true
		}
	}

	if strings.EqualFold(currentUser, targetUser) {
		return
	}

	if !targetExists {
		printWarning("gh current account: %s; target account: %s. Run 'gh auth login' for the target account first.", currentUser, targetUser)
		return
	}

	if err := exec.Command("gh", "auth", "switch", "--hostname", "github.com", "--user", targetUser).Run(); err != nil {
		printWarning("Failed to switch gh account from %s to %s.", currentUser, targetUser)
		return
	}

	printSuccess("gh active account: %s", targetUser)
}

func showStatus(config Config) {
	n, _ := exec.Command("git", "config", "user.name").Output()
	e, _ := exec.Command("git", "config", "user.email").Output()
	cn, ce := strings.TrimSpace(string(n)), strings.TrimSpace(string(e))

	fmt.Printf("Current: %s <%s>\n", cn, ce)
	markerOut, _ := exec.Command("git", "config", "--local", "--get", "git-swap.profile").Output()
	marker := strings.TrimSpace(string(markerOut))
	for k, p := range config {
		if p.Name == cn && p.Email == ce {
			printSuccess("Match: %s", k)
			if marker != "" && marker != k {
				printWarning("Repository marker is %s, but effective identity matches %s.", marker, k)
			}
			for provider, profileName := range repositoryProviderBindings(config) {
				account := normalizedProviderAccounts(profileName, config[profileName])[provider]
				fmt.Printf("  %s binding: %s (%s) via %s\n", supportedProviders[provider].Name, profileName, account.Username, accountHostAlias(provider, profileName, account))
			}
			return
		}
	}
	printWarning("No match found.")
}

func isGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func autoDetectProfile(config Config) {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}

	detection := detectProviderBindings(config)
	if len(detection.Conflicts) > 0 {
		for provider, profiles := range detection.Conflicts {
			printError("%s remotes match multiple accounts: %s", supportedProviders[provider].Name, strings.Join(profiles, ", "))
		}
		os.Exit(1)
	}

	bindings := repositoryProviderBindings(config)
	presentProviders := repositoryRemoteProviders(config)
	for provider := range supportedProviders {
		if !presentProviders[provider] {
			delete(bindings, provider)
			unsetGitConfig(providerBindingKey(provider))
		}
	}
	for provider, profileName := range detection.Bindings {
		bindings[provider] = profileName
		fmt.Printf("🔍 %s remote -> %s%s%s\n", supportedProviders[provider].Name, ColorCyan, profileName, ColorReset)
	}
	if err := setRepositoryProviderBindings(bindings); err != nil {
		printError("Provider binding failed: %v", err)
		os.Exit(1)
	}

	markerOut, _ := exec.Command("git", "config", "--local", "--get", "git-swap.profile").Output()
	commitProfile := strings.TrimSpace(string(markerOut))
	if _, ok := config[commitProfile]; !ok {
		if commitProfile != "" {
			printWarning("Repository binding references missing profile '%s'; continuing detection.", commitProfile)
		}
		commitProfile = ""
	}
	if commitProfile == "" {
		commitProfile = singleProfileBinding(detection.Bindings)
	}
	if commitProfile == "" {
		commitProfile = detectByEffectiveIdentity(config)
	}
	if commitProfile == "" {
		commitProfile, _ = detectByHistory(config)
	}
	if commitProfile != "" {
		applyCommitProfile(commitProfile, config[commitProfile])
		setGitConfig("git-swap.profile", commitProfile)
		fmt.Printf("🔍 Commit identity -> %s%s%s\n", ColorCyan, commitProfile, ColorReset)
	} else {
		printWarning("Provider accounts were resolved independently, but commit identity is ambiguous. Run git-swap <profile> once to bind it.")
	}

	if err := configureProviderAuthentication(bindings, config); err != nil {
		printError("Provider authentication setup failed: %v", err)
		os.Exit(1)
	}
	if githubProfile, ok := bindings[providerGitHub]; ok {
		syncGitHubCLIAccount(githubProfile, config[githubProfile])
	}
}

func singleProfileBinding(bindings map[string]string) string {
	unique := ""
	for _, profileName := range bindings {
		if unique == "" {
			unique = profileName
		} else if unique != profileName {
			return ""
		}
	}
	return unique
}

func detectByEffectiveIdentity(config Config) string {
	nameOut, _ := exec.Command("git", "config", "user.name").Output()
	emailOut, _ := exec.Command("git", "config", "user.email").Output()
	name, email := strings.TrimSpace(string(nameOut)), strings.TrimSpace(string(emailOut))
	match := ""
	for profileName, profile := range config {
		if profile.Name == name && strings.EqualFold(profile.Email, email) {
			if match != "" {
				return ""
			}
			match = profileName
		}
	}
	return match
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

type gitURLConversion struct {
	key    string
	oldURL string
	newURL string
}

func githubHTTPSToSSH(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(u.Hostname(), supportedProviders[providerGitHub].Host) {
		return "", false
	}
	return providerHTTPSToSSH(rawURL, nil, nil)
}

func providerHTTPSToSSH(rawURL string, config Config, bindings map[string]string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme != "https" {
		return "", false
	}
	provider := ""
	for name, spec := range supportedProviders {
		if strings.EqualFold(u.Hostname(), spec.Host) {
			provider = name
			break
		}
	}
	if provider == "" {
		return "", false
	}

	repoPath := strings.Trim(strings.TrimSpace(u.Path), "/")
	if repoPath == "" || strings.Contains(repoPath, " ") {
		return "", false
	}

	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}

	if !strings.HasSuffix(repoPath, ".git") {
		repoPath += ".git"
	}
	host := supportedProviders[provider].Host
	profileName := bindings[provider]
	if profile, ok := config[profileName]; ok {
		if account, exists := normalizedProviderAccounts(profileName, profile)[provider]; exists {
			host = accountHostAlias(provider, profileName, account)
		}
	}
	return "git@" + host + ":" + repoPath, true
}

func gitConfigURLConversions(config Config, bindings map[string]string, configArgs ...string) []gitURLConversion {
	pattern := `^(remote\..*\.url|remote\..*\.pushurl|submodule\..*\.url)$`
	args := append([]string{"config"}, configArgs...)
	args = append(args, "--get-regexp", pattern)

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil
	}

	var conversions []gitURLConversion
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}

		oldURL := strings.TrimSpace(value)
		newURL, converted := providerHTTPSToSSH(oldURL, config, bindings)
		if !converted {
			continue
		}

		seenKey := key + "\x00" + oldURL
		if seen[seenKey] {
			continue
		}
		seen[seenKey] = true

		conversions = append(conversions, gitURLConversion{
			key:    key,
			oldURL: oldURL,
			newURL: newURL,
		})
	}
	return conversions
}

func applyGitConfigURLConversion(c gitURLConversion, configArgs ...string) error {
	args := append([]string{"config"}, configArgs...)
	args = append(args, "--replace-all", c.key, c.newURL, "^"+regexp.QuoteMeta(c.oldURL)+"$")
	return exec.Command("git", args...).Run()
}

func convertSSH(config Config) {
	if !isGitRepo() {
		printError("Not a git repository.")
		os.Exit(1)
	}
	convertedAny := false
	markerOut, _ := exec.Command("git", "config", "--local", "--get", "git-swap.profile").Output()
	profileName := strings.TrimSpace(string(markerOut))
	bindings := repositoryProviderBindings(config)
	if profile, ok := config[profileName]; ok {
		for provider, fallbackProfile := range profileProviderBindings(profileName, profile) {
			if _, bound := bindings[provider]; !bound {
				bindings[provider] = fallbackProfile
			}
		}
	}

	for _, c := range gitConfigURLConversions(config, bindings, "--local") {
		if err := applyGitConfigURLConversion(c, "--local"); err == nil {
			printSuccess("Converted %s: %s -> %s", c.key, c.oldURL, c.newURL)
			convertedAny = true
		} else {
			printError("Failed to convert %s", c.key)
		}
	}

	modulesConverted := false
	if _, err := os.Stat(".gitmodules"); err == nil {
		for _, c := range gitConfigURLConversions(config, bindings, "-f", ".gitmodules") {
			if err := applyGitConfigURLConversion(c, "-f", ".gitmodules"); err == nil {
				printSuccess("Converted .gitmodules %s: %s -> %s", c.key, c.oldURL, c.newURL)
				convertedAny = true
				modulesConverted = true
			} else {
				printError("Failed to convert .gitmodules %s", c.key)
			}
		}

		if modulesConverted {
			if err := exec.Command("git", "submodule", "sync", "--recursive").Run(); err != nil {
				printWarning("Converted .gitmodules, but failed to sync submodule config.")
			}
		}
	}

	if !convertedAny {
		printWarning("No HTTPS GitHub or Gitee remotes found to convert.")
	}
}
