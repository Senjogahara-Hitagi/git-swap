package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizedProviderAccountsMigratesLegacyGitHub(t *testing.T) {
	p := Profile{SSHKey: "~/.ssh/legacy", GitHubUser: "octocat"}
	accounts := normalizedProviderAccounts("profile", p)
	github, ok := accounts[providerGitHub]
	if !ok {
		t.Fatal("legacy GitHub account was not migrated")
	}
	if github.Username != "octocat" || github.SSHKey != "~/.ssh/legacy" {
		t.Fatalf("unexpected migrated account: %#v", github)
	}
	if len(github.Owners) != 1 || github.Owners[0] != "octocat" {
		t.Fatalf("unexpected owners: %#v", github.Owners)
	}
}

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		host  string
		owner string
		ok    bool
	}{
		{name: "scp GitHub", input: "git@github-work:acme/repo.git", host: "github-work", owner: "acme", ok: true},
		{name: "ssh Gitee", input: "ssh://git@gitee.com/team/repo.git", host: "gitee.com", owner: "team", ok: true},
		{name: "https", input: "https://github.com/octocat/repo.git", host: "github.com", owner: "octocat", ok: true},
		{name: "Windows path", input: `C:\\repos\\project`, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, ok := parseGitURL(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok=%v, want %v (%#v)", ok, tt.ok, parsed)
			}
			if !ok {
				return
			}
			if parsed.Host != tt.host || gitURLOwner(parsed) != tt.owner {
				t.Fatalf("parsed=%#v, owner=%q", parsed, gitURLOwner(parsed))
			}
		})
	}
}

func TestProviderHTTPSToSSHUsesBoundAliases(t *testing.T) {
	config := Config{
		"work": {
			Providers: map[string]ProviderAccount{
				providerGitHub: {Username: "octocat", HostAlias: "github-work"},
				providerGitee:  {Username: "octocat-cn", HostAlias: "gitee-work"},
			},
		},
	}
	tests := map[string]string{
		"https://github.com/octocat/repo":       "git@github-work:octocat/repo.git",
		"https://gitee.com/octocat-cn/repo.git": "git@gitee-work:octocat-cn/repo.git",
	}
	for input, want := range tests {
		got, ok := providerHTTPSToSSH(input, config, map[string]string{
			providerGitHub: "work",
			providerGitee:  "work",
		})
		if !ok || got != want {
			t.Fatalf("providerHTTPSToSSH(%q)=(%q,%v), want %q", input, got, ok, want)
		}
	}
}

func TestProfileMatchesRemoteByOwnerAndAlias(t *testing.T) {
	p := Profile{Providers: map[string]ProviderAccount{
		providerGitHub: {Username: "person", HostAlias: "github-work", Owners: []string{"acme"}},
	}}
	byOwner, _ := parseGitURL("git@github.com:acme/repo.git")
	byAlias, _ := parseGitURL("git@github-work:someone/repo.git")
	wrong, _ := parseGitURL("git@github.com:other/repo.git")
	if !profileMatchesRemote("work", p, providerGitHub, byOwner) {
		t.Fatal("expected owner match")
	}
	if !profileMatchesRemote("work", p, providerGitHub, byAlias) {
		t.Fatal("expected alias match")
	}
	if profileMatchesRemote("work", p, providerGitHub, wrong) {
		t.Fatal("unexpected owner match")
	}
}

func TestValidateProviderAliasesRejectsCrossProfileCollision(t *testing.T) {
	config := Config{
		"one": {Providers: map[string]ProviderAccount{
			providerGitee: {Username: "one", HostAlias: "gitee-shared"},
		}},
		"two": {Providers: map[string]ProviderAccount{
			providerGitee: {Username: "two", HostAlias: "gitee-shared"},
		}},
	}
	if err := validateProviderAliases(config); err == nil {
		t.Fatal("expected duplicate alias error")
	}
}

func TestRewriteRepositorySSHRemotesKeepsProviderKeysIndependent(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "remote", "add", "origin", "git@github.com:octocat/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@github.com:octocat/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@gitee.com:octocat-cn/repo.git")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	profile := Profile{Providers: map[string]ProviderAccount{
		providerGitHub: {Username: "octocat", HostAlias: "github-work"},
		providerGitee:  {Username: "octocat-cn", HostAlias: "gitee-work"},
	}}
	config := Config{"work": profile}
	if err := rewriteRepositorySSHRemotes(map[string]string{
		providerGitHub: "work",
		providerGitee:  "work",
	}, config); err != nil {
		t.Fatal(err)
	}

	fetch := strings.TrimSpace(runGitTest(t, repo, "config", "--get", "remote.origin.url"))
	push := strings.Fields(runGitTest(t, repo, "config", "--get-all", "remote.origin.pushurl"))
	if fetch != "git@github-work:octocat/repo.git" {
		t.Fatalf("fetch URL=%q", fetch)
	}
	wantPush := []string{"git@github-work:octocat/repo.git", "git@gitee-work:octocat-cn/repo.git"}
	if strings.Join(push, "\n") != strings.Join(wantPush, "\n") {
		t.Fatalf("push URLs=%#v, want %#v", push, wantPush)
	}
}

func TestDetectProviderBindingsAllowsDifferentProfilesAcrossProviders(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "remote", "add", "origin", "git@github.com:github-one/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@gitee.com:gitee-two/repo.git")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	config := Config{
		"one": {Providers: map[string]ProviderAccount{
			providerGitHub: {Username: "github-one"},
		}},
		"two": {Providers: map[string]ProviderAccount{
			providerGitee: {Username: "gitee-two"},
		}},
	}
	detection := detectProviderBindings(config)
	if len(detection.Conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %#v", detection.Conflicts)
	}
	if detection.Bindings[providerGitHub] != "one" || detection.Bindings[providerGitee] != "two" {
		t.Fatalf("bindings=%#v", detection.Bindings)
	}
}

func TestDetectProviderBindingsRejectsConflictWithinOneProvider(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "remote", "add", "origin", "git@github.com:github-one/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@github.com:github-two/repo.git")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	config := Config{
		"one": {Providers: map[string]ProviderAccount{providerGitHub: {Username: "github-one"}}},
		"two": {Providers: map[string]ProviderAccount{providerGitHub: {Username: "github-two"}}},
	}
	detection := detectProviderBindings(config)
	if got := strings.Join(detection.Conflicts[providerGitHub], ","); got != "one,two" {
		t.Fatalf("GitHub conflict=%q; want one,two", got)
	}
}

func TestFourFetchAndPushURLsResolveIndependently(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "remote", "add", "origin", "https://github.com/github-one/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "origin", "https://gitee.com/gitee-two/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@github-one:github-one/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@gitee-two:gitee-two/repo.git")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	config := Config{
		"one": {Providers: map[string]ProviderAccount{providerGitHub: {Username: "github-one", HostAlias: "github-one"}}},
		"two": {Providers: map[string]ProviderAccount{providerGitee: {Username: "gitee-two", HostAlias: "gitee-two"}}},
	}
	detection := detectProviderBindings(config)
	if len(detection.Conflicts) != 0 || detection.Bindings[providerGitHub] != "one" || detection.Bindings[providerGitee] != "two" {
		t.Fatalf("detection=%#v", detection)
	}
}

func TestSingleProfileBindingRequiresAgreement(t *testing.T) {
	if got := singleProfileBinding(map[string]string{providerGitHub: "one", providerGitee: "two"}); got != "" {
		t.Fatalf("mixed provider bindings selected commit profile %q", got)
	}
	if got := singleProfileBinding(map[string]string{providerGitHub: "one", providerGitee: "one"}); got != "one" {
		t.Fatalf("same provider bindings selected %q", got)
	}
}

func TestRepositoryRemoteProvidersSeesFetchAndPushPlatforms(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "remote", "add", "origin", "https://github.com/octocat/repo.git")
	runGitTest(t, repo, "remote", "set-url", "--add", "--push", "origin", "git@gitee.com:octocat-cn/repo.git")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	present := repositoryRemoteProviders(nil)
	if !present[providerGitHub] || !present[providerGitee] {
		t.Fatalf("providers=%#v", present)
	}
}

func TestDoctorFailsOnRepositoryWideSSHOverride(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init")
	runGitTest(t, repo, "config", "--local", "core.sshCommand", "ssh -i wrong-key")

	oldWorkingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWorkingDirectory)

	if doctor(Config{}) {
		t.Fatal("doctor should fail for a repository-wide SSH override")
	}
}

func TestManagedSSHBlockPrecedesWildcardAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	keyDirectory := filepath.Join(dir, "key dir")
	if err := os.MkdirAll(keyDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(keyDirectory, "private key")
	if err := os.WriteFile(key, []byte("test key"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config")
	original := "Host *\n  ServerAliveInterval 60\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	account := ProviderAccount{Username: "octocat", SSHKey: key}
	if err := syncManagedSSHHostFile(configPath, "github-work", account, supportedProviders[providerGitHub]); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(configPath)
	if !strings.HasPrefix(string(content), "# BEGIN git-swap github-work\n") {
		t.Fatalf("managed block should precede Host *:\n%s", content)
	}
	if !strings.Contains(string(content), `IdentityFile "`+filepath.ToSlash(key)+`"`) {
		t.Fatalf("IdentityFile path should be quoted:\n%s", content)
	}
	backup, err := os.ReadFile(configPath + ".git-swap.bak")
	if err != nil || string(backup) != original {
		t.Fatalf("backup=%q, err=%v", backup, err)
	}
}

func TestNewManagedSSHConfigIsAcceptedByWindowsOpenSSH(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows ACL behavior")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "private-key")
	if err := os.WriteFile(key, []byte("test key"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config")
	account := ProviderAccount{Username: "octocat", SSHKey: key}
	if err := syncManagedSSHHostFile(configPath, "github-work", account, supportedProviders[providerGitHub]); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ssh", "-F", configPath, "-G", "github-work")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("OpenSSH rejected generated config: %v\n%s", err, output)
	}
}

func runGitTest(t *testing.T, directory string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = directory
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
