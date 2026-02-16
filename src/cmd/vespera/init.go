package main

import (
	"flag"
	"log"
	"os"

	"vespera/internal/config"
	"vespera/internal/scanner"
)

func runInitialization() {
	var chain string
	flag.StringVar(&chain, "c", "eth", "Blockchain: eth, bsc, polygon, arbitrum")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database with fallback
	var db *config.Database
	db, err = config.NewPostgresDB(&cfg.Database)
	if err != nil {
		log.Printf("⚠️ PostgreSQL connection failed: %v", err)
		log.Println("🔄 Falling back to SQLite...")
		db, err = config.NewSQLiteDB("./data/vespera.db")
		if err != nil {
			log.Fatalf("Failed to connect to SQLite: %v", err)
		}
		if err := config.InitSQLiteTables(db, []string{"eth", "bsc", "polygon", "arbitrum"}); err != nil {
			log.Printf("Warning: SQLite init failed: %v", err)
		}
	}
	defer db.Close()

	log.Println("✅ Database connected")

	// Get current count
	initializer := scanner.NewInitializer(db, cfg.AI.APIKey)
	beforeCount := initializer.GetContractCount(chain)
	log.Printf("📊 Current contracts in database: %d", beforeCount)

	// Initialize database with Etherscan data
	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	if apiKey == "" {
		log.Fatal("❌ ETHERSCAN_API_KEY environment variable is required")
	}

	initializer = scanner.NewInitializer(db, apiKey)

	log.Printf("🚀 Starting database initialization for %s chain...", chain)
	if err := initializer.InitializeDatabase(chain); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	afterCount := initializer.GetContractCount(chain)
	log.Printf("✅ Database initialization completed!")
	log.Printf("📈 Contracts added: %d", afterCount - beforeCount)
	log.Printf("📊 Total contracts in database: %d", afterCount)

	os.Exit(0)
}
