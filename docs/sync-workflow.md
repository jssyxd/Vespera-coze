# Upstream Sync Workflow

## 快速同步（日常使用）

```bash
# 1. 获取上游更新
git fetch upstream

# 2. 查看变更
git log --oneline HEAD...upstream/main

# 3. 合并上游更新
git merge --no-ff upstream/main -m "sync: Merge upstream changes"

# 4. 测试
cd src && go build ./... && go test ./...

# 5. 推送
git push origin main
```

## 处理冲突

如果非保护文件有冲突：

```bash
# 查看冲突文件
git status

# 编辑解决冲突
vim <conflicted-file>

# 标记已解决
git add <conflicted-file>

# 完成合并
git commit
```

## 选择性接受上游变更

```bash
# 接受上游版本（覆盖本地）
git checkout --theirs <file>

# 接受本地版本
git checkout --ours <file>
```

## 保护文件列表

以下文件永远不会被上游覆盖（已配置 merge=ours）：
- `.github/workflows/*`
- `scripts/deploy.sh`
- `scripts/init.sh`
- `CLAUDE.md`
- `README.md`
- `task.json`
- `docs/**`
- `.gitattributes`

## 首次同步历史

首次同步使用了 `--allow-unrelated-histories` 选项，因为 vespera-coze 不是通过 fork 创建的：

```bash
git remote add upstream https://github.com/VectorBits/Vespera.git
git fetch upstream
git merge --no-ff --allow-unrelated-histories upstream/main
```

## 相关文档

- 设计文档: `docs/plans/2026-02-21-upstream-sync-design.md`
- 实施计划: `docs/plans/2026-02-21-upstream-sync-implementation.md`
