package parser

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type AIVerificationResult struct {
	IsVulnerability bool   `json:"is_vulnerability"`
	Severity        string `json:"severity"`
	Reason          string `json:"reason"`
	VulnType        string `json:"vuln_type"`
}

type Parser struct {
	jsonExtractor *regexp.Regexp
	Strategy      string
}

func NewParser(strategy string) *Parser {
	jsonRegex := regexp.MustCompile("(?s)```(?:json)?\\s*({.*?})\\s*```")
	return &Parser{
		jsonExtractor: jsonRegex,
		Strategy:      strategy,
	}
}

// helloq Parse 解析 AI 响应
func (p *Parser) Parse(response string) (*AnalysisResult, error) {
	if p.Strategy == "mode2_fuzzy" {
		vr, err := p.ParseVerificationResult(response)
		if err != nil {
			return &AnalysisResult{
				RawResponse: response,
				ParseError:  err.Error(),
			}, nil
		}
		vulns := []Vulnerability{}
		if vr.IsVulnerability {
			vulns = append(vulns, Vulnerability{
				Type:        vr.VulnType,
				Severity:    vr.Severity,
				Description: vr.Reason,
			})
		}
		return &AnalysisResult{
			Vulnerabilities: vulns,
			Summary:         vr.Reason,
			RawResponse:     response,
		}, nil
	}

	var result AnalysisResult

	// 尝试直接解析
	if err := json.Unmarshal([]byte(response), &result); err == nil {
		normalizeAnalysisResult(&result)
		return &result, nil
	}

	var vulnsOnly []Vulnerability
	if err := json.Unmarshal([]byte(response), &vulnsOnly); err == nil {
		result.Vulnerabilities = vulnsOnly
		normalizeAnalysisResult(&result)
		return &result, nil
	}

	// 清理 Markdown 标记后解析
	cleaned := p.cleanResponse(response)
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		normalizeAnalysisResult(&result)
		return &result, nil
	}

	if err := json.Unmarshal([]byte(cleaned), &vulnsOnly); err == nil {
		result.Vulnerabilities = vulnsOnly
		normalizeAnalysisResult(&result)
		return &result, nil
	}

	if jsonPart, ok := extractFirstJSONObject(cleaned); ok {
		if err := json.Unmarshal([]byte(jsonPart), &result); err == nil {
			normalizeAnalysisResult(&result)
			return &result, nil
		}
	}

	if jsonPart, ok := extractFirstJSONObject(response); ok {
		if err := json.Unmarshal([]byte(jsonPart), &result); err == nil {
			normalizeAnalysisResult(&result)
			return &result, nil
		}
	}

	// 如果还是失败，尝试修复常见的 JSON 错误（如未闭合的引号，虽难但可尝试）或者回退到文本解析
	return p.parseTextFormat(response)
}

func (p *Parser) ParseVerificationResult(response string) (*AIVerificationResult, error) {
	var result AIVerificationResult

	if err := json.Unmarshal([]byte(response), &result); err == nil {
		normalizeVerificationResult(&result)
		return &result, nil
	}

	cleaned := p.cleanResponse(response)
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil {
		normalizeVerificationResult(&result)
		return &result, nil
	}

	if jsonPart, ok := extractFirstJSONObject(cleaned); ok {
		if err := json.Unmarshal([]byte(jsonPart), &result); err == nil {
			normalizeVerificationResult(&result)
			return &result, nil
		}
	}

	if jsonPart, ok := extractFirstJSONObject(response); ok {
		if err := json.Unmarshal([]byte(jsonPart), &result); err == nil {
			normalizeVerificationResult(&result)
			return &result, nil
		}
	}

	return nil, fmt.Errorf("failed to parse JSON verification result")
}

func normalizeVerificationResult(r *AIVerificationResult) {
	if r == nil {
		return
	}

	if strings.TrimSpace(r.Reason) == "" {
		r.Reason = "No reason provided"
	}
	if strings.TrimSpace(r.VulnType) == "" {
		r.VulnType = "Unknown"
	}
	r.Severity = normalizeSeverity(r.Severity)
	if !r.IsVulnerability {
		r.Severity = "None"
	}
}

func normalizeSeverity(severity string) string {
	s := strings.TrimSpace(severity)
	if s == "" {
		return "Unknown"
	}

	lower := strings.ToLower(s)
	switch lower {
	case "none", "null", "nil":
		return "None"
	case "safe", "secure", "healthy", "安全":
		return "None"
	case "low", "l", "低":
		return "Low"
	case "medium", "med", "m", "中":
		return "Medium"
	case "high", "h", "高":
		return "High"
	case "critical", "crit", "c", "严重":
		return "Critical"
	default:
		switch s {
		case "Low", "Medium", "High", "Critical", "None":
			return s
		default:
			return "Unknown"
		}
	}
}

func (p *Parser) cleanResponse(response string) string {
	matches := p.jsonExtractor.FindStringSubmatch(response)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	response = strings.TrimPrefix(strings.TrimSpace(response), "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	return strings.TrimSpace(response)
}

func extractFirstJSONObject(s string) (string, bool) {
	start := -1
	depth := 0
	inString := false
	escape := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inString {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			continue
		}

		if ch == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			continue
		}

		if ch == '}' {
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start != -1 {
				return s[start : i+1], true
			}
		}
	}

	return "", false
}

func normalizeAnalysisResult(r *AnalysisResult) {
	if r == nil {
		return
	}

	if strings.TrimSpace(r.Summary) == "" {
		r.Summary = "No summary provided"
	}
	if r.Vulnerabilities == nil {
		r.Vulnerabilities = []Vulnerability{}
	}
	if r.Recommendations == nil {
		r.Recommendations = []string{}
	}

	r.Severity = normalizeSeverity(r.Severity)
	for i := range r.Vulnerabilities {
		normalizeVulnerability(&r.Vulnerabilities[i])
	}
}

func normalizeVulnerability(v *Vulnerability) {
	if v == nil {
		return
	}

	if strings.TrimSpace(v.Type) == "" {
		v.Type = "Unknown"
	}
	v.Severity = normalizeSeverity(v.Severity)
	if strings.TrimSpace(v.Description) == "" {
		v.Description = "No description provided"
	}
}

// qhello 提示词json用这些
type AnalysisResult struct {
	ContractAddress  string          `json:"contract_address,omitempty"`
	Vulnerabilities  []Vulnerability `json:"vulnerabilities"`
	Summary          string          `json:"summary,omitempty"`
	RiskScore        interface{}     `json:"risk_score,omitempty"`
	VulnProbability  string          `json:"vuln_probability,omitempty"`
	Severity         string          `json:"severity,omitempty"`
	Recommendations  []string        `json:"recommendations,omitempty"`
	RawResponse      string          `json:"-"`
	ParseError       string          `json:"parse_error,omitempty"`
	AnalysisDuration time.Duration   `json:"-"`
}

type Vulnerability struct {
	Type        string   `json:"type"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Location    string   `json:"location,omitempty"`
	LineNumbers []int    `json:"line_numbers,omitempty"`
	CodeSnippet string   `json:"code_snippet,omitempty"`
	Impact      string   `json:"impact,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
	References  []string `json:"references,omitempty"`
	SWCID       string   `json:"swc_id,omitempty"`
}

func (r *AnalysisResult) GetHighSeverityCount() int {
	count := 0
	for _, v := range r.Vulnerabilities {
		if v.Severity == "Critical" || v.Severity == "High" {
			count++
		}
	}
	return count
}

func (r *AnalysisResult) HasCriticalVulnerabilities() bool {
	for _, v := range r.Vulnerabilities {
		if v.Severity == "Critical" {
			return true
		}
	}
	return false
}

func (r *AnalysisResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *Parser) parseTextFormat(response string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		RawResponse: response,
		ParseError:  "failed to parse JSON; used text fallback",
	}

	if strings.Contains(response, "漏洞等级：严重") || strings.Contains(response, "Severity: Critical") {
		result.Vulnerabilities = append(result.Vulnerabilities, Vulnerability{
			Type:        "Unknown (Text Parsed)",
			Severity:    "Critical",
			Description: "Parsed from text response",
		})
	} else if strings.Contains(response, "漏洞等级：高") || strings.Contains(response, "Severity: High") {
		result.Vulnerabilities = append(result.Vulnerabilities, Vulnerability{
			Type:        "Unknown (Text Parsed)",
			Severity:    "High",
			Description: "Parsed from text response",
		})
	} else if strings.Contains(response, "漏洞等级：中") || strings.Contains(response, "Severity: Medium") {
		result.Vulnerabilities = append(result.Vulnerabilities, Vulnerability{
			Type:        "Unknown (Text Parsed)",
			Severity:    "Medium",
			Description: "Parsed from text response",
		})
	}

	normalizeAnalysisResult(result)
	return result, nil
}

func (p *Parser) extractValue(text, startKey, endKey string) string {
	startIndex := strings.Index(text, startKey)
	if startIndex == -1 {
		return ""
	}

	startIndex += len(startKey)
	var endIndex int

	if endKey != "" {
		endIndex = strings.Index(text[startIndex:], endKey)
		if endIndex == -1 {
			endIndex = len(text)
		} else {
			endIndex += startIndex
		}
	} else {
		endIndex = strings.Index(text[startIndex:], "\n")
		if endIndex == -1 {
			endIndex = len(text)
		} else {
			endIndex += startIndex
		}
	}

	value := strings.TrimSpace(text[startIndex:endIndex])
	return value
}

func (p *Parser) mapSeverity(severity string) string {
	severity = strings.TrimSpace(severity)
	switch severity {
	case "低":
		return "Low"
	case "中":
		return "Medium"
	case "高":
		return "High"
	case "严重":
		return "Critical"
	default:
		return "Unknown"
	}
}

func (p *Parser) extractPercentage(text string) string {
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1] + "%"
	}
	return ""
}
