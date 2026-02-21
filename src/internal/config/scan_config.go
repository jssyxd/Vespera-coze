package config

import (
	"time"
)

type ScanConfiguration struct {
	// AI 相关
	AIProvider string
	APIKey     string // 最终使用的 API Key
	BaseURL    string
	Model      string
	Timeout    time.Duration

	// 扫描相关
	Mode          string
	Strategy      string
	TargetSource  string
	TargetFile    string
	TargetAddress string
	BlockRange    *BlockRange
	Chain         string
	Concurrency   int
	InputFile     string

	// 系统相关
	Proxy     string
	Verbose   bool
	ReportDir string

	// Benchmark related
	Benchmark bool
	Database  string
}

type BlockRange struct {
	Start uint64
	End   uint64
}

func DefaultScanConfiguration() ScanConfiguration {
	return ScanConfiguration{
		Timeout:      120 * time.Second,
		Concurrency:  5, // 默认并发数
		Chain:        "eth",
		TargetSource: "db",
		Strategy:     "all",
		ReportDir:    "reports",
	}
}
