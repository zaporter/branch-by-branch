package experiment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type NoopExecutorConfig struct {
	A int `json:"a"`
	B int `json:"b"`
}

type NoopExecutor struct{}

var _ ExperimentExecutor = &NoopExecutor{}

// Execute implements ExperimentExecutor.
func (n *NoopExecutor) Execute(ctx context.Context, config *experimentConfig) error {
	parsedConfig, err := readExperimentConfig[NoopExecutorConfig](config)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.FullPath, "output.txt"), []byte(fmt.Sprintf("a: %d, b: %d", parsedConfig.A, parsedConfig.B)), 0644)
}

// GetStats implements ExperimentExecutor.
func (n *NoopExecutor) GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error) {
	savedValue, err := os.ReadFile(filepath.Join(config.FullPath, "output.txt"))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"savedValue": string(savedValue),
	}, nil
}
