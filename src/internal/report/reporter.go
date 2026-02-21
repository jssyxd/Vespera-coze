package report

import (
	"fmt"
	"time"
)

type Reporter struct {
	generator Generator
	storage   Storage
}

func NewReporter(generator Generator, storage Storage) *Reporter {
	return &Reporter{
		generator: generator,
		storage:   storage,
	}
}

func (r *Reporter) GenerateAndSave(report *Report) (string, error) {
	// 生成报告内容
	content, err := r.generator.Generate(report)
	if err != nil {
		return "", fmt.Errorf("failed to generate report: %w", err)
	}

	// 保存报告
	filepath, err := r.storage.Save(report, content)
	if err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	return filepath, nil
}

func NewReport(mode, strategy, aiProvider string) *Report {
	return &Report{
		Mode:                 mode,
		Strategy:             strategy,
		AIProvider:           aiProvider,
		ScanTime:             time.Now(),
		TotalContracts:       0,
		VulnerableContracts:  0,
		SeverityDistribution: make(map[string]int),
		Results:              make([]ScanResult, 0),
	}
}

func (r *Report) AddScanResult(result ScanResult) {
	r.Results = append(r.Results, result)
	r.TotalContracts++

	if len(result.Vulnerabilities) > 0 {
		r.VulnerableContracts++

		// 统计严重性分布
		for _, vuln := range result.Vulnerabilities {
			r.SeverityDistribution[vuln.Severity]++
		}
	}
}

func NewScanResult(contractAddress string) ScanResult {
	return ScanResult{
		ContractAddress: contractAddress,
		ScanTime:        time.Now(),
		Status:          "✅ Scan Completed",
		Vulnerabilities: make([]Vulnerability, 0),
	}
}

func (s *ScanResult) AddVulnerability(vuln Vulnerability) {
	s.Vulnerabilities = append(s.Vulnerabilities, vuln)
}

func (s *ScanResult) SetStatus(status string) {
	s.Status = status
}

func (s *ScanResult) SetAnalysisSummary(summary string) {
	s.AnalysisSummary = summary
}

func (s *ScanResult) SetRawResponse(response string) {
	s.RawResponse = response
}

func (s *ScanResult) SetRiskScore(score string) {
	s.RiskScore = score
}

func (s *ScanResult) SetVulnProbability(prob string) {
	s.VulnProbability = prob
}
