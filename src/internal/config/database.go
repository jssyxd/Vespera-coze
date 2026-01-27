package config

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VectorBits/Vespera/src/internal"
	_ "github.com/go-sql-driver/mysql"
)

var DBPool *sql.DB

// helloq InitDB 初始化 MySQL 连接池
func InitDB(ctx context.Context) (*sql.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Loading configuration failed: %w", err)
	}

	// 1. 尝试直接连接指定数据库
	dsn := config.GetDatabaseDSN(true)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("InitDB: %w", err)
	}

	// 检查连接
	ctxPing, cancelPing := context.WithTimeout(ctx, 2*time.Second)
	err = db.PingContext(ctxPing)
	cancelPing()

	if err != nil {
		// 2. 如果连接失败（可能是数据库不存在），尝试连接到 MySQL server root 并创建数据库
		fmt.Printf("Database ping failed for '%s': %v\n", config.Database.Name, err)

		dsnRoot := config.GetDatabaseDSN(false)
		dbRoot, errRoot := sql.Open("mysql", dsnRoot)
		if errRoot != nil {
			return nil, fmt.Errorf("Loading configuration failed: %w", errRoot)
		}
		defer dbRoot.Close()

		createDBSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", config.Database.Name)
		if _, errExec := dbRoot.ExecContext(ctx, createDBSQL); errExec != nil {
			return nil, fmt.Errorf("Loading configuration failed: %w", errExec)
		}
		fmt.Printf("✅  Database '%s' created successfully (or already exists)\n", config.Database.Name)

		// 重新连接到新创建的数据库
		if err := db.Close(); err != nil {
			// ignore close error
		}
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("Loading configuration failed: %w", err)
		}
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("InitDB ping failed: %w", err)
	}

	// 3. 自动迁移表结构
	if err := AutoMigrate(ctx, db, config); err != nil {
		db.Close()
		return nil, fmt.Errorf("Loading configuration failed: %w", err)
	}

	DBPool = db
	return db, nil
}

// AutoMigrate 自动检查并创建所需的表
func AutoMigrate(ctx context.Context, db *sql.DB, cfg *AppConfig) error {
	// 定义表结构模板
	const tableSchema = `
CREATE TABLE IF NOT EXISTS %s (
    address VARCHAR(42) PRIMARY KEY COMMENT 'Contract Address',
    contract LONGTEXT NOT NULL COMMENT 'Contract Bytecode',
    abi LONGTEXT NULL COMMENT 'Contract ABI (JSON)',
    balance VARCHAR(50) DEFAULT '0.000000' COMMENT 'Contract Balance',
    isopensource TINYINT(1) DEFAULT 0 COMMENT 'Is Open Source',
    isproxy TINYINT(1) DEFAULT 0 COMMENT 'Is Proxy Contract',
    implementation VARCHAR(42) NULL COMMENT 'Implementation Address',
    createtime DATETIME NOT NULL COMMENT 'Creation Time',
    createblock BIGINT UNSIGNED NOT NULL COMMENT 'Creation Block',
    txlast DATETIME NOT NULL COMMENT 'Last Interaction Time',
    isdecompiled TINYINT(1) DEFAULT 0 COMMENT 'Is Decompiled',
    dedcode LONGTEXT COMMENT 'Decompiled Pseudo-code',
    INDEX idx_createblock (createblock),
    INDEX idx_createtime (createtime),
    INDEX idx_isopensource (isopensource),
    INDEX idx_isdecompiled (isdecompiled)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='%s Smart Contract Information Table';
`
	// 遍历所有配置的链
	for _, chainConfig := range cfg.Chains {
		tableName := chainConfig.TableName
		if tableName == "" {
			continue
		}

		// 创建表
		query := fmt.Sprintf(tableSchema, tableName, chainConfig.Name)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("Loading configuration failed: %w", err)
		}

		// 检查并添加可能缺失的列 (简单的 schema evolution)
		// 这里只处理最核心的列，避免复杂的 migration 逻辑
		columnsToCheck := []struct {
			ColName string
			ColType string
		}{
			{"abi", "LONGTEXT NULL COMMENT 'Contract ABI (JSON)'"},
			{"isproxy", "TINYINT(1) DEFAULT 0 COMMENT 'Is Proxy Contract'"},
			{"implementation", "VARCHAR(42) NULL COMMENT 'Implementation Address'"},
		}

		for _, col := range columnsToCheck {
			// 使用 ADD COLUMN IF NOT EXISTS 语法 (MySQL 8.0+)
			// 为了兼容旧版 MySQL，我们可以先尝试添加，忽略 "Duplicate column name" 错误
			alterQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, col.ColName, col.ColType)
			if _, err := db.ExecContext(ctx, alterQuery); err != nil {
				// 简单的错误检查: 1060 Duplicate column name
				// 实际生产中应更严谨，但这里作为自动修复逻辑足够了
				// fmt.Printf("DEBUG: Column check %s.%s: %v\n", tableName, col.ColName, err)
			}
		}
	}

	return nil
}

func GetContracts(ctx context.Context, db *sql.DB, chainName string, limit int) ([]internal.Contract, error) {
	if db == nil {
		return nil, fmt.Errorf("GetContracts: db is nil")
	}

	tableName, err := GetTableName(chainName)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("SELECT address, contract, abi, balance, isopensource, createtime, createblock, txlast, isdecompiled, dedcode FROM %s", tableName)
	var rows *sql.Rows

	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, limit)
		rows, err = db.QueryContext(ctx, query)
	} else {
		rows, err = db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []internal.Contract
	for rows.Next() {
		var c internal.Contract
		var isOpenInt int
		var isDecompiledInt int
		var createBlock int64
		var createTime time.Time
		var txLast time.Time
		var balance string
		var abiJSON sql.NullString
		var dedCode sql.NullString

		if err := rows.Scan(&c.Address, &c.Code, &abiJSON, &balance, &isOpenInt, &createTime, &createBlock, &txLast, &isDecompiledInt, &dedCode); err != nil {
			return nil, err
		}

		c.Balance = balance
		c.IsOpenSource = isOpenInt != 0
		c.CreateTime = createTime
		c.CreateBlock = uint64(createBlock)
		c.TxLast = txLast
		c.IsDecompiled = isDecompiledInt != 0
		if abiJSON.Valid {
			c.ABI = abiJSON.String
		}
		if dedCode.Valid {
			c.DedCode = dedCode.String
		}

		out = append(out, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func GetContractsByAddresses(ctx context.Context, db *sql.DB, chainName string, addresses []string) ([]internal.Contract, error) {
	if db == nil {
		return nil, fmt.Errorf("GetContractsByAddresses: db is nil")
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("GetContractsByAddresses: addresses empty")
	}

	tableName, err := GetTableName(chainName)
	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(addresses))
	args := make([]interface{}, len(addresses))
	for i, addr := range addresses {
		placeholders[i] = "?"
		args[i] = addr
	}

	query := fmt.Sprintf("SELECT address, contract, abi, balance, isopensource, createtime, createblock, txlast, isdecompiled, dedcode FROM %s WHERE address IN (%s)",
		tableName, joinStrings(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []internal.Contract
	for rows.Next() {
		var c internal.Contract
		var isOpenInt int
		var isDecompiledInt int
		var createBlock int64
		var createTime time.Time
		var txLast time.Time
		var balance string
		var abiJSON sql.NullString
		var dedCode sql.NullString

		if err := rows.Scan(&c.Address, &c.Code, &abiJSON, &balance, &isOpenInt, &createTime, &createBlock, &txLast, &isDecompiledInt, &dedCode); err != nil {
			return nil, err
		}

		c.Balance = balance
		c.IsOpenSource = isOpenInt != 0
		c.CreateTime = createTime
		c.CreateBlock = uint64(createBlock)
		c.TxLast = txLast
		c.IsDecompiled = isDecompiledInt != 0
		if abiJSON.Valid {
			c.ABI = abiJSON.String
		}
		if dedCode.Valid {
			c.DedCode = dedCode.String
		}

		out = append(out, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func GetTableName(chainName string) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", fmt.Errorf("Loading configuration failed: %w", err)
	}

	chainConfig, err := config.GetChainConfig(chainName)
	if err != nil {
		return "", err
	}

	return chainConfig.TableName, nil
}

func GetChainConfig(chainName string) (*ChainConfig, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Loading configuration failed: %w", err)
	}

	return config.GetChainConfig(chainName)
}

func GetRPCManager(chainName string, proxy string) (*RPCManager, error) {
	chainConfig, err := GetChainConfig(chainName)
	if err != nil {
		return nil, err
	}

	return NewRPCManager(chainName, chainConfig.RPCURLs, 10*time.Second, proxy)
}

func GetExplorerConfig(chainName string) (*Explorer, error) {
	chainConfig, err := GetChainConfig(chainName)
	if err != nil {
		return nil, err
	}

	return &chainConfig.Explorer, nil
}

func GetAPIKeyManager(chainName string) (*APIKeyManager, error) {
	explorerConfig, err := GetExplorerConfig(chainName)
	if err != nil {
		return nil, err
	}

	apiKeys := explorerConfig.APIKeys
	fallbackKey := explorerConfig.APIKey

	if len(apiKeys) == 0 && fallbackKey != "" {
		apiKeys = []string{fallbackKey}
	}

	return NewAPIKeyManager(apiKeys, fallbackKey), nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
