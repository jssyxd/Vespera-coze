package solc

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const MetadataPrefix = "// SolExcavator Metadata: "

type Metadata struct {
	ContractName string `json:"ContractName"`
}

// AttachMetadata 附加元数据到源码
func AttachMetadata(source, contractName string) string {
	if contractName == "" {
		return source
	}
	meta := Metadata{ContractName: contractName}
	bs, _ := json.Marshal(meta)
	return MetadataPrefix + string(bs) + "\n" + source
}

// DetachMetadata 分离元数据和源码
func DetachMetadata(source string) (string, *Metadata) {
	if strings.HasPrefix(source, MetadataPrefix) {
		idx := strings.Index(source, "\n")
		if idx != -1 {
			jsonStr := source[len(MetadataPrefix):idx]
			var meta Metadata
			if err := json.Unmarshal([]byte(jsonStr), &meta); err == nil {
				return source[idx+1:], &meta
			}
		}
	}
	return source, nil
}

// StandardInputJSON 标准 JSON 输入格式
type StandardInputJSON struct {
	Language string                 `json:"language"`
	Sources  map[string]SourceFile  `json:"sources"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

type SourceFile struct {
	Content string `json:"content"`
}

// IsJSONSource 检查源代码是否为多文件 JSON 格式
func IsJSONSource(source string) bool {
	cleanSource, _ := DetachMetadata(source)
	trimmed := strings.TrimSpace(cleanSource)
	return strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, "\"content\"")
}

// FlattenJSONSource 尝试扁平化多文件 JSON 格式代码
// 返回扁平化后的代码，如果失败则返回原始代码
func FlattenJSONSource(source string) (string, error) {
	cleanSource, meta := DetachMetadata(source)
	if !IsJSONSource(cleanSource) {
		return cleanSource, nil
	}
	contractName := ""
	if meta != nil {
		contractName = meta.ContractName
	}
	return FlattenWithFoundry(cleanSource, contractName)
}

// FlattenWithFoundry 使用 Foundry 扁平化多文件合约
func FlattenWithFoundry(jsonStr string, mainContractName string) (string, error) {
	normalized := normalizeJSONSource(jsonStr)

	// 解析 JSON
	var standardInput StandardInputJSON
	if err := json.Unmarshal([]byte(normalized), &standardInput); err != nil || len(standardInput.Sources) == 0 {
		var directSources map[string]SourceFile
		if err := json.Unmarshal([]byte(normalized), &directSources); err != nil || len(directSources) == 0 {
			return "", fmt.Errorf("invalid multi-file JSON format")
		}
		standardInput.Sources = directSources
	}

	// 创建 Foundry 项目目录
	foundryDir, err := os.MkdirTemp("", "foundry_flatten_*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(foundryDir)

	// 写入所有源文件
	var mainFile string
	var candidates []string
	fileSizes := make(map[string]int)

	for relPath, source := range standardInput.Sources {
		cleanPath := strings.TrimPrefix(relPath, "./")
		cleanPath = strings.TrimPrefix(cleanPath, "/")
		if !strings.HasSuffix(cleanPath, ".sol") {
			cleanPath += ".sol"
		}

		absPath := filepath.Join(foundryDir, cleanPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return "", err
		}

		content := strings.ReplaceAll(source.Content, "\r\n", "\n")
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			return "", err
		}

		// 记录候选文件（排除库文件）
		if !strings.HasPrefix(cleanPath, "@") && !strings.Contains(cleanPath, "node_modules") {
			candidates = append(candidates, cleanPath)
			fileSizes[cleanPath] = len(content)
		}
	}

	// 智能选择主文件
	// 策略 0: 如果提供了 mainContractName，优先匹配
	// 策略 1: 排除路径中包含 "interface" 或 "abstract" 的文件
	// 策略 2: 选择剩余文件中最大的那个（通常主合约代码量最大）
	var bestCandidate string
	var maxScore int // 简单的评分系统

	for _, cand := range candidates {
		score := 100 // 基础分

		// 策略 0: 匹配主合约名
		if mainContractName != "" {
			baseName := filepath.Base(cand)
			baseNameNoExt := strings.TrimSuffix(baseName, ".sol")
			if strings.EqualFold(baseNameNoExt, mainContractName) {
				score += 10000 // 极高优先级
			} else if strings.Contains(strings.ToLower(baseNameNoExt), strings.ToLower(mainContractName)) {
				score += 5000 // 包含名字，较高优先级
			}
		}

		lowerPath := strings.ToLower(cand)
		if strings.Contains(lowerPath, "interface") {
			score -= 50
		}
		if strings.Contains(lowerPath, "abstract") {
			score -= 30
		}
		if strings.Contains(lowerPath, "test") {
			score -= 80
		}
		if strings.Contains(lowerPath, "mock") {
			score -= 80
		}

		// 加上文件大小权重 (每 100 字节 +1 分，上限 50 分)
		sizeBonus := fileSizes[cand] / 100
		if sizeBonus > 50 {
			sizeBonus = 50
		}
		score += sizeBonus

		if score > maxScore {
			maxScore = score
			bestCandidate = cand
		}
	}

	if bestCandidate != "" {
		mainFile = bestCandidate
	} else if len(candidates) > 0 {
		// 兜底：如果没有合适的，选第一个候选者
		sort.Strings(candidates)
		mainFile = candidates[0]
	} else {
		// 极端情况：全是库文件？随便选一个 sol 文件
		// 重新收集所有文件
		var allFiles []string
		for relPath := range standardInput.Sources {
			allFiles = append(allFiles, relPath)
		}
		if len(allFiles) > 0 {
			sort.Strings(allFiles)
			mainFile = allFiles[0]
		}
	}

	// 检查 forge 是否可用
	if _, err := exec.LookPath("forge"); err != nil {
		return "", fmt.Errorf("forge not found in PATH, please install Foundry")
	}

	// 初始化 Foundry 项目
	initCmd := exec.Command("forge", "init", ".", "--force", "--no-git")
	initCmd.Dir = foundryDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("forge init failed: %s", string(output))
	}

	// 删除 Foundry 默认文件
	defaultFiles := []string{
		filepath.Join(foundryDir, "script", "Counter.s.sol"),
		filepath.Join(foundryDir, "src", "Counter.sol"),
		filepath.Join(foundryDir, "test", "Counter.t.sol"),
	}
	for _, f := range defaultFiles {
		os.Remove(f)
	}

	// 配置 foundry.toml
	if err := configureFoundryToml(foundryDir, standardInput); err != nil {
		return "", err
	}

	// 使用 forge flatten 扁平化
	flattenCmd := exec.Command("forge", "flatten", mainFile)
	flattenCmd.Dir = foundryDir
	output, err := flattenCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("forge flatten failed: %s", string(output))
	}

	flattened := string(output)
	if !strings.Contains(flattened, "pragma solidity") {
		return "", fmt.Errorf("forge flatten output invalid")
	}

	// Clean up pragma versions: keep only the highest version
	flattened = cleanupPragmas(flattened)

	return flattened, nil
}

// normalizeJSONSource 规范化 JSON 字符串
func normalizeJSONSource(jsonStr string) string {
	trimmed := strings.TrimSpace(jsonStr)
	if strings.HasPrefix(trimmed, "{{") && strings.HasSuffix(trimmed, "}}") {
		return trimmed[1 : len(trimmed)-1]
	}
	return trimmed
}

// cleanupPragmas removes multiple pragma declarations and keeps the highest version
func cleanupPragmas(source string) string {
	// Regular expression to find all pragma solidity lines
	pragmaRe := regexp.MustCompile(`pragma\s+solidity\s+([^;]+);`)

	matches := pragmaRe.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return source
	}

	var highestVersion string
	var highestVerMajor, highestVerMinor, highestVerPatch int

	for _, match := range matches {
		versionStr := match[1]
		// Regex to find things looking like version numbers
		verNumRe := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
		verMatches := verNumRe.FindAllStringSubmatch(versionStr, -1)

		for _, verMatch := range verMatches {
			major, _ := strconv.Atoi(verMatch[1])
			minor, _ := strconv.Atoi(verMatch[2])
			patch, _ := strconv.Atoi(verMatch[3])

			isHigher := false
			if major > highestVerMajor {
				isHigher = true
			} else if major == highestVerMajor {
				if minor > highestVerMinor {
					isHigher = true
				} else if minor == highestVerMinor {
					if patch > highestVerPatch {
						isHigher = true
					}
				}
			}

			if isHigher || highestVersion == "" {
				highestVerMajor = major
				highestVerMinor = minor
				highestVerPatch = patch
				highestVersion = fmt.Sprintf("%d.%d.%d", major, minor, patch)
			}
		}
	}

	if highestVersion == "" {
		return source
	}

	// Remove all existing pragma solidity lines
	cleanedSource := pragmaRe.ReplaceAllString(source, "")

	// Insert the highest version
	finalPragma := fmt.Sprintf("pragma solidity ^%s;", highestVersion)

	// Check if file starts with SPDX
	lines := strings.Split(cleanedSource, "\n")
	var finalLines []string

	spdxIndex := -1
	for i, line := range lines {
		if strings.Contains(line, "SPDX-License-Identifier") {
			spdxIndex = i
			break
		}
	}

	if spdxIndex != -1 {
		// Insert after SPDX
		finalLines = append(finalLines, lines[:spdxIndex+1]...)
		finalLines = append(finalLines, finalPragma)
		finalLines = append(finalLines, lines[spdxIndex+1:]...)
	} else {
		// Insert at top
		finalLines = append([]string{finalPragma}, lines...)
	}

	return strings.Join(finalLines, "\n")
}

// configureFoundryToml 配置 foundry.toml
func configureFoundryToml(foundryDir string, input StandardInputJSON) error {
	tomlPath := filepath.Join(foundryDir, "foundry.toml")

	var config strings.Builder
	config.WriteString("[profile.default]\n")
	config.WriteString("src = \".\"\n")
	config.WriteString("out = \"out\"\n")
	config.WriteString("libs = [\"lib\"]\n")

	// 提取并设置 solc 版本
	solcVersion := extractVersionFromSources(input.Sources)
	if solcVersion != "" {
		// 检查版本兼容性
		solcVersion = normalizeSolcVersion(solcVersion)
		config.WriteString(fmt.Sprintf("solc_version = \"%s\"\n", solcVersion))
	}

	// 提取 remappings
	var remappings []string
	if settings := input.Settings; settings != nil {
		if remap, ok := settings["remappings"].([]interface{}); ok {
			for _, r := range remap {
				if s, ok := r.(string); ok {
					remappings = append(remappings, s)
				}
			}
		}
		// 提取 viaIR
		if viaIR, ok := settings["viaIR"].(bool); ok && viaIR {
			config.WriteString("via_ir = true\n")
		}
		// 提取 evmVersion
		if evmVersion, ok := settings["evmVersion"].(string); ok && evmVersion != "" {
			config.WriteString(fmt.Sprintf("evm_version = \"%s\"\n", evmVersion))
		}
	}

	// 自动添加常见 remappings
	remappings = autoDetectRemappings(remappings, foundryDir)

	if len(remappings) > 0 {
		config.WriteString("remappings = [\n")
		for _, r := range remappings {
			config.WriteString(fmt.Sprintf("    \"%s\",\n", r))
		}
		config.WriteString("]\n")
	}

	return os.WriteFile(tomlPath, []byte(config.String()), 0644)
}

// extractVersionFromSources 从源文件中提取 solc 版本
func extractVersionFromSources(sources map[string]SourceFile) string {
	versionRe := regexp.MustCompile(`pragma\s+solidity\s+([^;]+);`)
	fullVersionRe := regexp.MustCompile(`(\d+\.\d+\.\d+)`)

	for _, src := range sources {
		matches := versionRe.FindStringSubmatch(src.Content)
		if len(matches) > 1 {
			verMatches := fullVersionRe.FindStringSubmatch(matches[1])
			if len(verMatches) > 1 {
				return verMatches[1]
			}
		}
	}
	return ""
}

func normalizeSolcVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

func autoDetectRemappings(existing []string, rootDir string) []string {
	// Simple mock implementation as we don't scan disk here
	// In real world this would scan node_modules etc.
	// For now we just return what we have plus some defaults if missing

	hasOpenZeppelin := false
	for _, r := range existing {
		if strings.Contains(r, "openzeppelin") {
			hasOpenZeppelin = true
			break
		}
	}

	if !hasOpenZeppelin {
		// existing = append(existing, "@openzeppelin/=node_modules/@openzeppelin/")
	}

	return existing
}
