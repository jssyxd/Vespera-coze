package ai

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"vespera/internal/config"
)

type MultiModelClient struct {
	client  *Client
	models  map[string]config.ModelConfig
	baseURL string
}

func NewMultiModelClient(cfg config.AIConfig) *MultiModelClient {
	return &MultiModelClient{
		client:  NewClient(cfg.APIKey, cfg.BaseURL),
		models:  cfg.Models,
		baseURL: cfg.BaseURL,
	}
}

func (m *MultiModelClient) AnalyzeWithModel(modelKey, contractCode string) (*AnalysisResult, error) {
	modelCfg, exists := m.models[modelKey]
	if !exists {
		return nil, fmt.Errorf("unknown model: %s", modelKey)
	}

	log.Printf("🤖 Analyzing with %s (%s)...", modelKey, modelCfg.Name)

	result, err := m.client.AnalyzeContract(modelCfg.Name, contractCode)
	if err != nil {
		return nil, fmt.Errorf("%s analysis failed: %w", modelKey, err)
	}

	return result, nil
}

func (m *MultiModelClient) ParallelAnalyze(contractCode string, modelKeys []string) (*MultiModelResult, error) {
	if len(modelKeys) == 0 {
		modelKeys = []string{"deepseek", "glm", "minimax"}
	}

	log.Printf("🔄 Starting parallel analysis with %d models...", len(modelKeys))

	results := make(map[string]*AnalysisResult)
	errors := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, key := range modelKeys {
		wg.Add(1)
		go func(modelKey string) {
			defer wg.Done()

			result, err := m.AnalyzeWithModel(modelKey, contractCode)

			mu.Lock()
			if err != nil {
				errors[modelKey] = err
				log.Printf("❌ %s failed: %v", modelKey, err)
			} else {
				results[modelKey] = result
				log.Printf("✅ %s completed, found %d vulnerabilities", modelKey, len(result.Vulnerabilities))
			}
			mu.Unlock()
		}(key)
	}

	wg.Wait()

	// Aggregate results
	aggregated := m.aggregateResults(results)

	return &MultiModelResult{
		IndividualResults: results,
		Errors:            errors,
		Aggregated:        aggregated,
	}, nil
}

func (m *MultiModelClient) aggregateResults(results map[string]*AnalysisResult) *AggregatedResult {
	vulnMap := make(map[string]*Vulnerability)
	arbMap := make(map[string]*ArbitrageOpportunity)
	var summaries []string

	for model, result := range results {
		summaries = append(summaries, fmt.Sprintf("[%s] %s", model, result.Summary))

		for _, v := range result.Vulnerabilities {
			key := fmt.Sprintf("%s:%d", v.Type, v.Line)
			if existing, ok := vulnMap[key]; ok {
				// Merge confidence
				existing.Severity = maxSeverity(existing.Severity, v.Severity)
			} else {
				vulnMap[key] = &v
			}
		}

		for _, a := range result.ArbitrageOpportunities {
			key := a.Type
			arbMap[key] = &a
		}
	}

	// Convert maps to slices
	var vulnerabilities []Vulnerability
	for _, v := range vulnMap {
		vulnerabilities = append(vulnerabilities, *v)
	}

	var arbitrage []ArbitrageOpportunity
	for _, a := range arbMap {
		arbitrage = append(arbitrage, *a)
	}

	return &AggregatedResult{
		Vulnerabilities:        vulnerabilities,
		ArbitrageOpportunities: arbitrage,
		ConsensusSummary:       fmt.Sprintf("Analysis from %d models. %s", len(results), time.Now().Format(time.RFC3339)),
		ModelCount:             len(results),
	}
}

func maxSeverity(a, b string) string {
	severityOrder := map[string]int{
		"CRITICAL": 4,
		"HIGH":     3,
		"MEDIUM":   2,
		"LOW":      1,
		"INFO":     0,
	}

	if severityOrder[a] > severityOrder[b] {
		return a
	}
	return b
}

type MultiModelResult struct {
	IndividualResults map[string]*AnalysisResult
	Errors            map[string]error
	Aggregated        *AggregatedResult
}

type AggregatedResult struct {
	Vulnerabilities        []Vulnerability        `json:"vulnerabilities"`
	ArbitrageOpportunities []ArbitrageOpportunity `json:"arbitrage_opportunities"`
	ConsensusSummary       string                 `json:"consensus_summary"`
	ModelCount             int                    `json:"model_count"`
}

func (a *AggregatedResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}
