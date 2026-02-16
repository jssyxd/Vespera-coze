package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_completion_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (c *Client) Chat(model string, messages []Message) (*ChatResponse, error) {
	reqBody := ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   4096,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (c *Client) AnalyzeContract(model, contractCode string) (*AnalysisResult, error) {
	systemPrompt := `You are a smart contract security auditor. Analyze the provided Solidity code for:
1. Security vulnerabilities (reentrancy, overflow, access control, etc.)
2. Code quality issues
3. Gas optimization opportunities
4. Potential arbitrage opportunities if it's a DEX or financial contract

Return your analysis in JSON format with the following structure:
{
  "vulnerabilities": [
    {
      "type": "vulnerability name",
      "severity": "CRITICAL|HIGH|MEDIUM|LOW",
      "description": "detailed description",
      "line": line_number,
      "recommendation": "how to fix"
    }
  ],
  "arbitrage_opportunities": [
    {
      "type": "opportunity type",
      "description": "description",
      "expected_profit": "profit estimate"
    }
  ],
  "summary": "overall assessment"
}`

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Analyze this Solidity contract:\n\n```solidity\n%s\n```", contractCode)},
	}

	resp, err := c.Chat(model, messages)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI")
	}

	// Parse the JSON response
	content := resp.Choices[0].Message.Content
	var result AnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// If JSON parsing fails, wrap the raw content
		result = AnalysisResult{
			Summary: fmt.Sprintf("Raw analysis: %s", content),
		}
	}

	return &result, nil
}

type AnalysisResult struct {
	Vulnerabilities        []Vulnerability        `json:"vulnerabilities"`
	ArbitrageOpportunities []ArbitrageOpportunity `json:"arbitrage_opportunities"`
	Summary                string                 `json:"summary"`
}

type Vulnerability struct {
	Type           string `json:"type"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	Line           int    `json:"line"`
	Recommendation string `json:"recommendation"`
}

type ArbitrageOpportunity struct {
	Type           string `json:"type"`
	Description    string `json:"description"`
	ExpectedProfit string `json:"expected_profit"`
}
