package llm

import (
	"regexp"
	"strings"
)

// Analysis is the model's answer split into its two parts.
type Analysis struct {
	Explanation string
	Fix         string
}

var (
	thinkBlock = regexp.MustCompile(`(?is)<think>.*?</think>`)
	fixMarker  = regexp.MustCompile(`(?i)\*{0,2}suggested\s+fix\*{0,2}\s*:\s*\*{0,2}`)
	explMarker = regexp.MustCompile(`(?i)^\s*\*{0,2}explanation\*{0,2}\s*:\s*\*{0,2}\s*`)
)

// ParseAnalysis splits a reply like
//
//	Explanation: ...
//	Suggested Fix: ...
//
// into its parts. It tolerates markdown bold, reasoning blocks, and
// replies that ignore the format (everything goes into Explanation).
func ParseAnalysis(raw string) Analysis {
	s := strings.TrimSpace(thinkBlock.ReplaceAllString(raw, ""))

	var expl, fix string
	if loc := fixMarker.FindStringIndex(s); loc != nil {
		expl = s[:loc[0]]
		fix = s[loc[1]:]
	} else {
		expl = s
	}
	expl = explMarker.ReplaceAllString(strings.TrimSpace(expl), "")

	return Analysis{
		Explanation: strings.TrimSpace(expl),
		Fix:         strings.TrimSpace(fix),
	}
}
