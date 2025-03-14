package orchestrator

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofrs/uuid"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createTestDeferCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("test defer")
		defer fmt.Println("world")
		fmt.Println("hello")
		for i := 0; i < 10; i++ {
			fmt.Println("hello", i)
			defer fmt.Println(i)
		}
		return nil
	}
	return &cli.Command{
		Name:   "test-defer",
		Usage:  "test defer",
		Action: action,
	}
}

func createTestGoRoutineCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		type testStruct struct {
			Value int
		}
		input := make(chan testStruct, 1)
		i := 0
		var toInsert *testStruct
		for {
			select {
			case <-ticker.C:
				logger.Info().Msg("tick")
				if i == 10 {
					return nil
				}
				i++
				if i == 5 {
					toInsert = &testStruct{Value: i}
				}
				//damn
			case input <- *toInsert:
				logger.Info().Msgf("input: %d", i)
			}
		}
	}
	return &cli.Command{
		Name:   "test-goroutine",
		Action: action,
	}
}

func playgroundEngineStartupTestCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("simple engine test")
		rdb, err := ConnectToRedis(c)
		if err != nil {
			return err
		}
		schedulingParams := SchedulingParams{
			MinTaskQueueSize:      10,
			MaxTaskQueueSize:      100,
			TaskProcessingTimeout: 10 * time.Second,
			CamShaftInterval:      10 * time.Second,
			CrankShaftInterval:    10 * time.Second,
			TimingBeltInterval:    10 * time.Second,
			ODBInterval:           10 * time.Second,
			InputChanSize:         100,
			OutputChanSize:        100,
		}
		engine := NewEngine(c, EngineJobNameTest, rdb, schedulingParams)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			// main thread should wait for stop
			engine.TriggerStop()
		}()
		engine.Start(c)
		engine.WaitForStop()

		return nil
	}
	return &cli.Command{
		Name:   "engine-startup-test",
		Usage:  "engine startup test",
		Action: action,
	}
}

func playgroundEngineSimpleInferenceTestCli() *cli.Command {
	var numTasks int64
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("simple inference engine test")
		rdb, err := ConnectToRedis(c)
		if err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceEnabled, "true"); err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceBaseModel, "meta-llama/Llama-3.1-8B-Instruct"); err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceAdapter, ""); err != nil {
			return err
		}
		schedulingParams := SchedulingParams{
			MinTaskQueueSize:      10,
			MaxTaskQueueSize:      100,
			TaskProcessingTimeout: 1 * time.Minute,
			CamShaftInterval:      1 * time.Second,
			CrankShaftInterval:    1 * time.Second,
			TimingBeltInterval:    2 * time.Second,
			ODBInterval:           10 * time.Second,
			InputChanSize:         10,
			OutputChanSize:        10,
		}
		engine := NewEngine(c, EngineJobNameInference, rdb, schedulingParams)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			// main thread should wait for stop
			engine.TriggerStop()
		}()
		engine.Start(c)
		dieChan := make(chan bool, 1)
		input := engine.GetInput()
		go func() {
			for i := int64(0); i < numTasks; i++ {
				logger.Info().Msgf("enqueuing task %d", i)
				select {
				case <-dieChan:
					fmt.Println("die")
					return
				case input <- EngineTaskMsg{
					Task: InferenceTask{
						Prompt: fmt.Sprintf("A poem about the number %d", i),
					}.ToJSON(),
				}:
				}
			}
		}()
		output := engine.GetOutput()
		go func() {
			for {
				select {
				case <-dieChan:
					fmt.Println("die")
					return
				case val := <-output:
					fmt.Println("output", val)
					task := InferenceTaskResponseFromJSON(val.Result)
					fmt.Printf("task: %+v\n", task)
				}
			}
		}()

		engine.WaitForStop()
		close(dieChan)

		return nil
	}
	return &cli.Command{
		Name:   "engine-simple-inference-test",
		Usage:  "engine simple inference test",
		Action: action,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        "num-tasks",
				Usage:       "number of tasks to enqueue",
				Value:       100,
				Destination: &numTasks,
			},
		},
	}
}
func playgroundEngineSimpleCompilationTestCli() *cli.Command {
	var numTasks int64
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("simple compilation engine test")
		rdb, err := ConnectToRedis(c)
		if err != nil {
			return err
		}
		schedulingParams := SchedulingParams{
			MinTaskQueueSize:      10,
			MaxTaskQueueSize:      100,
			TaskProcessingTimeout: 1 * time.Minute,
			CamShaftInterval:      1 * time.Second,
			CrankShaftInterval:    1 * time.Second,
			TimingBeltInterval:    200 * time.Millisecond,
			ODBInterval:           10 * time.Second,
			InputChanSize:         10,
			OutputChanSize:        10,
		}
		engine := NewEngine(c, EngineJobNameCompilation, rdb, schedulingParams)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			// main thread should wait for stop
			engine.TriggerStop()
		}()
		engine.Start(c)
		dieChan := make(chan bool, 1)
		input := engine.GetInput()
		go func() {
			for i := int64(0); i < numTasks; i++ {
				logger.Info().Msgf("enqueuing task %d", i)
				select {
				case <-dieChan:
					fmt.Println("die")
					return
				case input <- EngineTaskMsg{
					Task: CompilationTask{
						BranchName:        BranchName("core-1"),
						NewBranchName:     NewBranchName(),
						CompilationScript: "lake build",
						PreCommands: []CompilationPreCommand{
							{
								Name:   "write",
								Script: "echo \"def hello2 : Nat := 1\" > Corelib/Hello.lean",
							},
							{
								Name:   "write test",
								Script: "cat << 'EOF' >> Test.lean\nexample : (P → Q) ∧ (Q → R) → (P → R) := by exact?\nEOF",
							},
							{
								Name:   "pwd",
								Script: "pwd",
							},
							{
								Name:   "mk_all",
								Script: "lake exec mk_all --lib Corelib",
							},
							{
								Name:   "prebuild",
								Script: "lake build",
							},
						},
					}.ToJSON(),
				}:
				}
			}
		}()
		output := engine.GetOutput()
		go func() {
			for {
				select {
				case <-dieChan:
					fmt.Println("die")
					return
				case val := <-output:
					fmt.Println("output", val)
					_ = CompilationTaskResponseFromJSON(val.Result)
				}
			}
		}()

		engine.WaitForStop()
		close(dieChan)

		return nil
	}
	return &cli.Command{
		Name:   "engine-simple-compilation-test",
		Usage:  "engine simple compilation test",
		Action: action,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:        "num-tasks",
				Usage:       "number of tasks to enqueue",
				Value:       100,
				Destination: &numTasks,
			},
		},
	}
}

// Very advanced playground test.
// This stands up an inference engine & training message handlers along with weighting & reward logic.
// This allows me to test GRPO training.
func playgroundGRPOLoopTestCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("GRPO loop test")

		rdb, err := ConnectToRedis(c)
		if err != nil {
			return err
		}
		rdb.Del(c, RedisTrainingAdvList)
		rdb.Set(c, string(RedisInferenceAdapter), "pissa_init", 0)
		rdb.Set(c, string(RedisInferenceEnabled), "true", 0)
		rdb.Set(c, string(RedisTrainingAdapter), "pissa_init", 0)
		inferenceSchedulingParams := SchedulingParams{
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
		err = rdb.Set(c, string(RedisInferenceEnabled), "true", 0).Err()
		if err != nil {
			return err
		}
		err = rdb.Set(c, string(RedisTrainingAdapter), "pissa_init", 0).Err()
		if err != nil {
			return err
		}
		err = rdb.Set(c, string(RedisInferenceAdapter), "pissa_init", 0).Err()
		if err != nil {
			return err
		}
		err = DropTrainingChans(c, rdb)
		if err != nil {
			return err
		}
		inferenceEngine := NewEngine(c, EngineJobNameInference, rdb, inferenceSchedulingParams)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			close(sigChan)
			// main thread should wait for stop
			inferenceEngine.TriggerStop()
		}()
		err = inferenceEngine.Start(c)
		if err != nil {
			return err
		}
		messageList := NewMessageList()
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
		for grpoIter := 0; grpoIter < 120; grpoIter++ {
			logger.Info().Msgf("GRPO iteration %d", grpoIter)
			select {
			case <-sigChan:
				logger.Info().Msg("stopping")
				return nil
			default:
			}
			taskIDToPrompt := map[EngineTaskID]string{}
			for _, prompt := range prompts {
				select {
				case <-sigChan:
					logger.Info().Msg("stopping")
					return nil
				default:
				}
				taskID := NewEngineTaskID()
				taskIDToPrompt[taskID] = prompt
				infTx <- EngineTaskMsg{ID: taskID, Task: InferenceTask{Prompt: prompt}.ToJSON()}
			}
			type output struct {
				TaskID EngineTaskID
				Output *InferenceTaskResponse
			}
			outputs := []*output{}
			for i := 0; i < len(prompts); i++ {
				select {
				case <-sigChan:
					logger.Info().Msg("stopping")
					return nil
				case msg := <-infRx:
					outputs = append(outputs, &output{TaskID: msg.ID, Output: InferenceTaskResponseFromJSON(msg.Result)})
				}
			}
			fmt.Println("outputs", outputs)
			for _, output := range outputs {
				allSame := true
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
				groupID := TrainingGroupID(uuid.String())
				data := TrainingDataGroup{
					GroupID: groupID,
					Prompt:  taskIDToPrompt[output.TaskID],
				}
				logger.Warn().Msgf("MEAN REWARD: %+v, %+v", meanReward, math.Sqrt(meanReward))
				for _, retSeq := range output.Output.ReturnSequences {
					data.Outputs = append(data.Outputs, GroupOutput{
						Output:    retSeq,
						Advantage: math.Sqrt(rewardFn(retSeq)) + ((rewardFn(retSeq) - meanReward) / rewardStdDev),
					})
				}
				logger.Info().Msgf("training data: %+v", data)
				err = messageList.AddAdvertisement(c, rdb, RedisTrainingAdvList, string(groupID), data)
				if err != nil {
					return err
				}
			}
			err = rdb.Set(c, string(RedisInferenceEnabled), "false", 0).Err()
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
				request, err := ReadNextTrainingRequest(c, rdb)
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
				err = rdb.LPush(c, RedisTrainingTxChan, group).Err()
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
				enabled, err := rdb.Get(c, string(RedisInferenceEnabled)).Result()
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

		inferenceEngine.WaitForStop()
		return nil
	}
	return &cli.Command{
		Name:   "grpo-loop-test",
		Usage:  "GRPO loop test",
		Action: action,
	}
}

func CreatePlaygroundCli() *cli.Command {
	return &cli.Command{
		Name:  "playground",
		Usage: "playground",
		Commands: []*cli.Command{
			createTestDeferCli(),
			createTestGoRoutineCli(),
			playgroundEngineStartupTestCli(),
			playgroundEngineSimpleInferenceTestCli(),
			playgroundEngineSimpleCompilationTestCli(),
			playgroundGRPOLoopTestCli(),
		},
	}
}
