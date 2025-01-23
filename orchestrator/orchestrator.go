package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createOrchestratorStartCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		return runOrchestrator(c)
	}
	return &cli.Command{
		Name:   "start",
		Usage:  "start the orchestrator",
		Action: action,
	}
}

func createOrchestratorCli() *cli.Command {
	return &cli.Command{
		Name:    "orchestrator",
		Aliases: []string{"or"},
		Usage:   "orchestrator",
		Commands: []*cli.Command{
			createOrchestratorStartCli(),
		},
	}
}

func runOrchestrator(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("starting orchestrator")
	rdb, err := connectToRedis(ctx)
	if err != nil {
		return err
	}
	if err := setRouterParam(ctx, rdb, RedisInferenceEnabled, "true"); err != nil {
		return err
	}
	if err := setRouterParam(ctx, rdb, RedisInferenceModelDir, "meta-llama/Llama-3.1-8B-Instruct"); err != nil {
		return err
	}
	if err := setRouterParam(ctx, rdb, RedisInferenceAdapterDir, ""); err != nil {
		return err
	}
	inferenceSchedulingParams := SchedulingParams{
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
	compilationSchedulingParams := SchedulingParams{
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
	inferenceEngine := NewEngine(ctx, EngineJobNameInference, rdb, inferenceSchedulingParams)
	compilationEngine := NewEngine(ctx, EngineJobNameCompilation, rdb, compilationSchedulingParams)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		// main thread should wait for stop
		inferenceEngine.TriggerStop()
		compilationEngine.TriggerStop()
	}()

	inferenceEngine.Start(ctx)
	compilationEngine.Start(ctx)

	dieChan := make(chan bool, 1)
	innerOrchestratorWg := sync.WaitGroup{}
	innerOrchestratorWg.Add(1)

	go func() {
		defer innerOrchestratorWg.Done()
		executeInnerOrchestrator(ctx, rdb, inferenceEngine, compilationEngine, dieChan)
	}()

	inferenceEngine.WaitForStop()
	compilationEngine.WaitForStop()
	close(dieChan)
	innerOrchestratorWg.Wait()

	return nil
}

func executeInnerOrchestrator(ctx context.Context, rdb *redis.Client, inferenceEngine *Engine, compilationEngine *Engine, dieChan chan bool) error {
	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("starting inner orchestrator")
	inferenceInput := inferenceEngine.GetInput()
	compilationInput := compilationEngine.GetInput()
	inferenceOutput := inferenceEngine.GetOutput()
	compilationOutput := compilationEngine.GetOutput()

	// All goroutines share these variables in deeply interleaved ways
	// If this becomes a bottleneck, scrap the entire project.
	mu := sync.Mutex{}
	repoGraph := RepoGraph{}
	inferenceTaskToNodeLocator := map[EngineTaskID]NodeLocator{}
	//compilationTaskToNodeLocator := map[EngineTaskID]NodeLocator{}

	wg := sync.WaitGroup{}

	wg.Add(4)
	// inferenceInput
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		// We use this list every time we are looking for new nodes to push to the queue.
		// We traverse this in the same order every time, always filling from the front.
		// We only remove from the front of the list when the CommitGraph is finished.
		// We only add to the end when we are out of nodes to push.
		//
		// This ensures that the parallelism we enjoy from job distribution does not increase graph-finishing latency.
		graphPriorityQueue := []CommitGraphLocator{}
		for {
			select {
			case <-dieChan:
				logger.Info().Msg("inferenceInput listener closing")
				return
			case <-ticker.C:
			}
			func() {
				mu.Lock()
				defer mu.Unlock()

				// clear stale (finished) graphs
				for i, graph := range graphPriorityQueue {
					tree, err := repoGraph.GetTreeAtCommitGraphLocator(graph)
					if err != nil {
						logger.Fatal().Err(err).Msg("error getting tree at commit graph locator")
					}
					if tree.CommitGraph.State != GraphStateInProgress {
						graphPriorityQueue = append(graphPriorityQueue[:i], graphPriorityQueue[i+1:]...)
					}
				}
				graphIndex := 0
			outerInsertion:
				for len(inferenceInput) < cap(inferenceInput) {
					if graphIndex >= len(graphPriorityQueue) {
						unfinishedGraphs := repoGraph.UnfinishedGraphs()
						if len(unfinishedGraphs) == 0 {
							newGraph, ok := repoGraph.SpawnNewGraph()
							if !ok {
								logger.Info().Msg("no new graphs can be spawned")
								break outerInsertion
							}
							graphPriorityQueue = append(graphPriorityQueue, *newGraph)
						} else {
							graphPriorityQueue = append(graphPriorityQueue, unfinishedGraphs...)
						}
					}
					graph := graphPriorityQueue[graphIndex]
					tree, err := repoGraph.GetTreeAtCommitGraphLocator(graph)
					if err != nil {
						logger.Fatal().Err(err).Msg("error getting tree at commit graph locator")
					}
					nodes := tree.CommitGraph.AllNodesInState(NodeStateAwaitingInference)
					nodeIndex := 0
				innerInsertion:
					for len(inferenceInput) < cap(inferenceInput) {
						if nodeIndex >= len(nodes) {
							break innerInsertion
						}
						node := nodes[nodeIndex]
						task := EngineTaskMsg{
							ID: NewEngineTaskID(),
							Task: InferenceTask{
								Prompt: node.GetPrompt(),
							}.ToJSON(),
						}
						inferenceTaskToNodeLocator[task.ID] = NodeLocator{
							BranchName: graph.BranchName,
							ProblemID:  graph.ProblemID,
							NodeID:     node.ID,
						}
						inferenceInput <- task
						nodeIndex++
					}

					graphIndex++
				}
			}()
		}
	}()
	// inferenceOutput
	go func() {
		defer wg.Done()
		for {
			select {
			case <-dieChan:
				logger.Info().Msg("inferenceOutput listener closing")
				return
			case val, ok := <-inferenceOutput:
				if !ok {
					logger.Fatal().Msg("inference output channel closed")
				}
				func() {
					mu.Lock()
					defer mu.Unlock()
					locator, ok := inferenceTaskToNodeLocator[val.ID]
					if !ok {
						logger.Fatal().Msg("inference output reader is missing locator for task ID")
					}
					response := InferenceTaskResponseFromJSON(val.Result)
					err := repoGraph.HandleInferenceOutput(locator, response)
					if err != nil {
						logger.Fatal().Err(err).Msg("error handling inference output")
					}
					delete(inferenceTaskToNodeLocator, val.ID)
				}()
			}
		}
	}()
	// compilationInput
	go func() {
		defer wg.Done()
		for {
			select {
			case <-dieChan:
				logger.Info().Msg("compilationInput listener closing")
				return
			case compilationInput <- EngineTaskMsg{
				Task: CompilationTask{
					BranchName: "main",
				}.ToJSON(),
			}:
			}
		}
	}()
	// compilationOutput
	go func() {
		defer wg.Done()
		for {
			select {
			case <-dieChan:
				logger.Info().Msg("compilationOutput listener closing")
				return
			case val, ok := <-compilationOutput:
				if !ok {
					logger.Fatal().Msg("compilation output channel closed")
				}
				logger.Info().Msgf("compilation output: %s", val.Result)
			}
		}
	}()
	wg.Wait()
	return nil
}
