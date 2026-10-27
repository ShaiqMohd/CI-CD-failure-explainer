package llm

import "testing"

func TestParseAnalysis(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantExpl string
		wantFix  string
	}{
		{"plain format", "Explanation: A broke.\nSuggested Fix: Do B.", "A broke.", "Do B."},
		{"markdown bold", "**Explanation:** A broke.\n\n**Suggested Fix:** Do B.", "A broke.", "Do B."},
		{"reasoning block", "<think>hmm</think>Explanation: A.\nSuggested Fix: B.", "A.", "B."},
		{"no fix section", "Explanation: A only.", "A only.", ""},
		{"ignores the format", "Just some text.", "Just some text.", ""},
		{"multiline fix", "Explanation: A.\nSuggested Fix: Step 1.\nStep 2.", "A.", "Step 1.\nStep 2."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseAnalysis(tc.in)
			if got.Explanation != tc.wantExpl || got.Fix != tc.wantFix {
				t.Errorf("got {%q, %q}, want {%q, %q}",
					got.Explanation, got.Fix, tc.wantExpl, tc.wantFix)
			}
		})
	}
}
