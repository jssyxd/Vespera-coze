package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
)

type LocalLLMClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type LocalLLMConfig struct {
	BaseURL string // 例如 "http://localhost:11434"
	Model   string // 例如 "llama2", "codellama"
	Timeout time.Duration
	Proxy   string // HTTP 代理
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

func NewLocalLLMClient(cfg LocalLLMConfig) (*LocalLLMClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}

	if cfg.Model == "" {
		cfg.Model = "llama2"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 120 * time.Second // 本地模型可能需要更长时间
	}

	httpClient, err := internal.CreateProxyHTTPClient(cfg.Proxy, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("Failed to create HTTP client: %w", err)
	}

	if cfg.Proxy != "" {
		fmt.Printf("Using proxy: %s\n", cfg.Proxy)
	}

	return &LocalLLMClient{
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		httpClient: httpClient,
	}, nil
}

// helloq Analyze call local LLM analysis
func (c *LocalLLMClient) Analyze(ctx context.Context, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if apiResp.Error != "" {
		return "", fmt.Errorf("ollama API error: %s", apiResp.Error)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return apiResp.Response, nil
}

func (c *LocalLLMClient) GetName() string {
	return fmt.Sprintf("Local LLM (%s)", c.model)
}

func (c *LocalLLMClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
