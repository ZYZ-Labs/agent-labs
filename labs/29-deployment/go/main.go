package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ZYZ-Labs/agent-labs/shared/config"
)

type ChatRequest struct {
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
	Model string `json:"model"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Println("Warning: OPENAI_API_KEY is not set. /chat will return 503 until configured.")
	}

	client := openaiclient.NewClient()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "configured": apiKey != ""})
	})

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if apiKey == "" {
			http.Error(w, "OPENAI_API_KEY is not configured", http.StatusServiceUnavailable)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		resp, err := client.ChatCompletion(openaiclient.ChatCompletionRequest{
			Messages: []openaiclient.Message{{Role: "user", Content: req.Message}},
			Model:    req.Model,
			MaxTokens: intPtr(300),
			Temperature: floatPtr(0.0),
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
			return
		}

		reply := client.ExtractMessage(resp).Content
		model := req.Model
		if model == "" {
			model = client.Config.Model
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatResponse{Reply: reply, Model: model})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Printf("Server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func intPtr(v int) *int       { return &v }
func floatPtr(v float64) *float64 { return &v }
