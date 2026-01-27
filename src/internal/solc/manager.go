package solc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// helloq SolcManager solc 版本管理器
type SolcManager struct {
	mu           sync.RWMutex
	versionCache map[string]string // version -> solc path
	currentVer   string            // 当前 solc-select 激活的版本
	installLocks sync.Map          // version -> *sync.Once，用于确保每个版本只安装一次
}

var (
	defaultManager *SolcManager
	once           sync.Once
)

func GetManager() *SolcManager {
	once.Do(func() {
		defaultManager = &SolcManager{
			versionCache: make(map[string]string),
		}
	})
	return defaultManager
}

// helloq ExtractPragmaVersion 从合约源码中提取 pragma solidity 版本
func ExtractPragmaVersion(source string) string {
	// pragma solidity ^0.8.16; 或 pragma solidity >=0.8.0 <0.9.0;
	re := regexp.MustCompile(`pragma\s+solidity\s+([^;]+);`)
	matches := re.FindAllStringSubmatch(source, -1)

	if len(matches) == 0 {
		return ""
	}

	// 收集所有版本
	versionRe := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	var versions []string

	for _, match := range matches {
		constraint := match[1]
		verMatches := versionRe.FindAllString(constraint, -1)
		versions = append(versions, verMatches...)
	}

	if len(versions) == 0 {
		return ""
	}

	// 选择最高版本（确保能满足所有 pragma 约束）
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions[0]
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	for i := 0; i < 3; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}
		if n1 != n2 {
			return n1 - n2
		}
	}
	return 0
}

// helloq GetSolcPath 获取指定版本的 solc 路径（带缓存）
func (m *SolcManager) GetSolcPath(version string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("version is empty")
	}

	version = normalizeVersion(version)

	// 检查缓存
	m.mu.RLock()
	if path, ok := m.versionCache[version]; ok {
		m.mu.RUnlock()
		if fileExists(path) {
			return path, nil
		}
	} else {
		m.mu.RUnlock()
	}

	// 方法1: 检查 solc-select 安装的版本
	path, err := m.trySolcSelect(version)
	if err == nil && path != "" {
		m.cachePath(version, path)
		return path, nil
	}

	// 方法2: 检查 ~/.solcx 目录（py-solc-x 安装位置）
	path, err = m.trySolcx(version)
	if err == nil && path != "" {
		m.cachePath(version, path)
		return path, nil
	}

	// 方法3: 尝试安装
	path, err = m.installVersion(version)
	if err == nil && path != "" {
		m.cachePath(version, path)
		return path, nil
	}

	return "", fmt.Errorf("failed to get solc %s: %v", version, err)
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	// 移除约束符号
	for _, prefix := range []string{"^", ">=", "<=", ">", "<", "~", "="} {
		version = strings.TrimPrefix(version, prefix)
	}
	return strings.TrimSpace(version)
}

func (m *SolcManager) cachePath(version, path string) {
	m.mu.Lock()
	m.versionCache[version] = path
	m.mu.Unlock()
}

func (m *SolcManager) trySolcSelect(version string) (string, error) {
	// 检查 solc-select 是否可用
	if _, err := exec.LookPath("solc-select"); err != nil {
		return "", err
	}

	// 检查版本是否已安装
	cmd := exec.Command("solc-select", "versions")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	installed := false
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// solc-select versions 输出格式: "0.8.16" 或 "0.8.16 (current)"
		if strings.HasPrefix(line, version) {
			installed = true
			break
		}
	}

	// 如果未安装，尝试安装（使用 sync.Once 避免竞态）
	if !installed {
		// 获取或创建该版本的安装锁
		once, _ := m.installLocks.LoadOrStore(version, &sync.Once{})
		installOnce := once.(*sync.Once)

		var installErr error
		installOnce.Do(func() {
			installCmd := exec.Command("solc-select", "install", version)
			if err := installCmd.Run(); err != nil {
				installErr = fmt.Errorf("solc-select install failed: %v", err)
			}
		})
		if installErr != nil {
			return "", installErr
		}
	}

	// 直接返回 solc-select 安装的二进制文件路径（避免并发切换问题）
	// solc-select 安装路径:
	//   Linux/macOS: ~/.solc-select/artifacts/solc-{version}/solc-{version}
	//   Windows: %USERPROFILE%\.solc-select\artifacts\solc-{version}\solc-{version}.exe
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// 根据操作系统生成可能的路径
	var possiblePaths []string
	solcSelectDir := filepath.Join(homeDir, ".solc-select", "artifacts", fmt.Sprintf("solc-%s", version))

	if runtime.GOOS == "windows" {
		// Windows 路径 (.exe 扩展名)
		possiblePaths = []string{
			filepath.Join(solcSelectDir, fmt.Sprintf("solc-%s.exe", version)),
			filepath.Join(solcSelectDir, "solc.exe"),
		}
	} else {
		// Linux/macOS 路径
		possiblePaths = []string{
			filepath.Join(solcSelectDir, fmt.Sprintf("solc-%s", version)),
			filepath.Join(homeDir, ".solc-select", "artifacts", version, fmt.Sprintf("solc-%s", version)),
		}
	}

	for _, path := range possiblePaths {
		if fileExists(path) && isExecutable(path) {
			return path, nil
		}
	}

	// 回退：使用 solc-select use 切换后获取路径（单线程场景）
	m.mu.Lock()
	defer m.mu.Unlock()

	useCmd := exec.Command("solc-select", "use", version)
	if err := useCmd.Run(); err != nil {
		return "", fmt.Errorf("solc-select use failed: %v", err)
	}
	m.currentVer = version

	solcPath, err := exec.LookPath("solc")
	if err != nil {
		return "", err
	}

	return solcPath, nil
}

func (m *SolcManager) trySolcx(version string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// py-solc-x 安装路径
	solcxDir := filepath.Join(homeDir, ".solcx")

	// 尝试不同的路径格式
	possiblePaths := []string{
		filepath.Join(solcxDir, fmt.Sprintf("solc-v%s", version)),
		filepath.Join(solcxDir, fmt.Sprintf("solc-v%s", version), "solc-v%s", version),
		filepath.Join(solcxDir, fmt.Sprintf("solc-%s", version)),
	}

	// macOS 特殊路径
	if runtime.GOOS == "darwin" {
		possiblePaths = append(possiblePaths,
			filepath.Join(solcxDir, fmt.Sprintf("solc-v%s", version), "bin", "solc"),
		)
	}

	for _, path := range possiblePaths {
		if fileExists(path) && isExecutable(path) {
			return path, nil
		}
	}

	return "", fmt.Errorf("solcx version %s not found", version)
}

func (m *SolcManager) installVersion(version string) (string, error) {
	// 优先使用 solc-select 安装
	if _, err := exec.LookPath("solc-select"); err == nil {
		cmd := exec.Command("solc-select", "install", version)
		if err := cmd.Run(); err == nil {
			return m.trySolcSelect(version)
		}
	}

	return "", fmt.Errorf("failed to install solc %s, please install manually: solc-select install %s", version, version)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Windows 上所有文件都可以执行，只需检查文件存在
	if runtime.GOOS == "windows" {
		return !info.IsDir()
	}
	return info.Mode()&0111 != 0
}

// helloq CompileWithVersion 使用指定版本的 solc 编译
func (m *SolcManager) CompileWithVersion(version, filePath string, args ...string) ([]byte, error) {
	solcPath, err := m.GetSolcPath(version)
	if err != nil {
		return nil, err
	}

	cmdArgs := append(args, filePath)
	cmd := exec.Command(solcPath, cmdArgs...)
	return cmd.CombinedOutput()
}

// helloq GetASTJSON 获取合约的 AST JSON（自动匹配版本）
func GetASTJSON(sourceCode string) ([]byte, string, error) {
	version := ExtractPragmaVersion(sourceCode)
	if version == "" {
		return nil, "", fmt.Errorf("failed to extract Solidity version from source code")
	}

	manager := GetManager()
	solcPath, err := manager.GetSolcPath(version)
	if err != nil {
		return nil, "", err
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "*.sol")
	if err != nil {
		return nil, "", err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(sourceCode); err != nil {
		return nil, "", err
	}
	tmpFile.Close()

	// 执行 solc --ast-compact-json
	cmd := exec.Command(solcPath, "--ast-compact-json", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("solc execution failed: %v, output: %s", err, string(output))
	}

	return output, version, nil
}
