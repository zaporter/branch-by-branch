package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gofrs/uuid"
	"github.com/rs/zerolog"
	"github.com/zaporter/branch-by-branch/orchestrator"
)

type GrpoLoopExecutorConfig struct {
	NumLoops int `json:"num_loops"`
}

type GrpoLoopExecutor struct{}

var _ ExperimentExecutor = &GrpoLoopExecutor{}

type OutputData struct {
	Outputs []string `json:"outputs"`
}

func (o *OutputData) WriteTo(config *experimentConfig) error {
	bytes, err := json.MarshalIndent(o, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(config.FullPath, "output.json"), bytes, 0644)
}
func ReadOutputData(config *experimentConfig) (*OutputData, error) {
	bytes, err := os.ReadFile(filepath.Join(config.FullPath, "output.json"))
	if err != nil {
		return nil, err
	}
	var outputData OutputData
	if err := json.Unmarshal(bytes, &outputData); err != nil {
		return nil, err
	}
	return &outputData, nil
}

// Execute implements ExperimentExecutor.
func (n *GrpoLoopExecutor) Execute(ctx context.Context, config *experimentConfig) error {
	parsedConfig, err := readExperimentConfig[GrpoLoopExecutorConfig](config)
	outputData := &OutputData{}
	defer outputData.WriteTo(config)

	if err != nil {
		return err
	}
	logger := zerolog.Ctx(ctx)
	logger.Info().Msg("GRPO loop test")

	rdb, err := orchestrator.ConnectToRedis(ctx)
	if err != nil {
		return err
	}
	rdb.Del(ctx, orchestrator.RedisTrainingAdvList)
	inferenceSchedulingParams := orchestrator.SchedulingParams{
		MinTaskQueueSize:      4,
		MaxTaskQueueSize:      8,
		TaskProcessingTimeout: 3 * time.Minute,
		CamShaftInterval:      1 * time.Second,
		CrankShaftInterval:    1 * time.Second,
		TimingBeltInterval:    2 * time.Second,
		ODBInterval:           10 * time.Second,
		InputChanSize:         8,
		OutputChanSize:        8,
		DisableBackpressure:   true,
	}
	err = orchestrator.DropTrainingChans(ctx, rdb)
	if err != nil {
		return err
	}
	inferenceEngine := orchestrator.NewEngine(ctx, orchestrator.EngineJobNameInference, rdb, inferenceSchedulingParams)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		close(sigChan)
		// main thread should wait for stop
		inferenceEngine.TriggerStop()
	}()
	err = inferenceEngine.Start(ctx)
	if err != nil {
		return err
	}
	messageList := orchestrator.NewMessageList()
	infTx := inferenceEngine.GetInput()
	infRx := inferenceEngine.GetOutput()
	// rewardFn := func(output string) float64 {
	// numWords := strings.Split(strings.TrimSpace(output), " ")
	// return 1.0 / (math.Sqrt(math.Abs(float64(len(numWords)-10))) + 0.01 + rand.Float64()*0.001)
	// }
	rewardFn := func(output string) float64 {
		return 1.0 / (math.Sqrt(math.Abs(float64(len(output)-30))) + 0.1 + rand.Float64()*0.01)
	}
	prompts := []string{
		"A poem about the number 1",
		"My favorite color is blue",
		"I like to eat pizza because",
		"I like to sleep and it is",
	}
	for grpoIter := 0; grpoIter < parsedConfig.NumLoops; grpoIter++ {
		logger.Info().Msgf("GRPO iteration %d", grpoIter)
		select {
		case <-sigChan:
			logger.Info().Msg("stopping")
			return nil
		default:
		}
		taskIDToPrompt := map[orchestrator.EngineTaskID]string{}
		for _, prompt := range prompts {
			select {
			case <-sigChan:
				logger.Info().Msg("stopping")
				return nil
			default:
			}
			taskID := orchestrator.NewEngineTaskID()
			taskIDToPrompt[taskID] = prompt
			infTx <- orchestrator.EngineTaskMsg{ID: taskID, Task: orchestrator.InferenceTask{Prompt: prompt}.ToJSON()}
		}
		type output struct {
			TaskID orchestrator.EngineTaskID
			Output *orchestrator.InferenceTaskResponse
		}
		outputs := []*output{}
		for i := 0; i < len(prompts); i++ {
			select {
			case <-sigChan:
				logger.Info().Msg("stopping")
				return nil
			case msg := <-infRx:
				outputs = append(outputs, &output{TaskID: msg.ID, Output: orchestrator.InferenceTaskResponseFromJSON(msg.Result)})
			}
		}
		fmt.Println("outputs", outputs)
		for _, output := range outputs {
			allSame := true
			outputData.Outputs = append(outputData.Outputs, output.Output.ReturnSequences...)
			firstSeq := rewardFn(output.Output.ReturnSequences[0])
			for _, seq := range output.Output.ReturnSequences {
				if rewardFn(seq) != firstSeq {
					allSame = false
				}
			}
			if allSame {
				output.Output.ReturnSequences[0] = " and" + output.Output.ReturnSequences[0]
			}
			totalReward := 0.0
			for _, retSeq := range output.Output.ReturnSequences {
				totalReward += rewardFn(retSeq)
				fmt.Println("reward", rewardFn(retSeq))
				fmt.Println("retSeq", retSeq)
			}
			meanReward := totalReward / float64(len(output.Output.ReturnSequences))
			rewardVariance := 0.0
			for _, retSeq := range output.Output.ReturnSequences {
				rewardVariance += math.Pow(rewardFn(retSeq)-meanReward, 2)
			}
			rewardVariance /= float64(len(output.Output.ReturnSequences))
			rewardStdDev := math.Sqrt(rewardVariance)

			uuid, err := uuid.NewV4()
			if err != nil {
				return err
			}
			groupID := orchestrator.TrainingGroupID(uuid.String())
			data := orchestrator.TrainingDataGroup{
				GroupID: groupID,
				Prompt:  taskIDToPrompt[output.TaskID],
			}
			logger.Warn().Msgf("MEAN REWARD: %+v, %+v", meanReward, math.Sqrt(meanReward))
			for _, retSeq := range output.Output.ReturnSequences {
				data.Outputs = append(data.Outputs, orchestrator.GroupOutput{
					Output:    retSeq,
					Advantage: math.Sqrt(rewardFn(retSeq)) + ((rewardFn(retSeq) - meanReward) / rewardStdDev),
				})
			}
			logger.Info().Msgf("training data: %+v", data)
			err = messageList.AddAdvertisement(ctx, rdb, orchestrator.RedisTrainingAdvList, string(groupID), data)
			if err != nil {
				return err
			}
		}
		err = rdb.Set(ctx, string(orchestrator.RedisInferenceEnabled), "false", 0).Err()
		if err != nil {
			return err
		}
		numRequestsServed := 0
		for numRequestsServed < len(prompts) {
			select {
			case <-sigChan:
				logger.Info().Msg("stopping")
				return nil
			default:
			}
			request, err := orchestrator.ReadNextTrainingRequest(ctx, rdb)
			if err != nil {
				logger.Warn().Err(err).Msg("error reading next training request")
				continue
			}
			logger.Info().Msgf("request: %s", request)
			group, ok := messageList.Get(string(request))
			if !ok {
				return fmt.Errorf("group not found")
			}
			logger.Info().Msgf("group: %+v", group)
			err = rdb.LPush(ctx, orchestrator.RedisTrainingTxChan, group).Err()
			if err != nil {
				return err
			}
			numRequestsServed++
		}
		// wait until inference is reenabled (️🚩 len(prompts) must equal batch size)
		// TODO: do something more clever here
		logger.Info().Msg("waiting for inference to be reenabled")
		for {
			select {
			case <-sigChan:
				logger.Info().Msg("stopping")
				return nil
			default:
			}
			enabled, err := rdb.Get(ctx, string(orchestrator.RedisInferenceEnabled)).Result()
			if err != nil {
				return err
			}
			if enabled == "true" {
				break
			}
			time.Sleep(1 * time.Second)
		}
		logger.Info().Msg("inference enabled. Starting next iteration")
	}

	inferenceEngine.TriggerStop()
	inferenceEngine.WaitForStop()
	return nil
}

// GetStats implements ExperimentExecutor.
func (n *GrpoLoopExecutor) GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error) {
	outputData, err := ReadOutputData(config)
	if err != nil {
		return nil, err
	}
	outputLengths := []int{}
	for _, output := range outputData.Outputs {
		outputLengths = append(outputLengths, len(output))
	}
	return map[string]any{
		"output_lengths": outputLengths,
	}, nil
}
