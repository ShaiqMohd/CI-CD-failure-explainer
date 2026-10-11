package models

// FailureRequest is the JSON body Jenkins sends when a build fails.
type FailureRequest struct {
	PipelineName string `json:"pipeline_name"`
	BuildNumber  string `json:"build_number"`
	LogText      string `json:"log_text"`
}

// AnalyzeResponse is what our service sends back.
type AnalyzeResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	LogLines int    `json:"log_lines"`
}
