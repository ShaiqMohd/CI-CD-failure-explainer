package models

import "time"

// FailureRequest is the JSON body Jenkins sends when a build fails.
type FailureRequest struct {
	PipelineName  string `json:"pipeline_name"`
	BuildNumber   string `json:"build_number"`
	LogText       string `json:"log_text"`
	SourceContext string `json:"source_context"`
}

// Failure is one stored row in the failures table.
type Failure struct {
	ID            int64     `json:"id"`
	PipelineName  string    `json:"pipeline_name"`
	BuildNumber   string    `json:"build_number"`
	LogExcerpt    string    `json:"log_excerpt"`
	AIExplanation string    `json:"ai_explanation"`
	FixSuggestion string    `json:"fix_suggestion"`
	CreatedAt     time.Time `json:"created_at"`
}

// AnalyzeResponse is what our service sends back.
type AnalyzeResponse struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	ID          int64  `json:"id,omitempty"`
	LogLines    int    `json:"log_lines,omitempty"`
	Explanation string `json:"explanation,omitempty"`
	Fix         string `json:"fix,omitempty"`
}
