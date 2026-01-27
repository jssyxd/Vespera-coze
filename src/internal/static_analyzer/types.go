package static_analyzer

type AnalysisResult struct {
	StateVariables []StateVariable `json:"state_variables"`
	Functions      []Function      `json:"functions"`
	Detectors      []Detector      `json:"detectors"`
}

type StateVariable struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Visibility string `json:"visibility"`
	IsConstant bool   `json:"is_constant"`
}

type Function struct {
	Name            string   `json:"name"`
	Signature       string   `json:"signature"`
	Visibility      string   `json:"visibility"`
	StateMutability string   `json:"state_mutability"`
	Parameters      []string `json:"parameters"`
	Returns         []string `json:"returns"`
}

type Detector struct {
	Check       string `json:"check"`
	Impact      string `json:"impact"`
	Confidence  string `json:"confidence"`
	Description string `json:"description"`
	LineNumbers []int  `json:"line_numbers"`
}

type AnalysisConfig struct {
	ContractName string `json:"contract_name"`
	SolcVersion  string `json:"solc_version"`
	Address      string `json:"address"`
	Optimization bool   `json:"optimization"`
	ViaIR        bool   `json:"via_ir"`
}
