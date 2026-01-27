package backend

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

//go:embed slither_wrapper.py
var slitherScriptContent []byte

// extractMutex ensures we only extract the script once per process execution
var extractMutex sync.Once
var extractedScriptPath string
var extractErr error

// PythonScriptBackend 实现 Python 脚本后端
type PythonScriptBackend struct {
	scriptPath string
	pythonPath string
	timeout    time.Duration
}

// NewPythonScriptBackend 创建 Python 脚本后端
// 自动将嵌入的 Python 脚本释放到临时目录
func NewPythonScriptBackend(pythonPath string) *PythonScriptBackend {
	// 确保脚本已提取
	path, err := ensureScriptExtracted()
	if err != nil {
		// 这里如果不 panic，就得修改接口返回 error。
		// 为了保持兼容性，我们打印日志，但实际 Execute 时会失败。
		// 更好的做法是让 New 返回 (*PythonScriptBackend, error)，但这里我们先 log。
		fmt.Fprintf(os.Stderr, "CRITICAL: Failed to extract embedded script: %v\n", err)
	}

	if pythonPath == "" {
		pythonPath = "python3"
	}

	return &PythonScriptBackend{
		scriptPath: path,
		pythonPath: pythonPath,
		timeout:    120 * time.Second,
	}
}

// ensureScriptExtracted 将嵌入的脚本写入临时文件
func ensureScriptExtracted() (string, error) {
	extractMutex.Do(func() {
		// 1. 创建临时文件
		// 使用 pattern 确保文件名唯一且有 .py 后缀
		tmpFile, err := os.CreateTemp(os.TempDir(), "slither_wrapper_*.py")
		if err != nil {
			extractErr = fmt.Errorf("failed to create temp file: %w", err)
			return
		}

		// 2. 写入内容
		if _, err := tmpFile.Write(slitherScriptContent); err != nil {
			_ = tmpFile.Close()
			extractErr = fmt.Errorf("failed to write script content: %w", err)
			return
		}

		// 3. 关闭文件
		if err := tmpFile.Close(); err != nil {
			extractErr = fmt.Errorf("failed to close temp file: %w", err)
			return
		}

		// 4. 赋予执行权限 (可选，视系统而定)
		_ = os.Chmod(tmpFile.Name(), 0755)

		extractedScriptPath = tmpFile.Name()

		// 5. 注册清理 (Best Effort)
		// 注意：os.Remove 在主程序退出时可能不会执行，但这是临时目录，OS 会定期清理
		// 如果需要严格清理，可以在 main 中 defer cleanup
	})

	return extractedScriptPath, extractErr
}

// AnalyzeContract 分析合约代码
func (b *PythonScriptBackend) AnalyzeContract(ctx context.Context, code string, config interface{}) (interface{}, error) {
	if b.scriptPath == "" {
		return nil, fmt.Errorf("python script not available (extraction failed)")
	}

	// 构建输入
	configMap := map[string]interface{}{}
	if cfg, ok := config.(map[string]interface{}); ok {
		configMap = cfg
	} else {
		// 默认配置
		configMap = map[string]interface{}{
			"contract_name": "",
			"solc_version":  "",
			"address":       "",
			"optimization":  false,
			"via_ir":        false,
		}
	}

	input := map[string]interface{}{
		"code":   code,
		"config": configMap,
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input failed: %w", err)
	}

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	// 执行 Python 脚本
	cmd := exec.CommandContext(ctx, b.pythonPath, b.scriptPath)
	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = stdout.String()
		}
		if len(errMsg) > 4096 {
			errMsg = errMsg[:4096] + "...(truncated)"
		}
		return nil, fmt.Errorf("python script execution failed: %w, stderr: %s", err, errMsg)
	}

	// 解析输出
	output := stdout.Bytes()
	var response struct {
		Success bool                   `json:"success"`
		Result  map[string]interface{} `json:"result"`
		Error   string                 `json:"error"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		stderrMsg := stderr.String()
		if len(stderrMsg) > 4096 {
			stderrMsg = stderrMsg[:4096] + "...(truncated)"
		}
		if stderrMsg != "" {
			return nil, fmt.Errorf("parse output failed: %w, stderr: %s", err, stderrMsg)
		}
		return nil, fmt.Errorf("parse output failed: %w, output: %s", err, string(output))
	}

	if !response.Success {
		return nil, fmt.Errorf("python script error: %s", response.Error)
	}

	// Log debug info from stderr if needed
	if stderrMsg := stderr.String(); stderrMsg != "" && strings.Contains(stderrMsg, "[DEBUG]") {
		// Ideally utilize a proper logger here
		// fmt.Printf("[Backend Debug] %s\n", stderrMsg)
	}

	return response.Result, nil
}

// Close 目前不需要关闭操作
func (b *PythonScriptBackend) Close() error {
	return nil
}
