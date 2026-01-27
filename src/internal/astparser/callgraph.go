package astparser

import (
	"fmt"
	"strings"
)

// FunctionRef 函数引用，用于标识跨合约的函数
type FunctionRef struct {
	ContractName string
	FunctionName string
	NodeID       int
}

// CallGraphFull 完整的双向调用图
type CallGraphFull struct {
	// 函数 -> 它调用的函数列表 (向下追踪)
	Callees map[int][]int
	// 函数 -> 调用它的函数列表 (向上追踪)
	Callers map[int][]int
	// 所有函数节点
	Functions map[int]*Node
	// 函数 ID -> 函数引用信息
	FunctionRefs map[int]*FunctionRef
	// 源码
	ps *ParsedSource
}

// BuildCallGraph 构建完整的双向调用图
func (ps *ParsedSource) BuildCallGraph() *CallGraphFull {
	cg := &CallGraphFull{
		Callees:      make(map[int][]int),
		Callers:      make(map[int][]int),
		Functions:    make(map[int]*Node),
		FunctionRefs: make(map[int]*FunctionRef),
		ps:           ps,
	}

	// 第一遍：收集所有函数定义
	for _, node := range ps.AST.Nodes {
		if node.NodeType == "ContractDefinition" {
			for i := range node.Nodes {
				subNode := &node.Nodes[i]
				if subNode.NodeType == "FunctionDefinition" && subNode.Implemented {
					cg.Functions[subNode.ID] = subNode
					cg.FunctionRefs[subNode.ID] = &FunctionRef{
						ContractName: node.Name,
						FunctionName: subNode.Name,
						NodeID:       subNode.ID,
					}
				}
			}
		}
	}

	// 第二遍：分析调用关系
	for funcID, funcNode := range cg.Functions {
		callees := cg.extractCallees(funcNode)
		cg.Callees[funcID] = callees

		// 建立反向索引 (callers)
		for _, calleeID := range callees {
			cg.Callers[calleeID] = append(cg.Callers[calleeID], funcID)
		}
	}

	return cg
}

// extractCallees 从函数体中提取所有被调用的函数
func (cg *CallGraphFull) extractCallees(node *Node) []int {
	callees := make([]int, 0)
	visited := make(map[int]bool)
	cg.extractCalleesRecursive(node, &callees, visited)
	return callees
}

func (cg *CallGraphFull) extractCalleesRecursive(node *Node, callees *[]int, visited map[int]bool) {
	if node == nil {
		return
	}

	// 检查函数调用引用
	if node.ReferencedDeclaration != 0 {
		refID := node.ReferencedDeclaration
		if _, isFunc := cg.Functions[refID]; isFunc {
			if !visited[refID] {
				visited[refID] = true
				*callees = append(*callees, refID)
			}
		}
	}

	// 递归遍历子节点
	if node.Body != nil {
		cg.extractCalleesRecursive(node.Body, callees, visited)
	}
	if node.Expression != nil {
		cg.extractCalleesRecursive(node.Expression, callees, visited)
	}
	for i := range node.Nodes {
		cg.extractCalleesRecursive(&node.Nodes[i], callees, visited)
	}
	for i := range node.Statements {
		cg.extractCalleesRecursive(&node.Statements[i], callees, visited)
	}
	for i := range node.Arguments {
		cg.extractCalleesRecursive(&node.Arguments[i], callees, visited)
	}
	for i := range node.Modifiers {
		cg.extractCalleesRecursive(&node.Modifiers[i], callees, visited)
	}
}

// GetCallers 获取调用指定函数的所有函数 (向上追踪)
func (cg *CallGraphFull) GetCallers(funcID int) []*Node {
	result := make([]*Node, 0)
	if callerIDs, ok := cg.Callers[funcID]; ok {
		for _, id := range callerIDs {
			if node, exists := cg.Functions[id]; exists {
				result = append(result, node)
			}
		}
	}
	return result
}

// GetCallees 获取指定函数调用的所有函数 (向下追踪)
func (cg *CallGraphFull) GetCallees(funcID int) []*Node {
	result := make([]*Node, 0)
	if calleeIDs, ok := cg.Callees[funcID]; ok {
		for _, id := range calleeIDs {
			if node, exists := cg.Functions[id]; exists {
				result = append(result, node)
			}
		}
	}
	return result
}

// GetCallersRecursive 递归获取所有调用者（包括间接调用者）
func (cg *CallGraphFull) GetCallersRecursive(funcID int, maxDepth int) []*Node {
	result := make([]*Node, 0)
	visited := make(map[int]bool)
	cg.getCallersRecursiveHelper(funcID, &result, visited, 0, maxDepth)
	return result
}

func (cg *CallGraphFull) getCallersRecursiveHelper(funcID int, result *[]*Node, visited map[int]bool, depth, maxDepth int) {
	if depth >= maxDepth {
		return
	}
	if callerIDs, ok := cg.Callers[funcID]; ok {
		for _, callerID := range callerIDs {
			if !visited[callerID] {
				visited[callerID] = true
				if node, exists := cg.Functions[callerID]; exists {
					*result = append(*result, node)
					cg.getCallersRecursiveHelper(callerID, result, visited, depth+1, maxDepth)
				}
			}
		}
	}
}

// GetCalleesRecursive 递归获取所有被调用者（包括间接被调用者）
func (cg *CallGraphFull) GetCalleesRecursive(funcID int, maxDepth int) []*Node {
	result := make([]*Node, 0)
	visited := make(map[int]bool)
	cg.getCalleesRecursiveHelper(funcID, &result, visited, 0, maxDepth)
	return result
}

func (cg *CallGraphFull) getCalleesRecursiveHelper(funcID int, result *[]*Node, visited map[int]bool, depth, maxDepth int) {
	if depth >= maxDepth {
		return
	}
	if calleeIDs, ok := cg.Callees[funcID]; ok {
		for _, calleeID := range calleeIDs {
			if !visited[calleeID] {
				visited[calleeID] = true
				if node, exists := cg.Functions[calleeID]; exists {
					*result = append(*result, node)
					cg.getCalleesRecursiveHelper(calleeID, result, visited, depth+1, maxDepth)
				}
			}
		}
	}
}

// GetPublicEntryPoints 获取所有公开入口函数
func (cg *CallGraphFull) GetPublicEntryPoints() []*Node {
	result := make([]*Node, 0)
	for _, node := range cg.Functions {
		if node.Visibility == "public" || node.Visibility == "external" ||
			node.Kind == "constructor" || node.Kind == "fallback" || node.Kind == "receive" {
			result = append(result, node)
		}
	}
	return result
}

// BuildEnrichedContext 构建增强的上下文（包含调用链信息）
// 用于生成更丰富的 Prompt，让 LLM 看到完整的调用关系
func (cg *CallGraphFull) BuildEnrichedContext(funcID int, includeCallers, includeCallees bool, maxDepth int) string {
	var sb strings.Builder

	targetFunc := cg.Functions[funcID]
	if targetFunc == nil {
		return ""
	}

	targetRef := cg.FunctionRefs[funcID]
	if targetRef == nil {
		// 如果没有函数引用信息，使用默认值
		targetRef = &FunctionRef{
			ContractName: "Unknown",
			FunctionName: targetFunc.Name,
			NodeID:       funcID,
		}
	}

	// 标题
	sb.WriteString(fmt.Sprintf("// ========== Analysis Context for: %s.%s ==========\n\n",
		targetRef.ContractName, targetRef.FunctionName))

	// 调用者（谁调用了这个函数）
	if includeCallers {
		callers := cg.GetCallersRecursive(funcID, maxDepth)
		if len(callers) > 0 {
			sb.WriteString("// --- Callers (functions that call this function) ---\n")
			for _, caller := range callers {
				callerRef := cg.FunctionRefs[caller.ID]
				if callerRef != nil {
					sb.WriteString(fmt.Sprintf("// From: %s.%s\n", callerRef.ContractName, callerRef.FunctionName))
				} else {
					sb.WriteString(fmt.Sprintf("// From: %s\n", caller.Name))
				}
				sb.WriteString(cg.ps.GetSourceRange(caller.Src) + "\n\n")
			}
		}
	}

	// 目标函数
	sb.WriteString("// --- Target Function (under analysis) ---\n")
	sb.WriteString(cg.ps.GetSourceRange(targetFunc.Src) + "\n\n")

	// 被调用者（这个函数调用了谁）
	if includeCallees {
		callees := cg.GetCalleesRecursive(funcID, maxDepth)
		if len(callees) > 0 {
			sb.WriteString("// --- Callees (functions called by this function) ---\n")
			for _, callee := range callees {
				calleeRef := cg.FunctionRefs[callee.ID]
				if calleeRef != nil {
					sb.WriteString(fmt.Sprintf("// Called: %s.%s\n", calleeRef.ContractName, calleeRef.FunctionName))
				} else {
					sb.WriteString(fmt.Sprintf("// Called: %s\n", callee.Name))
				}
				sb.WriteString(cg.ps.GetSourceRange(callee.Src) + "\n\n")
			}
		}
	}

	return sb.String()
}

// FindFunctionByName 通过函数名查找函数
func (cg *CallGraphFull) FindFunctionByName(contractName, functionName string) *Node {
	for _, ref := range cg.FunctionRefs {
		if ref.ContractName == contractName && ref.FunctionName == functionName {
			return cg.Functions[ref.NodeID]
		}
	}
	// 如果没有指定合约名，只匹配函数名
	if contractName == "" {
		for _, ref := range cg.FunctionRefs {
			if ref.FunctionName == functionName {
				return cg.Functions[ref.NodeID]
			}
		}
	}
	return nil
}

// GetCallChainToEntry 获取从入口函数到目标函数的调用链
func (cg *CallGraphFull) GetCallChainToEntry(funcID int) [][]*Node {
	chains := make([][]*Node, 0)
	visited := make(map[int]bool)
	currentChain := make([]*Node, 0)

	cg.findPathsToEntry(funcID, &chains, currentChain, visited)
	return chains
}

func (cg *CallGraphFull) findPathsToEntry(funcID int, chains *[][]*Node, currentChain []*Node, visited map[int]bool) {
	if visited[funcID] {
		return
	}
	visited[funcID] = true

	node := cg.Functions[funcID]
	if node == nil {
		return
	}

	newChain := append(currentChain, node)

	// 检查是否是入口函数
	if node.Visibility == "public" || node.Visibility == "external" ||
		node.Kind == "constructor" || node.Kind == "fallback" || node.Kind == "receive" {
		// 找到一条到入口的路径
		chainCopy := make([]*Node, len(newChain))
		copy(chainCopy, newChain)
		*chains = append(*chains, chainCopy)
	}

	// 继续向上查找调用者
	if callerIDs, ok := cg.Callers[funcID]; ok {
		for _, callerID := range callerIDs {
			cg.findPathsToEntry(callerID, chains, newChain, visited)
		}
	}

	visited[funcID] = false
}

// GetNodeName 获取节点的可读名称
func (cg *CallGraphFull) GetNodeName(nodeID int) string {
	ref := cg.FunctionRefs[nodeID]
	if ref != nil {
		return fmt.Sprintf("%s.%s", ref.ContractName, ref.FunctionName)
	}
	if node, ok := cg.Functions[nodeID]; ok {
		return node.Name
	}
	return fmt.Sprintf("Node_%d", nodeID)
}

// GenerateCallGraphTree 生成调用关系的树状文本表示
func (cg *CallGraphFull) GenerateCallGraphTree() string {
	var sb strings.Builder
	sb.WriteString("Global Call Graph Tree:\n")

	entryPoints := cg.GetPublicEntryPoints()
	pathVisited := make(map[int]bool)

	for _, entry := range entryPoints {
		// 只有当该入口点有下级调用时，或者它是重要的入口点时才显示
		sb.WriteString(fmt.Sprintf("- Entry: %s\n", cg.GetNodeName(entry.ID)))

		pathVisited[entry.ID] = true
		cg.printCallTreeRecursive(&sb, entry.ID, 1, pathVisited, 10) // 深度限制 10
		delete(pathVisited, entry.ID)

		sb.WriteString("\n")
	}
	return sb.String()
}

func (cg *CallGraphFull) printCallTreeRecursive(sb *strings.Builder, funcID int, depth int, pathVisited map[int]bool, maxDepth int) {
	if depth > maxDepth {
		sb.WriteString(strings.Repeat("  ", depth) + "-> ... (max depth)\n")
		return
	}

	callees := cg.Callees[funcID]
	if len(callees) == 0 {
		return
	}

	for _, calleeID := range callees {
		prefix := strings.Repeat("  ", depth)
		name := cg.GetNodeName(calleeID)

		if pathVisited[calleeID] {
			sb.WriteString(fmt.Sprintf("%s-> %s (Recursive Cycle)\n", prefix, name))
			continue
		}

		sb.WriteString(fmt.Sprintf("%s-> %s\n", prefix, name))

		pathVisited[calleeID] = true
		cg.printCallTreeRecursive(sb, calleeID, depth+1, pathVisited, maxDepth)
		delete(pathVisited, calleeID)
	}
}

// GetAllRelatedFunctions 获取从入口点开始的所有相关函数（去重）
func (cg *CallGraphFull) GetAllRelatedFunctions(maxDepth int) []*Node {
	uniqueNodes := make(map[int]*Node)
	entryPoints := cg.GetPublicEntryPoints()

	for _, entry := range entryPoints {
		// 递归获取被调用者
		callees := cg.GetCalleesRecursive(entry.ID, maxDepth)
		for _, callee := range callees {
			uniqueNodes[callee.ID] = callee
		}
	}

	result := make([]*Node, 0, len(uniqueNodes))
	for _, node := range uniqueNodes {
		result = append(result, node)
	}
	return result
}
