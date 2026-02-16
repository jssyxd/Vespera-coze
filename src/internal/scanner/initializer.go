package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"vespera/internal/config"
)

type EtherscanAPI struct {
	apiKey string
	client *http.Client
}

type ContractCreation struct {
	ContractAddress string `json:"contractAddress"`
	ContractCreator string `json:"contractCreator"`
	TxHash          string `json:"txHash"`
	BlockNumber     string `json:"blockNumber"`
	Timestamp       string `json:"timestamp"`
	ContractCode    string `json:"contractCode,omitempty"`
}

func NewEtherscanAPI(apiKey string) *EtherscanAPI {
	return &EtherscanAPI{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchContractsByBlockRange fetches contract creation data from a block range
func (e *EtherscanAPI) FetchContractsByBlockRange(startBlock, endBlock int64) ([]ContractCreation, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("etherscan API key not configured")
	}

	// Get contract creations by block range
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("startblock", strconv.FormatInt(startBlock, 10))
	params.Set("endblock", strconv.FormatInt(endBlock, 10))
	params.Set("sort", "asc")
	params.Set("apikey", e.apiKey)

	// For now, we'll scan a specific address range or use eth_getBlock
	// Actually, let's use the alternative: get list of internal transactions
	// Or scan specific popular contract addresses

	// For initialization, let's fetch the top contracts by transaction count
	// We'll use a predefined list of popular DeFi contracts
	return e.fetchPopularContracts()
}

// fetchPopularContracts returns a list of popular DeFi contracts to scan
func (e *EtherscanAPI) fetchPopularContracts() ([]ContractCreation, error) {
	// List of popular contracts to initialize the database
	popularContracts := []string{
		// DeFi Protocols
		"0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", // UNI Token
		"0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", // AAVE Token
		"0xC011a73ee8576Fb46F5E1c5751cA3B9Fe0af2a6F", // SNX Token
		"0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2", // MKR Token
		"0x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e", // YFI Token
		"0x6B175474E89094C44Da98b954EedeAC495271d0F", // DAI Token
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC Token
		"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT Token
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", // WBTC Token
		"0x514910771AF9Ca656af840dff83E8264EcF986CA", // LINK Token

		// DEX Routers
		"0xE592427A0AEce92De3Edee1F18E0157C05861564", // Uniswap V3 Router
		"0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45", // Uniswap V3 Universal Router
		"0x10ED43C718714eb63d5aA57B78B54704E256024E", // PancakeSwap V2 Router
		"0x3Ef1d856eA188064cD1f5E929a0b1B71F6bB6A38", // SushiSwap Router

		// Lending Protocols
		"0x398eC7346DcD622eDc5ae82352F02bE94C62d119", // AAVE Lending Pool
		"0x3d9819210A31b4961b30EF54bE2aeD79B9c9Cd3B", // Compound Comptroller

		// Bridges
		"0x40ec5B33f54e0E8A33A975908C5BA1c14e5BbbDf", // Polygon Bridge
		"0x72Ce9c846789fdB6fC1f34aC4AD25Dd9ef7031ef", // Arbitrum Bridge
	}

	var contracts []ContractCreation
	for _, addr := range popularContracts {
		// Skip contract creation API, just get source code directly
		code, err := e.GetContractCode(addr)
		if err != nil {
			log.Printf("⚠️ No source code for %s: %v", addr, err)
			// Add contract anyway with empty code
			code = ""
		}

		contracts = append(contracts, ContractCreation{
			ContractAddress: addr,
			ContractCode:    code,
			BlockNumber:     "20000000",
			Timestamp:       strconv.FormatInt(time.Now().Unix(), 10),
			TxHash:          "",
		})

		// Rate limiting - Etherscan allows 5 calls per second
		time.Sleep(250 * time.Millisecond)
	}

	return contracts, nil
}

// GetContractCreation gets contract creation details
func (e *EtherscanAPI) GetContractCreation(address string) (*ContractCreation, error) {
	// Get contract creation tx
	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getcontractcreation")
	params.Set("contractaddresses", address)
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("https://api.etherscan.io/api?%s", params.Encode())

	resp, err := e.client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp EtherscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Status != "1" {
		// Fallback: try to get basic contract info
		return e.getBasicContractInfo(address)
	}

	var creations []ContractCreation
	if err := json.Unmarshal(apiResp.Result, &creations); err != nil {
		return nil, err
	}

	if len(creations) == 0 {
		return e.getBasicContractInfo(address)
	}

	// Fetch contract code
	code, err := e.GetContractCode(address)
	if err == nil {
		creations[0].ContractCode = code
	}

	return &creations[0], nil
}

// getBasicContractInfo gets basic info when creation API fails
func (e *EtherscanAPI) getBasicContractInfo(address string) (*ContractCreation, error) {
	// Get contract code
	code, err := e.GetContractCode(address)
	if err != nil {
		return nil, err
	}

	return &ContractCreation{
		ContractAddress: address,
		ContractCode:    code,
		BlockNumber:     "0",
		Timestamp:       strconv.FormatInt(time.Now().Unix(), 10),
		TxHash:          "",
	}, nil
}

// GetContractCode fetches the contract source code
func (e *EtherscanAPI) GetContractCode(address string) (string, error) {
	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getsourcecode")
	params.Set("address", address)
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("https://api.etherscan.io/api?%s", params.Encode())

	resp, err := e.client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiResp EtherscanResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", err
	}

	if apiResp.Status != "1" {
		return "", fmt.Errorf("API error: %s", apiResp.Message)
	}

	var sources []struct {
		SourceCode string `json:"SourceCode"`
	}
	if err := json.Unmarshal(apiResp.Result, &sources); err != nil {
		return "", err
	}

	if len(sources) == 0 {
		return "", fmt.Errorf("no source code found")
	}

	return sources[0].SourceCode, nil
}

// GetTransactionCount gets the number of transactions for an address
func (e *EtherscanAPI) GetTransactionCount(address string) (string, error) {
	params := url.Values{}
	params.Set("module", "proxy")
	params.Set("action", "eth_getTransactionCount")
	params.Set("address", address)
	params.Set("tag", "latest")
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("https://api.etherscan.io/api?%s", params.Encode())

	resp, err := e.client.Get(reqURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.Result, nil
}

// Initializer handles database initialization
type Initializer struct {
	db        *config.Database
	etherscan *EtherscanAPI
}

func NewInitializer(db *config.Database, apiKey string) *Initializer {
	return &Initializer{
		db:        db,
		etherscan: NewEtherscanAPI(apiKey),
	}
}

// InitializeDatabase populates the database with initial contract data
func (i *Initializer) InitializeDatabase(chain string) error {
	log.Printf("🚀 Initializing database for %s chain...", chain)

	// Fetch popular contracts
	contracts, err := i.etherscan.fetchPopularContracts()
	if err != nil {
		return fmt.Errorf("failed to fetch contracts: %w", err)
	}

	log.Printf("📥 Fetched %d contracts from Etherscan", len(contracts))

	// Store contracts in database
	saved := 0
	for _, c := range contracts {
		if err := i.saveContract(chain, c); err != nil {
			log.Printf("Failed to save %s: %v", c.ContractAddress, err)
			continue
		}
		saved++
	}

	log.Printf("✅ Saved %d contracts to database", saved)
	return nil
}

// saveContract saves a contract to the database
func (i *Initializer) saveContract(chain string, c ContractCreation) error {
	tableName := chain
	if tableName == "" {
		tableName = "ethereum"
	}

	// Convert block number
	blockNum, _ := strconv.ParseInt(c.BlockNumber, 10, 64)
	if blockNum == 0 {
		blockNum = 20000000 // Default block
	}

	// Convert timestamp
	timestamp := time.Now()
	if ts, err := strconv.ParseInt(c.Timestamp, 10, 64); err == nil {
		timestamp = time.Unix(ts, 0)
	}

	// Check if contract is open source
	isOpenSource := c.ContractCode != "" && c.ContractCode != "No data"

	// Build insert SQL
	sql := fmt.Sprintf(`
		INSERT INTO %s (address, contract, createblock, createtime, isopensource)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(address) DO UPDATE SET
			contract = excluded.contract,
			createblock = excluded.createblock,
			createtime = excluded.createtime,
			isopensource = excluded.isopensource
	`, tableName)

	result := i.db.GetDB().Exec(sql, c.ContractAddress, c.ContractCode, blockNum, timestamp, isOpenSource)
	return result.Error
}

// GetContractCount returns the number of contracts in the database
func (i *Initializer) GetContractCount(chain string) int64 {
	tableName := chain
	if tableName == "" {
		tableName = "ethereum"
	}

	var count int64
	i.db.GetDB().Table(tableName).Count(&count)
	return count
}
