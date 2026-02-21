package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
)

type ChatGPT5Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
}

type ChatGPT5Config struct {
	APIKey  string
	BaseURL string // 默认 "https://api.openai.com/v1"
	Model   string // 默认 "gpt-4" 或 "gpt-4-turbo"
	Timeout time.Duration
	Proxy   string // HTTP 代理
}

func NewChatGPT5Client(cfg ChatGPT5Config) (*ChatGPT5Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}

	if cfg.Model == "" {
		cfg.Model = "gpt-4-turbo"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	httpClient, err := internal.CreateProxyHTTPClient(cfg.Proxy, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if cfg.Proxy != "" {
		fmt.Printf("Using proxy: %s\n", cfg.Proxy)
	}

	return &ChatGPT5Client{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: httpClient,
		timeout:    cfg.Timeout,
		maxRetries: 3, // Default retry 3 times
	}, nil
}

// helloq SendPrompt Send prompt and retry
func (c *ChatGPT5Client) SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:       c.model,
		Messages:    []Message{},
		Temperature: 0.1,
		MaxTokens:   4096,
	}

	if systemPrompt != "" {
		reqBody.Messages = append(reqBody.Messages, Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	reqBody.Messages = append(reqBody.Messages, Message{
		Role:    "user",
		Content: userPrompt,
	})

	return c.sendChatCompletion(ctx, reqBody)
}

func (c *ChatGPT5Client) sendChatCompletion(ctx context.Context, reqBody ChatCompletionRequest) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	baseDelay := 2 * time.Second

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			fmt.Printf("    ⚠️ API temporary error, retrying in %v (attempt %d/%d)...\n", delay, attempt, c.maxRetries)

			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}

		content, err := c.doRequest(ctx, jsonData)
		if err == nil {
			return content, nil
		}

		lastErr = err
		if !c.isRetryableError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("exceeded max retries (%d), last error: %w", c.maxRetries, lastErr)
}

func (c *ChatGPT5Client) AnalyzeJSON(ctx context.Context, prompt string) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:       c.model,
		Messages:    []Message{{Role: "user", Content: prompt}},
		Temperature: 0.1,
		MaxTokens:   4096,
		ResponseFormat: map[string]interface{}{
			"type": "json_object",
		},
	}

	content, err := c.sendChatCompletion(ctx, reqBody)
	if err == nil {
		return content, nil
	}

	var nre *NonRetryableError
	if errors.As(err, &nre) && nre.StatusCode == 400 {
		lower := strings.ToLower(nre.Message)
		if strings.Contains(lower, "response_format") || strings.Contains(lower, "json") {
			return c.Analyze(ctx, prompt)
		}
	}

	return "", err
}

func (c *ChatGPT5Client) doRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err) // Non-retryable
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err // Network error, retryable
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := string(body)
		// 429 (Too Many Requests) 和 5xx (Server Errors) 是可重试的
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			return "", fmt.Errorf("API temporary error status %d: %s", resp.StatusCode, errMsg)
		}
		// 其他错误（如 400, 401）不可重试
		return "", &NonRetryableError{StatusCode: resp.StatusCode, Message: errMsg}
	}

	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content

	// 打印 token 使用情况
	// fmt.Printf("📊 Token 使用: Prompt=%d, Completion=%d, Total=%d\n",
	// 	apiResp.Usage.PromptTokens,
	// 	apiResp.Usage.CompletionTokens,
	// 	apiResp.Usage.TotalTokens)

	return content, nil
}

func (c *ChatGPT5Client) isRetryableError(err error) bool {
	if _, ok := err.(*NonRetryableError); ok {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "invalid_request_error") {
		return false
	}
	return true
}

type NonRetryableError struct {
	StatusCode int
	Message    string
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("API fatal error %d: %s", e.StatusCode, e.Message)
}

func (c *ChatGPT5Client) Analyze(ctx context.Context, prompt string) (string, error) {
	// Prompt is handled by the caller (ai_manager) in the full prompt
	return c.SendPrompt(ctx, "", prompt)
}

func (c *ChatGPT5Client) GetName() string {
	return fmt.Sprintf("ChatGPT-5 (%s)", c.model)
}

func (c *ChatGPT5Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
