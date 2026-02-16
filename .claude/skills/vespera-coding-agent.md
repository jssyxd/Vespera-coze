# Vespera Coding Agent

全自动托管区块链安全审计开发助手

## 简介

结合 Anthropic "Effective Harnesses for Long-Running Agents" 理念的 AI 辅助开发 Skill，专为 Vespera-Coze 项目设计。支持全托管、免代码、多 Agent 协作的智能合约安全审计开发流程。

## 核心理念

### 1. Initializer + Coding Agent 双模式

```
Initializer Agent（初始化代理）
├── 首次运行环境设置
├── 创建功能清单 (features.json)
├── 生成进度追踪文件
└── Git 初始提交

Coding Agent（编码代理）
├── 读取功能清单选择任务
├── 执行增量开发
├── Git 提交 + 更新进度
└── 留下清晰状态给下次会话
```

### 2. 全托管无代码架构

- **CI/CD**: GitHub Actions (2000 分钟/月免费)
- **数据库**: Supabase (500MB 免费) 或 SQLite 本地
- **前端**: Vercel (可选，100GB 流量免费)
- **AI 推理**: Self-hosted LLM (已配置 4 模型)
- **存储**: GitHub Artifacts (7 天保留)

### 3. 防 AI 幻觉机制

| 机制 | 实现 |
|------|------|
| 功能清单驱动 | JSON 格式清单，严格状态管理 |
| Git 作为记忆 | 每次变更强制提交 |
| 进度文件追踪 | `claude-progress.txt` 记录 |
| 多模型验证 | 3+ 模型交叉验证 |
| 端到端测试 | GitHub Actions 自动测试 |

## 可用命令

### 环境检查
```bash
# 检查所有配置
@vespera check-env

# 检查数据库连接
@vespera test-db

# 检查 API 可用性
@vespera test-apis
```

### 功能管理
```bash
# 添加新功能到清单
@vespera add-feature "功能描述"

# 标记功能完成
@vespera complete-feature "feature-id"

# 查看当前进度
@vespera status
```

### 开发工作流
```bash
# 开始新会话（读取进度，选择任务）
@vespera start-session

# 执行扫描
@vespera scan --chain=eth --mode=mode2

# 初始化数据库
@vespera init-db --chain=eth

# 生成报告
@vespera report --run-id=xxx
```

### GitHub Actions 管理
```bash
# 触发扫描工作流
@vespera trigger-scan

# 触发初始化工作流
@vespera trigger-init

# 查看最新运行状态
@vespera runs

# 下载最新报告
@vespera download-report
```

## 项目结构

```
vespera-coze/
├── .claude/
│   └── skills/
│       └── vespera-coding-agent.md    # 本 Skill 文件
├── .github/
│   └── workflows/
│       ├── init-db.yml                # Initializer Agent
│       └── scan.yml                   # Coding Agent
├── src/
│   ├── cmd/vespera/
│   │   ├── main.go                    # 扫描主程序
│   │   └── init.go                    # 初始化程序
│   └── internal/
│       ├── ai/                        # 多模型验证
│       ├── scanner/                   # 扫描引擎
│       └── config/                    # 数据库配置
├── data/
│   └── vespera.db                     # SQLite 数据库
├── docs/
│   └── features.json                  # 功能清单
├── DEVELOPMENT_GUIDE.md               # 开发指南
└── README.md                          # 项目说明
```

## 开发工作流

### Step 1: 开始新会话

```bash
@vespera start-session
```

AI Agent 将自动：
1. 读取 `claude-progress.txt` 了解上次工作
2. 查看 `docs/features.json` 选择最高优先级任务
3. 检查 Git 状态确保环境干净
4. 输出本次会话计划

### Step 2: 执行任务

根据功能清单执行任务，遵循以下原则：

1. **小步快跑**: 每个功能拆分为 < 30 分钟的小任务
2. **测试优先**: 先写测试，再写实现
3. **频繁提交**: 每完成一个小任务就提交
4. **更新进度**: 完成后更新 `claude-progress.txt`

### Step 3: 结束会话

```bash
@vespera end-session
```

AI Agent 将自动：
1. 运行所有测试确保通过
2. 提交所有变更到 Git
3. 更新 `claude-progress.txt` 记录本次工作
4. 更新 `docs/features.json` 标记完成的功能
5. 触发 GitHub Actions 自动化流程
6. 生成会话总结

## 技术栈

### 后端
- **语言**: Go 1.21
- **框架**: GORM (数据库 ORM)
- **数据库**: PostgreSQL (Supabase) / SQLite (本地)
- **API**: Etherscan API V2
- **AI**: DeepSeek + GLM + Minimax + Kimi (并行验证)

### CI/CD
- **平台**: GitHub Actions
- **触发器**: 定时 / 手动 / Webhook
- **缓存**: Go Modules, Python 包, 数据库
- **Artifact**: 扫描报告保留 7 天

### 云服务 (全托管)
| 服务 | 用途 | 免费额度 |
|------|------|---------|
| GitHub Actions | CI/CD | 2000 分钟/月 |
| Supabase | 数据库 | 500MB |
| Vercel | 前端 (可选) | 100GB 流量 |
| GitHub Artifacts | 报告存储 | 500MB |

## 多 Agent 协作

### Agent 角色

1. **Initializer Agent**
   - 职责: 环境设置、数据获取
   - 触发: 首次运行 / 数据库为空
   - 工作流: `init-db.yml`

2. **Scanner Agent**
   - 职责: 执行扫描、生成报告
   - 触发: 定时 / 手动
   - 工作流: `scan.yml`

3. **Analyzer Agent** (可选)
   - 职责: 深度分析高危漏洞
   - 触发: 发现 Critical 漏洞时
   - 工作流: `analyze.yml`

4. **Reporter Agent**
   - 职责: 通知、报告展示
   - 触发: 扫描完成
   - 集成: Slack / 邮件 / Vercel

### Agent 通信

**通过 Artifact 共享数据**:
```
Initializer → vespera-database-eth (Artifact)
     ↓
Scanner ← 读取 Artifact
     ↓
scan-report-{id} (Artifact)
     ↓
Reporter ← 下载报告
```

**通过数据库共享状态**:
```sql
-- 合约扫描状态表
CREATE TABLE ethereum (
    address VARCHAR(42) PRIMARY KEY,
    status VARCHAR(20),      -- pending / scanning / completed
    scan_result JSONB,       -- 扫描结果
    locked_by VARCHAR(50),   -- 哪个 Agent 在处理
    locked_at TIMESTAMP      -- 锁定时间
);
```

## 防幻觉策略

### 1. 功能清单驱动

所有功能必须先在 `docs/features.json` 中定义：

```json
{
  "features": [
    {
      "id": "etherscan-v2",
      "description": "迁移到 Etherscan V2 API",
      "status": "completed",
      "priority": "high",
      "tests": ["api-connectivity"],
      "created_at": "2026-02-17",
      "completed_at": "2026-02-17"
    }
  ]
}
```

### 2. Git 强制提交

每个会话必须提交：
```bash
git add -A
git commit -m "feat: 功能描述

- 修改1
- 修改2
- 测试通过

Co-Authored-By: Claude"
```

### 3. 进度追踪

`claude-progress.txt` 格式：
```
2026-02-17 10:00 - Session Start
2026-02-17 10:05 - 完成功能: etherscan-v2 迁移
2026-02-17 10:15 - 运行测试: 全部通过
2026-02-17 10:20 - Git 提交: commit-hash
2026-02-17 10:25 - Session End
```

## 扩展开发

### 添加新 Agent

1. 创建新的 workflow 文件：
```yaml
# .github/workflows/new-agent.yml
name: New Agent
on:
  workflow_dispatch:
jobs:
  agent:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # Agent 逻辑...
```

2. 注册到 Skill：
```markdown
### 新 Agent
```bash
@vespera trigger-new-agent
```
```

### 添加新模型

在 `src/internal/ai/multi_model.go` 中添加：
```go
models: map[string]ModelConfig{
    "new-model": {
        Name:    "new-model-v1",
        Timeout: 300,
    },
}
```

## 参考资料

1. [Anthropic - Effective Harnesses for Long-Running Agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
2. [GitHub Actions 文档](https://docs.github.com/en/actions)
3. [Supabase 文档](https://supabase.com/docs)
4. [Etherscan API V2](https://docs.etherscan.io/v2-migration)

## 维护者

- **项目**: jssyxd/Vespera-coze
- **AI Agent**: Claude (Anthropic)
- **开发模式**: Vibe Coding + Agent Teams

---

**版本**: 1.0
**最后更新**: 2026-02-17
**许可**: MIT
