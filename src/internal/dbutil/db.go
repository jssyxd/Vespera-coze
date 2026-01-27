package dbutil

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/VectorBits/Vespera/src/internal/config"
)

//qhello 这个文件可能有点问题。后面在改

func GetAddressesFromDB(db *sql.DB, chainName string, blockRange *config.BlockRange) ([]string, error) {
	tableName, err := config.GetTableName(chainName)
	if err != nil {
		return nil, err
	}

	var query string
	var args []interface{}

	// 基础条件：必须开源且有代码
	baseConditions := "isopensource = 1 AND contract IS NOT NULL AND contract != ''"

	effectiveAddrExpr := "CASE WHEN isproxy = 1 AND implementation IS NOT NULL AND implementation != '' THEN implementation ELSE address END"

	if blockRange != nil {
		query = fmt.Sprintf(`
			SELECT effective_address, MAX(createblock) AS max_block
			FROM (
				SELECT %s AS effective_address, createblock
				FROM %s
				WHERE %s AND createblock BETWEEN ? AND ?
			) t
			GROUP BY effective_address
			ORDER BY max_block DESC
			LIMIT 1000
		`, effectiveAddrExpr, tableName, baseConditions)
		args = []interface{}{blockRange.Start, blockRange.End}
	} else {
		query = fmt.Sprintf(`
			SELECT effective_address, MAX(createblock) AS max_block
			FROM (
				SELECT %s AS effective_address, createblock
				FROM %s
				WHERE %s
			) t
			GROUP BY effective_address
			ORDER BY max_block DESC
			LIMIT 1000
		`, effectiveAddrExpr, tableName, baseConditions)
		args = []interface{}{}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addrs := make([]string, 0)
	for rows.Next() {
		var a string
		var maxBlock uint64
		if err := rows.Scan(&a, &maxBlock); err != nil {
			return nil, err
		}
		// 过滤空字符串
		a = strings.TrimSpace(a)
		if a != "" {
			addrs = append(addrs, a)
		}
	}
	return addrs, nil
}
