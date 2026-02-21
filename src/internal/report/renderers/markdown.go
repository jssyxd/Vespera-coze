package renderers

import (
	"fmt"
	"strings"
)

type MarkdownRenderer struct{}

func NewMarkdownRenderer() *MarkdownRenderer {
	return &MarkdownRenderer{}
}

func (r *MarkdownRenderer) RenderVulnerability(vulnType, severity, description string) string {
	icon := getSeverityIcon(severity)
	// 移除 description 中的 "Slither检测: ...\nAI验证: ..." 等冗余前缀，
	// 假设 description 已经是格式化好的（或者在这里进行格式化）
	// 但考虑到 Mode 2 的 description 可能已经是多行的，直接输出即可
	// 只是可以考虑加粗一些关键字
	formattedDesc := strings.ReplaceAll(description, "Slither Detection:", "**Slither Detection**:")
	formattedDesc = strings.ReplaceAll(formattedDesc, "AI Verification:", "**AI Verification**:")
	formattedDesc = strings.ReplaceAll(formattedDesc, "Analysis:", "**Analysis**:")

	return fmt.Sprintf("### %s %s\n\n**类型**: %s\n\n%s", icon, severity, vulnType, formattedDesc)
}

func (r *MarkdownRenderer) RenderScanResult(address, status, summary, rawResponse string, vulnerabilities []string) string {
	var result strings.Builder

	// 合约地址作为一级标题
	result.WriteString(fmt.Sprintf("# 📄 Contract Address: `%s`\n\n", address))
	result.WriteString(fmt.Sprintf("**Status**: %s\n\n", status))

	// 漏洞详情（Mode 2 重点）
	if len(vulnerabilities) > 0 {
		result.WriteString("## 🛡️ Vulnerability Details\n\n")
		for _, vuln := range vulnerabilities {
			result.WriteString(fmt.Sprintf("%s\n\n---\n\n", vuln))
		}
	} else {
		result.WriteString("## ✅ No confirmed vulnerabilities found\n\n")
	}

	// AI分析摘要 (可选，如果觉得乱可以放到最后或折叠)
	if summary != "" {
		result.WriteString("## 📊 Scan Statistics\n\n")
		// 简单的处理 summary 格式，使其更易读
		formattedSummary := strings.ReplaceAll(summary, "|", "\n- ")
		if !strings.HasPrefix(formattedSummary, "-") && strings.Contains(formattedSummary, "\n-") {
			formattedSummary = "- " + formattedSummary
		}
		result.WriteString(fmt.Sprintf("%s\n\n", formattedSummary))
	}

	// 原始AI响应 (默认折叠，减少干扰)
	if rawResponse != "" {
		result.WriteString("<details>\n<summary>🔍 Click to view raw AI response</summary>\n\n")
		result.WriteString(fmt.Sprintf("```json\n%s\n```\n\n", rawResponse))
		result.WriteString("</details>\n\n")
	}

	return result.String()
}

func getSeverityIcon(severity string) string {
	switch severity {
	case "Critical":
		return "🔴"
	case "High":
		return "🟠"
	case "Medium":
		return "🟡"
	case "Low":
		return "🟢"
	default:
		return "⚪"
	}
}
