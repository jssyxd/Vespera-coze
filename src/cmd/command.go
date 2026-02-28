package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"vespera/internal/benchmark"
	"vespera/internal/config"
	"vespera/internal/download"
	"vespera/internal/handler"
	"vespera/internal/target"
	"vespera/internal/ui"
)

func ExecuteDownload(ctx context.Context, cfg *CLIConfig) error {
	fmt.Println(ui.Cyan + "🚀 Starting Contract Downloader..." + ui.Reset)

	fmt.Println(ui.Blue + "📊 Connecting to MySQL database..." + ui.Reset)
	db, err := config.InitDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()
	fmt.Println(ui.Green + "✅ Database connected!" + ui.Reset)

	fmt.Printf(ui.Cyan+"🔗 Creating downloader (Chain: %s)...\n"+ui.Reset, cfg.Chain)
	dl, err := download.NewDownloader(db, cfg.Chain, cfg.Proxy)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %w", err)
	}
	defer dl.Close()

	fmt.Println("\n" + ui.Gray + strings.Repeat("=", 50) + ui.Reset)
	fmt.Println(ui.Bold + "Start syncing contract data..." + ui.Reset)
	fmt.Println(ui.Gray + strings.Repeat("=", 50) + ui.Reset + "\n")

	if cfg.DownloadFile != "" {
		fpath := cfg.DownloadFile
		f, err := os.Open(fpath)
		if err != nil {
			return fmt.Errorf("failed to open address file: %w", err)
		}
		scanner := bufio.NewScanner(f)
		var addrs []string
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(line) == 42 && strings.HasPrefix(line, "0x") {
				addrs = append(addrs, line)
			} else {
				fmt.Printf(ui.Yellow+"⚠️  Skipping invalid address: %s"+ui.Reset+"\n", line)
			}
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read address file: %w", err)
		}
		if len(addrs) == 0 {
			return fmt.Errorf("address file is empty: %s", fpath)
		}

		failLog := "eoferror.txt"
		fmt.Printf(ui.Blue+"🔁 Retrying %d addresses from %s, failures logged to %s"+ui.Reset+"\n", len(addrs), fpath, failLog)
		if err := dl.DownloadContractsByAddresses(ctx, addrs, failLog); err != nil {
			return fmt.Errorf("failed to download by addresses: %w", err)
		}

		fmt.Println(ui.Green + "\n🎉 Address download completed!" + ui.Reset)
		return nil
	}

	if cfg.DownloadRange != nil {
		start := cfg.DownloadRange.Start
		end := cfg.DownloadRange.End
		if end == ^uint64(0) {
			return fmt.Errorf("end block of download range cannot be empty")
		}
		fmt.Printf(ui.Cyan+"📥 Downloading block range: %d to %d"+ui.Reset+"\n", start, end)
		if err := dl.DownloadBlockRange(ctx, start, end); err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
	} else {
		fmt.Println(ui.Cyan + "📥 Continuing from last download..." + ui.Reset)
		if err := dl.DownloadFromLast(ctx); err != nil {
			return fmt.Errorf("failed to continue download from last: %w", err)
		}
	}

	fmt.Println(ui.Green + "\n🎉 Download task completed!" + ui.Reset)
	return nil
}

func ExecuteBenchmark(ctx context.Context, cfg *CLIConfig) error {
	appConfig, err := config.LoadConfig()
	if err != nil {
		fmt.Printf(ui.Yellow+"⚠️  Warning: Failed to load config: %v"+ui.Reset+"\n", err)
	}
	scanConfig := cfg.MergeConfigs(appConfig)
	return benchmark.Run(scanConfig)
}

// qhello ExecuteScan 扫描命令入口
func ExecuteScan(ctx context.Context, cfg *CLIConfig) error {
	if err := handler.InitScanLogger(); err != nil {
		fmt.Printf(ui.Yellow+"⚠️  Warning: Failed to init logger: %v"+ui.Reset+"\n", err)
	}
	defer handler.CloseScanLogger()

	appConfig, err := config.LoadConfig()
	if err != nil {
		fmt.Printf(ui.Yellow+"⚠️  Warning: Failed to load config: %v"+ui.Reset+"\n", err)
	}

	scanConfig := cfg.MergeConfigs(appConfig)

	//helloq 模式分派
	switch scanConfig.Mode {
	case "mode1":
		return handler.RunMode1Targeted(ctx, scanConfig)

	case "mode2":
		db, err := config.InitDB(ctx)
		if err != nil {
			return fmt.Errorf("init db failed: %w", err)
		}
		defer db.Close()

		dl, err := download.NewDownloader(db, scanConfig.Chain, scanConfig.Proxy)
		if err != nil {
			return fmt.Errorf("init downloader failed: %w", err)
		}
		defer dl.Close()

		targetChan, err := target.GetTargetChannel(ctx, scanConfig, dl, db)
		if err != nil {
			return fmt.Errorf("failed to get targets: %w", err)
		}

		return handler.RunMode2Fuzzy(ctx, scanConfig, targetChan)

	default:
		return fmt.Errorf("unsupported mode: %s", scanConfig.Mode)
	}
}

func Execute(ctx context.Context, cfg *CLIConfig) error {
	if cfg.Download {
		return ExecuteDownload(ctx, cfg)
	}

	if cfg.Benchmark {
		return ExecuteBenchmark(ctx, cfg)
	}

	if cfg.Verbose {
		fmt.Printf(ui.Gray+"Running Excavator with config: %+v"+ui.Reset+"\n", cfg)
	}

	return ExecuteScan(ctx, cfg)
}
