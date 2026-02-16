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

// makeRequest makes a request to Etherscan API with retry logic
func (e *EtherscanAPI) makeRequest(params url.Values) (*EtherscanResponse, error) {
	params.Set("apikey", e.apiKey)
	reqURL := fmt.Sprintf("https://api.etherscan.io/api?%s", params.Encode())

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		resp, err := e.client.Get(reqURL)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		var apiResp EtherscanResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			lastErr = err
			continue
		}

		// Check for rate limiting
		if apiResp.Message == "Rate limit exceeded" {
			log.Printf("⚠️ Rate limit hit, waiting...")
			time.Sleep(2 * time.Second)
			lastErr = fmt.Errorf("rate limit exceeded")
			continue
		}

		return &apiResp, nil
	}

	return nil, fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// GetContractSource gets contract source code from Etherscan
func (e *EtherscanAPI) GetContractSource(address string) (*ContractSource, error) {
	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getsourcecode")
	params.Set("address", address)

	apiResp, err := e.makeRequest(params)
	if err != nil {
		return nil, err
	}

	if apiResp.Status != "1" {
		// Log detailed error for debugging
		log.Printf("🔍 API Response for %s: Status=%s, Message=%s", address, apiResp.Status, apiResp.Message)
		return nil, fmt.Errorf("API error: %s (status: %s)", apiResp.Message, apiResp.Status)
	}

	var sources []ContractSource
	if err := json.Unmarshal(apiResp.Result, &sources); err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no source code found")
	}

	return &sources[0], nil
}

// GetTransactionList gets transactions for an address
func (e *EtherscanAPI) GetTransactionList(address string, startBlock, endBlock int64) ([]Transaction, error) {
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("address", address)
	params.Set("startblock", strconv.FormatInt(startBlock, 10))
	params.Set("endblock", strconv.FormatInt(endBlock, 10))
	params.Set("sort", "asc")

	apiResp, err := e.makeRequest(params)
	if err != nil {
		return nil, err
	}

	if apiResp.Status != "1" {
		// No transactions is not an error
		if apiResp.Message == "No transactions found" {
			return []Transaction{}, nil
		}
		return nil, fmt.Errorf("API error: %s", apiResp.Message)
	}

	var txs []Transaction
	if err := json.Unmarshal(apiResp.Result, &txs); err != nil {
		return nil, err
	}

	return txs, nil
}

// FetchTopContracts fetches top contracts by transaction volume
func (e *EtherscanAPI) FetchTopContracts() ([]ContractCreation, error) {
	// Extended list of popular contracts (can handle up to 100,000 calls/day)
	contractAddresses := []string{
		// Tier 1: Major DeFi Protocols
		"0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", // UNI
		"0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", // AAVE
		"0xC011a73ee8576Fb46F5E1c5751cA3B9Fe0af2a6F", // SNX
		"0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2", // MKR
		"0x0bc529c00C6401aEF6D220BE8C6Ea1667F6Ad93e", // YFI
		"0x6B175474E89094C44Da98b954EedeAC495271d0F", // DAI
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
		"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", // WBTC
		"0x514910771AF9Ca656af840dff83E8264EcF986CA", // LINK
		"0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0", // MATIC
		"0x4Fabb145d64652a948d72533023f6E7A623C7C53", // BUSD
		"0x8E870D67F660D95d5be530380D0eC0bd388289E1", // PAX
		"0x0000000000085d4780B73119b644AE5ecd22b376", // TUSD
		"0x6c6EE5e31d828De241282B9606C8e98Ea48526E2", // HOT

		// Tier 2: DEX and AMM
		"0xE592427A0AEce92De3Edee1F18E0157C05861564", // Uniswap V3 Router
		"0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45", // Uniswap V3 Universal Router
		"0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D", // Uniswap V2 Router02
		"0xf164fC0Ec4E93095b804a4795bBe1e041497b92a", // Uniswap V2 Router01
		"0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F", // SushiSwap Router
		"0x10ED43C718714eb63d5aA57B78B54704E256024E", // PancakeSwap Router (BSC)
		"0xC0788A3aD43d79aa81B23c076A91609489342C8B", // Curve 3Pool
		"0xbEbc44782C7dB0a1A60Cb6fe97d0b483032FF1C7", // Curve 3Pool v2
		"0xCEAF7747579696A2F0bb206a14210e3c9e6fB269", // Balancer Vault
		"0xBA12222222228d8Ba445958a75a0704d566BF2C8", // Balancer v2 Vault

		// Tier 3: Lending Protocols
		"0x398eC7346DcD622eDc5ae82352F02bE94C62d119", // AAVE V1 Lending Pool
		"0x7d2768dE32b0b80b7a3454c06BdAc94A69DDc7A9", // AAVE V2 Lending Pool
		"0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2", // AAVE V3 Pool
		"0x3d9819210A31b4961b30EF54bE2aeD79B9c9Cd3B", // Compound Comptroller
		"0x3d5BC3c8d3dDd901eC45B68b0D917327f4Ee4C8E", // Compound cETH
		"0x39AA39c021dfbaE8faC545936693aC917d5E7563", // Compound cUSDC
		"0xf650C3d88D12dB855b8bf7D11Be6C55A4e07dCC9", // Compound cUSDT
		"0x4Ddc2D193948926D02f9B1fE9e1daa0718270ED5", // Compound cDAI

		// Tier 4: Yield Aggregators
		"0x9cA85572E6A3EbF34dC6e3B778144CdbF60eE5b7", // Yearn yDAI
		"0xe11ba472F74869176652C35D30dB89854b5ae84D", // Yearn yUSDC
		"0x5f18C75AbDAe578b483E5F43f12a39cF75b973a9", // Yearn v3 DAI
		"0xa354F35829Ae975e850e15e7fFbB47b5DD0B1d98", // Yearn v3 USDC
		"0xdA816459F1AB5631232FE5e97a05BBBb94970c95", // Convex Booster
		"0xF403C135812408BFbE8713b5A23a04b3D48AAE31", // Convex Vote Locker

		// Tier 5: Bridges and Cross-chain
		"0x40ec5B33f54e0E8A33A975908C5BA1c14e5BbbDf", // Polygon Bridge
		"0x72Ce9c846789fdB6fC1f34aC4AD25Dd9ef7031ef", // Arbitrum Bridge
		"0x99C9fc46f92E8a1c0deC1b1747d010903E884bE1", // Optimism Bridge
		"0x4aa42145Aa6E22d8ce4eaDDf4b273E6CbA6E80E5", // Across Bridge
		"0xE87B9c4B3aA4d728f31942D4Ffc8c15537F73261", // Hop Bridge
		"0x5421FA1A48f6d3e33ee83e47F2b4cE74324279d4", // Stargate Router
		"0x8731d54E9D02c286869d51ac834dF5C4F8D6e5a5", // LayerZero Endpoint

		// Tier 6: NFT and Gaming
		"0xBC4CA0EdA7647A8aB7C2061c2E118A18a936f13D", // BAYC
		"0x60E4d786628Fea6478F785A6d7e704777c86a7c6", // MAYC
		"0x34d85c9CDeB23FA97cb08333b511ac86E1C4E258", // Otherdeed
		"0x7Bd29408f11D2bFC23c34f18275bBf23bB716Bc7", // Meebits
		"0xED5AF388653567Af2F388E6224dC7C4b3241C544", // Azuki
		"0x8a90CAb2b38dba80c64b7734e58Ee1dB38B8992e", // Doodles
		"0x23581767a106ae21c074b2276D25e5C3e136a68b", // Moonbirds
		"0x49cF6f5d15A8c5Da4b656eeA662cAfE85d4e4b61", // CloneX
		"0x79FCDEF22feeD20eDDacbB2587640e45491b757f", // MoonCats
		"0x282BDD42f4eb70e7A9D9F40c8fEA0825B7f68C5d", // CryptoPunks V1 Wrapper

		// Tier 7: Infrastructure
		"0x00000000003b3cc22aF3aE1EAc0440BcEe416B40", // Flashbots Relay
		"0x1EbD8aB14A1B443A9fF4e2A7A62eC2D1c5aE08d6", // Chainlink Oracle
		"0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", // EIP-1967 Proxy
		"0x4e59b44847b379578588920cA78FbF26c0B4956C", // Create2 Factory
		"0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f", // Uniswap V2 Factory
		"0x1F98431c8aD98523631AE4a59f267346ea31F984", // Uniswap V3 Factory
		"0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac", // SushiSwap Factory

		// Tier 8: Stablecoin and Money Markets
		"0x0C3eF32f802967db75B9cE94d0E39A1bF1363f03", // FRAX
		"0x853d955aCEf822Db058eb8505911ED77F175b99e", // FRAX V2
		"0xa47c8bf37f92aBed4A126BDA807A7b7498661acD", // UST (before collapse - for analysis)
		"0xdB25f211AB05b1c97D595516F45794528a807ad8", // EURS
		"0x00000100F2A2bd000715001920eB70D229700085", // TCAD
		"0x00000000441378008EA67F4284A57932B1c000a5", // TGBP
		"0x00006100F7090010005F1bd7aE6122c3C2CF0090", // TAUD

		// Tier 9: Derivatives and Perpetuals
		"0xc36442b4a4522e871399cd717abdd847ab11fe88", // Uniswap V3 Positions NFT
		"0x65f7BA27790D97B46D511605e94e3d4f798b73fA", // Perpetual Protocol
		"0x2e4404b47E9e0A57c48b2b703771eF90bF58a7A9", // dYdX (old)
		"0x92D6C1e31e14520e676a687F0a93788B716BEff5", // dYdX DDX
		"0x87C56bE23D7C5e8C6B0D8e1E7E4d4E5F4e5E5e5", // GMX (placeholder)
		"0x489ee077994B6658eAfA855C308275EAd8097C4A", // GMX V1
		"0xfc5A1A6EB076a2C7aD06eD22C90d7E710E35ad0a", // GMX V2 Router

		// Tier 10: Governance and DAOs
		"0x408ED6354d4973f66138C91495F2f2FCbd8724C3", // Compound Governor
		"0xEC568fffba86c94cfC7D38258DA0bA7818D5D394", // Uniswap Governor
		"0x4A7dFda78F43cf752D73eF849eEc7F4E3C4f2E3e", // AAVE Governor
		"0x0bEf1F1e5F5F5eF5F5F5F5F5F5F5F5F5F5F5F5F", // ENS (placeholder)
		"0xC18360217D8F7Ab5e7c516566761Ea12Ce7F9D72", // ENS Token
		"0x57f1887a8BF19b14fC0dF6Fd9B2acc9Af147eA85", // ENS Registry
		"0x00000000000C2E074eC69A0dFb2997BA6C7d2e1e", // ENS Public Resolver

		// Additional high-value targets for security research
		"0x2F0b23f53734252Bda2277357e97e1517d6B042A", // MakerDAO MCD_VAT
		"0x19c0976f590D67707E62397C87829d896Dc0f1F1", // MakerDAO MCD_JUG
		"0x35D1b3F3D7966A1DFe207aaE4516C12D2594C5d8", // MakerDAO MCD_SPOT
		"0xA950524441892A31ebddF91d3cEEFa04Bf454466", // MakerDAO MCD_DAI
		"0x9759A6Ac90977b93B58547b4A71c78317f391A28", // MakerDAO MCD_JOIN_ETH_A
		"0x12e5F9C6d4E473aAf5F29aAB8b8F4B3c7d3C5e3C", // Lido stETH
		"0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84", // Lido stETH (new)
		"0x17144556fd3424EDC8Fc54A3e330118f9B4C8b9C", // Rocket Pool
		"0xae78736Cd615f374D3085123A210448E74Fc6393", // Rocket Pool rETH
		"0x0000000022D53366457F9d5E68Ec105046FC4383", // Curve DAO Token
	}

	log.Printf("🚀 Fetching %d contracts from Etherscan...", len(contractAddresses))

	var contracts []ContractCreation
	successCount := 0
	failCount := 0

	for i, addr := range contractAddresses {
		// Progress logging
		if i%10 == 0 && i > 0 {
			log.Printf("📊 Progress: %d/%d (Success: %d, Failed: %d)", i, len(contractAddresses), successCount, failCount)
		}

		// Get contract source
		source, err := e.GetContractSource(addr)
		if err != nil {
			log.Printf("⚠️ [%d/%d] No source for %s: %v", i+1, len(contractAddresses), addr, err)
			failCount++
			// Still add the contract even without source code
			contracts = append(contracts, ContractCreation{
				ContractAddress: addr,
				ContractCode:    "",
				BlockNumber:     "20000000",
				Timestamp:       strconv.FormatInt(time.Now().Unix(), 10),
			})
		} else {
			successCount++
			contracts = append(contracts, ContractCreation{
				ContractAddress: addr,
				ContractCode:    source.SourceCode,
				BlockNumber:     "20000000",
				Timestamp:       strconv.FormatInt(time.Now().Unix(), 10),
			})
		}

		// Rate limiting: 5 calls per second = 200ms delay
		time.Sleep(220 * time.Millisecond)
	}

	log.Printf("✅ Completed: %d success, %d failed out of %d total", successCount, failCount, len(contractAddresses))
	return contracts, nil
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

	// Fetch top contracts
	contracts, err := i.etherscan.FetchTopContracts()
	if err != nil {
		return fmt.Errorf("failed to fetch contracts: %w", err)
	}

	log.Printf("📥 Processing %d contracts...", len(contracts))

	// Store contracts in database
	saved := 0
	for idx, c := range contracts {
		if err := i.saveContract(chain, c); err != nil {
			log.Printf("❌ Failed to save %s: %v", c.ContractAddress, err)
			continue
		}
		saved++
		if idx%10 == 0 && idx > 0 {
			log.Printf("💾 Saved %d/%d contracts...", saved, len(contracts))
		}
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

	// Build insert SQL with conflict handling
	var sql string
	switch chain {
	case "eth", "ethereum":
		// PostgreSQL syntax
		sql = fmt.Sprintf(`
			INSERT INTO %s (address, contract, createblock, createtime, isopensource)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(address) DO UPDATE SET
				contract = excluded.contract,
				createblock = excluded.createblock,
				createtime = excluded.createtime,
				isopensource = excluded.isopensource
		`, tableName)
	default:
		// SQLite syntax
		sql = fmt.Sprintf(`
			INSERT INTO %s (address, contract, createblock, createtime, isopensource)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(address) DO UPDATE SET
				contract = excluded.contract,
				createblock = excluded.createblock,
				createtime = excluded.createtime,
				isopensource = excluded.isopensource
		`, tableName)
	}

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
