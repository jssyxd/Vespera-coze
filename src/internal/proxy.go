package internal

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type ProxyConfig struct {
	URL     string        // 代理URL，例如 http://127.0.0.1:7897
	Timeout time.Duration // 超时时间
}

type ProxyManager struct {
	config *ProxyConfig
}

var (
	httpClientCacheMu sync.Mutex
	httpClientCache   = map[string]*http.Client{}
)

func NewProxyManager(proxyURL string, timeout time.Duration) (*ProxyManager, error) {
	if strings.TrimSpace(proxyURL) == "" {
		return &ProxyManager{config: nil}, nil
	}

	// 验证代理URL格式
	_, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	return &ProxyManager{
		config: &ProxyConfig{
			URL:     strings.TrimSpace(proxyURL),
			Timeout: timeout,
		},
	}, nil
}

func (pm *ProxyManager) CreateHTTPClient(timeout time.Duration) *http.Client {
	client := &http.Client{
		Timeout: timeout,
	}

	if pm.config != nil {
		proxyURL, _ := url.Parse(pm.config.URL)
		client.Transport = &http.Transport{
			Proxy:               http.ProxyURL(proxyURL),
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		}
	}

	return client
}

func (pm *ProxyManager) CreateHTTPTransport() *http.Transport {
	transport := &http.Transport{
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}

	if pm.config != nil {
		proxyURL, _ := url.Parse(pm.config.URL)
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	return transport
}

func (pm *ProxyManager) IsEnabled() bool {
	return pm.config != nil
}

func (pm *ProxyManager) GetProxyURL() string {
	if pm.config != nil {
		return pm.config.URL
	}
	return ""
}

func ValidateProxyURL(proxyURL string) error {
	if strings.TrimSpace(proxyURL) == "" {
		return nil // 空字符串表示不使用代理
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL format: %w", err)
	}

	// 检查协议
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return fmt.Errorf("unsupported proxy scheme: %s (supported: http, https, socks5)", u.Scheme)
	}

	// 检查主机名
	if u.Host == "" {
		return fmt.Errorf("proxy host cannot be empty")
	}

	return nil
}

func CreateProxyHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	key := strings.TrimSpace(proxyURL) + "|" + timeout.String()
	httpClientCacheMu.Lock()
	if cached := httpClientCache[key]; cached != nil {
		httpClientCacheMu.Unlock()
		return cached, nil
	}
	httpClientCacheMu.Unlock()

	pm, err := NewProxyManager(proxyURL, timeout)
	if err != nil {
		return nil, err
	}

	client := pm.CreateHTTPClient(timeout)

	httpClientCacheMu.Lock()
	if len(httpClientCache) >= 32 {
		httpClientCache = map[string]*http.Client{}
	}
	httpClientCache[key] = client
	httpClientCacheMu.Unlock()

	return client, nil
}

func CreateProxyTransport(proxyURL string) (*http.Transport, error) {
	pm, err := NewProxyManager(proxyURL, 0)
	if err != nil {
		return nil, err
	}

	return pm.CreateHTTPTransport(), nil
}
