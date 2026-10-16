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
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/models"
)

const (
	maxBodyBytes      = 1 << 20 // 1 MB request limit
	maxStoredLogBytes = 8000    // how much of the log we keep
	dbTimeout         = 5 * time.Second
)

// Handler holds the dependencies the endpoints need.
type Handler struct {
	Store *db.Store
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
	log.Printf("Failure saved: id=%d pipeline=%s build=%s log_lines=%d",
		f.ID, f.PipelineName, f.BuildNumber, lines)

	writeJSON(w, http.StatusCreated, models.AnalyzeResponse{
		Status:   "received",
		Message:  "failure saved; analysis not implemented yet",
		ID:       f.ID,
		LogLines: lines,
	})
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
