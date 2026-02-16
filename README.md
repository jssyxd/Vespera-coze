# Vespera-Coze

> 基于多模型并行验证的智能合约安全审计与套利机会挖掘系统

[![GitHub Actions](https://github.com/jssyxd/vespera-coze/actions/workflows/scan.yml/badge.svg)](https://github.com/jssyxd/vespera-coze/actions)

## 🚀 核心特性

- **🔍 多模型并行验证**: 集成 DeepSeek、GLM、MiniMax、Kimi 等多个大模型，利用"振荡效应"降低误报率
- **🤖 完全自动化**: GitHub Actions + Supabase，零成本云端部署
- **💎 漏洞赏金支持**: 自动生成专业报告，支持 Immunefi/HackerOne 提交
- **📈 套利机会挖掘**: Mode2 混合扫描模式，发现潜在套利空间
- **🌐 多链支持**: Ethereum、BSC、Polygon、Arbitrum 等 61 条链

## 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                      GitHub Actions                         │
│           (定时触发 / 手动触发 / Webhook触发)                │
└──────────────┬──────────────────────────────────────────────┘
               │
    ┌──────────▼──────────┐    ┌──────────────┐    ┌──────────────┐
    │   Vespera Scanner   │◄──►│ Supabase DB  │    │   LLM API    │
    │   (Go + Slither)    │    │ (PostgreSQL) │    │139.224.113.163│
    └──────────┬──────────┘    └──────────────┘    └──────────────┘
               │
    ┌──────────▼──────────┐
    │  EVM Blockchain     │
    │ (Mainnet/BSC/等)    │
    └─────────────────────┘
```

## 📋 快速开始

### 1. 环境准备

需要以下账号和凭证：

| 服务 | 用途 | 获取方式 |
|------|------|---------|
| **GitHub** | CI/CD 平台 | [github.com](https://github.com) |
| **Supabase** | 云数据库 | [supabase.com](https://supabase.com) |
| **Etherscan** | 链上数据 | [etherscan.io/myapikey](https://etherscan.io/myapikey) |
| **自建 LLM** | AI 分析 | 已配置: `139.224.113.163:8317` |

### 2. 部署步骤

```bash
# 1. 克隆仓库
git clone https://github.com/jssyxd/vespera-coze.git
cd vespera-coze

# 2. 配置环境变量
cp .env.example .env
# 编辑 .env 填入你的凭证

# 3. 运行部署脚本
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

### 3. 配置 GitHub Secrets

在仓库 Settings → Secrets and variables → Actions 中添加：

```
SUPABASE_HOST=db.xxx.supabase.co
SUPABASE_PORT=5432
SUPABASE_USER=postgres
SUPABASE_PASSWORD=your_password
SUPABASE_DB=postgres
AI_API_KEY=api
AI_BASE_URL=http://139.224.113.163:8317/v1
ETHERSCAN_API_KEY=your_etherscan_key
```

### 4. 初始化数据库

在 Supabase SQL Editor 中执行：`scripts/init_db.sql`

### 5. 触发首次扫描

1. 访问 Actions 页面
2. 点击 "Vespera Scan" 工作流
3. 点击 "Run workflow"
4. 查看执行日志

## 🔧 三种扫描模式

| 模式 | 用途 | 示例命令 |
|------|------|---------|
| **Mode 1** | 定向扫描特定合约 | `./vespera -m mode1 -c eth -addr 0x...` |
| **Mode 2** | 混合扫描（漏洞+套利） | `./vespera -m mode2 -c eth -range 20000000-20000200` |
| **Mode 3** | 实时监听新合约 | `./vespera -m mode3 -c eth` |

## 📊 扫描报告示例

```markdown
# Vespera Scan Report

**Scan Time:** 2025-01-09T12:00:00Z
**Duration:** 15m30s
**Contracts Scanned:** 50

## 🚨 Vulnerabilities

### [HIGH] Reentrancy
- **Description:** External call before state update
- **Line:** 45
- **Recommendation:** Use Checks-Effects-Interactions pattern

## 💰 Arbitrage Opportunities

### Price Discrepancy
- **Description:** Price difference between DEXs
- **Expected Profit:** 0.05 ETH
```

## 💰 商业化路径

### 1. 漏洞赏金 (短期)
- 平台: [Immunefi](https://immunefi.com/), [HackerOne](https://hackerone.com)
- 收益: Critical $50K-$1M, High $10K-$100K
- 使用 Mode1 定向扫描热门 DeFi 协议

### 2. 套利机器人 (中期)
- 使用 Mode2 发现套利机会
- 测试网验证 → 主网部署
- 预期: 月收益 1-5 ETH

### 3. SaaS 服务 (长期)
- Web 界面 + API 服务
- 订阅制收费
- 定价: 免费版 / $99月 / $999月

## 📁 项目结构

```
vespera-coze/
├── .github/workflows/     # GitHub Actions
├── src/
│   ├── cmd/vespera/       # 主程序入口
│   ├── internal/
│   │   ├── ai/            # AI 客户端
│   │   ├── config/        # 配置管理
│   │   └── scanner/       # 扫描引擎
│   └── go.mod
├── config/                # 配置文件
├── scripts/               # 部署脚本
├── reports/               # 扫描报告输出
└── README.md
```

## 🔐 安全说明

- 所有敏感信息通过 GitHub Secrets 管理
- 数据库使用 Row Level Security (RLS)
- API Key 定期轮换

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

- 原项目: [VectorBits/Vespera](https://github.com/VectorBits/Vespera)
- 文档: [vespera-doc.vectorbits.net](https://vespera-doc.vectorbits.net/)

---

**免责声明**: 本工具仅用于安全研究和教育目的。使用本工具发现的漏洞请遵循负责任的披露原则。
