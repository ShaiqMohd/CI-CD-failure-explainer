package main

import (
	"log"
	"net/http"

	"github.com/ShaiqMohd/CI-CD-failure-explainer/internal/api"
)

func main() {
	unusedvar := 42
	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/analyze-failure", api.AnalyzeFailureHandler)

	log.Println("Server starting on :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal(err)
	}
}
