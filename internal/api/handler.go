package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/models"
)

const maxBodyBytes = 1 << 20 // 1 MB limit on request size

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "OK - CI-CD Failure Explainer service is running")
}

func AnalyzeFailureHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Only allow POST
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, models.AnalyzeResponse{
			Status: "error", Message: "use POST",
		})
		return
	}

	// 2. Limit body size, then decode JSON
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req models.FailureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.AnalyzeResponse{
			Status: "error", Message: "invalid JSON body",
		})
		return
	}

	// 3. Validate required fields
	if strings.TrimSpace(req.PipelineName) == "" || strings.TrimSpace(req.LogText) == "" {
		writeJSON(w, http.StatusBadRequest, models.AnalyzeResponse{
			Status: "error", Message: "pipeline_name and log_text are required",
		})
		return
	}

	// 4. For now, just log it (the LLM comes on Day 5)
	lines := strings.Count(req.LogText, "\n") + 1
	log.Printf("Failure received: pipeline=%s build=%s log_lines=%d",
		req.PipelineName, req.BuildNumber, lines)

	writeJSON(w, http.StatusOK, models.AnalyzeResponse{
		Status:   "received",
		Message:  "failure log received; analysis not implemented yet",
		LogLines: lines,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
