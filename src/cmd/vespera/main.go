package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"vespera/internal/ai"
	"vespera/internal/config"
	"vespera/internal/scanner"
)

type ScanOptions struct {
	Mode            string
	Chain           string
	BlockRange      string
	ContractAddress string
	AIProvider      string
	Models          []string
	OutputDir       string
	TestDBOnly      bool
}

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			// Remove subcommand from args
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
			runInitialization()
			return
		case "scan":
			// Remove subcommand from args
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
			runScan()
			return
		}
	}

	// Default: run scan
	runScan()
}

func runScan() {
	opts := parseFlags()

	if opts.TestDBOnly {
		testDatabaseConnection()
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database - try PostgreSQL first, then SQLite fallback
	var db *config.Database
	db, err = config.NewPostgresDB(&cfg.Database)
	if err != nil {
		log.Printf("⚠️ PostgreSQL connection failed: %v", err)
		log.Println("🔄 Falling back to SQLite...")
		db, err = config.NewSQLiteDB("./data/vespera.db")
		if err != nil {
			log.Fatalf("Failed to connect to SQLite: %v", err)
		}
		// Initialize SQLite tables
		if err := config.InitSQLiteTables(db, []string{"eth", "bsc", "polygon", "arbitrum"}); err != nil {
			log.Printf("Warning: SQLite init failed: %v", err)
		}
	}
	defer db.Close()

	log.Println("✅ Database connected successfully")

	// Auto-migrate tables
	if err := db.AutoMigrate(opts.Chain); err != nil {
		log.Printf("Warning: Auto-migrate failed: %v", err)
	}

	// Initialize AI client
	aiClient := ai.NewMultiModelClient(cfg.AI)

	// Initialize scanner
	scan := scanner.New(db, aiClient, opts.OutputDir)

	// Execute scan based on mode
	startTime := time.Now()
	var report *scanner.Report

	switch opts.Mode {
	case "mode1":
		log.Printf("🔍 Running Mode 1: Targeted scan for contract %s", opts.ContractAddress)
		report, err = scan.Mode1(opts.Chain, opts.ContractAddress)
	case "mode2":
		log.Printf("🔍 Running Mode 2: Hybrid scan for blocks %s", opts.BlockRange)
		report, err = scan.Mode2(opts.Chain, opts.BlockRange, opts.Models)
	case "mode3":
		log.Println("🔍 Running Mode 3: Real-time monitoring")
		report, err = scan.Mode3(opts.Chain, opts.Models)
	default:
		log.Fatalf("Invalid mode: %s", opts.Mode)
	}

	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	duration := time.Since(startTime)
	log.Printf("✅ Scan completed in %v", duration)

	// Save report
	if err := scan.SaveReport(report, opts.OutputDir); err != nil {
		log.Printf("Warning: Failed to save report: %v", err)
	}

	// Print summary
	printSummary(report)
}

func parseFlags() *ScanOptions {
	opts := &ScanOptions{}

	flag.StringVar(&opts.Mode, "m", "mode2", "Scan mode: mode1, mode2, or mode3")
	flag.StringVar(&opts.Chain, "c", "eth", "Blockchain: eth, bsc, polygon, arbitrum")
	flag.StringVar(&opts.BlockRange, "range", "", "Block range for mode2 (e.g., 20000000-20000200)")
	flag.StringVar(&opts.ContractAddress, "addr", "", "Contract address for mode1")
	flag.StringVar(&opts.AIProvider, "ai", "deepseek", "AI provider")
	flag.StringVar(&opts.OutputDir, "o", "reports", "Output directory for reports")
	flag.BoolVar(&opts.TestDBOnly, "test-db", false, "Test database connection only")

	flag.Parse()

	// Parse models from environment or use defaults
	modelsEnv := os.Getenv("AI_MODELS")
	if modelsEnv != "" {
		opts.Models = strings.Split(modelsEnv, ",")
	} else {
		opts.Models = []string{"deepseek-v3.2", "glm-4.7", "minimax-m2.1"}
	}

	return opts
}

func testDatabaseConnection() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Try PostgreSQL first
	db, err := config.NewPostgresDB(&cfg.Database)
	if err != nil {
		log.Printf("⚠️ PostgreSQL connection failed: %v", err)
		log.Println("🔄 Trying SQLite fallback...")
		db, err = config.NewSQLiteDB("./data/vespera.db")
		if err != nil {
			log.Fatalf("❌ Database connection failed: %v", err)
		}
		log.Println("✅ SQLite connection successful")
	} else {
		log.Println("✅ PostgreSQL connection successful")
	}
	defer db.Close()
	os.Exit(0)
}

func printSummary(report *scanner.Report) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("VESPERA SCAN REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Scan Time: %s\n", report.ScanTime.Format(time.RFC3339))
	fmt.Printf("Duration: %v\n", report.Duration)
	fmt.Printf("Contracts Scanned: %d\n", report.ContractsScanned)
	fmt.Printf("Vulnerabilities Found: %d\n", len(report.Vulnerabilities))
	fmt.Printf("Arbitrage Opportunities: %d\n", len(report.ArbitrageOpps))
	fmt.Println(strings.Repeat("=", 60))

	if len(report.Vulnerabilities) > 0 {
		fmt.Println("\n🚨 VULNERABILITIES:")
		for _, v := range report.Vulnerabilities {
			fmt.Printf("  [%s] %s (Line %d)\n", v.Severity, v.Type, v.Line)
		}
	}

	if len(report.ArbitrageOpps) > 0 {
		fmt.Println("\n💰 ARBITRAGE OPPORTUNITIES:")
		for _, a := range report.ArbitrageOpps {
			fmt.Printf("  [%s] Expected profit: %s\n", a.Type, a.ExpectedProfit)
		}
	}
}
