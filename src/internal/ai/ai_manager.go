package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VectorBits/Vespera/src/internal/ai/parser"
	"github.com/VectorBits/Vespera/src/internal/config"
	"github.com/VectorBits/Vespera/src/internal/logger"
)

// qhello Manager AI 客户端管理器
type Manager struct {
	client          AIClient
	parser          *parser.Parser
	rateLimit       *rateLimiter
	timeout         time.Duration
	verbose         bool
	limiter         *concurrencyLimiter
	baseInterval    time.Duration
	baseConcurrency int
	latencyEWMA     time.Duration
	mu              sync.Mutex
}

type jsonAnalyzer interface {
	AnalyzeJSON(ctx context.Context, prompt string) (string, error)
}

type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
	closed   bool
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	if requestsPerMinute <= 0 {
		requestsPerMinute = 1
	}
	interval := time.Minute / time.Duration(requestsPerMinute)
	return &rateLimiter{interval: interval}
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	if rl.closed {
		rl.mu.Unlock()
		return fmt.Errorf("rate limiter closed")
	}
	now := time.Now()
	wait := time.Duration(0)
	if !rl.last.IsZero() {
		next := rl.last.Add(rl.interval)
		if next.After(now) {
			wait = next.Sub(now)
		}
	}
	rl.last = now.Add(wait)
	rl.mu.Unlock()
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (rl *rateLimiter) UpdateInterval(interval time.Duration) {
	if rl == nil || interval <= 0 {
		return
	}
	rl.mu.Lock()
	rl.interval = interval
	rl.mu.Unlock()
}

func (rl *rateLimiter) Close() {
	if rl == nil {
		return
	}
	rl.mu.Lock()
	rl.closed = true
	rl.mu.Unlock()
}

type concurrencyLimiter struct {
	mu       sync.Mutex
	inflight int
	max      int
}

func newConcurrencyLimiter(max int) *concurrencyLimiter {
	if max <= 0 {
		max = 1
	}
	return &concurrencyLimiter{max: max}
}

func (l *concurrencyLimiter) Acquire(ctx context.Context) error {
	for {
		l.mu.Lock()
		if l.inflight < l.max {
			l.inflight++
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (l *concurrencyLimiter) Release() {
	l.mu.Lock()
	if l.inflight > 0 {
		l.inflight--
	}
	l.mu.Unlock()
}

func (l *concurrencyLimiter) UpdateMax(max int) {
	if max <= 0 {
		max = 1
	}
	l.mu.Lock()
	l.max = max
	l.mu.Unlock()
}

type multiAIClient struct {
	clients []AIClient
	next    uint64
}

func (m *multiAIClient) pick() AIClient {
	if len(m.clients) == 0 {
		return nil
	}
	idx := atomic.AddUint64(&m.next, 1)
	return m.clients[int(idx)%len(m.clients)]
}

func (m *multiAIClient) Analyze(ctx context.Context, prompt string) (string, error) {
	client := m.pick()
	if client == nil {
		return "", fmt.Errorf("no available ai client")
	}
	return client.Analyze(ctx, prompt)
}

func (m *multiAIClient) AnalyzeJSON(ctx context.Context, prompt string) (string, error) {
	client := m.pick()
	if client == nil {
		return "", fmt.Errorf("no available ai client")
	}
	if jsonClient, ok := client.(jsonAnalyzer); ok {
		return jsonClient.AnalyzeJSON(ctx, prompt)
	}
	return client.Analyze(ctx, prompt)
}

func (m *multiAIClient) GetName() string {
	if len(m.clients) == 0 {
		return "multi"
	}
	return fmt.Sprintf("multi[%d]-%s", len(m.clients), m.clients[0].GetName())
}

func (m *multiAIClient) Close() error {
	for _, c := range m.clients {
		if c != nil {
			_ = c.Close()
		}
	}
	return nil
}

func parseAPIKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})
	keys := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		k := strings.TrimSpace(p)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	return keys
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type ManagerConfig struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	Timeout        time.Duration
	Proxy          string
	RequestsPerMin int
	Strategy       string
	Verbose        bool
}

func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.APIKey == "" {
		appConfig, err := config.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}

		aiConfig, err := appConfig.GetAIConfig(cfg.Provider)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI config: %w", err)
		}

		cfg.APIKey = aiConfig.APIKey
		if cfg.BaseURL == "" {
			cfg.BaseURL = aiConfig.BaseURL
		}
		if cfg.Model == "" {
			cfg.Model = aiConfig.Model
		}
		if cfg.Proxy == "" {
			cfg.Proxy = aiConfig.Proxy
		}
	}

	keys := parseAPIKeys(cfg.APIKey)
	if len(keys) == 0 {
		keys = []string{cfg.APIKey}
	}
	var client AIClient
	if len(keys) == 1 {
		single, err := NewAIClient(AIClientConfig{
			Provider: cfg.Provider,
			APIKey:   keys[0],
			BaseURL:  cfg.BaseURL,
			Model:    cfg.Model,
			Timeout:  cfg.Timeout,
			Proxy:    cfg.Proxy,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create AI client: %w", err)
		}
		client = single
	} else {
		clients := make([]AIClient, 0, len(keys))
		for _, key := range keys {
			c, err := NewAIClient(AIClientConfig{
				Provider: cfg.Provider,
				APIKey:   key,
				BaseURL:  cfg.BaseURL,
				Model:    cfg.Model,
				Timeout:  cfg.Timeout,
				Proxy:    cfg.Proxy,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create AI client: %w", err)
			}
			clients = append(clients, c)
		}
		client = &multiAIClient{clients: clients}
	}

	if cfg.RequestsPerMin <= 0 {
		cfg.RequestsPerMin = 20
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	keyCount := len(keys)
	requestsPerMin := cfg.RequestsPerMin
	if keyCount > 1 {
		requestsPerMin = cfg.RequestsPerMin * keyCount
	}
	baseInterval := time.Minute / time.Duration(maxInt(1, requestsPerMin))
	baseConcurrency := maxInt(1, minInt(32, maxInt(keyCount*2, cfg.RequestsPerMin/30)))

	return &Manager{
		client:          client,
		parser:          parser.NewParser(cfg.Strategy),
		rateLimit:       newRateLimiter(requestsPerMin),
		timeout:         cfg.Timeout,
		verbose:         cfg.Verbose,
		limiter:         newConcurrencyLimiter(baseConcurrency),
		baseInterval:    baseInterval,
		baseConcurrency: baseConcurrency,
	}, nil
}

func (m *Manager) AnalyzeContract(ctx context.Context, contractCode, prompt string) (*parser.AnalysisResult, error) {
	return m.AnalyzeContractWithStrategy(ctx, contractCode, prompt, m.parser.Strategy)
}

// qhello AnalyzeContractWithStrategy 核心分析方法
func (m *Manager) AnalyzeContractWithStrategy(ctx context.Context, contractCode, prompt, strategy string) (*parser.AnalysisResult, error) {
	reqCtx := ctx
	cancel := func() {}
	if m.timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, m.timeout)
	}
	defer cancel()

	if err := m.rateLimit.Wait(reqCtx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}
	if m.limiter != nil {
		if err := m.limiter.Acquire(reqCtx); err != nil {
			return nil, err
		}
		defer m.limiter.Release()
	}

	fullPrompt := prompt
	snippet := strings.TrimSpace(contractCode)
	if len(snippet) > 200 {
		snippet = snippet[:200]
	}
	lower := strings.ToLower(fullPrompt)
	if !(strings.Contains(fullPrompt, snippet) || strings.Contains(lower, "```solidity")) {
		fullPrompt = fmt.Sprintf("%s\n\nContract Code:\n```solidity\n%s\n```", prompt, contractCode)
	}
	fullPrompt = fullPrompt + "\n\n" + schemaInstruction(strategy)

	startTime := time.Now()

	if m.verbose {
		logger.InfoFileOnly("🤖 Sending AI Request [Strategy: %s] (prompt_len=%d)", strategy, len(fullPrompt))
		logger.InfoFileOnly("%s", fullPrompt)
	} else {
		logger.InfoFileOnly("🤖 Sending AI Request [Strategy: %s] (prompt_len=%d prompt_sha256=%s)", strategy, len(fullPrompt), hashForLog(fullPrompt))
	}

	var response string
	var err error

	if jsonClient, ok := m.client.(jsonAnalyzer); ok {
		response, err = jsonClient.AnalyzeJSON(reqCtx, fullPrompt)
	} else {
		response, err = m.client.Analyze(reqCtx, fullPrompt)
	}
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	if m.verbose {
		logger.InfoFileOnly("✅ AI Response Received (resp_len=%d)", len(response))
		logger.InfoFileOnly("%s", response)
	} else {
		logger.InfoFileOnly("✅ AI Response Received (resp_len=%d resp_sha256=%s)", len(response), hashForLog(response))
	}

	strategyParser := parser.NewParser(strategy)
	result, parseErr := strategyParser.Parse(response)

	if (parseErr != nil || (result != nil && result.ParseError != "")) && canRetryParse(reqCtx) {
		reformatPrompt := buildReformatPrompt(strategy, response)
		var retryResp string
		if jsonClient, ok := m.client.(jsonAnalyzer); ok {
			retryResp, err = jsonClient.AnalyzeJSON(reqCtx, reformatPrompt)
		} else {
			retryResp, err = m.client.Analyze(reqCtx, reformatPrompt)
		}
		if err == nil {
			retryResult, retryParseErr := strategyParser.Parse(retryResp)
			if retryParseErr == nil && retryResult != nil && retryResult.ParseError == "" {
				response = retryResp
				result = retryResult
				parseErr = nil
			}
		}
	}

	if result == nil {
		result = &parser.AnalysisResult{}
	}
	if result.ParseError == "" && parseErr != nil {
		result.ParseError = parseErr.Error()
	}

	duration := time.Since(startTime)
	result.RawResponse = response
	result.AnalysisDuration = duration
	m.adjustLimits(duration)

	return result, nil
}

func hashForLog(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) adjustLimits(duration time.Duration) {
	m.mu.Lock()
	if m.latencyEWMA == 0 {
		m.latencyEWMA = duration
	} else {
		m.latencyEWMA = time.Duration(float64(m.latencyEWMA)*0.8 + float64(duration)*0.2)
	}
	ewma := m.latencyEWMA
	baseInterval := m.baseInterval
	baseConcurrency := m.baseConcurrency
	m.mu.Unlock()

	targetInterval := baseInterval
	targetConcurrency := baseConcurrency
	if ewma > 25*time.Second {
		targetInterval = ewma / 2
		if targetInterval < baseInterval {
			targetInterval = baseInterval
		}
		targetConcurrency = maxInt(1, baseConcurrency/2)
	}
	if ewma > 40*time.Second {
		targetConcurrency = 1
	}

	if m.rateLimit != nil {
		m.rateLimit.UpdateInterval(targetInterval)
	}
	if m.limiter != nil {
		m.limiter.UpdateMax(targetConcurrency)
	}
}

func canRetryParse(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	if deadline, ok := ctx.Deadline(); ok {
		return time.Until(deadline) > 2*time.Second
	}
	return true
}

func schemaInstruction(strategy string) string {
	if strategy == "mode2_fuzzy" {
		return `Output ONLY one JSON object:
{"is_vulnerability":true|false,"severity":"Critical|High|Medium|Low|None|Unknown","reason":"...","vuln_type":"..."}
No markdown, no extra text.`
	}

	return `Output ONLY one JSON object:
{"contract_address":"0x...","risk_score":0,"vuln_probability":"85%|High|Medium|Low","severity":"Critical|High|Medium|Low|None|Unknown","summary":"...","recommendations":["..."],"vulnerabilities":[{"type":"...","severity":"Critical|High|Medium|Low|Unknown","description":"...","location":"...","line_numbers":[1,2]}]}
No markdown, no extra text. Use [] for empty lists.`
}

func buildReformatPrompt(strategy, text string) string {
	const maxText = 64 * 1024
	if len(text) > maxText {
		text = text[:maxText]
	}
	return fmt.Sprintf("Convert the following text into the required JSON.\n\n%s\n\nTEXT:\n%s", schemaInstruction(strategy), text)
}

func (m *Manager) AnalyzeBatch(ctx context.Context, contracts []ContractInput, concurrency int) ([]*parser.AnalysisResult, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	results := make([]*parser.AnalysisResult, len(contracts))
	errChan := make(chan error, len(contracts))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, contract := range contracts {
		wg.Add(1)
		go func(idx int, c ContractInput) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := m.AnalyzeContract(ctx, c.Code, c.Prompt)
			if err != nil {
				errChan <- fmt.Errorf("contract %d (%s) failed: %w", idx, c.Address, err)
				return
			}

			result.ContractAddress = c.Address
			results[idx] = result
		}(i, contract)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("batch analysis completed with %d errors: %v", len(errs), errs[0])
	}

	return results, nil
}

type ContractInput struct {
	Address string
	Code    string
	Prompt  string
}

func (m *Manager) GetClientInfo() string {
	return m.client.GetName()
}

func (m *Manager) GetParser() *parser.Parser {
	return m.parser
}

func (m *Manager) Close() error {
	if m.rateLimit != nil {
		m.rateLimit.Close()
	}
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

func (m *Manager) TestConnection(ctx context.Context) error {
	fmt.Println("🔍 Testing AI client connection...")
	testPrompt := "Please respond with 'OK' if you can read this message."
	_, err := m.client.Analyze(ctx, testPrompt)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	fmt.Println("✅ AI client connected successfully!")
	return nil
}
