package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	EtherscanAPIURL = "https://api.etherscan.io/api"
	BscscanAPIURL   = "https://api.bscscan.com/api"
	PolygonAPIURL   = "https://api.polygonscan.com/api"
)

type EtherscanClient struct {
	apiKey string
	client *http.Client
}

type EtherscanResponse struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type ContractSource struct {
	SourceCode           string `json:"SourceCode"`
	ABI                  string `json:"ABI"`
	ContractName         string `json:"ContractName"`
	CompilerVersion      string `json:"CompilerVersion"`
	OptimizationUsed     string `json:"OptimizationUsed"`
	Runs                 string `json:"Runs"`
	ConstructorArguments string `json:"ConstructorArguments"`
	EVMVersion           string `json:"EVMVersion"`
	Library              string `json:"Library"`
	LicenseType          string `json:"LicenseType"`
	Proxy                string `json:"Proxy"`
	Implementation       string `json:"Implementation"`
	SwarmSource          string `json:"SwarmSource"`
}

func NewEtherscanClient(apiKey string) *EtherscanClient {
	return &EtherscanClient{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *EtherscanClient) GetContract(address string) (*Contract, error) {
	if e.apiKey == "" {
		return nil, fmt.Errorf("etherscan API key not configured")
	}

	// Fetch contract source
	source, err := e.getContractSource(address)
	if err != nil {
		return nil, err
	}

	// Fetch ABI
	abi, err := e.getContractABI(address)
	if err != nil {
		// Non-critical error
		abi = nil
	}

	isOpenSource := source.SourceCode != ""
	isProxy := source.Proxy == "1"

	return &Contract{
		Address:        address,
		Contract:       source.SourceCode,
		ABI:            abi,
		IsOpenSource:   isOpenSource,
		IsProxy:        isProxy,
		Implementation: source.Implementation,
	}, nil
}

func (e *EtherscanClient) getContractSource(address string) (*ContractSource, error) {
	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getsourcecode")
	params.Set("address", address)
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("%s?%s", EtherscanAPIURL, params.Encode())

	resp, err := e.client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("etherscan request failed: %w", err)
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
		return nil, fmt.Errorf("etherscan API error: %s", apiResp.Message)
	}

	var sources []ContractSource
	if err := json.Unmarshal(apiResp.Result, &sources); err != nil {
		return nil, err
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no contract source found for %s", address)
	}

	return &sources[0], nil
}

func (e *EtherscanClient) getContractABI(address string) ([]byte, error) {
	params := url.Values{}
	params.Set("module", "contract")
	params.Set("action", "getabi")
	params.Set("address", address)
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("%s?%s", EtherscanAPIURL, params.Encode())

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
		return nil, fmt.Errorf("etherscan API error: %s", apiResp.Message)
	}

	return apiResp.Result, nil
}

func (e *EtherscanClient) GetTransactions(address string, startBlock, endBlock int64) ([]Transaction, error) {
	params := url.Values{}
	params.Set("module", "account")
	params.Set("action", "txlist")
	params.Set("address", address)
	params.Set("startblock", fmt.Sprintf("%d", startBlock))
	params.Set("endblock", fmt.Sprintf("%d", endBlock))
	params.Set("sort", "asc")
	params.Set("apikey", e.apiKey)

	reqURL := fmt.Sprintf("%s?%s", EtherscanAPIURL, params.Encode())

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
		return nil, fmt.Errorf("etherscan API error: %s", apiResp.Message)
	}

	var txs []Transaction
	if err := json.Unmarshal(apiResp.Result, &txs); err != nil {
		return nil, err
	}

	return txs, nil
}

type Transaction struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nonce             string `json:"nonce"`
	BlockHash         string `json:"blockHash"`
	TransactionIndex  string `json:"transactionIndex"`
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	IsError           string `json:"isError"`
	TxReceiptStatus   string `json:"txreceipt_status"`
	Input             string `json:"input"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	GasUsed           string `json:"gasUsed"`
	Confirmations     string `json:"confirmations"`
}
