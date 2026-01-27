package astparser

import (
	"fmt"
	"strings"
)

type CallGraph struct {
	EntryFunction *Node
	Dependencies  map[int]*Node
}

func (ps *ParsedSource) BuildContextForFunction(contractName, functionName string) (string, error) {
	var targetFunc *Node
	var targetContract *Node

	for _, node := range ps.AST.Nodes {
		if node.NodeType == "ContractDefinition" && node.Name == contractName {
			targetContract = &node
			for _, subNode := range node.Nodes {
				if subNode.NodeType == "FunctionDefinition" && subNode.Name == functionName {
					if subNode.Implemented {
						targetFunc = &subNode
						break
					}
				}
			}
		}
	}

	if targetFunc == nil {
		return "", fmt.Errorf("function %s.%s not found or not implemented", contractName, functionName)
	}

	deps := make(map[int]*Node)
	ps.collectDependencies(targetFunc, deps)

	var sb strings.Builder

	contractSrc := ps.GetSourceRange(targetContract.Src)
	contractBodyStart := strings.Index(contractSrc, "{")
	if contractBodyStart != -1 {
		sb.WriteString(contractSrc[:contractBodyStart+1] + "\n")
	} else {
		sb.WriteString(fmt.Sprintf("contract %s {\n", targetContract.Name))
	}

	for _, node := range targetContract.Nodes {
		if node.NodeType != "FunctionDefinition" {
			sb.WriteString(ps.GetSourceRange(node.Src) + "\n")
		}
	}

	sb.WriteString("\n    // --- Target Function ---\n")
	sb.WriteString(ps.GetSourceRange(targetFunc.Src) + "\n")

	if len(deps) > 0 {
		sb.WriteString("\n    // --- Dependencies ---\n")
		for _, node := range deps {
			if node.ID != targetFunc.ID {
				sb.WriteString(ps.GetSourceRange(node.Src) + "\n")
			}
		}
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

func (ps *ParsedSource) collectDependencies(node *Node, visited map[int]*Node) {
	if node.Body != nil {
		ps.collectDependencies(node.Body, visited)
	}
	if node.Expression != nil {
		ps.collectDependencies(node.Expression, visited)
	}

	for i := range node.Nodes {
		ps.collectDependencies(&node.Nodes[i], visited)
	}
	for i := range node.Statements {
		ps.collectDependencies(&node.Statements[i], visited)
	}
	for i := range node.Arguments {
		ps.collectDependencies(&node.Arguments[i], visited)
	}
	for i := range node.Modifiers {
		ps.collectDependencies(&node.Modifiers[i], visited)
	}

	checkReference(node, ps, visited)
}

// PruneDeadCode 死代码消除 - 基于入口函数递归收集依赖
// conservative: 如果为 true，则保留所有状态变量、修饰符、事件和错误定义，以及它们引用的内容
func (ps *ParsedSource) PruneDeadCode(mainContractName string, conservative bool) (string, error) {
	var mainContract *Node
	if mainContractName == "" {
		for i := len(ps.AST.Nodes) - 1; i >= 0; i-- {
			if ps.AST.Nodes[i].NodeType == "ContractDefinition" {
				mainContract = &ps.AST.Nodes[i]
				break
			}
		}
	} else {
		for i := range ps.AST.Nodes {
			if ps.AST.Nodes[i].NodeType == "ContractDefinition" && ps.AST.Nodes[i].Name == mainContractName {
				mainContract = &ps.AST.Nodes[i]
				break
			}
		}
	}

	if mainContract == nil {
		return "", fmt.Errorf("main contract not found")
	}

	// 入口函数识别: public/external + constructor/fallback/receive
	visited := make(map[int]*Node)

	for i := range mainContract.Nodes {
		node := &mainContract.Nodes[i]

		// 策略 1: 收集入口函数
		if node.NodeType == "FunctionDefinition" {
			isEntry := false
			if node.Kind == "constructor" || node.Kind == "fallback" || node.Kind == "receive" {
				isEntry = true
			} else if node.Visibility == "public" || node.Visibility == "external" {
				isEntry = true
			}

			if isEntry && node.Implemented {
				visited[node.ID] = node
				ps.collectDependencies(node, visited)
			}
		}

		// 策略 2: 保守模式下，保留关键定义
		if conservative {
			shouldKeep := false
			switch node.NodeType {
			case "VariableDeclaration": // 状态变量
				shouldKeep = true
			case "ModifierDefinition":
				shouldKeep = true
			case "EventDefinition":
				shouldKeep = true
			case "ErrorDefinition":
				shouldKeep = true
			case "UsingForDirective":
				shouldKeep = true
			}

			if shouldKeep {
				visited[node.ID] = node
				ps.collectDependencies(node, visited)
			}
		}
	}

	var sb strings.Builder

	for _, rootNode := range ps.AST.Nodes {
		if rootNode.NodeType == "PragmaDirective" || rootNode.NodeType == "ImportDirective" {
			sb.WriteString(ps.GetSourceRange(rootNode.Src) + "\n")
			continue
		}

		if rootNode.NodeType == "ContractDefinition" {
			contractSrc := ps.GetSourceRange(rootNode.Src)
			contractBodyStart := strings.Index(contractSrc, "{")
			if contractBodyStart == -1 {
				continue
			}

			sb.WriteString("\n" + contractSrc[:contractBodyStart+1] + "\n")

			// 遍历合约子节点，只输出被标记为 visited 的节点
			for _, child := range rootNode.Nodes {
				if _, exists := visited[child.ID]; exists {
					sb.WriteString(ps.GetSourceRange(child.Src) + "\n")
				} else if child.NodeType != "FunctionDefinition" && !conservative {
					// 非保守模式下，保留非函数节点（旧逻辑可能是这样，但为了安全起见，这里严格按照 visited）
					// 旧逻辑: } else { sb.WriteString(ps.GetSourceRange(child.Src) + "\n") }
					// 我们现在统一使用 visited，但为了兼容旧逻辑的效果：
					// 旧逻辑其实保留了所有非 FunctionDefinition。
					// 所以这里我们应该：
					// 如果是 FunctionDefinition -> 必须在 visited 中
					// 如果不是 FunctionDefinition -> 在 conservative=false 时默认保留？
					// 让我们看旧代码：
					// if child.NodeType == "FunctionDefinition" { if exists... } else { write... }
					// 这意味着旧逻辑确实保留了所有变量和事件。
					// 那么 conservative 的区别是什么？
					// 区别在于：collectDependencies 是否会去追踪变量引用的函数！

					// 修正：旧逻辑其实已经保留了变量声明的代码，但没有追踪变量引用的其他内容（如结构体定义、枚举等）。
					// 并且 checkReference 只追踪 FunctionDefinition。
					// 所以我们需要修改 checkReference。
					sb.WriteString(ps.GetSourceRange(child.Src) + "\n")
				}
			}

			sb.WriteString("}\n")
		} else {
			// 根级别的其他节点（如库定义、接口定义）
			// 这里如果 conservative=true，应该也保留相关的库
			if conservative {
				// 简单起见，根节点全部保留（或者是被引用的）
				// 更好的做法是：如果根节点是 ContractDefinition/Library，检查是否被 visited 引用
				// 但这比较复杂，目前先保留所有根节点
				sb.WriteString(ps.GetSourceRange(rootNode.Src) + "\n")
			} else {
				sb.WriteString(ps.GetSourceRange(rootNode.Src) + "\n")
			}
		}
	}

	return sb.String(), nil
}

func checkReference(node *Node, ps *ParsedSource, visited map[int]*Node) {
	if node.ReferencedDeclaration != 0 {
		refID := node.ReferencedDeclaration
		if _, exists := visited[refID]; !exists {
			if refNode, ok := ps.NodesByID[refID]; ok {
				// 修改：不再限制只追踪 FunctionDefinition
				// 只要是被引用的节点，都应该保留并递归分析依赖
				visited[refID] = refNode
				ps.collectDependencies(refNode, visited)
			}
		}
	}
}
