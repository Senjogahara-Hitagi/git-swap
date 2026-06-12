package main

import "testing"

func TestGitHubHTTPSToSSH(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{
			name: "adds git suffix",
			in:   "https://github.com/crftwr/keyhac-win",
			want: "git@github.com:crftwr/keyhac-win.git",
			ok:   true,
		},
		{
			name: "keeps git suffix",
			in:   "https://github.com/kkRelation/keyhac-win.git",
			want: "git@github.com:kkRelation/keyhac-win.git",
			ok:   true,
		},
		{
			name: "rejects non github",
			in:   "https://example.com/owner/repo.git",
			ok:   false,
		},
		{
			name: "rejects ssh",
			in:   "git@github.com:owner/repo.git",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := githubHTTPSToSSH(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
