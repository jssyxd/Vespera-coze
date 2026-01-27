package static_analyzer

import (
	"context"
)

type Analyzer interface {
	AnalyzeContract(ctx context.Context, code string, config *AnalysisConfig) (*AnalysisResult, error)

	GetStateVariables(ctx context.Context, code string) ([]StateVariable, error)

	GetFunctions(ctx context.Context, code string) ([]Function, error)

	Close() error
}

func GetStateVariables(a Analyzer, ctx context.Context, code string) ([]StateVariable, error) {
	result, err := a.AnalyzeContract(ctx, code, &AnalysisConfig{})
	if err != nil {
		return nil, err
	}
	return result.StateVariables, nil
}

func GetFunctions(a Analyzer, ctx context.Context, code string) ([]Function, error) {
	result, err := a.AnalyzeContract(ctx, code, &AnalysisConfig{})
	if err != nil {
		return nil, err
	}
	return result.Functions, nil
}

type NoOpAnalyzer struct{}

func (n *NoOpAnalyzer) AnalyzeContract(ctx context.Context, code string, config *AnalysisConfig) (*AnalysisResult, error) {
	return &AnalysisResult{}, nil
}

func (n *NoOpAnalyzer) GetStateVariables(ctx context.Context, code string) ([]StateVariable, error) {
	return []StateVariable{}, nil
}

func (n *NoOpAnalyzer) GetFunctions(ctx context.Context, code string) ([]Function, error) {
	return []Function{}, nil
}

func (n *NoOpAnalyzer) Close() error {
	return nil
}

func NewNoOpAnalyzer() Analyzer {
	return &NoOpAnalyzer{}
}
