package main

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

type RedisKey string

const (
	RedisInferenceEnabled    RedisKey = "inference:enabled"
	RedisInferenceModelDir   RedisKey = "inference:model_dir"
	RedisInferenceAdapterDir RedisKey = "inference:adapter_dir"
)

var AllRouterKeys = []RedisKey{
	RedisInferenceEnabled,
	RedisInferenceModelDir,
	RedisInferenceAdapterDir,
}

func createRouterParamsCli() *cli.Command {
	set := false
	read := false
	toSet := ""
	valToSet := ""
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		rdb, err := connectToRedis(ctx)
		if err != nil {
			return err
		}
		if set {
			if toSet == "" {
				return errors.New("key is required")
			}
			if valToSet == "" {
				return errors.New("value is required")
			}
			var statusCmd *redis.StatusCmd
			for _, key := range AllRouterKeys {
				if string(key) == toSet {
					statusCmd = rdb.Set(ctx, string(key), valToSet, 0)
					break
				}
			}
			if statusCmd == nil {
				return errors.New("invalid key")
			}
			if statusCmd.Err() != nil {
				return statusCmd.Err()
			}
			logger.Info().Msgf("set inference params %s=%s", toSet, valToSet)
		} else if read {
			if toSet != "" {
				return errors.New("read with key is not supported")
			}
			for _, key := range AllRouterKeys {
				val, err := rdb.Get(ctx, string(key)).Result()
				if err != nil {
					return err
				}
				logger.Info().Msgf("%s: %s", key, val)
			}
		} else {
			return errors.New("no action specified")
		}
		return nil
	}

	return &cli.Command{
		Name:   "params",
		Usage:  "router params",
		Action: action,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "set",
				Usage:       "set router params",
				Destination: &set,
			},
			&cli.BoolFlag{
				Name:        "read",
				Usage:       "read router params",
				Destination: &read,
			},
		},
		ArgsUsage: "[key] [value]",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:        "key",
				Destination: &toSet,
				Max:         1,
			},
			&cli.StringArg{
				Name:        "value",
				Destination: &valToSet,
				Max:         1,
			},
		},
	}
}

func askForConfirmation(ctx context.Context, msg string) bool {
	reader := bufio.NewReader(os.Stdin)
	logger := zerolog.Ctx(ctx)
	logger.Info().Msgf("%s (y/n): ", msg)
	response, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(response)) == "y"
}

func createInitializeRouterParamsCli() *cli.Command {
	action := func(ctx context.Context, _ *cli.Command) error {
		if !askForConfirmation(ctx, "Are you sure you want to initialize the params? This will overwrite all existing params.") {
			return nil
		}
		logger := zerolog.Ctx(ctx)
		rdb, err := connectToRedis(ctx)
		if err != nil {
			return err
		}
		valsMap := map[RedisKey]string{
			RedisInferenceEnabled:    "true",
			RedisInferenceModelDir:   "/models",
			RedisInferenceAdapterDir: "/adapters",
		}
		for key, val := range valsMap {
			statusCmd := rdb.Set(ctx, string(key), val, 0)
			if statusCmd.Err() != nil {
				return statusCmd.Err()
			}
			logger.Info().Msgf("set %s=%s", key, val)
		}
		return nil
	}
	return &cli.Command{
		Name:   "init",
		Usage:  "initialize router params",
		Action: action,
	}
}

func createRouterCli() *cli.Command {
	return &cli.Command{
		Name:  "router",
		Usage: "router",
		Commands: []*cli.Command{
			createRouterParamsCli(),
			createInitializeRouterParamsCli(),
		},
	}
}
