package static_analyzer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VectorBits/Vespera/src/internal/logger"
	"github.com/VectorBits/Vespera/src/internal/static_analyzer/backend"
)

type BackendType string

const (
	BackendPythonScript BackendType = "python_script"
	BackendNoOp         BackendType = "noop" // No-op implementation for testing
)

type AnalyzerConfig struct {
	Backend    BackendType
	ScriptPath string // Deprecated: Python script is embedded, this field is no longer used
	PythonPath string // Python executable path
	Enabled    bool   // Whether the analyzer is enabled
}

// helloq NewAnalyzer creates an analyzer instance
func NewAnalyzer(cfg AnalyzerConfig) (Analyzer, error) {
	if !cfg.Enabled {
		return NewNoOpAnalyzer(), nil
	}

	switch cfg.Backend {
	case BackendPythonScript:
		// Use embedded script, ScriptPath is no longer needed
		pyBackend := backend.NewPythonScriptBackend(cfg.PythonPath)
		return &pythonScriptAdapter{backend: pyBackend}, nil

	case BackendNoOp:
		return NewNoOpAnalyzer(), nil

	default:
		return nil, fmt.Errorf("unsupported backend: %s (supported: python_script, noop)", cfg.Backend)
	}
}

func DefaultConfig() AnalyzerConfig {
	return AnalyzerConfig{
		Backend:    BackendPythonScript,
		PythonPath: "python3",
		Enabled:    true,
	}
}

type pythonScriptAdapter struct {
	backend *backend.PythonScriptBackend
}

func (a *pythonScriptAdapter) AnalyzeContract(ctx context.Context, code string, config *AnalysisConfig) (*AnalysisResult, error) {
	// Convert config to map
	configMap := map[string]interface{}{
		"contract_name": config.ContractName,
		"solc_version":  config.SolcVersion,
		"address":       config.Address,
		"optimization":  config.Optimization,
		"via_ir":        config.ViaIR,
	}

	// Call backend
	result, err := a.backend.AnalyzeContract(ctx, code, configMap)
	if err != nil {
		return nil, err
	}

	// Convert result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var analysisResult AnalysisResult
	if err := json.Unmarshal(resultJSON, &analysisResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	// Debug: check if detectors are parsed correctly
	if resultMap, ok := result.(map[string]interface{}); ok {
		if detectorsRaw, exists := resultMap["detectors"]; exists {
			if detectorsList, ok := detectorsRaw.([]interface{}); ok {
				// Only log when data exists but parsing resulted in empty
				if len(detectorsList) > 0 && len(analysisResult.Detectors) == 0 {
					logger.Debug("Python returned %d detectors, but parsed result is empty", len(detectorsList))
				}
			}
		}
	}

	return &analysisResult, nil
}

func (a *pythonScriptAdapter) GetStateVariables(ctx context.Context, code string) ([]StateVariable, error) {
	return GetStateVariables(a, ctx, code)
}

func (a *pythonScriptAdapter) GetFunctions(ctx context.Context, code string) ([]Function, error) {
	return GetFunctions(a, ctx, code)
}

func (a *pythonScriptAdapter) Close() error {
	return a.backend.Close()
}
