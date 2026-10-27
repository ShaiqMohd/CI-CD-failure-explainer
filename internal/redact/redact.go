// Package redact removes likely secrets from text before it is stored
// or sent to a third-party service.
package redact

import "regexp"

type rule struct {
	re   *regexp.Regexp
	repl string
}

// Order matters: specific patterns first, generic key=value last.
var rules = []rule{
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "[REDACTED PRIVATE KEY]"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[REDACTED AWS KEY]"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), "[REDACTED GITHUB TOKEN]"},
	{regexp.MustCompile(`\bgsk_[A-Za-z0-9]{20,}\b`), "[REDACTED API KEY]"},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`), "[REDACTED API KEY]"},
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{16,}`), "${1}[REDACTED]"},
	// credentials inside URLs: scheme://user:password@host
	{regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:)[^\s@/]+@`), "${1}[REDACTED]@"},
	// password=..., token: ..., api_key="..."
	{regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s"',;]+)`), "${1}${2}[REDACTED]"},
}

// Text returns s with likely secrets replaced by placeholders.
func Text(s string) string {
	for _, r := range rules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
