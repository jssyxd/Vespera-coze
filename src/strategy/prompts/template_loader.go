package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadTemplate(mode, strategy string) (string, error) {
	if mode == "mode1" && (strategy == "" || strategy == "default" || strategy == "all") {
		strategy = "generic_scan"
	}
	if mode != "mode1" && (strategy == "" || strategy == "default") {
		return LoadDefaultTemplate(mode)
	}

	// 构建模板文件路径，支持从项目根目录或src目录运行
	templatePath := filepath.Join("strategy", "prompts", mode, strategy+".tmpl")

	// 首先尝试从当前目录加载
	content, err := os.ReadFile(templatePath)
	if err != nil {
		// 如果失败，尝试从src目录加载
		srcPath := filepath.Join("src", "strategy", "prompts", mode, strategy+".tmpl")
		content, err = os.ReadFile(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to load template %s or %s: %w", templatePath, srcPath, err)
		}
	}

	return string(content), nil
}

func LoadDefaultTemplate(mode string) (string, error) {
	// 构建默认模板文件路径
	templatePath := filepath.Join("strategy", "prompts", mode, "default.tmpl")

	// 首先尝试从当前目录加载
	content, err := os.ReadFile(templatePath)
	if err != nil {
		// 如果失败，尝试从src目录加载
		srcPath := filepath.Join("src", "strategy", "prompts", mode, "default.tmpl")
		content, err = os.ReadFile(srcPath)
		if err != nil {
			return "", fmt.Errorf("failed to load default template %s or %s: %w", templatePath, srcPath, err)
		}
	}

	return string(content), nil
}

func LoadInputFile(inputFile string) (string, error) {
	if inputFile == "" {
		return "", nil
	}

	// 如果输入的是文件名（不包含路径），则在默认目录中查找
	if !strings.Contains(inputFile, "/") && !strings.Contains(inputFile, "\\") {
		// 首先尝试从当前目录加载
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			// 尝试从 strategy/exp_libs/mode1/ 目录加载 (部署环境)
			deployPath := filepath.Join("strategy", "exp_libs", "mode1", inputFile)
			if _, err := os.Stat(deployPath); err == nil {
				inputFile = deployPath
			} else {
				deployTomlPath := filepath.Join("strategy", "exp_libs", "mode1", inputFile+".toml")
				if _, err := os.Stat(deployTomlPath); err == nil {
					inputFile = deployTomlPath
				} else {
					// 如果失败，尝试从 src/strategy/exp_libs/mode1/ 目录加载 (开发环境)
					// 先尝试不带扩展名的文件
					defaultPath := filepath.Join("src", "strategy", "exp_libs", "mode1", inputFile)
					if _, err := os.Stat(defaultPath); err == nil {
						inputFile = defaultPath
					} else {
						// 如果还是找不到，尝试添加 .toml 扩展名
						tomlPath := filepath.Join("src", "strategy", "exp_libs", "mode1", inputFile+".toml")
						if _, err := os.Stat(tomlPath); err == nil {
							inputFile = tomlPath
						}
					}
				}
			}
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return "", fmt.Errorf("input file not found: %s", inputFile)
	}

	// 读取文件内容
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("failed to load input file %s: %w", inputFile, err)
	}

	// 根据文件扩展名处理不同格式
	ext := filepath.Ext(inputFile)
	switch ext {
	case ".toml":
		processedContent := processTOMLFile(string(content))
		return processedContent, nil
	case ".sol":
		processedContent := processMarkers(string(content))
		return processedContent, nil
	default:
		// 默认按TOML处理
		processedContent := processTOMLFile(string(content))
		return processedContent, nil
	}
}

func processTOMLFile(content string) string {
	var result strings.Builder

	// 查找[漏洞合约源码]部分
	vulnStart := strings.Index(content, "[漏洞合约源码]")
	if vulnStart != -1 {
		// 查找code = """开始位置
		codeStart := strings.Index(content[vulnStart:], "code = \"\"\"")
		if codeStart != -1 {
			codeStart += vulnStart + len("code = \"\"\"")
			// 查找结束的"""
			codeEnd := strings.Index(content[codeStart:], "\"\"\"")
			if codeEnd != -1 {
				vulnCode := strings.TrimSpace(content[codeStart : codeStart+codeEnd])
				result.WriteString("[漏洞合约源码]\n")
				result.WriteString(vulnCode)
				result.WriteString("\n\n")
			}
		}
	}

	// 查找[漏洞描述]部分
	descStart := strings.Index(content, "[漏洞描述]")
	if descStart != -1 {
		// 查找code = """开始位置
		codeStart := strings.Index(content[descStart:], "code = \"\"\"")
		if codeStart != -1 {
			codeStart += descStart + len("code = \"\"\"")
			// 查找结束的"""
			codeEnd := strings.Index(content[codeStart:], "\"\"\"")
			if codeEnd != -1 {
				descCode := strings.TrimSpace(content[codeStart : codeStart+codeEnd])
				result.WriteString("[漏洞描述]\n")
				result.WriteString(descCode)
				result.WriteString("\n\n")
			}
		}
	}

	// 查找[Foundry复现代码]部分
	foundryStart := strings.Index(content, "[Foundry复现代码]")
	if foundryStart != -1 {
		// 查找code = """开始位置
		codeStart := strings.Index(content[foundryStart:], "code = \"\"\"")
		if codeStart != -1 {
			codeStart += foundryStart + len("code = \"\"\"")
			// 查找结束的"""
			codeEnd := strings.Index(content[codeStart:], "\"\"\"")
			if codeEnd != -1 {
				foundryCode := strings.TrimSpace(content[codeStart : codeStart+codeEnd])
				result.WriteString("[Foundry复现代码]\n")
				result.WriteString(foundryCode)
				result.WriteString("\n")
			}
		}
	}

	// 如果没有找到TOML结构，直接返回原内容
	if result.Len() == 0 {
		return content
	}

	return result.String()
}

func processMarkers(content string) string {
	// 对于.sol文件，直接返回原内容，不再处理旧格式的标记
	return content
}
