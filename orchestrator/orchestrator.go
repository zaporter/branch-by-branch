package orchestrator

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
	var viewOnly bool
	var graphPath string
	var goalFile string
	var doTraining bool
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		logger.Info().Msg("starting orchestrator")
		rdb, err := ConnectToRedis(ctx)
		if err != nil {
			return err
		}
		goalProvider := StaticGoalProviderFromFile(goalFile)
		if !viewOnly {
			if err := setRouterParam(ctx, rdb, RedisInferenceEnabled, "true"); err != nil {
				return err
			}
			if err := DropTrainingChans(ctx, rdb); err != nil {
				return err
			}
		}
		rg := &RepoGraph{}
		if err := rg.LoadFromFile(graphPath); err != nil {
			return err
		}
		rg.Ctx = ctx
		if doTraining {
			// otherwise, nil
			rg.ShouldAdvertiseChan = make(chan CommitGraphLocator, 128)
		}
		inferenceSchedulingParams := SchedulingParams{
			MinTaskQueueSize:      32,
			MaxTaskQueueSize:      64,
			TaskProcessingTimeout: 5 * time.Minute,
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

		if !viewOnly {
			// preserve transient states so I can debug crashes
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
		}

		orchestratorParams := OrchestratorParams{
			Rdb:                   rdb,
			RepoGraph:             rg,
			GraphPath:             graphPath,
			GoalProvider:          goalProvider,
			InferenceEngine:       inferenceEngine,
			CompilationEngine:     compilationEngine,
			GoalCompilationEngine: goalCompilationEngine,
			DoTraining:            doTraining,
		}
		orchestrator := NewOrchestrator(ctx, logger, orchestratorParams)

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

		if !viewOnly {
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
		} else {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			server.Shutdown(shutdownCtx)
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
			&cli.StringFlag{
				Name:        "goal-file",
				Usage:       "path to the goal file",
				Destination: &goalFile,
				Required:    true,
			},
			&cli.BoolFlag{
				Name:        "view-only",
				Aliases:     []string{"view"},
				Usage:       "don't execute the engines. Will still save on SIGTERM",
				Value:       false,
				Destination: &viewOnly,
			},
			&cli.BoolFlag{
				Name:        "train",
				Usage:       "enable training",
				Value:       false,
				Destination: &doTraining,
			},
		},
	}
}

func CreateOrchestratorCli() *cli.Command {
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
	MaxCommitGraphDepth   = 8
	MaxSimultaneousGraphs = 1
)

type Orchestrator struct {
	OrchestratorParams
	logger *zerolog.Logger
	ctx    context.Context
	wg     *sync.WaitGroup

	mu                               sync.Mutex
	inferenceTaskToNodeLocator       map[EngineTaskID]NodeLocator
	compilationTaskToNodeLocator     map[EngineTaskID]NodeLocator
	goalCompilationTaskToNodeLocator map[EngineTaskID]NodeLocator
	trainingDataMessageList          *MessageList
}
type OrchestratorParams struct {
	Rdb                   *redis.Client
	RepoGraph             *RepoGraph
	GraphPath             string
	GoalProvider          GoalProvider
	InferenceEngine       *Engine
	CompilationEngine     *Engine
	GoalCompilationEngine *Engine
	DoTraining            bool
}

func NewOrchestrator(ctx context.Context, logger *zerolog.Logger, params OrchestratorParams) *Orchestrator {
	return &Orchestrator{
		OrchestratorParams:               params,
		logger:                           logger,
		ctx:                              ctx,
		wg:                               &sync.WaitGroup{},
		mu:                               sync.Mutex{},
		trainingDataMessageList:          NewMessageList(),
		inferenceTaskToNodeLocator:       map[EngineTaskID]NodeLocator{},
		compilationTaskToNodeLocator:     map[EngineTaskID]NodeLocator{},
		goalCompilationTaskToNodeLocator: map[EngineTaskID]NodeLocator{},
	}
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

	if o.DoTraining {
		o.wg.Add(2)
		go o.startTrainingTx()
		go o.startTrainingRx()
	}
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
			return MaxSimultaneousGraphs - numUnfinished
		}()
		o.logger.Error().Int("numToAdd", numToAdd).Msg("numToAdd")
		if numToAdd <= 0 {
			continue
		}
		attempts := 0
		numAdded := 0

	inner:
		for {
			attempts += 1
			// terrible but needed to avoid lock-holding
			// when we will never find a branch target
			if attempts > 100 || numAdded >= numToAdd {
				break inner
			}
			toAdd := func() *EngineTaskMsg {
				o.mu.Lock()
				defer o.mu.Unlock()
				// TODO: This is the wrong way. It should be BT->find a goal instead of goal->find a BT
				goal := o.GoalProvider.GetNext()
				if goal == nil {
					o.logger.Error().Msg("goal provider returned nil goal")
					return nil
				}
				bt := o.RepoGraph.FindNewBranchTargetForGoal(goal.ID())
				if bt == nil {
					return nil
				}
				cg := NewCommitGraph(goal.ID())
				cg.Nodes[cg.RootNode].State = NodeStateRunningGoalSetup
				bt.Subgraphs[goal.ID()] = cg
				locator := NodeLocator{
					CommitGraphLocator: CommitGraphLocator{
						BranchTargetLocator: BranchTargetLocator{
							BranchName: bt.BranchName,
						},
						GoalID: cg.GoalID,
					},
					NodeID: cg.RootNode,
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
					numAdded += 1
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
							CommitGraphLocator: CommitGraphLocator{
								BranchTargetLocator: BranchTargetLocator{
									BranchName: graphLocator.BranchTargetLocator.BranchName,
								},
								GoalID: graphLocator.GoalID,
							},
							NodeID: node.ID,
						}
						inferenceTask, err := o.RepoGraph.BuildInferenceTaskForNode(locator, o.GoalProvider)
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
							CommitGraphLocator: CommitGraphLocator{
								BranchTargetLocator: BranchTargetLocator{
									BranchName: graphLocator.BranchTargetLocator.BranchName,
								},
								GoalID: graphLocator.GoalID,
							},
							NodeID: node.ID,
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
				err := o.RepoGraph.HandleCompilationOutput(locator, &response, MaxCommitGraphDepth, o.GoalProvider)
				if err != nil {
					o.logger.Fatal().Err(err).Msg("error handling compilation output")
				}
				delete(o.compilationTaskToNodeLocator, val.ID)
			}()
		}
	}
}

func (o *Orchestrator) startTrainingTx() {
	defer o.wg.Done()

	setupAdvertisements := func(cgl CommitGraphLocator) {
		slice, err := o.RepoGraph.GetCommitGraphSlice(cgl)
		if err != nil {
			o.logger.Fatal().Err(err).Msg("error getting commit graph slice")
		}
		cg := slice.CommitGraph
		if cg.State != GraphStateSuccess {
			return
		}
		data, err := o.RepoGraph.ExtractData(cgl, o.GoalProvider)
		if err != nil {
			o.logger.Fatal().Err(err).Msg("error extracting data")
		}
		// Extract data will omit many nodes that have no advantage data.
		// only add the ones that do.
		for _, node := range data.Nodes {
			tgid := NewTrainingGroupID(
				o.RepoGraph.ID,
				NodeLocator{
					CommitGraphLocator: cgl,
					NodeID:             node.NodeID,
				},
			)
			extracted, ok := data.Nodes[node.NodeID]
			if !ok {
				o.logger.Fatal().Str("node_id", string(node.NodeID)).Msg("error extracting data. node not found (should not be advertised)")
			}
			group := TrainingDataGroup{
				GroupID: tgid,
				Prompt:  extracted.Prompt,
				Outputs: []GroupOutput{},
			}
			for _, output := range extracted.Outputs {
				group.Outputs = append(group.Outputs, GroupOutput{
					Output:    output.Output,
					Advantage: output.Advantage,
				})
			}
			err = o.trainingDataMessageList.AddAdvertisement(o.ctx, o.Rdb, RedisTrainingAdvList, string(tgid), group)
			if err != nil {
				// maybe this shouldn't be fatal
				o.logger.Fatal().Err(err).Msg("error adding advertisement")
			}
		}
	}
	o.mu.Lock()
	for _, bt := range o.RepoGraph.BranchTargets {
		for _, cg := range bt.Subgraphs {
			setupAdvertisements(CommitGraphLocator{
				BranchTargetLocator: BranchTargetLocator{BranchName: bt.BranchName},
				GoalID:              cg.GoalID,
			})
		}
	}
	o.mu.Unlock()

	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("trainingInput listener closing")
			return
		// flag: rg cannot change under us
		case cgl := <-o.RepoGraph.ShouldAdvertiseChan:
			o.mu.Lock()
			setupAdvertisements(cgl)
			o.mu.Unlock()
		}
	}
}

func (o *Orchestrator) startTrainingRx() {
	defer o.wg.Done()
	for {
		select {
		case <-o.ctx.Done():
			o.logger.Info().Msg("trainingInput listener closing")
			return
		default:
		}
		request, err := ReadNextTrainingRequest(o.ctx, o.Rdb)
		if err != nil {
			o.logger.Error().Err(err).Msg("error reading next training request")
			continue
		}
		group, ok := o.trainingDataMessageList.Get(string(request))
		if !ok {
			o.logger.Error().Str("request", string(request)).Msg("error getting training data group")
			continue
		}
		err = o.Rdb.LPush(o.ctx, RedisTrainingTxChan, group).Err()
		if err != nil {
			o.logger.Fatal().Err(err).Msg("error sending training data group")
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
