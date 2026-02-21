package config

import (
	"math/rand"
	"sync"
	"time"
)

type APIKeyManager struct {
	apiKeys []string
	current int
	mutex   sync.RWMutex
	rng     *rand.Rand
}

func NewAPIKeyManager(apiKeys []string, fallbackKey string) *APIKeyManager {
	// 过滤空字符串
	validKeys := make([]string, 0, len(apiKeys)+1)
	for _, key := range apiKeys {
		if key != "" {
			validKeys = append(validKeys, key)
		}
	}

	// 如果没有有效的 keys，使用 fallback
	if len(validKeys) == 0 && fallbackKey != "" {
		validKeys = append(validKeys, fallbackKey)
	}

	// 如果仍然为空，返回 nil
	if len(validKeys) == 0 {
		return nil
	}

	manager := &APIKeyManager{
		apiKeys: validKeys,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// 随机选择起始 key
	manager.current = manager.rng.Intn(len(manager.apiKeys))

	return manager
}

func (m *APIKeyManager) GetKey() string {
	if m == nil || len(m.apiKeys) == 0 {
		return ""
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.apiKeys[m.current]
}

func (m *APIKeyManager) GetNextKey() string {
	if m == nil || len(m.apiKeys) == 0 {
		return ""
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 切换到下一个 key
	m.current = (m.current + 1) % len(m.apiKeys)
	return m.apiKeys[m.current]
}

func (m *APIKeyManager) GetRandomKey() string {
	if m == nil || len(m.apiKeys) == 0 {
		return ""
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	idx := m.rng.Intn(len(m.apiKeys))
	return m.apiKeys[idx]
}

func (m *APIKeyManager) GetKeyCount() int {
	if m == nil {
		return 0
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.apiKeys)
}

func (m *APIKeyManager) HasKeys() bool {
	return m != nil && len(m.apiKeys) > 0
}
