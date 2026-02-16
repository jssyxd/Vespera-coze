package scanner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vespera/internal/ai"
	"vespera/internal/config"

	"gorm.io/gorm"
)

type Scanner struct {
	db        *config.Database
	aiClient  *ai.MultiModelClient
	outputDir string
	etherscan *EtherscanClient
}

type Contract struct {
	Address        string          `gorm:"column:address;primaryKey"`
	Contract       string          `gorm:"column:contract"`
	ABI            json.RawMessage `gorm:"column:abi"`
	Balance        string          `gorm:"column:balance"`
	IsOpenSource   bool            `gorm:"column:isopensource"`
	IsProxy        bool            `gorm:"column:isproxy"`
	Implementation string          `gorm:"column:implementation"`
	CreateTime     time.Time       `gorm:"column:createtime"`
	CreateBlock    int64           `gorm:"column:createblock"`
	TxLast         time.Time       `gorm:"column:txlast"`
	IsDecompiled   bool            `gorm:"column:isdecompiled"`
	DedCode        string          `gorm:"column:dedcode"`
	ScanResult     json.RawMessage `gorm:"column:scan_result"`
	ScanTime       *time.Time      `gorm:"column:scan_time"`
}

type Report struct {
	ScanTime         time.Time                `json:"scan_time"`
	Duration         time.Duration            `json:"duration"`
	Chain            string                   `json:"chain"`
	ContractsScanned int                      `json:"contracts_scanned"`
	Vulnerabilities  []ai.Vulnerability       `json:"vulnerabilities"`
	ArbitrageOpps    []ai.ArbitrageOpportunity `json:"arbitrage_opportunities"`
	Details          []ContractReport         `json:"details"`
}

type ContractReport struct {
	Address      string              `json:"address"`
	ScanResult   *ai.AggregatedResult `json:"scan_result"`
	Error        string              `json:"error,omitempty"`
	ScanDuration time.Duration       `json:"scan_duration"`
}

func New(db *config.Database, aiClient *ai.MultiModelClient, outputDir string) *Scanner {
	return &Scanner{
		db:        db,
		aiClient:  aiClient,
		outputDir: outputDir,
		etherscan: NewEtherscanClient(os.Getenv("ETHERSCAN_API_KEY")),
	}
}

// Mode 1: Targeted scan for specific contract
func (s *Scanner) Mode1(chain, address string) (*Report, error) {
	startTime := time.Now()
	report := &Report{
		ScanTime: startTime,
		Chain:    chain,
		Details:  []ContractReport{},
	}

	// Fetch contract code
	contract, err := s.fetchContract(chain, address)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contract: %w", err)
	}

	// Run multi-model analysis
	result, err := s.analyzeContract(contract)
	if err != nil {
		report.Details = append(report.Details, ContractReport{
			Address: address,
			Error:   err.Error(),
		})
	} else {
		report.Details = append(report.Details, *result)
		report.Vulnerabilities = append(report.Vulnerabilities, result.ScanResult.Vulnerabilities...)
		report.ArbitrageOpps = append(report.ArbitrageOpps, result.ScanResult.ArbitrageOpportunities...)
	}

	report.ContractsScanned = 1
	report.Duration = time.Since(startTime)

	return report, nil
}

// Mode 2: Hybrid scan for block range
func (s *Scanner) Mode2(chain, blockRange string, models []string) (*Report, error) {
	startTime := time.Now()
	report := &Report{
		ScanTime: startTime,
		Chain:    chain,
		Details:  []ContractReport{},
	}

	// Parse block range
	contracts, err := s.fetchContractsInRange(chain, blockRange)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contracts: %w", err)
	}

	log.Printf("Found %d contracts in block range %s", len(contracts), blockRange)

	// Analyze each contract
	for i, contract := range contracts {
		if !contract.IsOpenSource {
			log.Printf("Skipping closed-source contract %s", contract.Address)
			continue
		}

		log.Printf("[%d/%d] Analyzing %s...", i+1, len(contracts), contract.Address)

		result, err := s.analyzeContract(&contract)
		if err != nil {
			log.Printf("❌ Failed to analyze %s: %v", contract.Address, err)
			report.Details = append(report.Details, ContractReport{
				Address: contract.Address,
				Error:   err.Error(),
			})
			continue
		}

		report.Details = append(report.Details, *result)
		report.Vulnerabilities = append(report.Vulnerabilities, result.ScanResult.Vulnerabilities...)
		report.ArbitrageOpps = append(report.ArbitrageOpps, result.ScanResult.ArbitrageOpportunities...)
		report.ContractsScanned++

		// Save to database
		s.saveScanResult(chain, &contract, result.ScanResult)
	}

	report.Duration = time.Since(startTime)
	return report, nil
}

// Mode 3: Real-time monitoring
func (s *Scanner) Mode3(chain string, models []string) (*Report, error) {
	log.Println("🔄 Mode 3: Real-time monitoring started (Press Ctrl+C to stop)")

	// This would typically run as a long-running service
	// For now, just scan recent blocks
	return s.Mode2(chain, "latest-100", models)
}

func (s *Scanner) analyzeContract(contract *Contract) (*ContractReport, error) {
	startTime := time.Now()

	result, err := s.aiClient.ParallelAnalyze(contract.Contract, nil)
	if err != nil {
		return nil, err
	}

	return &ContractReport{
		Address:      contract.Address,
		ScanResult:   result.Aggregated,
		ScanDuration: time.Since(startTime),
	}, nil
}

func (s *Scanner) fetchContract(chain, address string) (*Contract, error) {
	// Try to get from database first
	var contract Contract
	tableName := chain
	if tableName == "" {
		tableName = "ethereum"
	}

	result := s.db.GetDB().Table(tableName).Where("address = ?", address).First(&contract)
	if result.Error == nil && contract.Contract != "" {
		return &contract, nil
	}

	// Fetch from blockchain
	return s.etherscan.GetContract(address)
}

func (s *Scanner) fetchContractsInRange(chain, blockRange string) ([]Contract, error) {
	// Parse block range like "20000000-20000200" or "latest-100"
	var contracts []Contract

	if strings.HasPrefix(blockRange, "latest-") {
		// Get latest contracts from database
		limit := 100
		fmt.Sscanf(blockRange, "latest-%d", &limit)

		tableName := chain
		if tableName == "" {
			tableName = "ethereum"
		}

		result := s.db.GetDB().Table(tableName).
			Where("isopensource = ?", true).
			Order("createtime DESC").
			Limit(limit).
			Find(&contracts)

		if result.Error != nil {
			return nil, result.Error
		}
	} else {
		// Parse specific range
		var start, end int64
		_, err := fmt.Sscanf(blockRange, "%d-%d", &start, &end)
		if err != nil {
			return nil, fmt.Errorf("invalid block range format: %s", blockRange)
		}

		tableName := chain
		if tableName == "" {
			tableName = "ethereum"
		}

		result := s.db.GetDB().Table(tableName).
			Where("createblock BETWEEN ? AND ? AND isopensource = ?", start, end, true).
			Find(&contracts)

		if result.Error != nil {
			return nil, result.Error
		}
	}

	return contracts, nil
}

func (s *Scanner) saveScanResult(chain string, contract *Contract, result *ai.AggregatedResult) {
	resultJSON, _ := json.Marshal(result)
	now := time.Now()

	tableName := chain
	if tableName == "" {
		tableName = "ethereum"
	}

	update := map[string]interface{}{
		"scan_result": resultJSON,
		"scan_time":   now,
	}

	s.db.GetDB().Table(tableName).Where("address = ?", contract.Address).Updates(update)
}

func (s *Scanner) SaveReport(report *Report, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Save JSON report
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	jsonPath := filepath.Join(outputDir, fmt.Sprintf("report_%d.json", report.ScanTime.Unix()))
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return err
	}

	// Save Markdown report
	mdContent := s.generateMarkdownReport(report)
	mdPath := filepath.Join(outputDir, fmt.Sprintf("report_%d.md", report.ScanTime.Unix()))
	if err := os.WriteFile(mdPath, []byte(mdContent), 0644); err != nil {
		return err
	}

	// Save summary
	summaryPath := filepath.Join(outputDir, "summary.md")
	summary := fmt.Sprintf("# Scan Summary\n\n- Time: %s\n- Duration: %v\n- Contracts: %d\n- Vulnerabilities: %d\n- Arbitrage: %d\n",
		report.ScanTime.Format(time.RFC3339),
		report.Duration,
		report.ContractsScanned,
		len(report.Vulnerabilities),
		len(report.ArbitrageOpps),
	)
	os.WriteFile(summaryPath, []byte(summary), 0644)

	log.Printf("📄 Reports saved to %s", outputDir)
	return nil
}

func (s *Scanner) generateMarkdownReport(report *Report) string {
	var sb strings.Builder

	sb.WriteString("# Vespera Scan Report\n\n")
	sb.WriteString(fmt.Sprintf("**Scan Time:** %s\n\n", report.ScanTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration:** %v\n\n", report.Duration))
	sb.WriteString(fmt.Sprintf("**Chain:** %s\n\n", report.Chain))
	sb.WriteString(fmt.Sprintf("**Contracts Scanned:** %d\n\n", report.ContractsScanned))

	if len(report.Vulnerabilities) > 0 {
		sb.WriteString("## 🚨 Vulnerabilities\n\n")
		for _, v := range report.Vulnerabilities {
			sb.WriteString(fmt.Sprintf("### [%s] %s\n\n", v.Severity, v.Type))
			sb.WriteString(fmt.Sprintf("- **Description:** %s\n", v.Description))
			sb.WriteString(fmt.Sprintf("- **Line:** %d\n", v.Line))
			sb.WriteString(fmt.Sprintf("- **Recommendation:** %s\n\n", v.Recommendation))
		}
	}

	if len(report.ArbitrageOpps) > 0 {
		sb.WriteString("## 💰 Arbitrage Opportunities\n\n")
		for _, a := range report.ArbitrageOpps {
			sb.WriteString(fmt.Sprintf("### %s\n\n", a.Type))
			sb.WriteString(fmt.Sprintf("- **Description:** %s\n", a.Description))
			sb.WriteString(fmt.Sprintf("- **Expected Profit:** %s\n\n", a.ExpectedProfit))
		}
	}

	sb.WriteString("## Contract Details\n\n")
	for _, d := range report.Details {
		sb.WriteString(fmt.Sprintf("### %s\n\n", d.Address))
		if d.Error != "" {
			sb.WriteString(fmt.Sprintf("- **Error:** %s\n", d.Error))
		} else {
			sb.WriteString(fmt.Sprintf("- **Scan Duration:** %v\n", d.ScanDuration))
			sb.WriteString(fmt.Sprintf("- **Vulnerabilities:** %d\n", len(d.ScanResult.Vulnerabilities)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
