package experiment

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/zaporter/branch-by-branch/orchestrator"
)

type OrchestratorExecutorConfig struct {
	GoalFile   string `json:"goals"`
	GraphFile  string `json:"graph"`
	CloneGraph bool   `json:"clone_graph"`
}

type OrchestratorExecutor struct{}

var _ ExperimentExecutor = &OrchestratorExecutor{}

// Execute implements ExperimentExecutor.
func (o *OrchestratorExecutor) Execute(ctx context.Context, config *experimentConfig) error {
	parsedConfig, err := readExperimentConfig[OrchestratorExecutorConfig](config)
	if err != nil {
		return err
	}

	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("starting orchestrator")
	rdb, err := orchestrator.ConnectToRedis(ctx)
	if err != nil {
		return err
	}
	resolvedGoalFile := filepath.Join(config.FullPath, parsedConfig.GoalFile)
	goalProvider := orchestrator.StaticGoalProviderFromFile(resolvedGoalFile)
	if err := orchestrator.DropTrainingChans(ctx, rdb); err != nil {
		return err
	}
	rg := &orchestrator.RepoGraph{}
	graphPath := filepath.Join(config.FullPath, parsedConfig.GraphFile)
	if parsedConfig.CloneGraph {
		newFile := filepath.Join(config.FullPath, "cloned_graph.json")
		if err := orchestrator.CopyFile(graphPath, newFile); err != nil {
			return err
		}
		graphPath = newFile
	}
	if err := rg.LoadFromFile(graphPath); err != nil {
		return err
	}
	rg.Ctx = ctx
	rg.ShouldAdvertiseChan = make(chan orchestrator.CommitGraphLocator, 128)
	inferenceSchedulingParams := orchestrator.SchedulingParams{
		MinTaskQueueSize:      16,
		MaxTaskQueueSize:      32,
		TaskProcessingTimeout: 5 * time.Minute,
		CamShaftInterval:      1 * time.Second,
		CrankShaftInterval:    1 * time.Second,
		TimingBeltInterval:    2 * time.Second,
		ODBInterval:           10 * time.Second,
		InputChanSize:         8,
		OutputChanSize:        8,
	}
	compilationSchedulingParams := orchestrator.SchedulingParams{
		MinTaskQueueSize:      32,
		MaxTaskQueueSize:      64,
		TaskProcessingTimeout: 2 * time.Minute,
		CamShaftInterval:      1 * time.Second,
		CrankShaftInterval:    1 * time.Second,
		TimingBeltInterval:    2 * time.Second,
		ODBInterval:           10 * time.Second,
		InputChanSize:         32,
		OutputChanSize:        32,
	}
	inferenceEngine := orchestrator.NewEngine(ctx, orchestrator.EngineJobNameInference, rdb, inferenceSchedulingParams)
	compilationEngine := orchestrator.NewEngine(ctx, orchestrator.EngineJobNameCompilation, rdb, compilationSchedulingParams)
	goalCompilationEngine := orchestrator.NewEngine(ctx, orchestrator.EngineJobNameGoalCompilation, rdb, compilationSchedulingParams)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-sigChan
		// main thread should wait for stop
		cancel()
	}()

	rg.ResetTransientStates()
	if err := inferenceEngine.Start(ctx); err != nil {
		return err
	}
	if err := compilationEngine.Start(ctx); err != nil {
		return err
	}
	if err := goalCompilationEngine.Start(ctx); err != nil {
		return err
	}
	webServerPort := 8080

	orchestratorParams := orchestrator.OrchestratorParams{
		Rdb:                   rdb,
		RepoGraph:             rg,
		GraphPath:             graphPath,
		GoalProvider:          goalProvider,
		InferenceEngine:       inferenceEngine,
		CompilationEngine:     compilationEngine,
		GoalCompilationEngine: goalCompilationEngine,
		DoTraining:            true,
	}
	orchestrator := orchestrator.NewOrchestrator(ctx, logger, orchestratorParams)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", webServerPort),
		Handler: mux,
	}
	orchestrator.RegisterHandlers(mux)

	logger.Info().Msgf("starting web server on port %d", webServerPort)
	go func() {
		err := server.ListenAndServe()
		if err != nil {
			logger.Error().Err(err).Msg("error starting web server")
			cancel()
		}
	}()

	orchestrator.Start()
	orchestrator.WaitForStop()
	server.Shutdown(ctx)

	inferenceEngine.TriggerStop()
	compilationEngine.TriggerStop()
	goalCompilationEngine.TriggerStop()
	inferenceEngine.WaitForStop()
	compilationEngine.WaitForStop()
	goalCompilationEngine.WaitForStop()
	logger.Info().Msg("saving graph to file")
	if err := rg.SaveToFile(graphPath); err != nil {
		logger.Error().Err(err).Msg("error saving graph to file")
		return err
	}
	return nil
}

// GetStats implements ExperimentExecutor.
func (o *OrchestratorExecutor) GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error) {
	return nil, nil
}
