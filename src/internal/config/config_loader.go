package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type ChainConfig struct {
	Name      string   `yaml:"name"`
	ChainID   int      `yaml:"chain_id"`
	RPCURLs   []string `yaml:"rpc_urls"`
	Explorer  Explorer `yaml:"explorer"`
	TableName string   `yaml:"table_name"`
}

type Explorer struct {
	APIKey  string   `yaml:"api_key"`
	APIKeys []string `yaml:"api_keys"`
	BaseURL string   `yaml:"base_url"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

type AIConfig struct {
	OpenAI   AIProvider `yaml:"openai"`
	DeepSeek AIProvider `yaml:"deepseek"`
	Gemini   AIProvider `yaml:"gemini"`
	LocalLLM AIProvider `yaml:"local_llm"`
}

type AIProvider struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
	Proxy   string `yaml:"proxy"`
}

type AppConfig struct {
	AI       AIConfig               `yaml:"ai"`
	Chains   map[string]ChainConfig `yaml:"chains"`
	Database DatabaseConfig         `yaml:"database"`
}

var GlobalConfig *AppConfig
var loadOnce sync.Once
var loadedConfig *AppConfig
var loadedErr error

// helloq LoadConfig 加载 YAML 配置
func LoadConfig() (*AppConfig, error) {
	loadOnce.Do(func() {
		configPath := findConfigFile()
		if configPath == "" {
			loadedErr = fmt.Errorf("The configuration file settings.yaml was not found.")
			return
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			loadedErr = fmt.Errorf("Failed to read configuration file: %w", err)
			return
		}

		var config AppConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			loadedErr = fmt.Errorf("Failed to parse configuration file: %w", err)
			return
		}

		loadedConfig = &config
		GlobalConfig = loadedConfig
	})

	if loadedErr != nil {
		return nil, loadedErr
	}
	return loadedConfig, nil
}

func findConfigFile() string {
	possiblePaths := []string{
		"config/settings.yaml",
		"settings.yaml",
		"src/config/settings.yaml",
		"../config/settings.yaml",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func (c *AppConfig) GetChainConfig(chainName string) (*ChainConfig, error) {
	chain, exists := c.Chains[chainName]
	if !exists {
		return nil, fmt.Errorf("Unsupported chain: %s", chainName)
	}
	return &chain, nil
}

func (c *AppConfig) GetAIConfig(provider string) (*AIProvider, error) {
	switch provider {
	case "openai", "gpt4", "chatgpt5":
		return &AIProvider{
			APIKey:  c.AI.OpenAI.APIKey,
			BaseURL: c.AI.OpenAI.BaseURL,
			Model:   c.AI.OpenAI.Model,
			Proxy:   c.AI.OpenAI.Proxy,
		}, nil
	case "deepseek":
		return &AIProvider{
			APIKey:  c.AI.DeepSeek.APIKey,
			BaseURL: c.AI.DeepSeek.BaseURL,
			Model:   c.AI.DeepSeek.Model,
			Proxy:   c.AI.DeepSeek.Proxy,
		}, nil
	case "gemini":
		return &AIProvider{
			APIKey:  c.AI.Gemini.APIKey,
			BaseURL: c.AI.Gemini.BaseURL,
			Model:   c.AI.Gemini.Model,
			Proxy:   c.AI.Gemini.Proxy,
		}, nil
	case "local-llm", "ollama", "local_llm":
		return &AIProvider{
			APIKey:  "",
			BaseURL: c.AI.LocalLLM.BaseURL,
			Model:   c.AI.LocalLLM.Model,
			Proxy:   c.AI.LocalLLM.Proxy,
		}, nil
	default:
		return nil, fmt.Errorf("Unsupported AI provider: %s", provider)
	}
}

func (c *AppConfig) GetDatabaseDSN(includeDBName bool) string {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
	)
	if includeDBName {
		dsn += fmt.Sprintf("%s?parseTime=true&charset=utf8mb4", c.Database.Name)
	} else {
		dsn += "?parseTime=true&charset=utf8mb4"
	}
	return dsn
}

func GetConfigPath() string {
	return findConfigFile()
}

func GetConfigDir() string {
	configPath := findConfigFile()
	if configPath == "" {
		return "config"
	}
	return filepath.Dir(configPath)
}
