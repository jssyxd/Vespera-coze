package download

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VectorBits/Vespera/src/internal/config"
	"github.com/VectorBits/Vespera/src/internal/solc"
	"github.com/VectorBits/Vespera/src/internal/ui"
	"github.com/ethereum/go-ethereum/common"
)

type ContractInfo struct {
	Address        string
	Contract       string
	ABI            string
	Balance        string
	IsOpenSource   int
	IsProxy        int
	Implementation string
	CreateTime     time.Time
	CreateBlock    uint64
	TxLast         time.Time
	IsDecompiled   int
	DedCode        string
}

type Downloader struct {
	RPCManager      *config.RPCManager
	db              *sql.DB
	etherscanConfig EtherscanConfig
	rateLimiter     *RateLimiter
	ChainName       string
	Concurrency     int
}

func (d *Downloader) hasExplorerAPI() bool {
	if d == nil {
		return false
	}
	if d.etherscanConfig.APIKeyManager != nil {
		if mgr, ok := d.etherscanConfig.APIKeyManager.(interface{ HasKeys() bool }); ok {
			if mgr.HasKeys() {
				return true
			}
		}
	}
	return strings.TrimSpace(d.etherscanConfig.APIKey) != ""
}

func (d *Downloader) ResolveSourceForScan(address string) (string, bool, error) {
	if !d.hasExplorerAPI() {
		return "", false, nil
	}
	addr := strings.TrimSpace(address)
	if addr == "" {
		return "", false, fmt.Errorf("empty address")
	}
	if d.rateLimiter != nil {
		d.rateLimiter.Wait()
	}
	details, isVerified, err := GetContractDetails(addr, d.etherscanConfig)
	if err != nil {
		return "", false, err
	}
	if !isVerified || details == nil {
		return "", false, nil
	}
	source := strings.TrimSpace(details.SourceCode)
	proxyFlag := strings.TrimSpace(details.Proxy)
	implAddr := strings.TrimSpace(details.Implementation)
	if proxyFlag == "1" && implAddr != "" {
		if d.rateLimiter != nil {
			d.rateLimiter.Wait()
		}
		implDetails, implVerified, implErr := GetContractDetails(implAddr, d.etherscanConfig)
		if implErr != nil {
			if source == "" {
				return "", false, implErr
			}
			return source, true, nil
		}
		if implVerified && implDetails != nil && strings.TrimSpace(implDetails.SourceCode) != "" {
			return implDetails.SourceCode, true, nil
		}
	}
	if source == "" {
		return "", false, nil
	}
	return source, true, nil
}

func NewDownloader(db *sql.DB, chainName string, proxy string) (*Downloader, error) {
	if db == nil {
		return nil, fmt.Errorf("the database connection cannot be nil")
	}

	rpcManager, err := config.GetRPCManager(chainName, proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to get the RPC manager: %w", err)
	}

	explorerConfig, err := config.GetExplorerConfig(chainName)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain the blockchain browser configuration: %w", err)
	}

	apiKeyManager, err := config.GetAPIKeyManager(chainName)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain the API Key manager: %w", err)
	}

	chainConfig, err := config.GetChainConfig(chainName)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain chain configuration: %w", err)
	}

	ethersCfg := EtherscanConfig{
		APIKey:        explorerConfig.APIKey,
		APIKeyManager: apiKeyManager,
		BaseURL:       explorerConfig.BaseURL,
		Proxy:         strings.TrimSpace(proxy),
		ChainID:       chainConfig.ChainID,
	}

	if apiKeyManager != nil && apiKeyManager.HasKeys() {
		log.Printf(ui.Blue+"🔑 %d Etherscan API keys have been loaded "+ui.Reset+"\n", apiKeyManager.GetKeyCount())
	}

	log.Printf(ui.Green+"✅ The downloader has been successfully created. Chain: %s, current RPC: %s"+ui.Reset+"\n", chainName, rpcManager.GetCurrentURL())

	requestsPerSecond := 5
	if apiKeyManager != nil && apiKeyManager.HasKeys() {
		if n := apiKeyManager.GetKeyCount(); n > 0 {
			requestsPerSecond = 5 * n
		}
	}

	return &Downloader{
		RPCManager:      rpcManager,
		db:              db,
		etherscanConfig: ethersCfg,
		rateLimiter:     NewRateLimiter(requestsPerSecond),
		ChainName:       chainName,
		Concurrency:     1,
	}, nil
}

func (d *Downloader) SetConcurrency(concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 50 {
		concurrency = 50
	}
	d.Concurrency = concurrency
	log.Printf(ui.Cyan+"🔧 Set the concurrent number to: %d"+ui.Reset+"\n", d.Concurrency)
}

func (d *Downloader) GetCurrentBlock(ctx context.Context) (uint64, error) {
	client, err := d.RPCManager.GetClient()
	if err != nil {
		return 0, err
	}
	return client.BlockNumber(ctx)
}

func (d *Downloader) GetLastDownloadedBlock(ctx context.Context) (uint64, error) {
	tableName, err := config.GetTableName(d.ChainName)
	if err != nil {
		return 0, err
	}

	var maxBlock sql.NullInt64
	query := fmt.Sprintf("SELECT MAX(createblock) FROM %s", tableName)
	err = d.db.QueryRowContext(ctx, query).Scan(&maxBlock)
	if err != nil {
		return 0, fmt.Errorf("the query failed to download the last block: %w", err)
	}
	if !maxBlock.Valid {
		return 0, nil
	}
	return uint64(maxBlock.Int64), nil
}

func (d *Downloader) ContractExists(ctx context.Context, address string) (bool, error) {
	tableName, err := config.GetTableName(d.ChainName)
	if err != nil {
		return false, err
	}

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE address = ?", tableName)
	err = d.db.QueryRowContext(ctx, query, address).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Downloader) SaveContract(ctx context.Context, info *ContractInfo) error {
	tableName, err := config.GetTableName(d.ChainName)
	if err != nil {
		return err
	}

	query := fmt.Sprintf(`
	INSERT INTO %s (address, contract, abi, balance, isopensource, isproxy, implementation, createtime, createblock, txlast, isdecompiled, dedcode)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE 
		contract = VALUES(contract),
		abi = VALUES(abi),
		balance = VALUES(balance),
		isopensource = VALUES(isopensource),
		isproxy = VALUES(isproxy),
		implementation = VALUES(implementation),
		txlast = VALUES(txlast),
		isdecompiled = VALUES(isdecompiled),
		dedcode = VALUES(dedcode)
	`, tableName)

	_, err = d.db.ExecContext(ctx, query,
		info.Address,
		info.Contract,
		info.ABI,
		info.Balance,
		info.IsOpenSource,
		info.IsProxy,
		info.Implementation,
		info.CreateTime,
		int64(info.CreateBlock),
		info.TxLast,
		info.IsDecompiled,
		info.DedCode,
	)

	return err
}

func (d *Downloader) IsBlockDownloaded(ctx context.Context, blockNum uint64) (bool, error) {
	tableName, err := config.GetTableName(d.ChainName)
	if err != nil {
		return false, err
	}

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE createblock = ?", tableName)
	err = d.db.QueryRowContext(ctx, query, int64(blockNum)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func appendFailAddress(failFile, addr string) {
	if strings.TrimSpace(failFile) == "" || strings.TrimSpace(addr) == "" {
		return
	}
	f, err := os.OpenFile(failFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf(ui.Red+"⚠️ cannot open the failure record file %s: %v"+ui.Reset+"\n", failFile, err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(strings.TrimSpace(addr) + "\n"); err != nil {
		log.Printf(ui.Red+"⚠️ the failure record file %s: %v cannot be written"+ui.Reset+"\n", failFile, err)
	}
}

type BlockRangeRecord struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

const blockedFile = "blocked.json"

func loadBlockedRanges() ([]BlockRangeRecord, error) {
	bs, err := os.ReadFile(blockedFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", blockedFile, err)
	}
	var recs []BlockRangeRecord
	if err := json.Unmarshal(bs, &recs); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", blockedFile, err)
	}
	return recs, nil
}

func saveBlockedRanges(recs []BlockRangeRecord) error {
	bs, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize blocked ranges: %w", err)
	}
	if err := os.WriteFile(blockedFile, bs, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", blockedFile, err)
	}
	return nil
}

func mergeAndInsertRange(existing []BlockRangeRecord, newRange BlockRangeRecord) []BlockRangeRecord {
	existing = append(existing, newRange)
	if len(existing) == 0 {
		return existing
	}
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Start < existing[j].Start
	})
	merged := make([]BlockRangeRecord, 0, len(existing))
	curr := existing[0]
	for i := 1; i < len(existing); i++ {
		r := existing[i]
		if r.Start <= curr.End+1 {
			if r.End > curr.End {
				curr.End = r.End
			}
		} else {
			merged = append(merged, curr)
			curr = r
		}
	}
	merged = append(merged, curr)
	return merged
}

func getUncoveredRanges(existingRanges []BlockRangeRecord, requestStart, requestEnd uint64) []BlockRangeRecord {
	if requestStart > requestEnd {
		return nil
	}
	if len(existingRanges) == 0 {
		return []BlockRangeRecord{{Start: requestStart, End: requestEnd}}
	}
	sort.Slice(existingRanges, func(i, j int) bool { return existingRanges[i].Start < existingRanges[j].Start })
	merged := []BlockRangeRecord{}
	curr := existingRanges[0]
	for i := 1; i < len(existingRanges); i++ {
		r := existingRanges[i]
		if r.Start <= curr.End+1 {
			if r.End > curr.End {
				curr.End = r.End
			}
		} else {
			merged = append(merged, curr)
			curr = r
		}
	}
	merged = append(merged, curr)

	var out []BlockRangeRecord
	cursor := requestStart
	for _, r := range merged {
		if r.End < cursor {
			continue
		}
		if r.Start > requestEnd {
			break
		}
		if r.Start > cursor {
			end := minUint64(r.Start-1, requestEnd)
			if cursor <= end {
				out = append(out, BlockRangeRecord{Start: cursor, End: end})
			}
		}
		if r.End+1 > cursor {
			cursor = r.End + 1
		}
		if cursor > requestEnd {
			break
		}
	}
	if cursor <= requestEnd {
		out = append(out, BlockRangeRecord{Start: cursor, End: requestEnd})
	}
	return out
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func (d *Downloader) DownloadBlockRange(ctx context.Context, startBlock, endBlock uint64) error {
	log.Printf(ui.Cyan+"🔍 start downloading block %d to %d..."+ui.Reset+"\n", startBlock, endBlock)

	existing, err := loadBlockedRanges()
	if err != nil {
		log.Printf(ui.Yellow+"⚠️ failed to read the downloaded interval: %v (continue, but may download again)"+ui.Reset+"\n", err)
		existing = nil
	}

	uncovered := getUncoveredRanges(existing, startBlock, endBlock)
	if len(uncovered) == 0 {
		log.Printf(ui.Green+"✅ requests that the entire range [%d-%d] has been downloaded. Skip"+ui.Reset+"\n", startBlock, endBlock)
		return nil
	}

	totalContracts := 0
	skippedBlocks := 0

	for _, sub := range uncovered {
		log.Printf(ui.Blue+"🔁 handles uncovered subintervals: %d - %d"+ui.Reset+"\n", sub.Start, sub.End)
		for blockNum := sub.Start; blockNum <= sub.End; blockNum++ {
			downloaded, err := d.IsBlockDownloaded(ctx, blockNum)
			if err != nil {
				log.Printf(ui.Yellow+"⚠️ check block %d status failure: %v"+ui.Reset+"\n", blockNum, err)
			} else if downloaded {
				skippedBlocks++
				if skippedBlocks%100 == 0 {
					log.Printf(ui.Gray+"⏭️  skipped %d downloaded blocks..."+ui.Reset+"\n", skippedBlocks)
				}
				continue
			}

			client, err := d.RPCManager.GetClient()
			if err != nil {
				log.Printf(ui.Red+"❌ failed to obtain the RPC client: %v"+ui.Reset+"\n", err)
				continue
			}

			block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
			if err != nil {
				log.Printf(ui.Red+"❌ failed to obtain block %d: %v"+ui.Reset+"\n", blockNum, err)
				continue
			}

			txCount := len(block.Transactions())
			if txCount > 0 {
				log.Printf(ui.Blue+"📦 handle block %d (a total of %d transactions)..."+ui.Reset+"\n", blockNum, txCount)
			}

			blockTime := time.Unix(int64(block.Time()), 0)
			contractCount := 0

			for _, tx := range block.Transactions() {
				if tx.To() == nil {
					receipt, err := client.TransactionReceipt(ctx, tx.Hash())
					if err != nil {
						log.Printf(ui.Yellow+"⚠️ failed to obtain transaction receipt: %v"+ui.Reset+"\n", err)
						continue
					}

					if receipt.ContractAddress != (common.Address{}) {
						contractAddr := receipt.ContractAddress.Hex()

						exists, err := d.ContractExists(ctx, contractAddr)
						if err != nil {
							log.Printf(ui.Red+"❌ check for contract failure: %v"+ui.Reset+"\n", err)
							continue
						}

						if exists {
							continue
						}

						code, err := client.CodeAt(ctx, receipt.ContractAddress, nil)
						if err != nil {
							log.Printf(ui.Yellow+"⚠️ failed to get the contract code: %v"+ui.Reset+"\n", err)
							continue
						}

						var contractCode string
						var isOpenSource int
						var details *EtherscanContractInfo
						var isVerified bool

						contractAddr = strings.TrimSpace(contractAddr)

						abiJSON := ""
						if d.etherscanConfig.APIKeyManager != nil || d.etherscanConfig.APIKey != "" {
							if d.rateLimiter != nil {
								d.rateLimiter.Wait()
							}
							details, isVerified, err = GetContractDetails(contractAddr, d.etherscanConfig)
							if err != nil {
								log.Printf(ui.Yellow+"⚠️ failed to query Etherscan: %v, fallback to save bytecode"+ui.Reset+"\n", err)
								contractCode = fmt.Sprintf("0x%x", code)
								isOpenSource = 0
								appendFailAddress("eoferror.txt", contractAddr)
							} else if isVerified {
								if details != nil {
									contractCode = details.SourceCode
									if details.ContractName != "" {
										contractCode = solc.AttachMetadata(contractCode, details.ContractName)
									}
									isOpenSource = 1
									abiJSON = strings.TrimSpace(details.ABI)
								} else {
									contractCode = fmt.Sprintf("0x%x", code)
									isOpenSource = 0
								}
							} else {
								contractCode = fmt.Sprintf("0x%x", code)
								isOpenSource = 0
							}
						} else {
							contractCode = fmt.Sprintf("0x%x", code)
							isOpenSource = 0
						}

						balanceText := "0.000000"
						if isOpenSource == 1 {
							balance, err := client.BalanceAt(ctx, receipt.ContractAddress, nil)
							if err != nil {
								log.Printf(ui.Yellow+"⚠️ failed to get balance: %v"+ui.Reset+"\n", err)
								balance = big.NewInt(0)
							}

							balanceEth := new(big.Float).Quo(
								new(big.Float).SetInt(balance),
								big.NewFloat(1e18),
							)
							balanceText = balanceEth.Text('f', 6)
						}

						isProxy := 0
						implAddress := ""
						if d.etherscanConfig.APIKeyManager != nil || d.etherscanConfig.APIKey != "" && isVerified && details != nil {
							if strings.TrimSpace(details.Proxy) == "1" && strings.TrimSpace(details.Implementation) != "" {
								isProxy = 1
								implAddress = strings.TrimSpace(details.Implementation)
								if d.rateLimiter != nil {
									d.rateLimiter.Wait()
								}
								implDetails, implVerified, implErr := GetContractDetails(implAddress, d.etherscanConfig)
								if implErr == nil && implVerified && implDetails != nil && strings.TrimSpace(implDetails.SourceCode) != "" {
									proxyInfo := &ContractInfo{
										Address:        contractAddr,
										Contract:       "",
										ABI:            "",
										Balance:        balanceText,
										IsOpenSource:   1,
										IsProxy:        1,
										Implementation: implAddress,
										CreateTime:     blockTime,
										CreateBlock:    blockNum,
										TxLast:         blockTime,
										IsDecompiled:   0,
										DedCode:        "",
									}
									if err := d.SaveContract(ctx, proxyInfo); err != nil {
										log.Printf(ui.Red+"❌ failed to save proxy contract information: %v"+ui.Reset+"\n", err)
										continue
									}

									implCode := implDetails.SourceCode
									if implDetails.ContractName != "" {
										implCode = solc.AttachMetadata(implCode, implDetails.ContractName)
									}

									implInfo := &ContractInfo{
										Address:        implAddress,
										Contract:       implCode,
										ABI:            strings.TrimSpace(implDetails.ABI),
										Balance:        balanceText,
										IsOpenSource:   1,
										IsProxy:        0,
										Implementation: "",
										CreateTime:     blockTime,
										CreateBlock:    blockNum,
										TxLast:         blockTime,
										IsDecompiled:   0,
										DedCode:        "",
									}
									if err := d.SaveContract(ctx, implInfo); err != nil {
										log.Printf(ui.Red+"❌ failed to save implementation contract: %v"+ui.Reset+"\n", err)
										continue
									}

									contractCount++
									totalContracts++
									log.Printf(ui.Green+"✅ discover proxy contract: %s -> implementation: %s (block %d)"+ui.Reset+"\n", contractAddr, implAddress, blockNum)
									continue
								}
							}
						}

						info := &ContractInfo{
							Address:        contractAddr,
							Contract:       contractCode,
							ABI:            abiJSON,
							Balance:        balanceText,
							IsOpenSource:   isOpenSource,
							IsProxy:        isProxy,
							Implementation: implAddress,
							CreateTime:     blockTime,
							CreateBlock:    blockNum,
							TxLast:         blockTime,
							IsDecompiled:   0,
							DedCode:        "",
						}

						if err := d.SaveContract(ctx, info); err != nil {
							log.Printf(ui.Red+"❌ Failed to save contract: %v"+ui.Reset+"\n", err)
							continue
						}

						contractCount++
						totalContracts++
						log.Printf(ui.Green+"✅ discovered contract: %s (block %d)"+ui.Reset+"\n", contractAddr, blockNum)
					}
				}
			}

			if ctx.Err() != nil {
				return ctx.Err()
			}
		}

		merged := mergeAndInsertRange(existing, BlockRangeRecord{Start: sub.Start, End: sub.End})
		if err := saveBlockedRanges(merged); err != nil {
			log.Printf(ui.Yellow+"⚠️  failed to save downloaded interval to %s: %v"+ui.Reset+"\n", blockedFile, err)
		} else {
			existing = merged
		}
	}

	log.Printf("\n" + ui.Green + "✅ Download completed!" + ui.Reset + "\n")
	log.Printf(ui.Cyan+"   - Block range: %d - %d"+ui.Reset+"\n", startBlock, endBlock)
	log.Printf(ui.Cyan+"   - New contracts: %d"+ui.Reset+"\n", totalContracts)
	log.Printf(ui.Cyan+"   - Skipped blocks: %d"+ui.Reset+"\n", skippedBlocks)

	return nil
}

func (d *Downloader) DownloadFromLast(ctx context.Context) error {
	lastBlock, err := d.GetLastDownloadedBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the last downloaded block: %w", err)
	}

	currentBlock, err := d.GetCurrentBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get the current block: %w", err)
	}

	startBlock := lastBlock + 1
	if lastBlock == 0 {
		startBlock = 0
		log.Println(ui.Cyan + "📌 Database is empty, start downloading from the genesis block" + ui.Reset)
	} else {
		log.Printf(ui.Cyan+"📌 Continue downloading from block %d (last: %d)"+ui.Reset+"\n", startBlock, lastBlock)
	}

	log.Printf(ui.Green+"🎯 Target block: %d (current latest)"+ui.Reset+"\n", currentBlock)

	if startBlock > currentBlock {
		log.Println(ui.Green + "✅ Already up to date, no need to download" + ui.Reset)
		return nil
	}

	return d.DownloadBlockRange(ctx, startBlock, currentBlock)
}

func (d *Downloader) Close() {
	if d.RPCManager != nil {
		d.RPCManager.Close()
	}
	if d.rateLimiter != nil {
		d.rateLimiter.Stop()
	}
}

func (d *Downloader) downloadSingleContract(ctx context.Context, addr string, failLog string) error {
	exists, err := d.ContractExists(ctx, addr)
	if err != nil {
		log.Printf("⚠️ check if the contract %s has failed: %v\n", addr, err)
		appendFailAddress(failLog, addr)
		return err
	}
	if exists {
		log.Printf("⏭️ The contract already exists. Skip: %s\n", addr)
		return nil
	}

	client, err := d.RPCManager.GetClient()
	if err != nil {
		log.Printf("❌ failed to obtain the RPC client: %v\n", err)
		appendFailAddress(failLog, addr)
		return err
	}

	caddr := common.HexToAddress(addr)

	code, err := client.CodeAt(ctx, caddr, nil)
	if err != nil {
		log.Printf(ui.Yellow+"⚠️  failed to get contract bytecode: %s -> %v"+ui.Reset+"\n", addr, err)
		appendFailAddress(failLog, addr)
		return err
	}

	contractCode := fmt.Sprintf("0x%x", code)
	isOpenSource := 0
	abiJSON := ""
	isProxy := 0
	implAddress := ""

	var details *EtherscanContractInfo
	var isVerified bool
	if d.etherscanConfig.APIKeyManager != nil || d.etherscanConfig.APIKey != "" {
		if d.rateLimiter != nil {
			d.rateLimiter.Wait()
		}
		details, isVerified, err = GetContractDetails(addr, d.etherscanConfig)
		if err != nil {
			log.Printf(ui.Yellow+"⚠️ failed to query Etherscan for %s: %v, fallback to save the bytecode and record it to the failed file"+ui.Reset+"\n", addr, err)
			appendFailAddress(failLog, addr)
		} else if isVerified && details != nil {
			isProxyFlag := strings.TrimSpace(details.Proxy) == "1"
			implAddress = strings.TrimSpace(details.Implementation)
			if isProxyFlag && implAddress != "" {
				if d.rateLimiter != nil {
					d.rateLimiter.Wait()
				}
				implDetails, implVerified, implErr := GetContractDetails(implAddress, d.etherscanConfig)
				if implErr == nil && implVerified && implDetails != nil && strings.TrimSpace(implDetails.SourceCode) != "" {
					isProxy = 1
					balance, berr := client.BalanceAt(ctx, caddr, nil)
					if berr != nil {
						log.Printf(ui.Yellow+"⚠️ failed to get balance: %s -> %v"+ui.Reset+"\n", addr, berr)
						balance = big.NewInt(0)
					}
					balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))

					proxyInfo := &ContractInfo{
						Address:        addr,
						Contract:       "",
						ABI:            "",
						Balance:        balanceEth.Text('f', 6),
						IsOpenSource:   1,
						IsProxy:        1,
						Implementation: implAddress,
						CreateTime:     time.Now(),
						CreateBlock:    0,
						TxLast:         time.Now(),
						IsDecompiled:   0,
						DedCode:        "",
					}
					if err := d.SaveContract(ctx, proxyInfo); err != nil {
						log.Printf(ui.Red+"❌ save proxy contract failed: %s -> %v"+ui.Reset+"\n", addr, err)
						appendFailAddress(failLog, addr)
						return err
					}

					implCode := implDetails.SourceCode
					if implDetails.ContractName != "" {
						implCode = solc.AttachMetadata(implCode, implDetails.ContractName)
					}
					implInfo := &ContractInfo{
						Address:        implAddress,
						Contract:       implCode,
						ABI:            strings.TrimSpace(implDetails.ABI),
						Balance:        balanceEth.Text('f', 6),
						IsOpenSource:   1,
						IsProxy:        0,
						Implementation: "",
						CreateTime:     time.Now(),
						CreateBlock:    0,
						TxLast:         time.Now(),
						IsDecompiled:   0,
						DedCode:        "",
					}
					if err := d.SaveContract(ctx, implInfo); err != nil {
						log.Printf(ui.Red+"❌ save implementation contract failed: %s -> %v"+ui.Reset+"\n", implAddress, err)
						appendFailAddress(failLog, implAddress)
						return err
					}

					log.Printf(ui.Green+"✅ download the proxy contract and its successful implementation: %s -> %s"+ui.Reset+"\n", addr, implAddress)
					return nil
				}
			}

			contractCode = details.SourceCode
			if details.ContractName != "" {
				contractCode = solc.AttachMetadata(contractCode, details.ContractName)
			}
			abiJSON = details.ABI
			isOpenSource = 1
			if isProxyFlag && implAddress != "" {
				isProxy = 1
			}
		}
	}

	balanceText := "0.000000"
	if isOpenSource == 1 {
		balance, err := client.BalanceAt(ctx, caddr, nil)
		if err != nil {
			log.Printf(ui.Yellow+"⚠️  failed to get balance: %s -> %v"+ui.Reset+"\n", addr, err)
			balance = big.NewInt(0)
		}
		balanceEth := new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18))
		balanceText = balanceEth.Text('f', 6)
	}

	info := &ContractInfo{
		Address:        addr,
		Contract:       contractCode,
		ABI:            abiJSON,
		Balance:        balanceText,
		IsOpenSource:   isOpenSource,
		IsProxy:        isProxy,
		Implementation: implAddress,
		CreateTime:     time.Now(),
		CreateBlock:    0,
		TxLast:         time.Now(),
		IsDecompiled:   0,
		DedCode:        "",
	}

	if err := d.SaveContract(ctx, info); err != nil {
		log.Printf(ui.Red+"❌ failed to save contract: %s -> %v"+ui.Reset+"\n", addr, err)
		appendFailAddress(failLog, addr)
		return err
	}

	log.Printf(ui.Green+"✅ contract download successful: %s"+ui.Reset+"\n", addr)
	return nil
}

func (d *Downloader) DownloadContractsByAddresses(ctx context.Context, addresses []string, failLog string) error {
	if len(addresses) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	uniqueAddresses := make([]string, 0, len(addresses))
	for _, a := range addresses {
		addr := strings.TrimSpace(a)
		if addr == "" {
			continue
		}
		addrLower := strings.ToLower(addr)
		if _, ok := seen[addrLower]; ok {
			continue
		}
		seen[addrLower] = struct{}{}
		uniqueAddresses = append(uniqueAddresses, addr)
	}

	if len(uniqueAddresses) == 0 {
		return nil
	}

	log.Printf(ui.Cyan+"📥 start downloading %d contract address (concurrency: %d)"+ui.Reset+"\n", len(uniqueAddresses), d.Concurrency)

	if d.Concurrency <= 1 {
		for _, addr := range uniqueAddresses {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			d.downloadSingleContract(ctx, addr, failLog)
		}
		return nil
	}

	type job struct {
		address string
		index   int
	}

	jobs := make(chan job, len(uniqueAddresses))
	results := make(chan error, len(uniqueAddresses))

	var wg sync.WaitGroup
	for i := 0; i < d.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok {
						return
					}
					err := d.downloadSingleContract(ctx, j.address, failLog)
					select {
					case results <- err:
					case <-ctx.Done():
						return
					}
					if ctx.Err() != nil {
						return
					}
				}
			}
		}(i)
	}

	go func() {
		defer close(jobs)
		for i, addr := range uniqueAddresses {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{address: addr, index: i}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	failCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	log.Printf(ui.Green+"✅ Download completed: success %d, failed %d"+ui.Reset+"\n", successCount, failCount)
	return nil
}
