package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vespera/internal/config"
)

// TestCase 定义单个测试用例
type TestCase struct {
	Address               string `json:"address"`
	Name                  string `json:"name"`
	Project               string `json:"project"`
	ExpectedVulnerability bool   `json:"expected_vulnerability"`
	VulnerabilityType     string `json:"vulnerability_type"`
	Description           string `json:"description"`
}

// TestResult 定义测试结果
type TestResult struct {
	TestCase     TestCase
	Success      bool   // 命令是否执行成功
	VulnDetected bool   // 是否检测出漏洞
	Duration     string // 耗时
	Error        string // 错误信息
	Logs         string // 部分日志
}

func Run(cfg config.ScanConfiguration) error {
	// 使用 cfg 中的策略，默认为 default
	strategy := cfg.Strategy
	if strategy == "" || strategy == "all" {
		strategy = "default"
	}
	fmt.Printf("🚀 Starting Benchmark Runner for %s Vulnerabilities...\n", strategy)

	// 1. 读取数据集
	datasetPath := cfg.Database
	if datasetPath == "" { //qhello 基准测试：在此处修改数据集
		datasetPath = "benchmark/dataset.json"
	}
	data, err := os.ReadFile(datasetPath)
	if err != nil {
		return fmt.Errorf("failed to read dataset: %w", err)
	}

	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		return fmt.Errorf("failed to parse dataset: %w", err)
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 5
	}
	fmt.Printf("📋 Loaded %d test cases. Running with concurrency: %d\n\n", len(testCases), concurrency)

	// 2. 准备结果统计
	var results []TestResult
	stats := struct {
		TP int // True Positive
		TN int // True Negative
		FP int // False Positive
		FN int // False Negative
	}{}

	// 互斥锁保护共享数据
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	// 3. 执行测试
	inputFile := cfg.InputFile
	if inputFile == "" {
		inputFile = "default" // 默认 input 为 default.toml
	}

	// 获取当前执行的二进制路径，用于递归调用
	executable, err := os.Executable()
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not get executable path, falling back to 'go run src/main.go': %v\n", err)
		executable = "go"
	}

	startTime := time.Now()

	for i, tc := range testCases {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(index int, tc TestCase) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			cmdStart := time.Now()

			// 构建参数
			args := []string{}
			if executable == "go" {
				args = append(args, "run", "src/main.go")
			}

			// 基础参数
			args = append(args,
				"-m", "mode1",
				"-i", inputFile,
				"-s", strategy,
				"-t", "contract",
				"-addr", tc.Address,
			)

			// AI Provider
			if cfg.AIProvider != "" {
				args = append(args, "-ai", cfg.AIProvider)
			} else {
				args = append(args, "-ai", "deepseek")
			}

			// 执行命令
			cmd := exec.Command(executable, args...)

			// 捕获输出
			output, err := cmd.CombinedOutput()
			duration := time.Since(cmdStart)

			outputStr := string(output)
			success := err == nil

			// 结果判定逻辑
			vulnDetected := strings.Contains(outputStr, "Vulnerable:  1") ||
				strings.Contains(outputStr, "Severity: High") ||
				strings.Contains(outputStr, "Severity: Critical") ||
				strings.Contains(outputStr, "[Vulnerability Detected]") ||
				strings.Contains(outputStr, "Severity: Medium") || // 增加中风险漏洞检测
				strings.Contains(outputStr, "[Critical]") ||
				strings.Contains(outputStr, "[High]") ||
				strings.Contains(outputStr, "[Medium]")

			res := TestResult{
				TestCase:     tc,
				Success:      success,
				VulnDetected: vulnDetected,
				Duration:     duration.String(),
				Logs:         outputStr,
			}

			if !success {
				res.Error = fmt.Sprintf("Command failed: %v", err)
			}

			// 临界区：更新统计和打印结果
			mu.Lock()
			defer mu.Unlock()

			// 简化的输出格式
			statusIcon := "✅"
			statusColor := "\033[32m" // Green
			if !success {
				statusIcon = "❌"
				statusColor = "\033[31m" // Red
			} else if vulnDetected {
				statusIcon = "⚠️ "
				statusColor = "\033[33m" // Yellow
			}
			resetColor := "\033[0m"

			fmt.Printf("%s[%d/%d]%s %s %-20s (%s)\n",
				statusColor, index+1, len(testCases), resetColor,
				statusIcon,
				tc.Name,
				duration.Round(time.Millisecond),
			)

			if !success {
				fmt.Printf("      %sError: %v%s\n", statusColor, err, resetColor)
			} else {
				if tc.ExpectedVulnerability {
					if vulnDetected {
						stats.TP++
						// fmt.Println("      -> Result: True Positive (Correct)")
					} else {
						stats.FN++
						fmt.Printf("      %s-> Missed (False Negative)%s\n", "\033[31m", resetColor)
					}
				} else {
					if vulnDetected {
						stats.FP++
						fmt.Printf("      %s-> False Alarm (False Positive)%s\n", "\033[31m", resetColor)
					} else {
						stats.TN++
						// fmt.Println("      -> Result: True Negative (Correct)")
					}
				}
			}

			results = append(results, res)
			// fmt.Println() // Remove extra newline for compactness

		}(i, tc)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// 4. 输出报告
	fmt.Println("\n==================================================")
	fmt.Println("📊 Benchmark Summary")
	fmt.Println("==================================================")
	fmt.Printf("Total Time: %s\n", totalDuration)
	fmt.Printf("Total Cases: %d\n", len(testCases))

	// Create a simple table
	fmt.Println("\nMetric          | Count | Percentage")
	fmt.Println("----------------|-------|-----------")

	accuracy := 0.0
	if len(testCases) > 0 {
		accuracy = float64(stats.TP+stats.TN) / float64(len(testCases)) * 100
	}

	fmt.Printf("True Positives  | %-5d | %.1f%%\n", stats.TP, float64(stats.TP)/float64(len(testCases))*100)
	fmt.Printf("True Negatives  | %-5d | %.1f%%\n", stats.TN, float64(stats.TN)/float64(len(testCases))*100)
	fmt.Printf("False Positives | %-5d | %.1f%%\n", stats.FP, float64(stats.FP)/float64(len(testCases))*100)
	fmt.Printf("False Negatives | %-5d | %.1f%%\n", stats.FN, float64(stats.FN)/float64(len(testCases))*100)
	fmt.Println("----------------|-------|-----------")
	fmt.Printf("Accuracy        |       | %.2f%%\n", accuracy)

	if stats.TP+stats.FP > 0 {
		precision := float64(stats.TP) / float64(stats.TP+stats.FP) * 100
		fmt.Printf("Precision       |       | %.2f%%\n", precision)
	} else {
		fmt.Printf("Precision       |       | N/A\n")
	}

	if stats.TP+stats.FN > 0 {
		recall := float64(stats.TP) / float64(stats.TP+stats.FN) * 100
		fmt.Printf("Recall          |       | %.2f%%\n", recall)
	} else {
		fmt.Printf("Recall          |       | N/A\n")
	}

	// 保存详细结果到文件
	reportDir := "benchmark/reports"
	if cfg.ReportDir != "" {
		reportDir = cfg.ReportDir
	}
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		fmt.Printf("Failed to create reports directory: %v\n", err)
	}

	reportPath := filepath.Join(reportDir, fmt.Sprintf("report_%s_%d.json", strategy, time.Now().Unix()))
	reportData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal report JSON: %v\n", err)
		return nil
	}
	if err := os.WriteFile(reportPath, reportData, 0644); err != nil {
		fmt.Printf("Failed to write report file: %v\n", err)
		return nil
	}
	fmt.Printf("\n📝 Detailed report saved to: %s\n", reportPath)

	return nil
}
