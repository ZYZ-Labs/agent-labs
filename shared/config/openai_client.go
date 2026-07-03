// Shared OpenAI-compatible client wrapper for Go labs.
// Reads configuration from environment variables.

package openaiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config holds OpenAI-compatible client settings.
type Config struct {
	APIKey   string
	BaseURL  string
	Model    string
	LogLevel string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the request body for chat completions.
type ChatCompletionRequest struct {
	Model            string                 `json:"model"`
	Messages         []Message              `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxTokens        *int                   `json:"max_tokens,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	Stop             interface{}            `json:"stop,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	ResponseFormat   map[string]string      `json:"response_format,omitempty"`
	Tools            []interface{}          `json:"tools,omitempty"`
	ToolChoice       interface{}            `json:"tool_choice,omitempty"`
	Stream           bool                   `json:"stream"`
	ExtraBody        map[string]interface{} `json:"-"`
}

// ChatCompletionResponse is the response body for chat completions.
type ChatCompletionResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// LoadConfig reads configuration from environment variables.
func LoadConfig() Config {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return Config{
		APIKey:   os.Getenv("OPENAI_API_KEY"),
		BaseURL:  baseURL,
		Model:    model,
		LogLevel: os.Getenv("LOG_LEVEL"),
	}
}

// Client is an OpenAI-compatible HTTP client.
type Client struct {
	Config     Config
	HTTPClient *http.Client
	MaxRetries int
}

// NewClient creates a new client with defaults.
func NewClient() *Client {
	cfg := LoadConfig()
	if cfg.APIKey == "" && !strings.HasPrefix(cfg.BaseURL, "http://localhost") {
		panic("OPENAI_API_KEY is required for non-local endpoints")
	}
	return &Client{
		Config:     cfg,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		MaxRetries: 3,
	}
}

// ChatCompletion sends a chat completion request.
func (c *Client) ChatCompletion(req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	url := c.Config.BaseURL + "/chat/completions"
	req.Model = firstNonEmpty(req.Model, c.Config.Model)

	bodyMap := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	if req.Temperature != nil {
		bodyMap["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		bodyMap["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		bodyMap["top_p"] = *req.TopP
	}
	if req.FrequencyPenalty != nil {
		bodyMap["frequency_penalty"] = *req.FrequencyPenalty
	}
	if req.PresencePenalty != nil {
		bodyMap["presence_penalty"] = *req.PresencePenalty
	}
	if req.Stop != nil {
		bodyMap["stop"] = req.Stop
	}
	if req.Seed != nil {
		bodyMap["seed"] = *req.Seed
	}
	if req.ResponseFormat != nil {
		bodyMap["response_format"] = req.ResponseFormat
	}
	if req.Tools != nil {
		bodyMap["tools"] = req.Tools
	}
	if req.ToolChoice != nil {
		bodyMap["tool_choice"] = req.ToolChoice
	}
	for k, v := range req.ExtraBody {
		bodyMap[k] = v
	}

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}

	for attempt := 1; attempt <= c.MaxRetries; attempt++ {
		reqHTTP, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		reqHTTP.Header.Set("Content-Type", "application/json")
		reqHTTP.Header.Set("Authorization", "Bearer "+c.Config.APIKey)

		resp, err := c.HTTPClient.Do(reqHTTP)
		if err != nil {
			if attempt == c.MaxRetries {
				return nil, err
			}
			continue
		}
		defer resp.Body.Close()

		respBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var result ChatCompletionResponse
			if err := json.Unmarshal(respBytes, &result); err != nil {
				return nil, err
			}
			return &result, nil
		}
		if attempt == c.MaxRetries {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	return nil, fmt.Errorf("max retries exceeded")
}

// ExtractMessage returns the first message from a response.
func (c *Client) ExtractMessage(resp *ChatCompletionResponse) Message {
	return resp.Choices[0].Message
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
