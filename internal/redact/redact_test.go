package redact

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	// Secrets are built at runtime so this file doesn't contain
	// token-shaped literals that secret scanners would flag.
	ghToken := "gh" + "p_" + strings.Repeat("a", 30)
	groqKey := "gs" + "k_" + strings.Repeat("b", 30)
	awsKey := "AK" + "IA" + strings.Repeat("C", 16)

	tests := []struct {
		name string
		in   string
		gone string // must not appear in the output
	}{
		{"github token", "clone failed with " + ghToken, ghToken},
		{"groq key", "key=" + groqKey, groqKey},
		{"aws key", "using " + awsKey + " for upload", awsKey},
		{"password assignment", "db login password=hunter2 failed", "hunter2"},
		{"quoted token", `token: "abc123secret"`, "abc123secret"},
		{"url credentials", "git clone https://user:s3cr3tpw@github.com/x/y", "s3cr3tpw"},
		{"bearer header", "Authorization: Bearer abcdefghijklmnop1234", "abcdefghijklmnop1234"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEabc\n-----END RSA PRIVATE KEY-----", "MIIEabc"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Text(tc.in)
			if strings.Contains(got, tc.gone) {
				t.Errorf("secret %q still present in %q", tc.gone, got)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Errorf("expected a REDACTED marker in %q", got)
			}
		})
	}
}

func TestTextLeavesNormalLogsAlone(t *testing.T) {
	in := "cmd/server/main.go:18:2: declared and not used: unused_var"
	if got := Text(in); got != in {
		t.Errorf("normal log was modified: %q", got)
	}
}
