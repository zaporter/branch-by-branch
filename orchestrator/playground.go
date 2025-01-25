package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		rdb, err := connectToRedis(c)
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
		rdb, err := connectToRedis(c)
		if err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceEnabled, "true"); err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceModelDir, "meta-llama/Llama-3.1-8B-Instruct"); err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceAdapterDir, ""); err != nil {
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
		rdb, err := connectToRedis(c)
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
					_ = CompilationResultFromJSON(val.Result)
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

func createPlaygroundCli() *cli.Command {
	return &cli.Command{
		Name:  "playground",
		Usage: "playground",
		Commands: []*cli.Command{
			createTestDeferCli(),
			createTestGoRoutineCli(),
			playgroundEngineStartupTestCli(),
			playgroundEngineSimpleInferenceTestCli(),
			playgroundEngineSimpleCompilationTestCli(),
		},
	}
}
