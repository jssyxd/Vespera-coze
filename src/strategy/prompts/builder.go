package prompts

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"text/template"
)

// PromptVariables 定义 Prompt 模板可用的所有变量
// qhello 可以给提示词用的
type PromptVariables struct {
	// 基础变量
	ContractAddress  string
	ContractCode     string
	Strategy         string
	InputFileContent string

	// 调用图增强变量
	EnableCallGraph bool   // 是否启用调用图分析
	CallGraphInfo   string // 调用图概要信息
	CallersCode     string // 调用者代码（谁调用了这些函数）
	CalleesCode     string // 被调用者代码（这些函数调用了谁）
	EnrichedContext string // 完整的增强上下文（包含调用链）

	// 统计信息
	TotalFunctions    int // 函数总数
	PublicFunctions   int // 公开函数数量
	InternalFunctions int // 内部函数数量
}

var (
	templateCacheMu sync.Mutex
	templateCache   = map[string]*template.Template{}
)

func templateKey(templateContent string) string {
	sum := sha256.Sum256([]byte(templateContent))
	return hex.EncodeToString(sum[:])
}

// BuildPrompt 使用模板和变量构建 Prompt
// 支持 map[string]string, map[string]interface{}, 或 PromptVariables
func BuildPrompt(templateContent string, variables interface{}) string {
	key := templateKey(templateContent)
	templateCacheMu.Lock()
	tmpl := templateCache[key]
	templateCacheMu.Unlock()

	if tmpl == nil {
		parsed, err := template.New("prompt").Parse(templateContent)
		if err != nil {
			return fmt.Sprintf("failed to parse template: %v\nRaw Template:\n%s", err, templateContent)
		}

		templateCacheMu.Lock()
		if templateCache[key] == nil {
			if len(templateCache) >= 64 {
				templateCache = map[string]*template.Template{}
			}
			templateCache[key] = parsed
			tmpl = parsed
		} else {
			tmpl = templateCache[key]
		}
		templateCacheMu.Unlock()
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, variables); err != nil {
		return fmt.Sprintf("failed to execute template: %v\nRaw Template:\n%s", err, templateContent)
	}

	return result.String()
}

// NewPromptVariables 创建带默认值的 PromptVariables
func NewPromptVariables() *PromptVariables {
	return &PromptVariables{
		EnableCallGraph: false,
	}
}

// ToMap 将 PromptVariables 转换为 map[string]interface{}
func (pv *PromptVariables) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"ContractAddress":   pv.ContractAddress,
		"ContractCode":      pv.ContractCode,
		"Strategy":          pv.Strategy,
		"InputFileContent":  pv.InputFileContent,
		"EnableCallGraph":   pv.EnableCallGraph,
		"CallGraphInfo":     pv.CallGraphInfo,
		"CallersCode":       pv.CallersCode,
		"CalleesCode":       pv.CalleesCode,
		"EnrichedContext":   pv.EnrichedContext,
		"TotalFunctions":    pv.TotalFunctions,
		"PublicFunctions":   pv.PublicFunctions,
		"InternalFunctions": pv.InternalFunctions,
	}
}
