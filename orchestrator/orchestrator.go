package main

import (
	"context"
	"fmt"
	"net/http"
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
	var webServerPort int64
	var graphPath string
	action := func(ctx context.Context, _ *cli.Command) error {
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
		rg := &RepoGraph{}
		if err := rg.LoadFromFile(graphPath); err != nil {
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
		goalCompilationEngine := NewEngine(ctx, EngineJobNameGoalCompilation, rdb, compilationSchedulingParams)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			<-sigChan
			// main thread should wait for stop
			cancel()
		}()

		inferenceEngine.Start(ctx)
		compilationEngine.Start(ctx)
		goalCompilationEngine.Start(ctx)

		orchestrator := Orchestrator{
			logger:    logger,
			ctx:       ctx,
			wg:        &sync.WaitGroup{},
			rdb:       rdb,
			mu:        sync.Mutex{},
			RepoGraph: rg,
			GraphPath: graphPath,
			//GoalProvider: &GoalProvider{},
			InferenceEngine:       inferenceEngine,
			CompilationEngine:     compilationEngine,
			GoalCompilationEngine: &Engine{},
		}

		mux := http.NewServeMux()
		orchestrator.RegisterHandlers(mux)

		logger.Info().Msgf("starting web server on port %d", webServerPort)
		go func() {
			err := http.ListenAndServe(fmt.Sprintf(":%d", webServerPort), mux)
			if err != nil {
				logger.Error().Err(err).Msg("error starting web server")
				cancel()
			}
		}()

		orchestrator.Start()
		orchestrator.WaitForStop()

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
	return &cli.Command{
		Name:   "start",
		Usage:  "start the orchestrator",
		Action: action,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        "port",
				Usage:       "port to run the web server on",
				Value:       8080,
				Destination: &webServerPort,
			},
			&cli.StringFlag{
				Name:        "graph",
				Usage:       "path to the graph file",
				Destination: &graphPath,
				Required:    true,
			},
		},
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

const (
	MaxCommitGraphDepth = 6
)

type Orchestrator struct {
	logger *zerolog.Logger
	ctx    context.Context
	wg     *sync.WaitGroup

	rdb *redis.Client

	mu                               sync.Mutex
	RepoGraph                        *RepoGraph
	GraphPath                        string
	GoalProvider                     GoalProvider
	inferenceTaskToNodeLocator       map[EngineTaskID]NodeLocator
	compilationTaskToNodeLocator     map[EngineTaskID]NodeLocator
	goalCompilationTaskToNodeLocator map[EngineTaskID]NodeLocator

	InferenceEngine       *Engine
	CompilationEngine     *Engine
	GoalCompilationEngine *Engine
}

func (o *Orchestrator) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
}

func (o *Orchestrator) WaitForStop() {
	o.wg.Wait()
}

func (o *Orchestrator) Start() {
	o.wg.Add(7)
	go o.startGoalCompilationTx()
	go o.startGoalCompilationRx()
	go o.startInferenceRx()
	go o.startInferenceTx()
	go o.startCompilationTx()
	go o.startCompilationRx()
	go o.startGraphPeriodicSave()
}

// Due to architectural complexity I am using polling here
// It would be better to have a channel that gets pushed to when a graph is finished
func (o *Orchestrator) startGoalCompilationTx() {
	defer o.wg.Done()
	goalCompilationInput := o.GoalCompilationEngine.GetInput()
	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("goalCompilationInput listener closing")
			return
		case <-time.After(6 * time.Second):
		}
		numToAdd := func() int {
			o.mu.Lock()
			defer o.mu.Unlock()
			numUnfinished := len(o.RepoGraph.UnfinishedGraphs())
			return numUnfinished - 10
		}()
		if numToAdd <= 0 {
			continue

		}
		attempts := 0
		numAdded := 0

		for {
			attempts += 1
			// terrible but needed to avoid lock-holding
			// when we will never find a branch target
			if attempts > 100 || numAdded >= numToAdd {
				return
			}
			toAdd := func() *EngineTaskMsg {
				o.mu.Lock()
				defer o.mu.Unlock()
				goal := o.GoalProvider.GetRandom()
				bt := o.RepoGraph.FindNewBranchTargetForGoal(goal.ID())
				if bt == nil {
					return nil
				}
				cg := NewCommitGraph(goal.ID())
				cg.Nodes[cg.RootNode].State = NodeStateRunningGoalSetup
				bt.Subgraphs[goal.ID()] = cg
				locator := NodeLocator{
					BranchTarget: bt.BranchName,
					ProblemID:    cg.GoalID,
					NodeID:       cg.RootNode,
				}
				validation := goal.SetupOnBranch(bt.BranchName, cg.Nodes[cg.RootNode].BranchName)
				task := EngineTaskMsg{
					ID:   NewEngineTaskID(),
					Task: validation.ToJSON(),
				}
				o.goalCompilationTaskToNodeLocator[task.ID] = locator
				return &task
			}()
			if toAdd != nil {
				select {
				case <-o.ctx.Done():
					o.logger.Info().Msg("goalCompilationInput listener closing")
					return
				case goalCompilationInput <- *toAdd:
				}

			}
		}

	}
}
func (o *Orchestrator) startGoalCompilationRx() {
	defer o.wg.Done()
	goalCompilationOutput := o.GoalCompilationEngine.GetOutput()
	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("inferenceOutput listener closing")
			return
		case val, ok := <-goalCompilationOutput:
			if !ok {
				o.logger.Fatal().Msg("goal compilation output channel closed")
			}
			func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				locator, ok := o.goalCompilationTaskToNodeLocator[val.ID]
				if !ok {
					o.logger.Fatal().Msg("goal compilation output reader is missing locator for task ID")
				}
				response := CompilationTaskResponseFromJSON(val.Result)
				err := o.RepoGraph.HandleSetupCompilationOutput(o.logger, locator, &response, o.GoalProvider)
				if err != nil {
					o.logger.Fatal().Err(err).Msg("error handling goal compilation output")
				}
				delete(o.goalCompilationTaskToNodeLocator, val.ID)
			}()
		}
	}
}

func (o *Orchestrator) startInferenceTx() {
	defer o.wg.Done()
	inferenceInput := o.InferenceEngine.GetInput()
	quickQueue := []EngineTaskMsg{}
	for {
		if len(quickQueue) > 0 {
			select {
			case <-o.ctx.Done():
				o.logger.Info().Msg("inferenceInput listener closing")
				return
			case inferenceInput <- quickQueue[0]:
				quickQueue = quickQueue[1:]
			}
		} else {
			func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				for _, graphLocator := range o.RepoGraph.UnfinishedGraphs() {
					slice, err := o.RepoGraph.GetCommitGraphSlice(graphLocator)
					if err != nil {
						o.logger.Fatal().Err(err).Msg("error getting commit graph slice")
					}
					nodes := slice.CommitGraph.AllNodesInState(NodeStateAwaitingInference)
					for _, node := range nodes {
						locator := NodeLocator{
							BranchTarget: graphLocator.BranchTarget,
							ProblemID:    graphLocator.ProblemID,
							NodeID:       node.ID,
						}
						inferenceTask, err := o.RepoGraph.BuildInferenceTaskForNode(locator)
						if err != nil {
							o.logger.Fatal().Err(err).Msg("error building inference task for node")
						}
						msg := EngineTaskMsg{
							ID:   NewEngineTaskID(),
							Task: inferenceTask.ToJSON(),
						}
						o.inferenceTaskToNodeLocator[msg.ID] = locator
						node.State = NodeStateRunningInference
						quickQueue = append(quickQueue, msg)
					}
				}
			}()

			// avoid busy-looping
			if len(quickQueue) == 0 {
				select {
				case <-o.ctx.Done():
					o.logger.Info().Msg("inferenceInput listener closing")
					return
				case <-time.After(2 * time.Second):
					continue
				}
			}

		}
	}
}

func (o *Orchestrator) startInferenceRx() {
	defer o.wg.Done()
	inferenceOutput := o.InferenceEngine.GetOutput()
	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("inferenceOutput listener closing")
			return
		case val, ok := <-inferenceOutput:
			if !ok {
				o.logger.Fatal().Msg("inference output channel closed")
			}
			func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				locator, ok := o.inferenceTaskToNodeLocator[val.ID]
				if !ok {
					o.logger.Fatal().Msg("inference output reader is missing locator for task ID")
				}
				response := InferenceTaskResponseFromJSON(val.Result)
				err := o.RepoGraph.HandleInferenceOutput(locator, response)
				if err != nil {
					o.logger.Fatal().Err(err).Msg("error handling inference output")
				}
				delete(o.inferenceTaskToNodeLocator, val.ID)
			}()
		}
	}
}

func (o *Orchestrator) startCompilationTx() {
	defer o.wg.Done()
	compilationInput := o.CompilationEngine.GetInput()
	quickQueue := []EngineTaskMsg{}
	for {
		if len(quickQueue) > 0 {
			select {
			case <-o.ctx.Done():
				o.logger.Info().Msg("compilationInput listener closing")
				return
			case compilationInput <- quickQueue[0]:
				quickQueue = quickQueue[1:]
			}
		} else {
			func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				for _, graphLocator := range o.RepoGraph.UnfinishedGraphs() {
					slice, err := o.RepoGraph.GetCommitGraphSlice(graphLocator)
					if err != nil {
						o.logger.Fatal().Err(err).Msg("error getting commit graph slice")
					}
					nodes := slice.CommitGraph.AllNodesInState(NodeStateAwaitingCompilation)
					for _, node := range nodes {
						locator := NodeLocator{
							BranchTarget: graphLocator.BranchTarget,
							ProblemID:    graphLocator.ProblemID,
							NodeID:       node.ID,
						}
						compilationTask, err := o.RepoGraph.BuildCompilationTasksForNode(locator)
						if err != nil {
							o.logger.Fatal().Err(err).Msg("error building compilation tasks for node")
						}
						msg := EngineTaskMsg{
							ID:   NewEngineTaskID(),
							Task: compilationTask.ToJSON(),
						}
						o.compilationTaskToNodeLocator[msg.ID] = locator
						node.State = NodeStateRunningCompilation
						quickQueue = append(quickQueue, msg)
					}
				}
			}()

			// avoid busy-looping
			if len(quickQueue) == 0 {
				select {
				case <-o.ctx.Done():
					o.logger.Info().Msg("compilationInput listener closing")
					return
				case <-time.After(2 * time.Second):
					continue
				}
			}

		}
	}
}

func (o *Orchestrator) startCompilationRx() {
	defer o.wg.Done()
	compilationOutput := o.CompilationEngine.GetOutput()
	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("compilationOutput listener closing")
			return
		case val, ok := <-compilationOutput:
			if !ok {
				o.logger.Fatal().Msg("compilation output channel closed")
			}
			func() {
				o.mu.Lock()
				defer o.mu.Unlock()
				locator, ok := o.compilationTaskToNodeLocator[val.ID]
				if !ok {
					o.logger.Fatal().Msg("compilation output reader is missing locator for task ID")
				}
				response := CompilationTaskResponseFromJSON(val.Result)
				err := o.RepoGraph.HandleCompilationOutput(locator, &response, MaxCommitGraphDepth)
				if err != nil {
					o.logger.Fatal().Err(err).Msg("error handling compilation output")
				}
				delete(o.compilationTaskToNodeLocator, val.ID)
			}()
		}
	}
}

func (o *Orchestrator) startGraphPeriodicSave() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			return
		case <-time.After(1 * time.Minute):
		}
		o.logger.Info().Msg("periodic saving graph to file")
		if err := o.RepoGraph.SaveToFile(o.GraphPath); err != nil {
			o.logger.Error().Err(err).Msg("error saving graph to file")
		}
	}
}
