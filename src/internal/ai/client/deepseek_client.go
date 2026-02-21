package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
	"github.com/VectorBits/Vespera/src/internal/logger"
)

type DeepSeekClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
	Proxy   string
}

func NewDeepSeekClient(cfg DeepSeekConfig) (*DeepSeekClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com/v1"
	}

	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	httpClient, err := internal.CreateProxyHTTPClient(cfg.Proxy, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	if cfg.Proxy != "" {
		logger.Debug("Using proxy: %s", cfg.Proxy)
	}

	return &DeepSeekClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: httpClient,
		timeout:    cfg.Timeout,
		maxRetries: 3,
	}, nil
}

func (c *DeepSeekClient) isRetryableError(err error) bool {
	if _, ok := err.(*NonRetryableError); ok {
		return false
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "context deadline exceeded") {
		return false
	}
	return true
}

func (c *DeepSeekClient) doRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := string(body)
		if len(errMsg) > 4096 {
			errMsg = errMsg[:4096] + "...(truncated)"
		}
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			return "", fmt.Errorf("API temporary error status %d: %s", resp.StatusCode, errMsg)
		}
		return "", &NonRetryableError{StatusCode: resp.StatusCode, Message: errMsg}
	}

	var apiResp ChatCompletionResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		msg := fmt.Sprintf("%s (type: %s, code: %s)", apiResp.Error.Message, apiResp.Error.Type, apiResp.Error.Code)
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "rate") || strings.Contains(lower, "limit") || strings.Contains(lower, "429") {
			return "", fmt.Errorf("API temporary error: %s", msg)
		}
		return "", &NonRetryableError{StatusCode: resp.StatusCode, Message: msg}
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	logger.Debug("📊 Token Usage: Prompt=%d, Completion=%d, Total=%d",
		apiResp.Usage.PromptTokens,
		apiResp.Usage.CompletionTokens,
		apiResp.Usage.TotalTokens)

	return apiResp.Choices[0].Message.Content, nil
}

func (c *DeepSeekClient) sendWithRetry(ctx context.Context, jsonData []byte) (string, error) {
	var lastErr error
	baseDelay := 2 * time.Second

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
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

func (c *DeepSeekClient) SendPrompt(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := ChatCompletionRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   4096,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	return c.sendWithRetry(ctx, jsonData)
}

// helloq SendPromptJSON 强制 JSON 输出
func (c *DeepSeekClient) SendPromptJSON(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	reqBody := ChatCompletionRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		MaxTokens:   2048,
		ResponseFormat: map[string]interface{}{
			"type": "json_object",
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	return c.sendWithRetry(ctx, jsonData)
}

// 这段后面更新用 先留着这些  qhello
// 固定json和角色前缀
func (c *DeepSeekClient) Analyze(ctx context.Context, prompt string) (string, error) {
	// System prompt is handled by the caller (ai_manager) in the full prompt
	return c.SendPrompt(ctx, "", prompt)
}

func (c *DeepSeekClient) AnalyzeJSON(ctx context.Context, prompt string) (string, error) {
	return c.SendPromptJSON(ctx, "", prompt)
}

func (c *DeepSeekClient) GetName() string {
	return fmt.Sprintf("DeepSeek (%s)", c.model)
}

func (c *DeepSeekClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
