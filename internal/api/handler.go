package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/db"
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/llm"
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/models"
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/redact"
)

const (
	maxBodyBytes      = 1 << 20 // 1 MB request limit
	maxStoredLogBytes = 8000    // how much of the log we keep
	dbTimeout         = 5 * time.Second
	llmTimeout        = 30 * time.Second
	maxSourceBytes    = 4000 // how much source context we accept
)

// Handler holds the dependencies the endpoints need.
type Handler struct {
	Store *db.Store
	LLM   *llm.Client // nil when no LLM is configured
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.Store.Ping(ctx); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprintln(w, "OK - CI-CD Failure Explainer service is running")
}

func (h *Handler) AnalyzeFailure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, models.AnalyzeResponse{
			Status: "error", Message: "use POST",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req models.FailureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.AnalyzeResponse{
			Status: "error", Message: "invalid JSON body",
		})
		return
	}

	if strings.TrimSpace(req.PipelineName) == "" || strings.TrimSpace(req.LogText) == "" {
		writeJSON(w, http.StatusBadRequest, models.AnalyzeResponse{
			Status: "error", Message: "pipeline_name and log_text are required",
		})
		return
	}

	// Redact secrets BEFORE anything is stored or sent to the LLM.
	req.LogText = redact.Text(req.LogText)
	req.SourceContext = redact.Text(clip(req.SourceContext, maxSourceBytes))

	// 1. Save first, so the failure is never lost even if the AI step fails.
	f := &models.Failure{
		PipelineName: req.PipelineName,
		BuildNumber:  req.BuildNumber,
		LogExcerpt:   excerpt(req.LogText),
	}
	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	if err := h.Store.InsertFailure(ctx, f); err != nil {
		log.Printf("insert failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.AnalyzeResponse{
			Status: "error", Message: "could not save failure",
		})
		return
	}

	lines := strings.Count(req.LogText, "\n") + 1
	log.Printf("Failure saved: id=%d pipeline=%s build=%s log_lines=%d source_context=%t",
		f.ID, f.PipelineName, f.BuildNumber, lines, req.SourceContext != "")

	resp := models.AnalyzeResponse{
		Status: "received", Message: "failure saved",
		ID: f.ID, LogLines: lines,
	}

	if h.LLM == nil {
		resp.Message = "failure saved; LLM not configured"
		writeJSON(w, http.StatusCreated, resp)
		return
	}

	// 2. Ask the LLM. If it fails, the failure is still saved.
	llmCtx, cancelLLM := context.WithTimeout(r.Context(), llmTimeout)
	defer cancelLLM()
	analysis, err := h.LLM.Analyze(llmCtx, f.PipelineName, f.BuildNumber, f.LogExcerpt, req.SourceContext)
	if err != nil {
		log.Printf("llm analysis failed for id=%d: %v", f.ID, err)
		resp.Status = "saved"
		resp.Message = "failure saved; AI analysis unavailable"
		writeJSON(w, http.StatusCreated, resp)
		return
	}

	// 3. Store explanation and fix in their own columns.
	saveCtx, cancelSave := context.WithTimeout(r.Context(), dbTimeout)
	defer cancelSave()
	if err := h.Store.UpdateAnalysis(saveCtx, f.ID, analysis.Explanation, analysis.Fix); err != nil {
		log.Printf("saving analysis failed for id=%d: %v", f.ID, err)
	}

	log.Printf("Analysis saved: id=%d", f.ID)
	resp.Message = "failure saved and analyzed"
	resp.Explanation = analysis.Explanation
	resp.Fix = analysis.Fix
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) ListFailures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, models.AnalyzeResponse{
			Status: "error", Message: "use GET",
		})
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
	defer cancel()
	failures, err := h.Store.ListFailures(ctx, limit)
	if err != nil {
		log.Printf("list failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, models.AnalyzeResponse{
			Status: "error", Message: "could not read failures",
		})
		return
	}
	writeJSON(w, http.StatusOK, failures)
}

// excerpt keeps the END of the log (that's where build errors usually are)
// and strips bytes that PostgreSQL text columns reject.
func excerpt(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if len(s) > maxStoredLogBytes {
		s = s[len(s)-maxStoredLogBytes:]
	}
	return strings.ToValidUTF8(s, "")
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// clip keeps at most n bytes of s, without leaving a broken UTF-8 character.
func clip(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	return strings.ToValidUTF8(s, "")
}
