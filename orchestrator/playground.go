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

func playgroundEngineStartupTest() *cli.Command {
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

func playgroundEngineSimpleInferenceTest() *cli.Command {
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
		if err := setRouterParam(c, rdb, RedisInferenceModelDir, "/share/models/models"); err != nil {
			return err
		}
		if err := setRouterParam(c, rdb, RedisInferenceAdapterDir, ""); err != nil {
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
		engine := NewEngine(c, EngineJobNameInference, rdb, schedulingParams)
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

func createPlaygroundCli() *cli.Command {
	return &cli.Command{
		Name:  "playground",
		Usage: "playground",
		Commands: []*cli.Command{
			createTestDeferCli(),
			playgroundEngineStartupTest(),
		},
	}
}
