package config

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type RPCManager struct {
	chainName         string
	urls              []string
	clients           []*ethclient.Client
	current           int
	mutex             sync.RWMutex
	timeout           time.Duration
	healthCacheWindow time.Duration
	lastHealthyAt     []time.Time
}

func dialEthClient(rawURL string, timeout time.Duration, proxy string) (*ethclient.Client, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty rpc url")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		httpClient, err := internal.CreateProxyHTTPClient(proxy, timeout)
		if err != nil {
			return nil, err
		}
		rpcClient, err := rpc.DialHTTPWithClient(rawURL, httpClient)
		if err != nil {
			return nil, err
		}
		return ethclient.NewClient(rpcClient), nil
	default:
		return ethclient.Dial(rawURL)
	}
}

func NewRPCManager(chainName string, urls []string, timeout time.Duration, proxy string) (*RPCManager, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("at least one RPC URL is required")
	}

	manager := &RPCManager{
		chainName:         chainName,
		urls:              urls,
		timeout:           timeout,
		clients:           make([]*ethclient.Client, len(urls)),
		healthCacheWindow: 5 * time.Second,
		lastHealthyAt:     make([]time.Time, len(urls)),
	}

	// 初始化所有客户端连接
	for i, url := range urls {
		client, err := dialEthClient(url, timeout, proxy)
		if err != nil {
			// 如果连接失败，记录错误但继续尝试其他URL
			fmt.Printf("⚠️  Failed to connect to RPC [%s]: %v\n", url, err)
			continue
		}
		manager.clients[i] = client
	}

	// 随机选择起始客户端
	manager.current = rand.Intn(len(manager.clients))

	return manager, nil
}

func (r *RPCManager) GetClient() (*ethclient.Client, error) {
	r.mutex.RLock()
	current := r.current
	timeout := r.timeout
	cacheWindow := r.healthCacheWindow
	var client *ethclient.Client
	var lastHealthy time.Time
	if current >= 0 && current < len(r.clients) {
		client = r.clients[current]
		lastHealthy = r.lastHealthyAt[current]
	}
	r.mutex.RUnlock()

	if client != nil {
		if !lastHealthy.IsZero() && time.Since(lastHealthy) < cacheWindow {
			return client, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		_, err := client.BlockNumber(ctx)
		if err == nil {
			r.mutex.Lock()
			if current >= 0 && current < len(r.lastHealthyAt) {
				r.lastHealthyAt[current] = time.Now()
			}
			r.mutex.Unlock()
			return client, nil
		}
	}

	return r.switchToNextClient()
}

func (r *RPCManager) switchToNextClient() (*ethclient.Client, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 尝试所有客户端
	for i := 0; i < len(r.clients); i++ {
		nextIndex := (r.current + 1 + i) % len(r.clients)

		if r.clients[nextIndex] != nil {
			// 测试连接
			ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
			_, err := r.clients[nextIndex].BlockNumber(ctx)
			cancel()

			if err == nil {
				r.current = nextIndex
				if nextIndex >= 0 && nextIndex < len(r.lastHealthyAt) {
					r.lastHealthyAt[nextIndex] = time.Now()
				}
				fmt.Printf("🔄 Switched to RPC: %s\n", r.urls[nextIndex])
				return r.clients[nextIndex], nil
			}
		}
	}

	return nil, fmt.Errorf("all RPC nodes are unavailable")
}

func (r *RPCManager) GetCurrentURL() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.current < len(r.urls) {
		return r.urls[r.current]
	}
	return ""
}

func (r *RPCManager) GetChainName() string {
	return r.chainName
}

func (r *RPCManager) Close() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, client := range r.clients {
		if client != nil {
			client.Close()
		}
	}
}

func (r *RPCManager) GetStatus() map[string]interface{} {
	r.mutex.RLock()
	chainName := r.chainName
	urls := append([]string(nil), r.urls...)
	clients := append([]*ethclient.Client(nil), r.clients...)
	current := r.current
	r.mutex.RUnlock()

	status := map[string]interface{}{
		"chain_name":    chainName,
		"total_urls":    len(urls),
		"current_index": current,
		"current_url": func() string {
			if current < len(urls) && current >= 0 {
				return urls[current]
			}
			return ""
		}(),
		"urls": urls,
	}

	// 检查每个URL的状态
	urlStatus := make([]map[string]interface{}, len(urls))
	for i, url := range urls {
		urlInfo := map[string]interface{}{
			"url":     url,
			"active":  clients[i] != nil,
			"current": i == current,
		}

		// 如果客户端存在，测试连接
		if clients[i] != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := clients[i].BlockNumber(ctx)
			cancel()
			urlInfo["healthy"] = err == nil
		} else {
			urlInfo["healthy"] = false
		}

		urlStatus[i] = urlInfo
	}
	status["url_status"] = urlStatus

	return status
}
