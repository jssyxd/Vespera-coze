package astparser

type AST struct {
	AbsolutePath    string      `json:"absolutePath"`
	ExportedSymbols interface{} `json:"exportedSymbols"`
	ID              int         `json:"id"`
	NodeType        string      `json:"nodeType"`
	Nodes           []Node      `json:"nodes"`
	Src             string      `json:"src"`
}

type Node struct {
	ID                    int    `json:"id"`
	NodeType              string `json:"nodeType"`
	Name                  string `json:"name,omitempty"`
	Src                   string `json:"src"`
	BaseContracts         []Node `json:"baseContracts,omitempty"`
	Kind                  string `json:"kind,omitempty"`
	Body                  *Node  `json:"body,omitempty"`
	Implemented           bool   `json:"implemented,omitempty"`
	Visibility            string `json:"visibility,omitempty"`
	StateMutability       string `json:"stateMutability,omitempty"`
	Modifiers             []Node `json:"modifiers,omitempty"`
	Expression            *Node  `json:"expression,omitempty"`
	ReferencedDeclaration int    `json:"referencedDeclaration,omitempty"`
	Arguments             []Node `json:"arguments,omitempty"`
	Nodes                 []Node `json:"nodes,omitempty"`
	Statements            []Node `json:"statements,omitempty"`
}

type ParsedSource struct {
	AST        *AST
	SourceCode string
	NodesByID  map[int]*Node
}
