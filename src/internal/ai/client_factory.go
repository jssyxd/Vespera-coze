package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/VectorBits/Vespera/src/internal/ai/client"
)

type AIClient interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	GetName() string
	Close() error
}

type AIClientConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Timeout  time.Duration
	Proxy    string
}

// helloq NewAIClient AI 客户端工厂
func NewAIClient(cfg AIClientConfig) (AIClient, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	switch cfg.Provider {
	case "chatgpt5", "openai", "gpt4":
		return client.NewChatGPT5Client(client.ChatGPT5Config{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
			Proxy:   cfg.Proxy,
		})

	case "deepseek":
		return client.NewDeepSeekClient(client.DeepSeekConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
			Proxy:   cfg.Proxy,
		})

	case "gemini":
		return client.NewGeminiClient(client.GeminiConfig{
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
			Proxy:   cfg.Proxy,
		})

	case "local-llm", "ollama":
		return client.NewLocalLLMClient(client.LocalLLMConfig{
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Timeout: cfg.Timeout,
			Proxy:   cfg.Proxy,
		})

	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (supported: chatgpt5, openai, gpt4, deepseek, gemini, local-llm, ollama)", cfg.Provider)
	}
}

func ValidateProvider(provider string) error {
	validProviders := map[string]bool{
		"chatgpt5":  true,
		"openai":    true,
		"gpt4":      true,
		"deepseek":  true,
		"gemini":    true,
		"local-llm": true,
		"ollama":    true,
	}

	if !validProviders[provider] {
		return fmt.Errorf("invalid provider '%s', must be one of: chatgpt5, openai, gpt4, deepseek, gemini, local-llm, ollama", provider)
	}

	return nil
}
