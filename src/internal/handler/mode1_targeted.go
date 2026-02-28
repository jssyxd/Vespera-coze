package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vespera/internal/ai"
	"vespera/internal/astparser"
	"vespera/internal/cleaner"
	"vespera/internal/config"
	"vespera/internal/dbutil"
	"vespera/internal/download"
	"vespera/internal/logger"
	"vespera/internal/ui"
	"vespera/strategy/prompts"
	"golang.org/x/sync/singleflight"
)

// qhello RunMode1Targeted Mode1 定向扫描入口
func RunMode1Targeted(ctx context.Context, cfg config.ScanConfiguration) error {
	logger.Info("🎯 Starting Mode1 Targeted Vulnerability Scan...")

	db, err := config.InitDB(ctx)
	if err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}
	defer db.Close()

	aiManager, err := ai.NewManager(ai.ManagerConfig{
		Provider:       cfg.AIProvider,
		Timeout:        cfg.Timeout,
		RequestsPerMin: 120, //qhello AI速率 这个不同模型都不一样,120也是比较保守了对于openai
		Strategy:       cfg.Strategy,
		APIKey:         cfg.APIKey,
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		Proxy:          cfg.Proxy,
		Verbose:        cfg.Verbose,
	})
	if err != nil {
		return fmt.Errorf("failed to create AI manager: %w", err)
	}
	defer aiManager.Close()

	if err := aiManager.TestConnection(ctx); err != nil {
		logger.Warn("⚠️  AI Connection Test Failed (Non-fatal): %v", err)
	} else {
		logger.Info("✅ AI Client Connected Successfully!")
	}

	//helloq 模板加载: -s 指定 .tmpl, -i 指定 .toml 漏洞特征
	templateName := cfg.Strategy
	// 不需要 TOML 输入的独立模板列表
	standaloneTemplates := map[string]bool{
		"generic_scan":       true,
		"callgraph_enhanced": true,
	}
	if cfg.Mode == "mode1" && cfg.InputFile == "" {
		if templateName == "" || templateName == "default" {
			templateName = "generic_scan"
			logger.Info("ℹ️  Mode1: No input file (-i) specified, forcing generic scan template: generic_scan.tmpl")
		} else if !standaloneTemplates[templateName] {
			// 只有非独立模板才需要警告
			logger.Warn("⚠️  Template '%s' specified without input file (-i). This template might require TOML input.", templateName)
		}
	}

	promptTemplate, err := prompts.LoadTemplate(cfg.Mode, templateName)
	if err != nil {
		return fmt.Errorf("failed to load prompt template: %w", err)
	}
	needCallGraph := strings.Contains(promptTemplate, ".EnableCallGraph") ||
		strings.Contains(promptTemplate, ".CallGraphInfo") ||
		strings.Contains(promptTemplate, ".CallersCode") ||
		strings.Contains(promptTemplate, ".CalleesCode") ||
		strings.Contains(promptTemplate, ".EnrichedContext")

	var inputFiles []string
	if cfg.InputFile != "" {
		if cfg.InputFile == "all" {
			expLibsDirs := []string{
				filepath.Join("strategy", "exp_libs", "mode1"),
				filepath.Join("src", "strategy", "exp_libs", "mode1"),
			}
			var globErr error
			for _, expLibsDir := range expLibsDirs {
				tomlFiles, err := filepath.Glob(filepath.Join(expLibsDir, "*.toml"))
				if err != nil {
					globErr = err
					continue
				}
				if len(tomlFiles) == 0 {
					continue
				}
				inputFiles = tomlFiles
				logger.Info("📁 Found %d toml files in %s, scanning sequentially:", len(tomlFiles), expLibsDir)
				for i, file := range tomlFiles {
					fileName := filepath.Base(file)
					logger.Info("   %d. %s", i+1, fileName)
				}
				break
			}
			if len(inputFiles) == 0 {
				if globErr != nil {
					logger.Warn("⚠️  Failed to glob toml files: %v", globErr)
				} else {
					logger.Warn("⚠️  No toml files found in strategy/exp_libs/mode1 or src/strategy/exp_libs/mode1")
				}
			}
		} else {
			inputFiles = []string{cfg.InputFile}
			logger.Info("📁 Using specified input file: %s", cfg.InputFile)
		}
	} else {
		inputFiles = []string{"__GENERIC_SCAN__"}
	}

	logger.Info("📝 Active Strategies:")
	logger.Info("   Template: %s", templateName)
	for i, f := range inputFiles {
		sName := strings.TrimSuffix(filepath.Base(f), ".toml")
		if f == "__GENERIC_SCAN__" {
			if templateName != "generic_scan" {
				sName = fmt.Sprintf("%s (Standalone Template)", templateName)
			} else {
				sName = "generic_scan (General Vulnerability Scan)"
			}
		}
		logger.Info("   %d. %s", i+1, sName)
	}

	var targetAddresses []string
	switch strings.ToLower(cfg.TargetSource) {
	case "db":
		targetAddresses, err = dbutil.GetAddressesFromDB(db, cfg.Chain, cfg.BlockRange)
		if err != nil {
			return fmt.Errorf("failed to get addresses from DB: %w", err)
		}
	case "file", "filepath":
		targetAddresses, err = getAddressesFromFile(cfg.TargetFile)
		if err != nil {
			return fmt.Errorf("failed to get addresses from file: %w", err)
		}
		targetAddresses = uniqueStrings(targetAddresses)
	case "contract", "address", "single":
		if strings.TrimSpace(cfg.TargetAddress) == "" {
			return fmt.Errorf("missing target address: -addr")
		}
		targetAddresses = []string{strings.TrimSpace(cfg.TargetAddress)}
	default:
		return fmt.Errorf("unsupported target source: %s", cfg.TargetSource)
	}

	if len(targetAddresses) == 0 {
		logger.Warn("⚠️  No target contracts found to scan")
		return nil
	}

	logger.Info("📋 Found %d unique target contracts", len(targetAddresses))

	downloader, err := download.NewDownloader(db, cfg.Chain, cfg.Proxy)
	if err != nil {
		return fmt.Errorf("failed to create downloader: %w", err)
	}
	defer func() {
		if downloader != nil {
			downloader.Close()
		}
	}()

	type cachedContract struct {
		Code          string
		EffectiveAddr string
		IsProxy       bool
		IsBytecode    bool
	}

	type cachedPreprocess struct {
		FinalCode         string
		EnableCallGraph   bool
		CallGraphInfo     string
		CallersCode       string
		CalleesCode       string
		EnrichedContext   string
		TotalFunctions    int
		PublicFunctions   int
		InternalFunctions int
		OriginalLen       int
	}

	var contractCache sync.Map
	var preprocessCache sync.Map
	var contractSF singleflight.Group
	var preprocessSF singleflight.Group

	hashString := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}

	getCachedContract := func(addr string) (*cachedContract, error) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return nil, fmt.Errorf("empty address")
		}
		if v, ok := contractCache.Load(addr); ok {
			return v.(*cachedContract), nil
		}
		v, err, _ := contractSF.Do(addr, func() (interface{}, error) {
			if vv, ok := contractCache.Load(addr); ok {
				return vv, nil
			}
			code, effectiveAddr, isProxy, err := getOrDownloadContract(ctx, db, downloader, addr)
			if err != nil {
				return nil, err
			}
			c := &cachedContract{
				Code:          code,
				EffectiveAddr: effectiveAddr,
				IsProxy:       isProxy,
				IsBytecode:    isOnlyBytecode(code),
			}
			contractCache.Store(addr, c)
			return c, nil
		})
		if err != nil {
			return nil, err
		}
		return v.(*cachedContract), nil
	}

	getCachedPreprocess := func(contract *cachedContract, targetAddr string) (*cachedPreprocess, error) {
		if contract == nil {
			return nil, fmt.Errorf("nil contract")
		}
		key := contract.EffectiveAddr + ":" + hashString(contract.Code)
		if v, ok := preprocessCache.Load(key); ok {
			return v.(*cachedPreprocess), nil
		}
		v, err, _ := preprocessSF.Do(key, func() (interface{}, error) {
			if vv, ok := preprocessCache.Load(key); ok {
				return vv, nil
			}

			originalLen := len(contract.Code)
			cleanCode := cleaner.CleanCode(contract.Code, true)

			if cfg.Verbose && astparser.IsJSONSource(contract.Code) && !astparser.IsJSONSource(cleanCode) {
				saveDir := "flattened_contracts"
				if err := os.MkdirAll(saveDir, 0755); err == nil {
					savePath := filepath.Join(saveDir, fmt.Sprintf("%s.sol", targetAddr))
					_ = os.WriteFile(savePath, []byte(cleanCode), 0644)
				}
			}

			finalCode := cleanCode

			enableCallGraph := false
			callGraphInfo := ""
			callersCode := ""
			calleesCode := ""
			enrichedContext := ""
			totalFunctions := 0
			publicFunctions := 0
			internalFunctions := 0

			parsedSource, err := astparser.ParseSource(cleanCode)
			if err == nil {
				if needCallGraph {
					callGraph := parsedSource.BuildCallGraph()
					if callGraph != nil {
						enableCallGraph = true
						totalFunctions = len(callGraph.Functions)
						publicFunctions = len(callGraph.GetPublicEntryPoints())
						internalFunctions = totalFunctions - publicFunctions
						if internalFunctions < 0 {
							internalFunctions = 0
						}

						enrichedContext, callersCode, calleesCode, callGraphInfo = buildCallGraphContext(callGraph, parsedSource)
					}
				}

				prunedCode, pruneErr := parsedSource.PruneDeadCode("", true)
				if pruneErr == nil && len(prunedCode) >= 100 {
					finalCode = prunedCode
				}
			}

			p := &cachedPreprocess{
				FinalCode:         finalCode,
				EnableCallGraph:   enableCallGraph,
				CallGraphInfo:     callGraphInfo,
				CallersCode:       callersCode,
				CalleesCode:       calleesCode,
				EnrichedContext:   enrichedContext,
				TotalFunctions:    totalFunctions,
				PublicFunctions:   publicFunctions,
				InternalFunctions: internalFunctions,
				OriginalLen:       originalLen,
			}
			preprocessCache.Store(key, p)
			return p, nil
		})
		if err != nil {
			return nil, err
		}
		return v.(*cachedPreprocess), nil
	}

	//helloq Worker Pool 并发扫描
	type ScanTask struct {
		InputFile        string
		InputFileContent string
		StrategyName     string
		TargetAddress    string
		FileIndex        int
		AddrIndex        int
		TotalFiles       int
		TotalAddrs       int
	}

	totalTasks := len(inputFiles) * len(targetAddresses)
	pb := ui.NewProgressBar(totalTasks, "🚀 Scanning")
	resultsChan := make(chan *ScanResult, totalTasks)
	var wg sync.WaitGroup
	var successCount int64
	var failCount int64
	taskChan := make(chan ScanTask, totalTasks)
	results := make([]*ScanResult, 0, totalTasks)
	var resultsMu sync.Mutex
	var reportOnce sync.Once
	var closeTasksOnce sync.Once

	writeReport := func() error {
		resultsMu.Lock()
		snapshot := append([]*ScanResult(nil), results...)
		resultsMu.Unlock()
		if len(snapshot) == 0 {
			return nil
		}
		return generateReport(snapshot, cfg)
	}

	go func() {
		<-ctx.Done()
		reportOnce.Do(func() {
			if err := writeReport(); err != nil {
				logger.Warn("⚠️  Failed to generate partial report: %v", err)
			} else {
				logger.Info("📝 Partial report generated (interrupted)")
			}
		})
	}()

	workerCount := cfg.Concurrency
	if workerCount <= 0 {
		workerCount = 1
	}
	logger.InfoFileOnly("🚀 Starting %d concurrent Workers...", workerCount)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskChan {
				if ctx.Err() != nil {
					return
				}
				logger.InfoFileOnly("[Worker %d] [File %d/%d] [Contract %d/%d] Processing: %s",
					workerID, task.FileIndex+1, task.TotalFiles, task.AddrIndex+1, task.TotalAddrs, task.TargetAddress)

				contract, err := getCachedContract(task.TargetAddress)
				if err != nil {
					logger.InfoFileOnly("⚠️  [Worker %d] Failed to get contract code: %v, skipping", workerID, err)
					pb.Increment()
					continue
				}

				if contract.IsBytecode {
					logger.InfoFileOnly("  ⏭️  [Worker %d] Contract not open source (bytecode only), skipping", workerID)
					pb.Increment()
					continue
				}

				pre, err := getCachedPreprocess(contract, task.TargetAddress)
				if err != nil {
					logger.InfoFileOnly("⚠️  [Worker %d] Preprocess failed: %v, skipping", workerID, err)
					pb.Increment()
					continue
				}

				contractCode := pre.FinalCode
				if len(contractCode) < pre.OriginalLen && pre.OriginalLen > 0 {
					logger.InfoFileOnly("  🧹 [Worker %d] Optimization: %d -> %d chars (reduced %.2f%%)",
						workerID, pre.OriginalLen, len(contractCode), float64(pre.OriginalLen-len(contractCode))/float64(pre.OriginalLen)*100)
				}

				// 构建 Prompt 变量（包含调用图信息）
				//qhello 给提示词可以用的
				promptVars := &prompts.PromptVariables{
					ContractAddress:  task.TargetAddress,
					ContractCode:     contractCode,
					Strategy:         task.StrategyName,
					InputFileContent: task.InputFileContent,
					EnableCallGraph:  pre.EnableCallGraph,
					CallGraphInfo:    pre.CallGraphInfo,
					CallersCode:      pre.CallersCode,
					CalleesCode:      pre.CalleesCode,
					EnrichedContext:  pre.EnrichedContext,
				}

				promptVars.TotalFunctions = pre.TotalFunctions
				promptVars.PublicFunctions = pre.PublicFunctions
				promptVars.InternalFunctions = pre.InternalFunctions

				prompt := prompts.BuildPrompt(promptTemplate, promptVars)

				analysisResult, err := aiManager.AnalyzeContractWithStrategy(ctx, contractCode, prompt, task.StrategyName)
				if err != nil {
					logger.InfoFileOnly("⚠️  [Worker %d] AI Analysis Failed: %v, skipping", workerID, err)
					pb.Increment()
					continue
				}

				scanResult := &ScanResult{
					Address:         task.TargetAddress,
					ResolvedAddress: contract.EffectiveAddr,
					IsProxy:         contract.IsProxy,
					AnalysisResult:  analysisResult,
					Timestamp:       time.Now(),
					Mode:            cfg.Mode,
					Strategy:        task.StrategyName,
					InputFile:       filepath.Base(task.InputFile),
				}

				resultsMu.Lock()
				results = append(results, scanResult)
				resultsMu.Unlock()

				resultsChan <- scanResult

				printVulnerabilitySummary(scanResult)

				if analysisResult != nil && len(analysisResult.Vulnerabilities) > 0 {
					pb.AddVuln()
					vulnTypes := make([]string, 0)
					for _, v := range analysisResult.Vulnerabilities {
						vulnTypes = append(vulnTypes, v.Type)
					}
					if len(vulnTypes) > 3 {
						vulnTypes = append(vulnTypes[:3], fmt.Sprintf("... (+%d)", len(vulnTypes)-3))
					}

					displayAddr := task.TargetAddress
					if contract.EffectiveAddr != "" && contract.EffectiveAddr != task.TargetAddress {
						displayAddr = fmt.Sprintf("%s -> %s", task.TargetAddress, contract.EffectiveAddr)
					}

					msg := ui.FormatVulnMsg(displayAddr, vulnTypes)
					pb.PrintMsg(msg)
				}

				pb.Increment()
			}
		}(i)
	}

	for fileIndex, inputFile := range inputFiles {
		if ctx.Err() != nil {
			closeTasksOnce.Do(func() { close(taskChan) })
			reportOnce.Do(func() {
				if err := writeReport(); err != nil {
					logger.Warn("⚠️  Failed to generate partial report: %v", err)
				} else {
					logger.Info("📝 Partial report generated (interrupted)")
				}
			})
			return ctx.Err()
		}
		var inputFileContent string
		var strategyName string
		var err error

		if inputFile == "__GENERIC_SCAN__" {
			strategyName = templateName
			inputFileContent = ""
		} else {
			inputFileContent, err = prompts.LoadInputFile(inputFile)
			if err != nil {
				logger.Warn("⚠️  Failed to load input file: %v, skipping", err)
				continue
			}

			if cfg.InputFile == "all" {
				strategyName = strings.TrimSuffix(filepath.Base(inputFile), ".toml")
			} else {
				strategyName = templateName
			}
		}

		for addrIndex, address := range targetAddresses {
			task := ScanTask{
				InputFile:        inputFile,
				InputFileContent: inputFileContent,
				StrategyName:     strategyName,
				TargetAddress:    address,
				FileIndex:        fileIndex,
				AddrIndex:        addrIndex,
				TotalFiles:       len(inputFiles),
				TotalAddrs:       len(targetAddresses),
			}
			select {
			case taskChan <- task:
			case <-ctx.Done():
				reportOnce.Do(func() {
					if err := writeReport(); err != nil {
						logger.Warn("⚠️  Failed to generate partial report: %v", err)
					} else {
						logger.Info("📝 Partial report generated (interrupted)")
					}
				})
				closeTasksOnce.Do(func() { close(taskChan) })
				return ctx.Err()
			}
		}
	}
	closeTasksOnce.Do(func() { close(taskChan) })
	wg.Wait()
	close(resultsChan)
	pb.Finish()

	for range resultsChan {
	}
	resultsMu.Lock()
	successCount = int64(len(results))
	resultsMu.Unlock()
	failCount = int64(totalTasks) - successCount

	logger.Info("\n%s", strings.Repeat("=", 50))
	logger.Info("✅ Scan Completed!")
	logger.Info("   - Input Files: %d", len(inputFiles))
	logger.Info("   - Targets:     %d", len(targetAddresses))
	logger.Info("   - Total Scans: %d", totalTasks)
	logger.Info("   - Success:     %d", successCount)

	if failCount > 0 {
		logger.Info("   - Failed/Skip: %d (Check logs for details)", failCount)
	} else {
		logger.Info("   - Failed/Skip: %d", failCount)
	}

	logger.Info("   - Vulnerable:  %d", countVulnerableContracts(results))
	logger.Info("%s\n", strings.Repeat("=", 50))

	reportOnce.Do(func() {
		if err := writeReport(); err != nil {
			logger.Warn("⚠️  Failed to generate report: %v", err)
		}
	})

	return nil
}

// buildCallGraphContext 构建调用图上下文信息
// 返回: enrichedContext, callersCode, calleesCode, callGraphInfo
func buildCallGraphContext(cg *astparser.CallGraphFull, ps *astparser.ParsedSource) (string, string, string, string) {
	if cg == nil {
		return "", "", "", ""
	}

	var enrichedBuilder strings.Builder
	var callersBuilder strings.Builder
	var calleesBuilder strings.Builder
	var infoBuilder strings.Builder
	const maxEnrichedChars = 20000
	const maxCalleesChars = 20000

	// 1. 构建调用图概要信息
	entryPoints := cg.GetPublicEntryPoints()
	infoBuilder.WriteString(fmt.Sprintf("// Call Graph Summary:\n"))
	infoBuilder.WriteString(fmt.Sprintf("// - Total Functions: %d\n", len(cg.Functions)))
	infoBuilder.WriteString(fmt.Sprintf("// - Public Entry Points: %d\n", len(entryPoints)))
	internalFuncs := len(cg.Functions) - len(entryPoints)
	if internalFuncs < 0 {
		internalFuncs = 0
	}
	infoBuilder.WriteString(fmt.Sprintf("// - Internal Functions: %d\n\n", internalFuncs))

	// 2. 构建结构化的调用树 (Enriched Context)
	tree := cg.GenerateCallGraphTree()
	if len(tree) > maxEnrichedChars {
		tree = tree[:maxEnrichedChars]
	}
	enrichedBuilder.WriteString(tree)

	// 3. 构建被调用者代码 (Callees Code)
	// 获取所有被调用的函数（深度5）
	relatedFuncs := cg.GetAllRelatedFunctions(5)

	for _, node := range relatedFuncs {
		if calleesBuilder.Len() >= maxCalleesChars {
			break
		}
		ref := cg.FunctionRefs[node.ID]
		if ref != nil {
			calleesBuilder.WriteString(fmt.Sprintf("// Internal/Called Function: %s.%s\n", ref.ContractName, ref.FunctionName))
		} else {
			calleesBuilder.WriteString(fmt.Sprintf("// Internal/Called Function: %s\n", node.Name))
		}
		src := ps.GetSourceRange(node.Src)
		remaining := maxCalleesChars - calleesBuilder.Len()
		if remaining <= 0 {
			break
		}
		if len(src)+2 > remaining {
			if remaining > 2 {
				src = src[:remaining-2]
			} else {
				break
			}
		}
		calleesBuilder.WriteString(src + "\n\n")
	}

	// 4. Callers Code (暂时留空，或用于未来扩展)
	// 在单合约分析中，外部调用者未知。内部调用关系已在 Call Graph Tree 中体现。

	return enrichedBuilder.String(), callersBuilder.String(), calleesBuilder.String(), infoBuilder.String()
}
