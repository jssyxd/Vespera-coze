package cmd

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/VectorBits/Vespera/src/internal/config"

	"github.com/VectorBits/Vespera/src/internal/ui"
)

// Reporter 先不写

type CLIConfig struct {
	AIProvider    string
	Mode          string
	Strategy      string
	TargetSource  string
	TargetFile    string
	TargetAddress string
	BlockRange    *BlockRange
	Chain         string
	Concurrency   int
	Verbose       bool
	Timeout       time.Duration
	Download      bool
	DownloadRange *BlockRange
	DownloadFile  string
	InputFile     string
	Proxy         string
	ReportDir     string
	Benchmark     bool
	Database      string
}

type BlockRange struct {
	Start uint64
	End   uint64
}

func (b *BlockRange) String() string {
	if b == nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", b.Start, b.End)
}

func parseBlockRange(s string) (*BlockRange, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, errors.New("invalid block range format, expected start-end")
	}
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])
	var br BlockRange
	if startStr == "" {
		return nil, errors.New("start block required")
	}
	start, err := strconv.ParseUint(startStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid start block: %w", err)
	}
	br.Start = start
	if endStr == "" {
		br.End = ^uint64(0) // max uint64 to indicate open-ended
	} else {
		end, err := strconv.ParseUint(endStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid end block: %w", err)
		}
		if end < start {
			return nil, errors.New("end block must be >= start block")
		}
		br.End = end
	}
	return &br, nil
}

func isHexAddress(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return false
	}
	_, err := hex.DecodeString(s[2:])
	return err == nil
}

func looksLikeBlockRange(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.Count(s, "-") != 1 {
		return false
	}
	_, err := parseBlockRange(s)
	return err == nil
}

func looksLikeTargetFile(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return true
	}
	info, err := os.Stat(s)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func (c *CLIConfig) Validate() error {
	// 如果是下载模式，仅需要下载相关配置
	if c.Download {
		return nil
	}

	// 如果是 Benchmark 模式，跳过常规验证，使用 Benchmark 专用验证
	if c.Benchmark {
		if c.AIProvider == "" {
			return errors.New("-ai is required for benchmark (e.g. -ai chatgpt5)")
		}
		// Benchmark 模式下 Mode 可以由系统自动设置为默认值，这里不再强制检查
		return nil
	}

	if c.AIProvider == "" {
		return errors.New("-ai is required (e.g. -ai chatgpt5)")
	}
	if c.Mode == "" {
		return errors.New("-m (mode) is required: mode1|mode2")
	}
	if c.Mode != "mode1" && c.Mode != "mode2" {
		return errors.New("-m must be one of: mode1, mode2")
	}
	// mode1 可以不指定 InputFile，此时默认为 generic_scan
	// 允许 db | file | contract | address | last
	if c.TargetSource != "db" && c.TargetSource != "file" && c.TargetSource != "contract" && c.TargetSource != "address" && c.TargetSource != "last" {
		return errors.New("-t must be: contract address / target file (.txt/.yaml) / block range (start-end) / or db|file|contract|last")
	}
	if c.Mode == "mode1" && c.TargetSource == "last" {
		return errors.New("-t=last is only supported in mode2")
	}
	if c.TargetSource == "file" && c.TargetFile == "" {
		return errors.New("-file is required when -t=file")
	}
	if (c.TargetSource == "contract" || c.TargetSource == "address") && c.TargetAddress == "" {
		return errors.New("-addr is required when -t=contract or -t=address")
	}
	if c.Chain == "" {
		c.Chain = "eth" // default
	}
	// 验证链名称
	validChains := []string{"eth", "bsc", "base"}
	validChain := false
	for _, valid := range validChains {
		if c.Chain == valid {
			validChain = true
			break
		}
	}
	if !validChain {
		return fmt.Errorf("unsupported chain: %s, supported chains: %v", c.Chain, validChains)
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 4
	}
	return nil
}

func (c *CLIConfig) MergeConfigs(appConfig *config.AppConfig) config.ScanConfiguration {
	// 1. Start with defaults
	cfg := config.DefaultScanConfiguration()

	// 2. Override with YAML config if available
	if appConfig != nil {
		// Load AI config
		if aiCfg, err := appConfig.GetAIConfig(c.AIProvider); err == nil {
			cfg.APIKey = aiCfg.APIKey
			cfg.BaseURL = aiCfg.BaseURL
			cfg.Model = aiCfg.Model
		}
	}

	// 3. Override with CLI arguments (if provided)
	cfg.AIProvider = c.AIProvider
	cfg.Mode = c.Mode
	cfg.Strategy = c.Strategy
	cfg.TargetSource = c.TargetSource
	cfg.TargetFile = c.TargetFile
	cfg.TargetAddress = c.TargetAddress
	cfg.Chain = c.Chain
	cfg.Concurrency = c.Concurrency
	cfg.Verbose = c.Verbose
	cfg.Timeout = c.Timeout
	cfg.InputFile = c.InputFile
	cfg.Proxy = c.Proxy
	cfg.ReportDir = c.ReportDir
	cfg.Benchmark = c.Benchmark
	cfg.Database = c.Database

	if c.BlockRange != nil {
		cfg.BlockRange = &config.BlockRange{
			Start: c.BlockRange.Start,
			End:   c.BlockRange.End,
		}
	}

	// 如果 CLI 没有提供某些值但 YAML 提供了，已经在 Step 2 加载了
	// 但注意 CLIConfig 结构体中没有 APIKey 等敏感字段，这些通常只在 YAML 或 ENV 中
	// 如果 CLI 将来支持 --api-key，可以在这里覆盖

	return cfg
}

func showHelp(topic string) {
	switch topic {
	case "d", "download":
		showDownloadHelp()
	case "ai":
		showAIHelp()
	case "m", "mode":
		showModeHelp()
	case "s", "strategy":
		showStrategyHelp()
	case "i", "input":
		showInputHelp()
	case "t", "target":
		showTargetHelp()
	case "c", "chain":
		showChainHelp()
	case "b", "benchmark":
		showBenchmarkHelp()
	default:
		showGeneralHelp()
	}
}

func showGeneralHelp() {
	// ui.PrintBanner() // Banner is handled by main.go

	fmt.Println(ui.Cyan + "USAGE:" + ui.Reset)
	fmt.Println("  vespera [COMMAND] [OPTIONS]")
	fmt.Println()

	fmt.Println(ui.Cyan + "CORE COMMANDS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "-d, --download", "Start contract download mode")
	fmt.Printf("  %-25s %s\n", "-ai <provider>", "Select AI provider for analysis")
	fmt.Printf("  %-25s %s\n", "-m  <mode>", "Scanning mode (mode1|mode2)")
	fmt.Printf("  %-25s %s\n", "-s  <strategy>", "Scanning strategy/prompt (default: default)")
	fmt.Printf("  %-25s %s\n", "-i  <input>", "Input file (Mode 1 only: TOML in strategy/exp_libs/mode1)")
	fmt.Printf("  %-25s %s\n", "-t  <target>", "Scan target (auto-detect)")
	fmt.Printf("  %-25s %s\n", "-c  <chain>", "Blockchain network")
	fmt.Printf("  %-25s %s\n", "-r  <dir>", "Report output directory (default: reports)")
	fmt.Printf("  %-25s %s\n", "-proxy <url>", "Proxy URL (HTTP/SOCKS5)")
	fmt.Printf("  %-25s %s\n", "-b, --benchmark", "Run benchmark mode")
	fmt.Println()

	fmt.Println(ui.Cyan + "HELP:" + ui.Reset)
	fmt.Println("  vespera [COMMAND] --help   Show detailed help for a specific command")
	fmt.Println()

	fmt.Println(ui.Cyan + "EXAMPLES:" + ui.Reset)
	fmt.Println(ui.Gray + "  # Targeted Scan (Mode 1)" + ui.Reset)
	fmt.Println("  vespera -ai chatgpt5 -m mode1 -s callgraph_enhanced -i hourglassvul -t 0x123... -c eth -concurrency 10")
	fmt.Println()
	fmt.Println(ui.Gray + "  # Fuzzy Scan (Mode 2) with DeepSeek" + ui.Reset)
	fmt.Println("  vespera -ai deepseek -m mode2 -t contracts.txt -c eth")
	fmt.Println()
	fmt.Println(ui.Gray + "  # Download Contracts" + ui.Reset)
	fmt.Println("  vespera -d -range 1000-2000")
}

func showBenchmarkHelp() {
	fmt.Println(ui.Cyan + "📊 BENCHMARK MODE (-b)" + ui.Reset)
	fmt.Println(ui.Gray + "Run benchmark tests against a dataset of known vulnerabilities." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "USAGE:" + ui.Reset)
	fmt.Println("  vespera -b [OPTIONS]")
	fmt.Println()

	fmt.Println(ui.Cyan + "OPTIONS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "--database <file>", "Dataset file for benchmark (default: benchmark/dataset.json)")
	fmt.Printf("  %-25s %s\n", "-ai <provider>", "Select AI provider for analysis (default: deepseek)")
	fmt.Printf("  %-25s %s\n", "-s <strategy>", "Scanning strategy/prompt (default: default)")
	fmt.Printf("  %-25s %s\n", "-i <input>", "Input file (default: default)")
	fmt.Printf("  %-25s %s\n", "-concurrency <n>", "Number of concurrent workers (default: 5)")
	fmt.Printf("  %-25s %s\n", "-r <dir>", "Report output directory (default: benchmark/reports)")
	fmt.Println()

	fmt.Println(ui.Cyan + "EXAMPLES:" + ui.Reset)
	fmt.Println("  vespera -b")
	fmt.Println("  vespera -b --database benchmark/custom_dataset.json -ai chatgpt5")
}

func showDownloadHelp() {
	fmt.Println(ui.Cyan + "📥 DOWNLOAD MODE (-d)" + ui.Reset)
	fmt.Println(ui.Gray + "Download contract source code from blockchain to local database." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "USAGE:" + ui.Reset)
	fmt.Println("  vespera -d [OPTIONS]")
	fmt.Println()

	fmt.Println(ui.Cyan + "OPTIONS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "-range <start-end>", "Block range to download (e.g. 1000-2000)")
	fmt.Printf("  %-25s %s\n", "-file <path>", "Retry/Download specific addresses from file")
	fmt.Printf("  %-25s %s\n", "-c <chain>", "Blockchain network (eth/bsc/base) [default: eth]")
	fmt.Printf("  %-25s %s\n", "-concurrency <n>", "Number of concurrent workers [default: 4]")
	fmt.Printf("  %-25s %s\n", "-proxy <url>", "Use HTTP proxy for requests")
	fmt.Println()

	fmt.Println(ui.Cyan + "EXAMPLES:" + ui.Reset)
	fmt.Println("  vespera -d -c eth                                        # Resume ETH download from last block")
	fmt.Println("  vespera -d -range 1000-2000 -c bsc                       # Download BSC blocks 1000-2000")
	fmt.Println("  vespera -d -file contracts.txt -concurrency 10           # Download addresses with 10 threads")
}

func showAIHelp() {
	fmt.Println(ui.Cyan + "🤖 AI PROVIDER (-ai)" + ui.Reset)
	fmt.Println(ui.Gray + "Select the AI model for contract analysis." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "SUPPORTED PROVIDERS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "chatgpt5", "OpenAI ChatGPT-5 (Recommended)")
	fmt.Printf("  %-25s %s\n", "deepseek", "DeepSeek AI (Cost-effective)")
	fmt.Printf("  %-25s %s\n", "gemini", "Google Gemini Pro")
	fmt.Printf("  %-25s %s\n", "local-llm", "Local LLM via Ollama")
	fmt.Println()

	fmt.Println(ui.Cyan + "CONFIGURATION:" + ui.Reset)
	fmt.Println("  Set API keys in " + ui.Bold + "config/settings.yaml" + ui.Reset)
	fmt.Println("  Or use env vars: OPENAI_API_KEY, DEEPSEEK_API_KEY")
}

func showModeHelp() {
	fmt.Println(ui.Cyan + "🎯 SCAN MODES (-m)" + ui.Reset)
	fmt.Println(ui.Gray + "Select the vulnerability scanning methodology." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "AVAILABLE MODES:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "mode1 (Targeted)", "Precise scan based on known vulnerability patterns")
	fmt.Printf("  %-25s %s\n", "mode2 (Fuzzy)", "Hybrid scan: Slither static analysis + AI verification")
	fmt.Println()

	fmt.Println(ui.Cyan + "DETAILS:" + ui.Reset)
	fmt.Println("  " + ui.Bold + "mode1" + ui.Reset + ": Best for finding specific, complex logic bugs using tailored prompts.")
	fmt.Println("         Optional -i <toml_file> (from strategy/exp_libs/mode1/). Default: generic_scan.")
	fmt.Println("  " + ui.Bold + "mode2" + ui.Reset + ": Best for general purpose scanning. Reduces false positives from static analysis.")
}

func showStrategyHelp() {
	fmt.Println(ui.Cyan + "📋 SCAN STRATEGY (-s)" + ui.Reset)
	fmt.Println(ui.Gray + "Specify the AI prompt template to use." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "STRATEGIES:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "default", "Use the default prompt template (mode1=generic_scan, mode2=default)")
	fmt.Printf("  %-25s %s\n", "<name>", "Use a specific template (e.g. 'custom' for custom.tmpl)")
	fmt.Println()

	fmt.Println(ui.Cyan + "LOCATIONS:" + ui.Reset)
	fmt.Println("  Templates: " + ui.Bold + "strategy/prompts/<mode>/<name>.tmpl" + ui.Reset)
}

func showInputHelp() {
	fmt.Println(ui.Cyan + "📄 INPUT FILE (-i)" + ui.Reset)
	fmt.Println(ui.Gray + "Specify the vulnerability definition file for Mode 1 (Targeted Scan)." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "USAGE:" + ui.Reset)
	fmt.Println("  vespera -m mode1 -i <filename>")
	fmt.Println()

	fmt.Println(ui.Cyan + "DETAILS:" + ui.Reset)
	fmt.Println("  The input file must be a TOML file located in: " + ui.Bold + "strategy/exp_libs/mode1/" + ui.Reset)
	fmt.Println("  It defines the vulnerability pattern, detection logic, and AI prompts.")
	fmt.Println()

	fmt.Println(ui.Cyan + "EXAMPLES:" + ui.Reset)
	fmt.Println("  vespera -m mode1 -i hourglass.toml")
	fmt.Println("  vespera -m mode1 -i reentrancy.toml")
}

func showTargetHelp() {
	fmt.Println(ui.Cyan + "🎯 SCAN TARGETS (-t)" + ui.Reset)
	fmt.Println(ui.Gray + "Specify the source of contracts to scan." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "AUTO DETECTION:" + ui.Reset)
	fmt.Println("  -t <0x...>           => scan single contract")
	fmt.Println("  -t <targets.txt>     => scan file targets")
	fmt.Println("  -t <start-end>       => scan db with block range filter")
	fmt.Println()

	fmt.Println(ui.Cyan + "TARGET TYPES:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "contract", "Single contract address")
	fmt.Printf("  %-25s %s\n", "file", "List of addresses from file (txt/yaml)")
	fmt.Printf("  %-25s %s\n", "db", "Contracts from local database")
	fmt.Printf("  %-25s %s\n", "last", "Real-time monitoring of new blocks (mode2 only)")
	fmt.Println()

	fmt.Println(ui.Cyan + "OPTIONS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "-addr <addr>", "Target address (for -t contract)")
	fmt.Printf("  %-25s %s\n", "-file <path>", "File path (for -t file)")
	fmt.Printf("  %-25s %s\n", "-range <range>", "Block range filter (for -t db) e.g. 1000-2000")
	fmt.Printf("  %-25s %s\n", "-c <chain>", "Target chain (default: eth)")
	fmt.Printf("  %-25s %s\n", "-concurrency <n>", "Number of concurrent scanning workers")
	fmt.Println()

	fmt.Println(ui.Cyan + "EXAMPLES:" + ui.Reset)
	fmt.Println("  # Unified Flags:")
	fmt.Println("  vespera -t contract -addr 0x123... -c eth")
	fmt.Println("  vespera -t file -file targets.txt -concurrency 5")
	fmt.Println("  vespera -t db -range 5000000-5000100 -c bsc")
	fmt.Println()
	fmt.Println("  # Auto Detection:")
	fmt.Println("  vespera -t 0x123... -c eth")
	fmt.Println("  vespera -t targets.txt -c eth")
	fmt.Println("  vespera -t 5000000-5000100 -c bsc")
	fmt.Println()
	fmt.Println("  # Shortcut Syntax:")
	fmt.Println("  vespera -t contract:0x123... -c eth")
	fmt.Println("  vespera -t file:targets.txt")
	fmt.Println("  vespera -t db:5000000-5000100")
}

func showChainHelp() {
	fmt.Println(ui.Cyan + "⛓️  BLOCKCHAIN NETWORK (-c)" + ui.Reset)
	fmt.Println(ui.Gray + "Specify the target blockchain network." + ui.Reset)
	fmt.Println()

	fmt.Println(ui.Cyan + "SUPPORTED NETWORKS:" + ui.Reset)
	fmt.Printf("  %-25s %s\n", "eth", "Ethereum Mainnet (Default)")
	fmt.Printf("  %-25s %s\n", "bsc", "Binance Smart Chain")
	fmt.Printf("  %-25s %s\n", "base", "Base Network")
}

// helloq ParseFlags 解析命令行参数
func ParseFlags() (*CLIConfig, error) {
	// 检查是否请求帮助
	if len(os.Args) > 1 {
		// 处理特定命令的帮助请求 (如 -d --help, -ai --help)
		for i := 1; i < len(os.Args)-1; i++ {
			if os.Args[i+1] == "--help" || os.Args[i+1] == "-h" {
				// 移除前缀的 - 或 --
				cmd := os.Args[i]
				if strings.HasPrefix(cmd, "--") {
					cmd = cmd[2:]
				} else if strings.HasPrefix(cmd, "-") {
					cmd = cmd[1:]
				}
				showHelp(cmd)
				os.Exit(0) // Exit cleanly after showing help
			}
		}

		// 处理通用帮助请求
		for _, arg := range os.Args[1:] {
			if arg == "--help" || arg == "-h" {
				showGeneralHelp()
				os.Exit(0) // Exit cleanly after showing help
			}
		}
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.Usage = func() {
		showGeneralHelp()
	}

	// 新增下载相关 flags（不包含 rpc/dbdsn）
	downloadFlag := fs.Bool("d", false, "Start block/contract download flow (resume from last DB block, or use -range)")
	drange := fs.String("d-range", "", "DEPRECATED: Use -range instead")
	proxy := fs.String("proxy", "", "Optional HTTP proxy, e.g. http://127.0.0.1:7897 (used for downloads/Etherscan)")

	ai := fs.String("ai", "", "AI provider to use (e.g. chatgpt5)")
	mode := fs.String("m", "", "Mode to run: mode1(targeted) | mode2(fuzzy)")
	strategy := fs.String("s", "default", "Scanning strategy/prompt (default: default)")
	target := fs.String("t", "db", "Target: db|file|contract|last OR <address>|<targets.txt>|<start-end>")
	blockRange := fs.String("t-block", "", "DEPRECATED: Use -range instead")
	tfile := fs.String("t-file", "", "DEPRECATED: Use -file instead")
	taddress := fs.String("t-address", "", "DEPRECATED: Use -addr instead")
	chain := fs.String("c", "eth", "Chain to scan: eth | bsc | base (default eth)")
	concurrency := fs.Int("concurrency", 4, "Worker concurrency")
	verbose := fs.Bool("v", false, "Verbose output")
	timeout := fs.Duration("timeout", 120*time.Second, "Per-AI request timeout")
	fileFlag := fs.String("file", "", "Input file path (for -t file or -d)")
	inputFile := fs.String("i", "", "Input file (Mode1 only: TOML under strategy/exp_libs/mode1/)")
	reportDir := fs.String("r", "reports", "Markdown report output directory (default: reports)")

	// Unified flags
	rangeFlag := fs.String("range", "", "Block range (start-end) for -t db or -d")
	addrFlag := fs.String("addr", "", "Target address for -t contract")

	// Benchmark flags
	benchmark := fs.Bool("b", false, "Run benchmark mode")
	benchmarkLong := fs.Bool("benchmark", false, "Run benchmark mode")
	database := fs.String("database", "benchmark/dataset.json", "Dataset file for benchmark")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, err
	}

	// Smart parsing for -t (type:value)
	targetSource := strings.TrimSpace(*target)
	targetAddress := strings.TrimSpace(*taddress)
	targetFile := strings.TrimSpace(*tfile)
	downloadFile := strings.TrimSpace(*fileFlag)
	targetRangeHint := ""

	// If -t contains ':', split it
	if strings.Contains(targetSource, ":") {
		parts := strings.SplitN(targetSource, ":", 2)
		if len(parts) == 2 {
			targetSource = parts[0]
			val := strings.TrimSpace(parts[1])
			// Assign value based on type
			switch targetSource {
			case "contract":
				targetAddress = val
			case "file":
				targetFile = val
			case "db":
				// Will be parsed later as range
				*blockRange = val // Use legacy var to pass to parser below
			}
		}
	} else {
		switch strings.ToLower(targetSource) {
		case "db", "file", "contract", "address", "last":
			targetSource = strings.ToLower(targetSource)
		default:
			if isHexAddress(targetSource) {
				targetAddress = targetSource
				targetSource = "contract"
			} else if looksLikeBlockRange(targetSource) {
				targetRangeHint = targetSource
				targetSource = "db"
			} else if looksLikeTargetFile(targetSource) {
				targetFile = targetSource
				targetSource = "file"
			}
		}
	}

	// Apply unified flags (override legacy/smart if provided explicitly)
	if *addrFlag != "" {
		targetAddress = *addrFlag
	}
	if *fileFlag != "" && !*downloadFlag {
		// If not in download mode, -file can be used for target file
		targetFile = *fileFlag
	}
	// For download mode, downloadFile is already set to *fileFlag

	// Range unification
	rangeStr := *rangeFlag
	if rangeStr == "" {
		// Fallback to legacy flags
		if *drange != "" {
			rangeStr = *drange
		} else if *blockRange != "" {
			rangeStr = *blockRange
		}
		if rangeStr == "" && targetRangeHint != "" {
			rangeStr = targetRangeHint
		}
	}

	cfg := &CLIConfig{
		AIProvider:    strings.TrimSpace(*ai),
		Mode:          strings.TrimSpace(*mode),
		Strategy:      strings.TrimSpace(*strategy),
		TargetSource:  targetSource,
		TargetFile:    targetFile,
		TargetAddress: targetAddress,
		Chain:         strings.TrimSpace(*chain),
		Concurrency:   *concurrency,
		Verbose:       *verbose,
		Timeout:       *timeout,
		Download:      *downloadFlag,
		Proxy:         strings.TrimSpace(*proxy),
		DownloadFile:  downloadFile,
		InputFile:     strings.TrimSpace(*inputFile),
		ReportDir:     strings.TrimSpace(*reportDir),
		Benchmark:     *benchmark || *benchmarkLong,
		Database:      strings.TrimSpace(*database),
	}

	// 解析下载区块范围（如果提供）
	if rangeStr != "" {
		br, err := parseBlockRange(rangeStr)
		if err != nil {
			return nil, err
		}
		if cfg.Download {
			cfg.DownloadRange = br
		} else {
			cfg.BlockRange = br
		}
	}

	// normalize target source
	cfg.TargetSource = strings.ToLower(cfg.TargetSource)
	if cfg.TargetSource == "yaml" {
		cfg.TargetSource = "file" // accept yaml alias
	}

	// 如果提供了文件路径但不是绝对路径，则将其转为相对于当前工作目录
	if cfg.TargetFile != "" {
		if !filepath.IsAbs(cfg.TargetFile) {
			cwd, _ := os.Getwd()
			cfg.TargetFile = filepath.Join(cwd, cfg.TargetFile)
		}
	}

	// Benchmark defaults
	if cfg.Benchmark {
		if cfg.Mode == "" {
			cfg.Mode = "mode1" // Default mode for benchmark
		}
		if cfg.Strategy == "all" || cfg.Strategy == "" {
			cfg.Strategy = "default"
		}
		// Note: InputFile is not strictly required if the runner handles it,
		// but if we want to be consistent with CLI args:
		if cfg.InputFile == "" {
			cfg.InputFile = "default"
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func Run() error {
	cfg, err := ParseFlags()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigChan)
		close(sigChan)
	}()

	go func() {
		count := 0
		for range sigChan {
			count++
			if count == 1 {
				fmt.Fprintln(os.Stderr, "\nInterrupt received, stopping... (press Ctrl+C again to force exit)")
				cancel()
				continue
			}
			fmt.Fprintln(os.Stderr, "\nForce exiting...")
			os.Exit(130)
		}
	}()

	return Execute(ctx, cfg)
}

func PrintFatal(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}

	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
