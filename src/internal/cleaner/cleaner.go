package cleaner

import (
	"regexp"
	"strings"

	"github.com/VectorBits/Vespera/src/internal/solc"
)

var LibraryPatterns = []string{
	"@openzeppelin",
	"node_modules",
	"lib/openzeppelin",
	"lib/solmate",
	"lib/forge-std",
	"test/",
	"mock/",
}

var TokenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^ERC\d{2,}.*\.sol$`),
	regexp.MustCompile(`(?i)^BEP\d{2,}.*\.sol$`),
	regexp.MustCompile(`(?i)^I?ERC20\.sol$`),
}

// CleanCode 识别并移除外部库代码
// useFlatten: 如果为 true，则尝试使用 forge flatten 进行扁平化，保留所有代码
func CleanCode(code string, useFlatten bool) string {
	// 如果启用扁平化且是 JSON 格式，尝试 Flatten
	if useFlatten && solc.IsJSONSource(code) {
		if flattened, err := solc.FlattenJSONSource(code); err == nil {
			return flattened
		}
		// 如果失败，回退到普通逻辑
	}

	// 保留模式：只注释掉库文件，而不是删除 (用户需求)
	re := regexp.MustCompile(`(?m)^//\s*File:?\s+(.*)$`)
	indexes := re.FindAllStringIndex(code, -1)
	matches := re.FindAllStringSubmatch(code, -1)

	if len(indexes) == 0 {
		return code
	}

	var sb strings.Builder

	for i, match := range matches {
		start := indexes[i][0]

		if i == 0 && start > 0 {
			sb.WriteString(code[:start])
		}

		var end int
		if i < len(indexes)-1 {
			end = indexes[i+1][0]
		} else {
			end = len(code)
		}

		filePath := strings.TrimSpace(match[1])

		if isLibrary(filePath) {
			// 如果启用扁平化，我们不应该进入这里，因为 Flatten 应该已经处理好了
			// 但如果是普通单文件代码，我们依然执行清洗
			if useFlatten {
				// Flatten 模式下，保留库代码
				sb.WriteString(code[start:end])
			} else {
				// 旧模式：注释掉库代码而不是删除
				sb.WriteString(code[start:indexes[i][1]]) // Write File header
				sb.WriteString("\n/* --- External Library Code Commented Out (Saved Tokens) ---\n")
				// 简单的注释处理可能遇到 */ 嵌套问题，这里简单处理
				content := code[indexes[i][1]:end]
				content = strings.ReplaceAll(content, "*/", "* /")
				sb.WriteString(content)
				sb.WriteString("\n*/\n\n")
			}
		} else {
			sb.WriteString(code[start:end])
		}
	}

	return sb.String()
}

func isLibrary(path string) bool {
	pathLower := strings.ToLower(path)
	for _, pattern := range LibraryPatterns {
		if strings.Contains(pathLower, pattern) {
			return true
		}
	}

	parts := strings.Split(path, "/")
	fileName := parts[len(parts)-1]

	for _, pattern := range TokenPatterns {
		if pattern.MatchString(fileName) {
			return true
		}
	}

	return false
}
