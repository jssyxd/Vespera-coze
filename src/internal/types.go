package internal

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Contract 表示待扫描的合约基础信息，包含数据库表字段映射
type Contract struct {
	Address      string    `json:"address"`      // 合约地址
	Code         string    `json:"contract"`     // 合约代码
	ABI          string    `json:"abi"`          // 合约 ABI（JSON 字符串）
	Balance      string    `json:"balance"`      // 余额（以字符串保存以避免精度/类型问题）
	IsOpenSource bool      `json:"isOpenSource"` // 是否开源 (true/false 对应 1/0)
	CreateTime   time.Time `json:"createtime"`   // 创建时间
	CreateBlock  uint64    `json:"createblock"`  // 创建区块号
	TxLast       time.Time `json:"txlast"`       // 最后一次交互时间
	IsDecompiled bool      `json:"isdecompiled"` //是否开源
	DedCode      string    `json:"dedcode"`      //伪代码

}

// ReadLines 从文件中读取每一行并返回切片
func ReadLines(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	if ext == ".yaml" || ext == ".yml" {
		bs, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var list []string
		if err := yaml.Unmarshal(bs, &list); err == nil && len(list) > 0 {
			return normalizeUniqueNonEmpty(list), nil
		}

		var wrapper struct {
			Targets   []string `yaml:"targets"`
			Addresses []string `yaml:"addresses"`
		}
		if err := yaml.Unmarshal(bs, &wrapper); err == nil {
			if len(wrapper.Targets) > 0 {
				return normalizeUniqueNonEmpty(wrapper.Targets), nil
			}
			if len(wrapper.Addresses) > 0 {
				return normalizeUniqueNonEmpty(wrapper.Addresses), nil
			}
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, strings.TrimSpace(fields[0]))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return normalizeUniqueNonEmpty(lines), nil
}

func normalizeUniqueNonEmpty(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		v := strings.TrimSpace(it)
		if v == "" || strings.HasPrefix(v, "#") || strings.HasPrefix(v, "//") {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}
