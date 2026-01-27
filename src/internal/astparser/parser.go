package astparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/VectorBits/Vespera/src/internal/solc"
)

// ParseFile 调用 solc 解析 AST（自动匹配版本）
func ParseFile(filePath string) (*ParsedSource, error) {
	sourceBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	sourceCode := string(sourceBytes)

	// 提取 pragma 版本并获取对应 solc
	version := solc.ExtractPragmaVersion(sourceCode)
	var cmd *exec.Cmd

	if version != "" {
		manager := solc.GetManager()
		solcPath, err := manager.GetSolcPath(version)
		if err == nil {
			cmd = exec.Command(solcPath, "--ast-compact-json", filePath)
		}
	}

	// 回退到系统默认 solc
	if cmd == nil {
		cmd = exec.Command("solc", "--ast-compact-json", filePath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("solc 执行失败: %w, stderr: %s", err, stderr.String())
	}

	return parseASTOutput(stdout.Bytes(), sourceCode)
}

// ParseSource 解析源代码字符串
func ParseSource(sourceCode string) (*ParsedSource, error) {
	// 如果是多文件 JSON 格式，使用 Foundry 扁平化
	if solc.IsJSONSource(sourceCode) {
		flattened, err := solc.FlattenJSONSource(sourceCode)
		if err != nil {
			return nil, fmt.Errorf("foundry flatten failed: %w", err)
		}
		sourceCode = flattened
	}

	// 写入临时文件并解析
	tmpFile, err := os.CreateTemp("", "*.sol")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(sourceCode); err != nil {
		return nil, err
	}
	tmpFile.Close()

	return ParseFile(tmpFile.Name())
}

// normalizeSolcVersion 规范化 solc 版本
func normalizeSolcVersion(version string) string {
	// 移除版本前缀
	for _, prefix := range []string{"^", ">=", "<=", ">", "<", "~", "="} {
		version = strings.TrimPrefix(version, prefix)
	}
	version = strings.TrimSpace(version)

	// Forge 不支持 0.9.x，使用 0.8.26 替代
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])
		if major == 0 && minor >= 9 {
			return "0.8.26"
		}
	}
	return version
}

// autoDetectRemappings 自动检测常见的 remappings
func autoDetectRemappings(remappings []string, baseDir string) []string {
	commonPrefixes := []string{"@openzeppelin", "@chainlink", "@uniswap"}

	for _, prefix := range commonPrefixes {
		hasMapping := false
		for _, r := range remappings {
			if strings.HasPrefix(r, prefix) {
				hasMapping = true
				break
			}
		}
		if !hasMapping {
			if _, err := os.Stat(filepath.Join(baseDir, prefix)); err == nil {
				remappings = append(remappings, fmt.Sprintf("%s/=%s/", prefix, prefix))
			}
		}
	}
	return remappings
}

// parseASTOutput 解析 solc 输出的 AST JSON
func parseASTOutput(output []byte, sourceCode string) (*ParsedSource, error) {
	jsonStart := strings.Index(string(output), "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("无法在 solc 输出中找到 JSON")
	}
	jsonContent := output[jsonStart:]

	var ast AST
	if err := json.Unmarshal(jsonContent, &ast); err != nil {
		lines := strings.Split(string(output), "\n")
		found := false
		for _, line := range lines {
			if strings.HasPrefix(line, "{\"absolutePath\"") {
				if err := json.Unmarshal([]byte(line), &ast); err == nil {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("解析 AST JSON 失败: %w", err)
		}
	}

	ps := &ParsedSource{
		AST:        &ast,
		SourceCode: sourceCode,
		NodesByID:  make(map[int]*Node),
	}
	ps.indexNodes(ast.Nodes)

	return ps, nil
}

// IsJSONSource 检查源代码是否为多文件 JSON 格式
func IsJSONSource(source string) bool {
	return solc.IsJSONSource(source)
}

// FlattenJSONSource 尝试扁平化多文件 JSON 格式代码
// 返回扁平化后的代码，如果失败则返回原始代码
func FlattenJSONSource(source string) (string, error) {
	// 代理给 solc 包
	return solc.FlattenJSONSource(source)
}

func (ps *ParsedSource) indexNodes(nodes []Node) {
	for i := range nodes {
		node := &nodes[i]
		ps.NodesByID[node.ID] = node

		if len(node.Nodes) > 0 {
			ps.indexNodes(node.Nodes)
		}
		if node.Body != nil {
			ps.indexNodes([]Node{*node.Body})
		}
		if len(node.Statements) > 0 {
			ps.indexNodes(node.Statements)
		}
		if node.Expression != nil {
			ps.indexNodes([]Node{*node.Expression})
		}
		if len(node.Arguments) > 0 {
			ps.indexNodes(node.Arguments)
		}
	}
}

func (ps *ParsedSource) GetSourceRange(src string) string {
	parts := strings.Split(src, ":")
	if len(parts) < 2 {
		return ""
	}
	offset, err1 := strconv.Atoi(parts[0])
	length, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return ""
	}

	if offset < 0 || offset >= len(ps.SourceCode) || offset+length > len(ps.SourceCode) {
		return ""
	}
	return ps.SourceCode[offset : offset+length]
}
