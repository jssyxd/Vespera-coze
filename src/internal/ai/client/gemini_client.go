package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
)

type GeminiClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
	maxRetries int
}

type GeminiConfig struct {
	APIKey  string
	BaseURL string // 默认 "https://generativelanguage.googleapis.com/v1beta"
	Model   string // 默认 "gemini-1.5-pro"
	Timeout time.Duration
	Proxy   string // HTTP 代理
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func NewGeminiClient(cfg GeminiConfig) (*GeminiClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-1.5-pro"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	httpClient, err := internal.CreateProxyHTTPClient(cfg.Proxy, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &GeminiClient{
		apiKey:     cfg.APIKey,
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: httpClient,
		timeout:    cfg.Timeout,
		maxRetries: 3,
	}, nil
}

func (c *GeminiClient) Analyze(ctx context.Context, prompt string) (string, error) {
	// Prompt is handled by the caller
	return c.GenerateContent(ctx, prompt)
}

// helloq GenerateContent 发送内容生成请求
func (c *GeminiClient) GenerateContent(ctx context.Context, text string) (string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: text},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 8192,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	var lastErr error
	baseDelay := 2 * time.Second

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			fmt.Printf("    ⚠️ Gemini API temporary error, retrying in %v (attempt %d/%d)...\n", delay, attempt, c.maxRetries)

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

func (c *GeminiClient) doRequest(ctx context.Context, jsonData []byte) (string, error) {
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("Gemini API error: %s (code: %d)", apiResp.Error.Message, apiResp.Error.Code)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return apiResp.Candidates[0].Content.Parts[0].Text, nil
}

func (c *GeminiClient) isRetryableError(err error) bool {
	errStr := err.Error()
	if strings.Contains(errStr, "status 429") || strings.Contains(errStr, "status 5") {
		return true
	}
	return false
}

func (c *GeminiClient) GetName() string {
	return fmt.Sprintf("Gemini (%s)", c.model)
}

func (c *GeminiClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
