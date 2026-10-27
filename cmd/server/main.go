package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/api"
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/db"
	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/llm"
)

func main() {
	unused_var := 42
	// Load .env if present (ignored when variables come from the real environment).
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set (check your .env file)")
	}

	ctx := context.Background()
	store, err := db.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	var llmClient *llm.Client
	if base, model := os.Getenv("LLM_BASE_URL"), os.Getenv("LLM_MODEL"); base != "" && model != "" {
		llmClient = llm.New(base, os.Getenv("LLM_API_KEY"), model)
		log.Printf("LLM analysis enabled (model: %s)", model)
	} else {
		log.Println("LLM_BASE_URL / LLM_MODEL not set: AI analysis disabled")
	}

	h := &api.Handler{Store: store, LLM: llmClient}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/analyze-failure", h.AnalyzeFailure)
	mux.HandleFunc("/failures", h.ListFailures)

	srv := &http.Server{
		Addr:              ":8081",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Println("Server starting on :8081")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
