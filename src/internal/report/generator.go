package report

import (
	"fmt"
	"strings"
	"time"
)

type ScanResult struct {
	ContractAddress string
	ScanTime        time.Time
	Status          string
	Vulnerabilities []Vulnerability
	AnalysisSummary string
	RawResponse     string
	RiskScore       string
	VulnProbability string
}

type Vulnerability struct {
	Type        string
	Severity    string
	Description string
}

type Report struct {
	Mode                 string
	Strategy             string
	AIProvider           string
	ScanTime             time.Time
	TotalContracts       int
	VulnerableContracts  int
	SeverityDistribution map[string]int
	Results              []ScanResult
}

type Generator interface {
	Generate(report *Report) (string, error)
}

type MarkdownGenerator struct{}

func NewMarkdownGenerator() *MarkdownGenerator {
	return &MarkdownGenerator{}
}

// helloq Generate 生成 markdown 报告
func (g *MarkdownGenerator) Generate(report *Report) (string, error) {
	var result string

	// 报告头部
	result += fmt.Sprintf("# Vespera Scan Report\n\n")
	result += fmt.Sprintf("**Scan Mode**: %s\n", report.Mode)
	result += fmt.Sprintf("**Strategy**: %s\n", report.Strategy)
	result += fmt.Sprintf("**AI Provider**: %s\n", report.AIProvider)
	result += fmt.Sprintf("**Scan Time**: %s\n\n", report.ScanTime.Format("2006-01-02 15:04:05"))

	// 扫描统计
	result += fmt.Sprintf("## Scan Statistics\n\n")
	result += fmt.Sprintf("- **Total Contracts**: %d\n", report.TotalContracts)
	result += fmt.Sprintf("- **Vulnerable Contracts**: %d\n\n", report.VulnerableContracts)

	// 漏洞严重性分布
	if len(report.SeverityDistribution) > 0 {
		result += fmt.Sprintf("## Vulnerability Severity Distribution\n\n")
		for severity, count := range report.SeverityDistribution {
			result += fmt.Sprintf("- **%s**: %d\n", severity, count)
		}
		result += "\n"
	}

	// 详细结果
	result += fmt.Sprintf("## Detailed Results\n\n")

	for i, scanResult := range report.Results {
		// 合约地址作为一级标题
		result += fmt.Sprintf("# Contract Address: %s\n\n", scanResult.ContractAddress)
		result += fmt.Sprintf("**Scan Time**: %s\n", scanResult.ScanTime.Format("2006-01-02 15:04:05"))
		result += fmt.Sprintf("**Status**: %s\n\n", scanResult.Status)

		// 风险评估
		if scanResult.RiskScore != "" || scanResult.VulnProbability != "" {
			result += fmt.Sprintf("### Risk Assessment\n\n")
			if scanResult.RiskScore != "" {
				result += fmt.Sprintf("- **Risk Score**: %s\n", scanResult.RiskScore)
			}
			if scanResult.VulnProbability != "" {
				result += fmt.Sprintf("- **Vulnerability Probability**: %s\n", scanResult.VulnProbability)
			}
			result += "\n"
		}

		// AI分析摘要
		if scanResult.AnalysisSummary != "" {
			result += fmt.Sprintf("### AI Analysis Summary\n\n")
			result += fmt.Sprintf("%s\n\n", scanResult.AnalysisSummary)
		}

		// 漏洞详情
		if len(scanResult.Vulnerabilities) > 0 {
			result += fmt.Sprintf("### Vulnerability Details\n\n")
			for j, vuln := range scanResult.Vulnerabilities {
				severityIcon := getSeverityIcon(vuln.Severity)
				result += fmt.Sprintf("%d. %s **[%s]** %s\n", j+1, severityIcon, vuln.Severity, vuln.Type)
				result += fmt.Sprintf("   **Description**: %s\n\n", vuln.Description)
			}
		}

		// 原始AI响应（可选）
		if scanResult.RawResponse != "" {
			result += fmt.Sprintf("### AI Raw Response\n\n")
			// 检查是否已经包含代码块标记，避免嵌套
			rawResp := strings.TrimSpace(scanResult.RawResponse)
			if strings.HasPrefix(rawResp, "```") {
				result += fmt.Sprintf("%s\n\n", rawResp)
			} else {
				result += fmt.Sprintf("```\n%s\n```\n\n", scanResult.RawResponse)
			}
		}

		// 如果不是最后一个结果，添加分隔线
		if i < len(report.Results)-1 {
			result += fmt.Sprintf("---\n\n")
		}
	}

	return result, nil
}

func getSeverityIcon(severity string) string {
	switch severity {
	case "Critical":
		return "🔴"
	case "High":
		return "🟠"
	case "Medium":
		return "🟡"
	case "Low":
		return "🟢"
	default:
		return "⚪"
	}
}
