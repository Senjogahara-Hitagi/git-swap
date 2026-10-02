package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	providerGitHub = "github"
	providerGitee  = "gitee"
)

type ProviderAccount struct {
	Username  string   `json:"username"`
	SSHKey    string   `json:"ssh_key,omitempty"`
	HostAlias string   `json:"host_alias,omitempty"`
	Owners    []string `json:"owners,omitempty"`
}

type providerSpec struct {
	Name     string
	Host     string
	SSHHost  string
	SSHUser  string
	GHClient bool
}

var supportedProviders = map[string]providerSpec{
	providerGitHub: {
		Name:     "GitHub",
		Host:     "github.com",
		SSHHost:  "github.com",
		SSHUser:  "git",
		GHClient: true,
	},
	providerGitee: {
		Name:    "Gitee",
		Host:    "gitee.com",
		SSHHost: "gitee.com",
		SSHUser: "git",
	},
}

func canonicalProviderName(raw string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(raw))
	_, ok := supportedProviders[name]
	return name, ok
}

func normalizedProviderAccounts(profileName string, p Profile) map[string]ProviderAccount {
	accounts := make(map[string]ProviderAccount, len(p.Providers)+1)
	for provider, account := range p.Providers {
		name, ok := canonicalProviderName(provider)
		if !ok {
			continue
		}
		account.Username = strings.TrimSpace(account.Username)
		account.SSHKey = strings.TrimSpace(account.SSHKey)
		account.HostAlias = strings.TrimSpace(account.HostAlias)
		account.Owners = normalizeOwners(account.Owners, account.Username)
		accounts[name] = account
	}

	// Backward compatibility: old profiles represented one GitHub account with
	// ssh_key/github_user at the profile root.
	if _, exists := accounts[providerGitHub]; !exists &&
		(strings.TrimSpace(p.SSHKey) != "" || strings.TrimSpace(p.GitHubUser) != "") {
		username := strings.TrimSpace(p.GitHubUser)
		if username == "" {
			username = strings.TrimSpace(profileName)
		}
		accounts[providerGitHub] = ProviderAccount{
			Username: username,
			SSHKey:   strings.TrimSpace(p.SSHKey),
			Owners:   normalizeOwners(nil, username),
		}
	}
	return accounts
}

func normalizeOwners(owners []string, username string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(owners)+1)
	for _, owner := range append([]string{username}, owners...) {
		owner = strings.TrimSpace(owner)
		lower := strings.ToLower(owner)
		if owner == "" || seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, owner)
	}
	return result
}

func defaultHostAlias(provider, profileName string) string {
	normalized := strings.ToLower(strings.TrimSpace(profileName))
	normalized = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "account"
	}
	return "git-swap-" + provider + "-" + normalized
}

func accountHostAlias(provider, profileName string, account ProviderAccount) string {
	if alias := strings.TrimSpace(account.HostAlias); alias != "" {
		return alias
	}
	if strings.TrimSpace(account.SSHKey) == "" {
		return supportedProviders[provider].Host
	}
	return defaultHostAlias(provider, profileName)
}

func listProviderAccounts(config Config, profileName string) {
	p, ok := config[profileName]
	if !ok {
		printError("Profile '%s' not found.", profileName)
		return
	}
	accounts := normalizedProviderAccounts(profileName, p)
	if len(accounts) == 0 {
		fmt.Printf("Profile %s has no provider accounts.\n", profileName)
		return
	}
	providers := make([]string, 0, len(accounts))
	for provider := range accounts {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		account := accounts[provider]
		fmt.Printf(
			"%s: user=%s host=%s owners=%s key=%s\n",
			supportedProviders[provider].Name,
			account.Username,
			accountHostAlias(provider, profileName, account),
			strings.Join(account.Owners, ","),
			account.SSHKey,
		)
	}
}

func handleProviderCommand(args []string, config Config) {
	if len(args) < 1 {
		fmt.Println("Usage: git-swap provider <list|set|remove> ...")
		return
	}
	switch strings.ToLower(args[0]) {
	case "list":
		if len(args) != 2 {
			fmt.Println("Usage: git-swap provider list <profile>")
			return
		}
		listProviderAccounts(config, args[1])
	case "set":
		if len(args) != 3 {
			fmt.Println("Usage: git-swap provider set <profile> <github|gitee>")
			return
		}
		setProviderAccount(config, args[1], args[2])
	case "remove":
		if len(args) != 3 {
			fmt.Println("Usage: git-swap provider remove <profile> <github|gitee>")
			return
		}
		removeProviderAccount(config, args[1], args[2])
	default:
		printError("Unknown provider command: %s", args[0])
	}
}

func setProviderAccount(config Config, profileName, rawProvider string) {
	p, ok := config[profileName]
	if !ok {
		printError("Profile '%s' not found.", profileName)
		return
	}
	provider, ok := canonicalProviderName(rawProvider)
	if !ok {
		printError("Unsupported provider '%s'. Supported providers: github, gitee.", rawProvider)
		return
	}
	accounts := normalizedProviderAccounts(profileName, p)
	current := accounts[provider]
	reader := bufio.NewReader(os.Stdin)

	currentAlias := accountHostAlias(provider, profileName, current)
	fmt.Printf("%s Username [%s]: ", supportedProviders[provider].Name, current.Username)
	username, _ := reader.ReadString('\n')
	if value := strings.TrimSpace(username); value != "" {
		current.Username = value
	}
	fmt.Printf("SSH Private Key [%s]: ", current.SSHKey)
	key, _ := reader.ReadString('\n')
	if value := strings.TrimSpace(key); value == "-" {
		current.SSHKey = ""
	} else if value != "" {
		if err := validateSSHKeyPath(value); err != nil {
			printError("%s", err.Error())
			return
		}
		current.SSHKey = value
	}
	fmt.Printf("SSH Host Alias [%s]: ", currentAlias)
	alias, _ := reader.ReadString('\n')
	if value := strings.TrimSpace(alias); value == "-" {
		current.HostAlias = ""
	} else if value != "" {
		current.HostAlias = value
	}
	fmt.Printf("Repository Owners (comma-separated) [%s]: ", strings.Join(current.Owners, ","))
	owners, _ := reader.ReadString('\n')
	if value := strings.TrimSpace(owners); value == "-" {
		current.Owners = nil
	} else if value != "" {
		current.Owners = strings.Split(value, ",")
	}
	if current.Username == "" {
		printError("%s username cannot be empty.", supportedProviders[provider].Name)
		return
	}
	if current.HostAlias != "" && !validSSHHostAlias.MatchString(current.HostAlias) {
		printError("Invalid SSH host alias %q. Use only letters, digits, dots, underscores, and hyphens.", current.HostAlias)
		return
	}
	if current.SSHKey != "" && strings.EqualFold(current.HostAlias, supportedProviders[provider].Host) {
		printError("Managed %s keys require an account-specific host alias, not %s.", supportedProviders[provider].Name, current.HostAlias)
		return
	}
	current.Owners = normalizeOwners(current.Owners, current.Username)
	if p.Providers == nil {
		p.Providers = make(map[string]ProviderAccount)
	}
	p.Providers[provider] = current
	if provider == providerGitHub {
		// The provider map is authoritative; clear migrated legacy values.
		p.SSHKey = ""
		p.GitHubUser = ""
	}
	config[profileName] = p
	saveConfig(config)
	printSuccess("Configured %s account for %s.", supportedProviders[provider].Name, profileName)
}

func removeProviderAccount(config Config, profileName, rawProvider string) {
	p, ok := config[profileName]
	if !ok {
		printError("Profile '%s' not found.", profileName)
		return
	}
	provider, ok := canonicalProviderName(rawProvider)
	if !ok {
		printError("Unsupported provider '%s'.", rawProvider)
		return
	}
	if p.Providers == nil && !(provider == providerGitHub && (p.SSHKey != "" || p.GitHubUser != "")) {
		printWarning("No explicit %s account is configured for %s.", provider, profileName)
		return
	}
	delete(p.Providers, provider)
	if provider == providerGitHub {
		p.SSHKey = ""
		p.GitHubUser = ""
	}
	config[profileName] = p
	saveConfig(config)
	printSuccess("Removed %s account from %s.", supportedProviders[provider].Name, profileName)
}

type sshEndpoint struct {
	Hostname string
	Port     int
	User     string
}

func resolveSSHEndpoint(host string, spec providerSpec) sshEndpoint {
	endpoint := sshEndpoint{Hostname: spec.SSHHost, Port: 22, User: spec.SSHUser}
	out, err := exec.Command("ssh", "-G", host).Output()
	if err != nil {
		return endpoint
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "hostname":
			endpoint.Hostname = fields[1]
		case "port":
			if port, err := strconv.Atoi(fields[1]); err == nil {
				endpoint.Port = port
			}
		}
	}
	return endpoint
}

func sshConfigPath() string {
	usr, _ := os.UserHomeDir()
	return filepath.Join(usr, ".ssh", "config")
}

func syncManagedSSHHost(alias string, account ProviderAccount, spec providerSpec) error {
	return syncManagedSSHHostFile(sshConfigPath(), alias, account, spec)
}

func syncManagedSSHHostFile(path, alias string, account ProviderAccount, spec providerSpec) error {
	if strings.TrimSpace(account.SSHKey) == "" {
		return nil // The alias is externally managed by the user's SSH config.
	}
	if !validSSHHostAlias.MatchString(alias) {
		return fmt.Errorf("invalid SSH host alias %q", alias)
	}
	key := expandPath(account.SSHKey)
	if err := validateSSHKeyPath(key); err != nil {
		return err
	}
	endpoint := resolveSSHEndpoint(spec.Host, spec)
	begin := "# BEGIN git-swap " + alias
	end := "# END git-swap " + alias
	block := strings.Join([]string{
		begin,
		"Host " + alias,
		"  HostName " + endpoint.Hostname,
		"  Port " + strconv.Itoa(endpoint.Port),
		"  User " + endpoint.User,
		"  IdentityFile " + quoteSSHConfigValue(key),
		"  IdentitiesOnly yes",
		end,
	}, "\n")

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := strings.ReplaceAll(string(existing), "\r\n", "\n")
	pattern := regexp.MustCompile(`(?ms)^` + regexp.QuoteMeta(begin) + `\n.*?^` + regexp.QuoteMeta(end) + `\n?`)
	if pattern.MatchString(content) {
		content = pattern.ReplaceAllStringFunc(content, func(string) string { return block + "\n" })
	} else {
		content = strings.TrimLeft(content, "\n")
		if content == "" {
			content = block + "\n"
		} else {
			content = block + "\n" + content
		}
	}
	return writeFileWithBackup(path, []byte(content), 0600)
}

func quoteSSHConfigValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func writeFileWithBackup(path string, content []byte, mode os.FileMode) error {
	existed := false
	if old, err := os.ReadFile(path); err == nil {
		existed = true
		if string(old) == string(content) {
			return nil
		}
		if err := os.WriteFile(path+".git-swap.bak", old, mode); err != nil {
			return err
		}
	}
	if existed {
		// Replacing an existing OpenSSH config via rename can inherit the temp
		// file's ACL on Windows. OpenSSH then rejects the config as too broad.
		// Writing the existing file in place preserves its owner and ACL; the
		// backup above keeps the operation recoverable.
		return os.WriteFile(path, content, mode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".git-swap-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// os.Rename cannot replace an existing file on Windows. The backup above
	// makes this replacement recoverable.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		if backup, readErr := os.ReadFile(path + ".git-swap.bak"); readErr == nil {
			_ = os.WriteFile(path, backup, mode)
		}
		return err
	}
	if err := restrictNewWindowsSSHConfig(path); err != nil {
		return err
	}
	return nil
}

func restrictNewWindowsSSHConfig(path string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	args := []string{
		path,
		"/inheritance:r",
		"/grant:r",
		current.Username + ":(F)",
		"*S-1-5-18:(F)",     // Local System
		"*S-1-5-32-544:(F)", // Administrators, language-independent SID
	}
	if output, err := exec.Command("icacls", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("secure new SSH config ACL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

type parsedGitURL struct {
	Scheme string
	User   string
	Host   string
	Path   string
}

var scpLikeURL = regexp.MustCompile(`^(?:([^@/:]+)@)?([^/:]+):(.+)$`)
var validSSHHostAlias = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func parseGitURL(raw string) (parsedGitURL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedGitURL{}, false
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return parsedGitURL{}, false
		}
		user := ""
		if u.User != nil {
			user = u.User.Username()
		}
		return parsedGitURL{
			Scheme: strings.ToLower(u.Scheme),
			User:   user,
			Host:   u.Hostname(),
			Path:   strings.TrimPrefix(u.Path, "/"),
		}, true
	}
	m := scpLikeURL.FindStringSubmatch(raw)
	if len(m) != 4 || strings.Contains(m[2], `\`) || (len(m[2]) == 1 && strings.HasPrefix(m[3], `\`)) {
		return parsedGitURL{}, false
	}
	return parsedGitURL{Scheme: "ssh", User: m[1], Host: m[2], Path: strings.TrimPrefix(m[3], "/")}, true
}

func gitURLProvider(parsed parsedGitURL, config Config) (string, bool) {
	host := strings.ToLower(parsed.Host)
	for provider, spec := range supportedProviders {
		if host == spec.Host || host == spec.SSHHost ||
			(provider == providerGitHub && host == "ssh.github.com") {
			return provider, true
		}
	}
	for profileName, profile := range config {
		for provider, account := range normalizedProviderAccounts(profileName, profile) {
			if strings.EqualFold(host, accountHostAlias(provider, profileName, account)) {
				return provider, true
			}
		}
	}
	endpoint := resolveSSHEndpoint(parsed.Host, providerSpec{SSHHost: parsed.Host, SSHUser: "git"})
	for provider, spec := range supportedProviders {
		if strings.EqualFold(endpoint.Hostname, spec.Host) ||
			(provider == providerGitHub && strings.EqualFold(endpoint.Hostname, "ssh.github.com")) {
			return provider, true
		}
	}
	return "", false
}

func gitURLOwner(parsed parsedGitURL) string {
	path := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if path == "" {
		return ""
	}
	return strings.Split(path, "/")[0]
}

func formatSSHGitURL(alias string, parsed parsedGitURL) string {
	user := parsed.User
	if user == "" {
		user = "git"
	}
	return user + "@" + alias + ":" + strings.TrimPrefix(parsed.Path, "/")
}

type remoteURLSetting struct {
	Key   string
	Value string
}

func repositoryRemoteURLs() []remoteURLSetting {
	out, err := exec.Command("git", "config", "--local", "--get-regexp", `^remote\..*\.(url|pushurl)$`).Output()
	if err != nil {
		return nil
	}
	var settings []remoteURLSetting
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " ")
		if ok && key != "" && strings.TrimSpace(value) != "" {
			settings = append(settings, remoteURLSetting{Key: key, Value: strings.TrimSpace(value)})
		}
	}
	return settings
}

func repositoryRemoteProviders(config Config) map[string]bool {
	present := make(map[string]bool)
	for _, setting := range repositoryRemoteURLs() {
		parsed, ok := parseGitURL(setting.Value)
		if !ok {
			continue
		}
		if provider, ok := gitURLProvider(parsed, config); ok {
			present[provider] = true
		}
	}
	return present
}

func providerBindingKey(provider string) string {
	return "git-swap.provider." + provider
}

func repositoryProviderBindings(config Config) map[string]string {
	bindings := make(map[string]string)
	for provider := range supportedProviders {
		out, _ := exec.Command("git", "config", "--local", "--get", providerBindingKey(provider)).Output()
		profileName := strings.TrimSpace(string(out))
		profile, ok := config[profileName]
		if !ok {
			continue
		}
		if _, ok := normalizedProviderAccounts(profileName, profile)[provider]; ok {
			bindings[provider] = profileName
		}
	}
	return bindings
}

func profileProviderBindings(profileName string, profile Profile) map[string]string {
	bindings := make(map[string]string)
	for provider := range normalizedProviderAccounts(profileName, profile) {
		bindings[provider] = profileName
	}
	return bindings
}

func setRepositoryProviderBindings(bindings map[string]string) error {
	for provider, profileName := range bindings {
		if err := setGitConfig(providerBindingKey(provider), profileName); err != nil {
			return fmt.Errorf("bind %s provider to %s: %w", provider, profileName, err)
		}
	}
	return nil
}

func rewriteRepositorySSHRemotes(bindings map[string]string, config Config) error {
	for _, setting := range repositoryRemoteURLs() {
		parsed, ok := parseGitURL(setting.Value)
		if !ok || parsed.Scheme != "ssh" {
			continue
		}
		provider, ok := gitURLProvider(parsed, config)
		if !ok {
			continue
		}
		profileName, bound := bindings[provider]
		profile, profileExists := config[profileName]
		if !bound || !profileExists {
			continue
		}
		account, exists := normalizedProviderAccounts(profileName, profile)[provider]
		if !exists {
			continue
		}
		newURL := formatSSHGitURL(accountHostAlias(provider, profileName, account), parsed)
		if newURL == setting.Value {
			continue
		}
		args := []string{"config", "--local", "--replace-all", setting.Key, newURL, "^" + regexp.QuoteMeta(setting.Value) + "$"}
		if err := exec.Command("git", args...).Run(); err != nil {
			return fmt.Errorf("rewrite %s: %w", setting.Key, err)
		}
		fmt.Printf("🔗 %s: %s -> %s\n", setting.Key, setting.Value, newURL)
	}
	return nil
}

func configureProviderAuthentication(bindings map[string]string, config Config) error {
	if len(bindings) == 0 {
		return nil
	}
	if err := validateProviderAliases(config); err != nil {
		return err
	}
	for provider, profileName := range bindings {
		profile, ok := config[profileName]
		if !ok {
			return fmt.Errorf("%s provider references missing profile %q", provider, profileName)
		}
		account, ok := normalizedProviderAccounts(profileName, profile)[provider]
		if !ok {
			return fmt.Errorf("profile %q has no %s account", profileName, provider)
		}
		if err := syncManagedSSHHost(accountHostAlias(provider, profileName, account), account, supportedProviders[provider]); err != nil {
			return fmt.Errorf("configure %s SSH account: %w", provider, err)
		}
	}
	// A repository-wide sshCommand cannot select a different key per remote.
	unsetGitConfig("core.sshCommand")
	if isGitRepo() {
		if err := rewriteRepositorySSHRemotes(bindings, config); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderAliases(config Config) error {
	owners := make(map[string]string)
	for profileName, profile := range config {
		for provider, account := range normalizedProviderAccounts(profileName, profile) {
			accountSpecific := strings.TrimSpace(account.HostAlias) != "" || strings.TrimSpace(account.SSHKey) != ""
			if !accountSpecific {
				continue
			}
			alias := strings.ToLower(accountHostAlias(provider, profileName, account))
			label := profileName + "/" + provider
			if existing, found := owners[alias]; found && existing != label {
				return fmt.Errorf("SSH host alias %q is shared by %s and %s", alias, existing, label)
			}
			owners[alias] = label
		}
	}
	return nil
}

func profileMatchesRemote(profileName string, p Profile, provider string, parsed parsedGitURL) bool {
	account, ok := normalizedProviderAccounts(profileName, p)[provider]
	if !ok {
		return false
	}
	accountSpecificAlias := strings.TrimSpace(account.HostAlias) != "" || strings.TrimSpace(account.SSHKey) != ""
	if accountSpecificAlias && strings.EqualFold(parsed.Host, accountHostAlias(provider, profileName, account)) {
		return true
	}
	owner := gitURLOwner(parsed)
	for _, accepted := range normalizeOwners(account.Owners, account.Username) {
		if strings.EqualFold(owner, accepted) {
			return true
		}
	}
	return false
}

type providerDetection struct {
	Bindings  map[string]string
	Evidence  map[string][]string
	Conflicts map[string][]string
}

func detectProviderBindings(config Config) providerDetection {
	providerMatches := make(map[string]map[string][]string)
	for _, setting := range repositoryRemoteURLs() {
		parsed, ok := parseGitURL(setting.Value)
		if !ok {
			continue
		}
		provider, ok := gitURLProvider(parsed, config)
		if !ok {
			continue
		}
		if providerMatches[provider] == nil {
			providerMatches[provider] = make(map[string][]string)
		}
		for profileName, profile := range config {
			if profileMatchesRemote(profileName, profile, provider, parsed) {
				providerMatches[provider][profileName] = append(providerMatches[provider][profileName], setting.Key+"="+setting.Value)
			}
		}
	}
	result := providerDetection{
		Bindings:  make(map[string]string),
		Evidence:  make(map[string][]string),
		Conflicts: make(map[string][]string),
	}
	for provider, matches := range providerMatches {
		profiles := make([]string, 0, len(matches))
		for profileName := range matches {
			profiles = append(profiles, profileName)
		}
		sort.Strings(profiles)
		if len(profiles) == 1 {
			result.Bindings[provider] = profiles[0]
			result.Evidence[provider] = matches[profiles[0]]
		} else if len(profiles) > 1 {
			result.Conflicts[provider] = profiles
		}
	}
	return result
}

func doctor(config Config) bool {
	if !isGitRepo() {
		printError("Not a git repository.")
		return false
	}
	healthy := true
	markerOut, _ := exec.Command("git", "config", "--local", "--get", "git-swap.profile").Output()
	marker := strings.TrimSpace(string(markerOut))
	selected, selectedOK := config[marker]
	if marker == "" {
		printWarning("Repository is not explicitly bound. Run git-swap <profile>.")
	} else if !selectedOK {
		printError("Repository is bound to missing profile '%s'.", marker)
		healthy = false
	} else {
		printSuccess("Repository profile: %s", marker)
		nameOut, _ := exec.Command("git", "config", "user.name").Output()
		emailOut, _ := exec.Command("git", "config", "user.email").Output()
		if strings.TrimSpace(string(nameOut)) != selected.Name || !strings.EqualFold(strings.TrimSpace(string(emailOut)), selected.Email) {
			printWarning("Effective commit identity does not match bound profile %s.", marker)
			healthy = false
		} else {
			printSuccess("Commit identity matches the bound profile.")
		}
	}
	bindings := repositoryProviderBindings(config)
	presentProviders := repositoryRemoteProviders(config)
	for _, provider := range []string{providerGitHub, providerGitee} {
		profileName, ok := bindings[provider]
		if !ok {
			if presentProviders[provider] {
				printWarning("%s remote exists, but no account is bound for this repository.", supportedProviders[provider].Name)
				healthy = false
			}
			continue
		}
		profile := config[profileName]
		account := normalizedProviderAccounts(profileName, profile)[provider]
		alias := accountHostAlias(provider, profileName, account)
		if account.SSHKey != "" {
			if err := validateSSHKeyPath(account.SSHKey); err != nil {
				printWarning("%s key: %v", supportedProviders[provider].Name, err)
				healthy = false
				continue
			}
		}
		endpoint := resolveSSHEndpoint(alias, supportedProviders[provider])
		expectedHost := supportedProviders[provider].Host
		validEndpoint := strings.EqualFold(endpoint.Hostname, expectedHost) ||
			(provider == providerGitHub && strings.EqualFold(endpoint.Hostname, "ssh.github.com"))
		if alias != expectedHost && !validEndpoint {
			printWarning("%s alias %s resolves to %s, not %s.", supportedProviders[provider].Name, alias, endpoint.Hostname, expectedHost)
			healthy = false
		} else {
			printSuccess("%s binding: %s (%s) via %s.", supportedProviders[provider].Name, profileName, account.Username, alias)
		}
	}
	sshOut, _ := exec.Command("git", "config", "--local", "--get", "core.sshCommand").Output()
	if value := strings.TrimSpace(string(sshOut)); value != "" {
		printWarning("core.sshCommand is repository-wide and conflicts with independent provider accounts: %s", value)
		healthy = false
	} else {
		printSuccess("No repository-wide core.sshCommand override.")
	}
	for _, setting := range repositoryRemoteURLs() {
		parsed, ok := parseGitURL(setting.Value)
		if !ok {
			printWarning("Cannot parse %s: %s", setting.Key, setting.Value)
			healthy = false
			continue
		}
		provider, ok := gitURLProvider(parsed, config)
		if !ok {
			fmt.Printf("• %s: unmanaged host %s\n", setting.Key, parsed.Host)
			continue
		}
		fmt.Printf("• %s: %s owner=%s host=%s\n", setting.Key, supportedProviders[provider].Name, gitURLOwner(parsed), parsed.Host)
		boundProfileName, hasBinding := bindings[provider]
		boundProfile, boundProfileExists := config[boundProfileName]
		if !hasBinding || !boundProfileExists {
			printWarning("%s has no valid %s account binding.", setting.Key, supportedProviders[provider].Name)
			healthy = false
			continue
		}
		if hasBinding && boundProfileExists && !profileMatchesRemote(boundProfileName, boundProfile, provider, parsed) {
			matchingProfiles := make([]string, 0)
			for profileName, profile := range config {
				if profileMatchesRemote(profileName, profile, provider, parsed) {
					matchingProfiles = append(matchingProfiles, profileName)
				}
			}
			sort.Strings(matchingProfiles)
			if len(matchingProfiles) > 0 {
				printWarning("%s matches profile(s) %s instead of %s binding %s.", setting.Key, strings.Join(matchingProfiles, ", "), provider, boundProfileName)
				healthy = false
			}
		}
	}
	return healthy
}
