package experiment

import (
	"context"
)

type OrchestratorExecutorConfig struct {
}

type OrchestratorExecutor struct{}

var _ ExperimentExecutor = &OrchestratorExecutor{}

// Execute implements ExperimentExecutor.
func (o *OrchestratorExecutor) Execute(ctx context.Context, config *experimentConfig) error {
	return nil
}

// GetStats implements ExperimentExecutor.
func (o *OrchestratorExecutor) GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error) {
	return nil, nil
}
