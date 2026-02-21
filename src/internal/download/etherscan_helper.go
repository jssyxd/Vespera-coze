package download

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
)

type EtherscanConfig struct {
	APIKey        string
	APIKeyManager interface {
		GetRandomKey() string
		HasKeys() bool
	}
	BaseURL string
	Proxy   string
	ChainID int
}

type EtherscanResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Result  interface{} `json:"result"`
}

type EtherscanContractInfo struct {
	SourceCode           string `json:"SourceCode"`
	ABI                  string `json:"ABI"`
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	ConstructorArguments string `json:"ConstructorArguments"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
}

func GetContractDetails(address string, config EtherscanConfig) (*EtherscanContractInfo, bool, error) {
	sourceCode, isVerified, raw, err := getContractSourceRaw(address, config)
	if err != nil {
		return nil, false, err
	}
	if !isVerified {
		return nil, false, nil
	}

	processedSourceCode := sourceCode
	if strings.TrimSpace(sourceCode) != "" {
		trimmed := strings.TrimSpace(sourceCode)
		if strings.HasPrefix(trimmed, "{") {
			extracted, success, extractErr := extractSourcesFromJSON(sourceCode)
			if extractErr == nil && success {
				processedSourceCode = extracted
			}
		}
	}

	if len(raw) == 0 {
		return &EtherscanContractInfo{SourceCode: processedSourceCode}, true, nil
	}
	entry := raw[0]
	info := &EtherscanContractInfo{
		SourceCode:           processedSourceCode,
		ABI:                  entry.ABI,
		ContractName:         entry.ContractName,
		CompilerVersion:      entry.CompilerVersion,
		OptimizationUsed:     entry.OptimizationUsed,
		Runs:                 entry.Runs,
		ConstructorArguments: entry.ConstructorArguments,
		EVMVersion:           entry.EVMVersion,
		Library:              entry.Library,
		LicenseType:          entry.LicenseType,
		Proxy:                entry.Proxy,
		Implementation:       entry.Implementation,
		SwarmSource:          entry.SwarmSource,
	}
	return info, true, nil
}

func getContractSourceRaw(address string, config EtherscanConfig) (sourceCode string, isVerified bool, raw []EtherscanContractInfo, err error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", false, nil, fmt.Errorf("empty address passed to GetContractSource")
	}

	base := strings.TrimRight(config.BaseURL, "/")
	u, err := url.Parse(base)
	if err != nil {
		return "", false, nil, fmt.Errorf("failed to parse Etherscan BaseURL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api"

	var apiKey string
	if config.APIKeyManager != nil && config.APIKeyManager.HasKeys() {
		apiKey = config.APIKeyManager.GetRandomKey()
	}
	if apiKey == "" {
		apiKey = config.APIKey
	}

	q := url.Values{}
	q.Set("module", "contract")
	q.Set("action", "getsourcecode")
	q.Set("address", address)
	q.Set("apikey", strings.TrimSpace(apiKey))
	if config.ChainID > 0 {
		q.Set("chainid", fmt.Sprintf("%d", config.ChainID))
	}

	u.RawQuery = q.Encode()
	finalURL := u.String()

	client, err := internal.CreateProxyHTTPClient(config.Proxy, 20*time.Second)
	if err != nil {
		return "", false, nil, fmt.Errorf("failed to create Etherscan HTTP client: %w", err)
	}

	var lastErr error
	maxAttempts := 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, _ := http.NewRequest("GET", finalURL, nil)
		req.Header.Set("User-Agent", "Vespera/1.0 (+https://github.com/)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if isTemporaryNetErr(err) && attempt < maxAttempts {
				sleep := time.Duration(attempt) * 500 * time.Millisecond
				time.Sleep(sleep)
				continue
			}
			return "", false, nil, fmt.Errorf("failed to request Etherscan API: %w (url=%s)", err, finalURL)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if (readErr == io.ErrUnexpectedEOF || isTemporaryNetErr(readErr)) && attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return "", false, nil, fmt.Errorf("failed to read Etherscan response: %w (url=%s)", readErr, finalURL)
		}

		if resp.StatusCode != http.StatusOK {
			snippet := string(body)
			if len(snippet) > 1024 {
				snippet = snippet[:1024]
			}
			return "", false, nil, fmt.Errorf("etherscan returned non-200 status: %d, body: %s", resp.StatusCode, snippet)
		}

		var etherscanResp EtherscanResponse
		if jerr := json.Unmarshal(body, &etherscanResp); jerr != nil {
			lastErr = jerr
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
				continue
			}
			return "", false, nil, fmt.Errorf("failed to parse Etherscan JSON: %w (url=%s)", jerr, finalURL)
		}

		if etherscanResp.Status != "1" {
			fmt.Printf("⚠️  API returned error: %s - %v\n", etherscanResp.Message, etherscanResp.Result)
			return "", false, nil, nil
		}

		switch result := etherscanResp.Result.(type) {
		case []interface{}:
			if len(result) == 0 {
				return "", false, nil, nil
			}
			var infos []EtherscanContractInfo
			bs, _ := json.Marshal(result)
			_ = json.Unmarshal(bs, &infos)
			if len(infos) > 0 {
				return infos[0].SourceCode, true, infos, nil
			}
			return "", true, nil, nil
		case string:
			fmt.Printf("⚠️  API returned error string: %s\n", result)
			return "", false, nil, nil
		default:
			fmt.Printf("⚠️  Unknown response format: %T\n", result)
			return "", false, nil, nil
		}
	}

	if lastErr != nil {
		return "", false, nil, fmt.Errorf("request to Etherscan failed multiple times: %w (url=%s)", lastErr, finalURL)
	}
	return "", false, nil, fmt.Errorf("request to Etherscan failed with unknown error (url=%s)", finalURL)
}

func GetContractSource(address string, config EtherscanConfig) (sourceCode string, isVerified bool, err error) {
	source, verified, _, err := getContractSourceRaw(address, config)
	if err != nil || !verified {
		return source, verified, err
	}

	processedSourceCode := source
	if strings.TrimSpace(source) != "" {
		trimmed := strings.TrimSpace(source)
		if strings.HasPrefix(trimmed, "{") {
			extracted, success, extractErr := extractSourcesFromJSON(source)
			if extractErr == nil && success {
				processedSourceCode = extracted
			}
		}
	}

	return processedSourceCode, verified, err
}

func extractSourcesFromJSON(jsonStr string) (string, bool, error) {
	var data struct {
		Sources map[string]interface{} `json:"sources"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return jsonStr, false, nil
	}

	if len(data.Sources) > 0 {
		sourcesJSON, err := json.Marshal(data.Sources)
		if err != nil {
			return "", false, fmt.Errorf("failed to serialize sources: %w", err)
		}
		return string(sourcesJSON), true, nil
	}

	return jsonStr, false, nil
}

func isTemporaryNetErr(err error) bool {
	if err == nil {
		return false
	}
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout() || ne.Temporary()
	}
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		return true
	}
	return false
}

type RateLimiter struct {
	ticker *time.Ticker
}

func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	interval := time.Second / time.Duration(requestsPerSecond)
	return &RateLimiter{
		ticker: time.NewTicker(interval),
	}
}

func (r *RateLimiter) Wait() {
	<-r.ticker.C
}

func (r *RateLimiter) Stop() {
	r.ticker.Stop()
}
