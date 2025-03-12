package experiment

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/zaporter/branch-by-branch/orchestrator"
)

type CompilationExecutorConfig struct {
	Task             orchestrator.CompilationTask `json:"task"`
	ExpectedExitCode int                          `json:"expected_exit_code"`
}

type CompilationExecutorResult struct {
	TaskResponse orchestrator.CompilationTaskResponse `json:"task_response"`
}

type CompilationExecutor struct{}

var _ ExperimentExecutor = &CompilationExecutor{}

// Execute implements ExperimentExecutor.
func (n *CompilationExecutor) Execute(ctx context.Context, config *experimentConfig) error {
	parsedConfig, err := readExperimentConfig[CompilationExecutorConfig](config)
	if err != nil {
		return err
	}
	if parsedConfig.Task.NewBranchName == "" {
		parsedConfig.Task.NewBranchName = orchestrator.NewBranchName()
	}
	rdb, err := orchestrator.ConnectToRedis(ctx)
	if err != nil {
		return err
	}
	compilationSchedulingParams := orchestrator.SchedulingParams{
		MinTaskQueueSize:      1,
		MaxTaskQueueSize:      1,
		TaskProcessingTimeout: 3 * time.Minute,
		CamShaftInterval:      1 * time.Second,
		CrankShaftInterval:    1 * time.Second,
		TimingBeltInterval:    2 * time.Second,
		ODBInterval:           10 * time.Second,
		InputChanSize:         1,
		OutputChanSize:        1,
	}
	compilationEngine := orchestrator.NewEngine(ctx, orchestrator.EngineJobNameCompilation, rdb, compilationSchedulingParams)
	if err := compilationEngine.Start(ctx); err != nil {
		return err
	}
	taskMsg := orchestrator.EngineTaskMsg{
		ID:   orchestrator.NewEngineTaskID(),
		Task: parsedConfig.Task.ToJSON(),
	}
	compilationEngine.GetInput() <- taskMsg
	taskResponse := <-compilationEngine.GetOutput()
	result := orchestrator.CompilationTaskResponseFromJSON(taskResponse.Result)
	executorResult := CompilationExecutorResult{
		TaskResponse: result,
	}
	output, err := json.Marshal(executorResult)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.FullPath, "output.json"), output, 0644)
}

// GetStats implements ExperimentExecutor.
func (n *CompilationExecutor) GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error) {
	parsedConfig, err := readExperimentConfig[CompilationExecutorConfig](config)
	if err != nil {
		return nil, err
	}
	savedValue, err := os.ReadFile(filepath.Join(config.FullPath, "output.json"))
	if err != nil {
		return nil, err
	}
	var executorResult CompilationExecutorResult
	if err := json.Unmarshal(savedValue, &executorResult); err != nil {
		return nil, err
	}
	return map[string]any{
		"result":               executorResult,
		"is_correct_exit_code": executorResult.TaskResponse.CompilationResult.ExitCode == parsedConfig.ExpectedExitCode,
	}, nil
}
