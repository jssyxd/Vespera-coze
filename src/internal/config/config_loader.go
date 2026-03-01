package config

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// replaceEnvVars replaces ${VAR} patterns with environment variable values
func replaceEnvVars(data []byte) []byte {
	// Pattern matches ${VAR} where VAR is alphanumeric or underscore
	re := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	return re.ReplaceAllFunc(data, func(match []byte) []byte {
		varName := string(match[2 : len(match)-1]) // Remove ${ and }
		value := os.Getenv(varName)
		if value == "" {
			return match // Keep original if env var not set
		}
		return []byte(value)
	})
}

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

// DatabaseConfigYML YAML配置用的结构
type DatabaseConfigYML struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"ssl_mode"`
}

type AIProvider struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
	Proxy   string `yaml:"proxy"`
}

type AIConfigYML struct {
	OpenAI   AIProvider `yaml:"openai"`
	DeepSeek AIProvider `yaml:"deepseek"`
	Gemini   AIProvider `yaml:"gemini"`
	LocalLLM AIProvider `yaml:"local_llm"`
}

type AppConfig struct {
	AI       AIConfigYML          `yaml:"ai"`
	Chains   map[string]ChainConfig `yaml:"chains"`
	Database DatabaseConfigYML    `yaml:"database"`
}

var GlobalConfig *AppConfig
var loadOnce sync.Once
var loadedConfig *AppConfig
var loadedErr error

// LoadConfig 加载 YAML 配置
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

		// Replace ${VAR} with environment variables
		data = replaceEnvVars(data)

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

// GetChainConfig 获取链配置 (包级函数)
func GetChainConfig(chainName string) (*ChainConfig, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.GetChainConfig(chainName)
}

// InitDB 初始化数据库连接
func InitDB(ctx context.Context) (*sql.DB, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	dbCfg := &DatabaseConfig{
		Host:     cfg.Database.Host,
		Port:     5432, // default
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}
	// Try to parse port from string
	if portStr := cfg.Database.Port; portStr != "" {
		var portInt int
		fmt.Sscanf(portStr, "%d", &portInt)
		dbCfg.Port = portInt
	}
	db, err := NewPostgresDB(dbCfg)
	if err != nil {
		return nil, err
	}
	return db.GetSQLDB()
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

func (c *AppConfig) GetDatabaseDSN(includeDBName bool) (string, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
	)
	if includeDBName {
		dsn += fmt.Sprintf("%s?parseTime=true&charset=utf8mb4", c.Database.DBName)
	} else {
		dsn += "?parseTime=true&charset=utf8mb4"
	}
	return dsn, nil
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

// GetTableName 获取表名
func GetTableName(chainName string) (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	chain, exists := cfg.Chains[chainName]
	if !exists {
		return "", fmt.Errorf("chain %s not found", chainName)
	}
	return chain.TableName, nil
}

// GetRPCManager 获取 RPC 管理器
func GetRPCManager(chainName string, proxy string) (*RPCManager, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	chain, exists := cfg.Chains[chainName]
	if !exists {
		return nil, fmt.Errorf("chain %s not found", chainName)
	}
	return NewRPCManager(chainName, chain.RPCURLs, 30*time.Second, proxy)
}

// GetExplorerConfig 获取浏览器配置
func GetExplorerConfig(chainName string) (*Explorer, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	chain, exists := cfg.Chains[chainName]
	if !exists {
		return nil, fmt.Errorf("chain %s not found", chainName)
	}
	return &chain.Explorer, nil
}

// GetAPIKeyManager 获取 API 密钥管理器
func GetAPIKeyManager(chainName string) (*APIKeyManager, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	chain, exists := cfg.Chains[chainName]
	if !exists {
		return nil, fmt.Errorf("chain %s not found", chainName)
	}
	return NewAPIKeyManager(chain.Explorer.APIKeys, ""), nil
}
