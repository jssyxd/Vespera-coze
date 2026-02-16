# 多链区块链浏览器研究：扩展 Vespera 支持多链合约扫描

> 本文档总结主流公链的区块链浏览器特性，为 Vespera 多链合约适配提供技术参考。

## 一、Etherscan 详解

### 1.1 什么是 Etherscan

**Etherscan** (https://etherscan.io/) 是目前最知名、最常用的 **以太坊区块链浏览器** (Ethereum Block Explorer)，也被称为"以太坊的搜索引擎"。

简单来说，它就像是专门用来查看以太坊网络上所有公开数据的"百度/Google"。

### 1.2 核心功能

| 功能 | 用途 | 示例 |
|------|------|------|
| **交易查询** | 输入 TxID 查看交易详情 | 转账金额、Gas费、状态、时间 |
| **地址查询** | 查看钱包地址资产 | ETH余额、代币持有、NFT、历史记录 |
| **合约查询** | 查看智能合约详情 | 源代码、创建者、持有者分布、读写函数 |
| **区块查询** | 查看区块信息 | 高度、矿工、Gas使用量、时间戳 |
| **代币查询** | 查询 ERC-20/721 代币 | 总量、转账记录、持币大户 |
| **Gas监控** | 实时 Gas 价格 | 网络拥堵情况、推荐 Gas Price |

### 1.3 技术特点

- **只读工具**: 不是钱包，不能转账，只能查询
- **数据公开**: 所有数据来自以太坊主网，完全公开透明
- **实时同步**: 数据与链上同步，延迟通常 < 15秒
- **API支持**: 提供免费 API（有调用限制），开发者常用
- **多语言**: 支持中文、英文等多种语言

### 1.4 API 使用

```bash
# V2 API 格式（2025年后必须使用）
https://api.etherscan.io/v2/api?chainid=1&module=contract&action=getsourcecode&address=0x...&apikey=YourApiKey

# 关键参数
# - chainid=1: 以太坊主网
# - module: 模块 (contract/account/stats等)
# - action: 操作 (getsourcecode/txlist/balance等)
```

---

## 二、EVM 兼容链浏览器

这些链使用与以太坊相同的技术栈，界面和功能几乎复制 Etherscan。

### 2.1 BSC (BNB Chain)

| 属性 | 详情 |
|------|------|
| **浏览器** | BscScan - https://bscscan.com/ |
| **API** | https://api.bscscan.com/api |
| **Chain ID** | 56 |
| **特点** | 低 Gas、高速、DeFi 生态丰富 |
| **API限制** | 5 calls/second, 100,000 calls/day |

**Vespera 适配要点**:
```go
const (
    BscscanAPI = "https://api.bscscan.com/api"
    ChainIDBSC = "56"
)
```

### 2.2 Polygon (MATIC)

| 属性 | 详情 |
|------|------|
| **浏览器** | PolygonScan - https://polygonscan.com/ |
| **API** | https://api.polygonscan.com/api |
| **Chain ID** | 137 |
| **特点** | 以太坊 L2、极低 Gas、NFT 和游戏常用 |
| **API限制** | 5 calls/second |

**Vespera 适配要点**:
```go
const (
    PolygonscanAPI = "https://api.polygonscan.com/api"
    ChainIDPolygon = "137"
)
```

### 2.3 Arbitrum

| 属性 | 详情 |
|------|------|
| **浏览器** | Arbiscan - https://arbiscan.io/ |
| **API** | https://api.arbiscan.io/api |
| **Chain ID** | 42161 |
| **特点** | Optimistic Rollup L2、高吞吐量、DeFi 热门 |
| **API限制** | 5 calls/second |

**Vespera 适配要点**:
```go
const (
    ArbiscanAPI = "https://api.arbiscan.io/api"
    ChainIDArbitrum = "42161"
)
```

### 2.4 Optimism (OP Mainnet)

| 属性 | 详情 |
|------|------|
| **浏览器** | Optimistic Etherscan - https://optimistic.etherscan.io/ |
| **API** | https://api-optimistic.etherscan.io/api |
| **Chain ID** | 10 |
| **特点** | Optimistic Rollup、与以太坊高度兼容 |
| **API限制** | 5 calls/second |

### 2.5 Base (Coinbase L2)

| 属性 | 详情 |
|------|------|
| **浏览器** | BaseScan - https://basescan.org/ |
| **API** | https://api.basescan.org/api |
| **Chain ID** | 8453 |
| **特点** | Coinbase 官方 L2、SocialFi 热门 |
| **API限制** | 5 calls/second |

### 2.6 Avalanche C-Chain

| 属性 | 详情 |
|------|------|
| **浏览器** | SnowTrace - https://snowtrace.io/ |
| **API** | https://api.snowtrace.io/api |
| **Chain ID** | 43114 |
| **特点** | 高吞吐、Subnet 技术、企业级应用 |

### 2.7 Fantom

| 属性 | 详情 |
|------|------|
| **浏览器** | FTMScan - https://ftmscan.com/ |
| **API** | https://api.ftmscan.com/api |
| **Chain ID** | 250 |
| **特点** | 极速确认、DAG 技术、DeFi 生态 |

---

## 三、非 EVM 主流链浏览器

这些链使用不同的技术架构，需要单独适配。

### 3.1 Solana

| 属性 | 详情 |
|------|------|
| **浏览器** | Solscan - https://solscan.io/ |
| **备选** | Orb (Helius) - https://orbmarkets.io/ |
| **特点** | 高吞吐、低延迟、Rust 智能合约 |
| **技术栈** | 非 EVM、不同地址格式、不同交易结构 |

**适配难度**: 🔴 高 (需要完全不同的扫描引擎)

### 3.2 Bitcoin

| 属性 | 详情 |
|------|------|
| **浏览器** | Blockchain.com - https://www.blockchain.com/explorer |
| **备选** | Blockchair - https://blockchair.com/bitcoin |
| **特点** | UTXO 模型、无智能合约、纯转账 |
| **用途** | 资产追踪、交易确认 |

**适配难度**: 🔴 高 (无智能合约概念)

### 3.3 Tron

| 属性 | 详情 |
|------|------|
| **浏览器** | Tronscan - https://tronscan.org/ |
| **特点** | EVM 兼容但修改版、Solidity 合约、低 Gas |
| **API** | TronGrid API |

**适配难度**: 🟡 中 (类似 EVM 但 API 不同)

---

## 四、多链聚合浏览器

### 4.1 Blockchair

- **网址**: https://blockchair.com/
- **支持链**: 40+ 链 (BTC, ETH, SOL, ADA, XRP 等)
- **特点**: 强大的分析功能、统一接口

### 4.2 OKLink

- **网址**: https://www.oklink.com/
- **支持链**: 多链 + 地址标签分析
- **特点**: 交易所背景、数据准确

---

## 五、Vespera 多链适配技术方案

### 5.1 优先级排序

| 优先级 | 链 | Chain ID | API URL | 理由 |
|--------|-----|----------|---------|------|
| P0 | Ethereum | 1 | api.etherscan.io/v2 | 已完成 ✅ |
| P1 | BSC | 56 | api.bscscan.com | 低 Gas、DeFi 多 |
| P1 | Polygon | 137 | api.polygonscan.com | NFT/游戏热门 |
| P1 | Arbitrum | 42161 | api.arbiscan.io | L2 龙头 |
| P2 | Optimism | 10 | api-optimistic.etherscan.io | 以太坊正统 L2 |
| P2 | Base | 8453 | api.basescan.org | Coinbase 背书 |
| P3 | Avalanche | 43114 | api.snowtrace.io | 企业级 |
| P3 | Fantom | 250 | api.ftmscan.com | 快速确认 |

### 5.2 架构设计

```go
// chain_config.go
package config

type ChainConfig struct {
    Name        string
    ChainID     string
    APIBaseURL  string
    ExplorerURL string
    APIKey      string // 各链需要独立 API Key
}

var SupportedChains = map[string]ChainConfig{
    "eth": {
        Name:        "Ethereum",
        ChainID:     "1",
        APIBaseURL:  "https://api.etherscan.io/v2/api",
        ExplorerURL: "https://etherscan.io",
    },
    "bsc": {
        Name:        "BSC",
        ChainID:     "56",
        APIBaseURL:  "https://api.bscscan.com/api",
        ExplorerURL: "https://bscscan.com",
    },
    "polygon": {
        Name:        "Polygon",
        ChainID:     "137",
        APIBaseURL:  "https://api.polygonscan.com/api",
        ExplorerURL: "https://polygonscan.com",
    },
    "arbitrum": {
        Name:        "Arbitrum",
        ChainID:     "42161",
        APIBaseURL:  "https://api.arbiscan.io/api",
        ExplorerURL: "https://arbiscan.io",
    },
}
```

### 5.3 数据库表设计

```sql
-- 支持多链的合约表
CREATE TABLE contracts (
    id SERIAL PRIMARY KEY,
    chain VARCHAR(20) NOT NULL,          -- eth/bsc/polygon/arbitrum
    address VARCHAR(42) NOT NULL,
    contract TEXT,
    abi JSONB,
    isopensource BOOLEAN DEFAULT FALSE,
    createblock BIGINT,
    createtime TIMESTAMP,
    scan_result JSONB,
    scan_time TIMESTAMP,
    UNIQUE(chain, address)               -- 复合唯一键
);

-- 索引优化
CREATE INDEX idx_contracts_chain ON contracts(chain);
CREATE INDEX idx_contracts_address ON contracts(address);
CREATE INDEX idx_contracts_chain_address ON contracts(chain, address);
```

### 5.4 API Key 管理

各链需要独立的 API Key：

```bash
# GitHub Secrets 配置
ETHERSCAN_API_KEY=xxx          # Etherscan
BSCSCAN_API_KEY=xxx            # BscScan
POLYGONSCAN_API_KEY=xxx        # PolygonScan
ARBISCAN_API_KEY=xxx           # Arbiscan
```

### 5.5 初始化工作流更新

```yaml
# .github/workflows/init-db.yml
name: Initialize Database
on:
  workflow_dispatch:
    inputs:
      chain:
        description: 'Blockchain to initialize'
        required: true
        default: 'eth'
        type: choice
        options:
          - eth
          - bsc
          - polygon
          - arbitrum
          - optimism
          - base
```

### 5.6 扫描工作流更新

```yaml
# .github/workflows/scan.yml
strategy:
  matrix:
    chain: [eth, bsc, polygon, arbitrum]
  fail-fast: false  # 一个链失败不影响其他链
```

---

## 六、开发任务清单

### Phase 1: EVM 兼容链 (P1 优先级)

- [ ] 重构数据库表结构支持多链
- [ ] 创建 `chain_config.go` 配置文件
- [ ] 更新 `initializer.go` 支持动态链配置
- [ ] 添加 BSC 支持
- [ ] 添加 Polygon 支持
- [ ] 添加 Arbitrum 支持
- [ ] 更新 GitHub Actions 工作流
- [ ] 测试各链 API 连通性

### Phase 2: 更多 EVM 链 (P2 优先级)

- [ ] 添加 Optimism 支持
- [ ] 添加 Base 支持
- [ ] 添加 Avalanche 支持
- [ ] 添加 Fantom 支持

### Phase 3: 非 EVM 链 (P3 优先级)

- [ ] 调研 Solana 扫描可行性
- [ ] 评估 Tron 适配成本

---

## 七、API Key 申请指南

### 申请地址

| 链 | 申请地址 |
|-----|----------|
| Ethereum | https://etherscan.io/myapikey |
| BSC | https://bscscan.com/myapikey |
| Polygon | https://polygonscan.com/myapikey |
| Arbitrum | https://arbiscan.io/myapikey |
| Optimism | https://optimistic.etherscan.io/myapikey |
| Base | https://basescan.org/myapikey |

### 免费额度

所有 Etherscan 系列浏览器统一限制：
- **速率**: 5 calls/second
- **日限额**: 100,000 calls/day

---

## 八、参考资料

1. **Etherscan API 文档**: https://docs.etherscan.io/
2. **Etherscan V2 迁移指南**: https://docs.etherscan.io/v2-migration
3. **链列表 (Chainlist)**: https://chainlist.org/
4. **EVM 兼容链对比**: https://ethereum.org/en/developers/docs/scaling/

---

**文档版本**: 1.0
**最后更新**: 2026-02-17
**作者**: Claude (AI Agent)
**用途**: Vespera 多链扩展开发参考
