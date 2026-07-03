package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestBody struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type ResponseBody struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}

	question := "Say hello in one sentence."
	fmt.Println("User:", question)

	body := RequestBody{
		Model:       model,
		Messages:    []Message{{Role: "user", Content: question}},
		MaxTokens:   50,
		Temperature: 0.0,
	}
	payload, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result ResponseBody
	json.Unmarshal(data, &result)
	fmt.Println("Assistant:", result.Choices[0].Message.Content)
}
