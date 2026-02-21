package handler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/VectorBits/Vespera/src/internal"
	"github.com/VectorBits/Vespera/src/internal/ai/parser"
	"github.com/VectorBits/Vespera/src/internal/config"
	"github.com/VectorBits/Vespera/src/internal/dbutil"
	"github.com/VectorBits/Vespera/src/internal/download"
	"github.com/VectorBits/Vespera/src/internal/logger"
	"github.com/VectorBits/Vespera/src/internal/report"
)

func InitScanLogger() error {
	return logger.InitLogger()
}

func CloseScanLogger() {
	logger.Close()
}

func isOnlyBytecode(code string) bool {
	code = strings.TrimSpace(code)
	if len(code) < 10 {
		return true
	}
	if !strings.HasPrefix(code, "0x") {
		return false
	}
	for _, c := range code[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// qhello getOrDownloadContract 合约获取与代理解析
func getOrDownloadContract(ctx context.Context, db *sql.DB, downloader *download.Downloader, address string) (string, string, bool, error) {
	tableName, err := config.GetTableName(downloader.ChainName)
	if err != nil {
		return "", "", false, err
	}

	addr := strings.TrimSpace(address)

	if downloader != nil {
		exists, errExists := downloader.ContractExists(ctx, addr)
		if errExists != nil {
			return "", "", false, errExists
		}
		if !exists {
			logger.InfoFileOnly("Contract not in DB, downloading...")
			if err := downloader.DownloadContractsByAddresses(ctx, []string{addr}, ""); err != nil {
				client, rcErr := downloader.RPCManager.GetClient()
				if rcErr != nil {
					return "", "", false, fmt.Errorf("failed to download contract: %v, and failed to get RPC client: %w", err, rcErr)
				}
				codeBytes, rcErr := client.CodeAt(ctx, common.HexToAddress(addr), nil)
				if rcErr != nil {
					return "", "", false, fmt.Errorf("failed to download contract: %v, and failed to fallback to bytecode: %w", err, rcErr)
				}
				return fmt.Sprintf("0x%x", codeBytes), addr, false, nil
			}
		}
	}

	var proxyFlag sql.NullInt64
	var implAddr sql.NullString
	proxyQuery := fmt.Sprintf("SELECT isproxy, implementation FROM %s WHERE address = ?", tableName)
	err = db.QueryRow(proxyQuery, addr).Scan(&proxyFlag, &implAddr)
	if err == nil && proxyFlag.Valid && proxyFlag.Int64 == 1 && implAddr.Valid && strings.TrimSpace(implAddr.String) != "" {
		impl := strings.TrimSpace(implAddr.String)

		if downloader != nil {
			existsImpl, errExistsImpl := downloader.ContractExists(ctx, impl)
			if errExistsImpl != nil {
				return "", "", false, errExistsImpl
			}
			if !existsImpl {
				logger.InfoFileOnly("Implementation contract not in DB, downloading...")
				if err := downloader.DownloadContractsByAddresses(ctx, []string{impl}, ""); err != nil {
					logger.Warn("Failed to download implementation, fallback to proxy bytecode: %v", err)
					client, rcErr := downloader.RPCManager.GetClient()
					if rcErr == nil {
						codeBytes, rcErr := client.CodeAt(ctx, common.HexToAddress(impl), nil)
						if rcErr == nil {
							return fmt.Sprintf("0x%x", codeBytes), impl, true, nil
						}
					}
				}
			}
		}

		var implCode string
		implQuery := fmt.Sprintf("SELECT contract FROM %s WHERE address = ? AND contract IS NOT NULL AND contract != ''", tableName)
		errImpl := db.QueryRow(implQuery, impl).Scan(&implCode)
		if errImpl == nil && strings.TrimSpace(implCode) != "" {
			logger.InfoFileOnly("Using implementation source code")
			return implCode, impl, true, nil
		}

		logger.Warn("Implementation source unavailable, trying entry point bytecode")
	}

	var contractCode string
	codeQuery := fmt.Sprintf("SELECT contract FROM %s WHERE address = ? AND contract IS NOT NULL AND contract != ''", tableName)
	err = db.QueryRow(codeQuery, addr).Scan(&contractCode)
	if err == nil && strings.TrimSpace(contractCode) != "" {
		logger.InfoFileOnly("Loaded contract code from DB")
		return contractCode, addr, false, nil
	}

	if downloader != nil {
		client, rcErr := downloader.RPCManager.GetClient()
		if rcErr == nil {
			codeBytes, rcErr := client.CodeAt(ctx, common.HexToAddress(addr), nil)
			if rcErr == nil {
				return fmt.Sprintf("0x%x", codeBytes), addr, false, nil
			}
		}
	}

	return "", "", false, fmt.Errorf("failed to get contract source code, only bytecode exists or contract not found")
}

func getAddressesFromDB(db *sql.DB, chainName string, blockRange *config.BlockRange) ([]string, error) {
	return dbutil.GetAddressesFromDB(db, chainName, blockRange)
}

func getAddressesFromFile(filepathStr string) ([]string, error) {
	if strings.TrimSpace(filepathStr) == "" {
		return nil, fmt.Errorf("file path is empty")
	}
	return internal.ReadLines(filepathStr)
}

type ScanResult struct {
	Address         string
	ResolvedAddress string
	IsProxy         bool
	AnalysisResult  *parser.AnalysisResult
	Timestamp       time.Time
	Mode            string
	Strategy        string
	InputFile       string
}

func printVulnerabilitySummary(result *ScanResult) {
	var sb strings.Builder
	addrInfo := result.Address
	if result.ResolvedAddress != "" && result.ResolvedAddress != result.Address {
		addrInfo = fmt.Sprintf("%s (Impl: %s)", result.Address, result.ResolvedAddress)
	}

	if result.AnalysisResult == nil {
		sb.WriteString(fmt.Sprintf("  🔍 [Contract: %s] ❓ Analysis result is empty", addrInfo))
	} else {
		vulnCount := len(result.AnalysisResult.Vulnerabilities)
		if vulnCount == 0 {
			sb.WriteString(fmt.Sprintf("  🔍 [Contract: %s] ✅ No vulnerabilities found", addrInfo))
		} else {
			sb.WriteString(fmt.Sprintf("  🔍 [Contract: %s] ⚠️  Found %d potential vulnerabilities:", addrInfo, vulnCount))
			for i, vuln := range result.AnalysisResult.Vulnerabilities {
				severityEmoji := getSeverityEmoji(vuln.Severity)
				sb.WriteString(fmt.Sprintf("\n    %d. %s [%s] %s",
					i+1, severityEmoji, vuln.Severity, vuln.Type))
			}
		}
	}

	logger.InfoFileOnly("%s", sb.String())
}

func getSeverityEmoji(severity string) string {
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

func countVulnerableContracts(results []*ScanResult) int {
	count := 0
	for _, r := range results {
		if r.AnalysisResult != nil && len(r.AnalysisResult.Vulnerabilities) > 0 {
			count++
		}
	}
	return count
}

// qhello generateReport 解析提示词和报告生成
func generateReport(results []*ScanResult, cfg config.ScanConfiguration) error {
	logger.Info("Generating scan report...")

	var strategyName string
	if len(results) > 0 {
		strategyName = results[0].Strategy
	} else {
		strategyName = cfg.Strategy
	}

	reportInstance := report.NewReport(cfg.Mode, strategyName, cfg.AIProvider)

	for _, result := range results {
		displayAddr := result.Address
		if result.ResolvedAddress != "" && result.ResolvedAddress != result.Address {
			if result.IsProxy {
				displayAddr = fmt.Sprintf("%s (Implementation: %s)", result.Address, result.ResolvedAddress)
			} else {
				displayAddr = fmt.Sprintf("%s (Scanned Address: %s)", result.Address, result.ResolvedAddress)
			}
		}

		scanResult := report.NewScanResult(displayAddr)

		// 空指针保护
		if result.AnalysisResult == nil {
			scanResult.SetStatus("❓ Analysis result is empty")
			reportInstance.AddScanResult(scanResult)
			continue
		}

		if len(result.AnalysisResult.Vulnerabilities) > 0 {
			scanResult.SetStatus(fmt.Sprintf("⚠️ discovered %d vulnerabilities", len(result.AnalysisResult.Vulnerabilities)))
		} else {
			scanResult.SetStatus("✅ No vulnerabilities found")
		}

		if result.AnalysisResult.Summary != "" {
			scanResult.SetAnalysisSummary(result.AnalysisResult.Summary)
		}

		if result.AnalysisResult.VulnProbability != "" {
			scanResult.SetVulnProbability(result.AnalysisResult.VulnProbability)
		}
		if result.AnalysisResult.RiskScore != nil {
			scanResult.SetRiskScore(fmt.Sprintf("%v", result.AnalysisResult.RiskScore))
		}

		if result.AnalysisResult.RawResponse != "" {
			scanResult.SetRawResponse(result.AnalysisResult.RawResponse)
		}

		for _, vuln := range result.AnalysisResult.Vulnerabilities {
			reportVuln := report.Vulnerability{
				Type:        vuln.Type,
				Severity:    vuln.Severity,
				Description: vuln.Description,
			}
			scanResult.AddVulnerability(reportVuln)
		}

		reportInstance.AddScanResult(scanResult)
	}

	generator := report.NewMarkdownGenerator()
	storage := report.NewFileStorage(cfg.ReportDir)
	reporter := report.NewReporter(generator, storage)

	filepath, err := reporter.GenerateAndSave(reportInstance)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	logger.Info("Report saved: %s", filepath)
	return nil
}

func uniqueStrings(input []string) []string {
	uniqueMap := make(map[string]struct{})
	var result []string
	for _, s := range input {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, exists := uniqueMap[s]; !exists {
			uniqueMap[s] = struct{}{}
			result = append(result, s)
		}
	}
	return result
}
