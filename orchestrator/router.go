package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

type RedisKey string

const (
	RedisInferenceEnabled              RedisKey = "inference:enabled"
	RedisInferenceBaseModel            RedisKey = "inference:base_model"
	RedisInferenceAdapter              RedisKey = "inference:adapter"
	RedisInferenceBatchSize            RedisKey = "inference:batch_size"
	RedisInferenceLoadFormat           RedisKey = "inference:load_format"
	RedisInferenceMaxModelLen          RedisKey = "inference:max_model_len"
	RedisInferenceGpuMemoryUtilization RedisKey = "inference:gpu_memory_utilization"
	RedisInferenceMaxNewTokens         RedisKey = "inference:max_new_tokens"
	RedisInferenceNumReturnSequences   RedisKey = "inference:num_return_sequences"
	RedisInferenceNumBeams             RedisKey = "inference:num_beams"

	RedisTrainingBaseModel       RedisKey = "training:base_model"
	RedisTrainingAdapter         RedisKey = "training:adapter"
	RedisTrainingDoUpdateAdapter RedisKey = "training:do_update_adapter"
	RedisTrainingAutogroupTokens RedisKey = "training:autogroup_tokens"

	RedisExecutionRepoUrl RedisKey = "execution:repo_url"
)

var AllRouterKeys = []RedisKey{
	RedisInferenceEnabled,
	RedisInferenceBaseModel,
	RedisInferenceAdapter,
	RedisInferenceBatchSize,
	RedisInferenceLoadFormat,
	RedisInferenceMaxModelLen,
	RedisInferenceGpuMemoryUtilization,
	RedisInferenceMaxNewTokens,
	RedisInferenceNumReturnSequences,
	RedisInferenceNumBeams,

	RedisTrainingBaseModel,
	RedisTrainingAdapter,
	RedisTrainingDoUpdateAdapter,
	RedisTrainingAutogroupTokens,

	RedisExecutionRepoUrl,
}

func setRouterParam(ctx context.Context, rdb *redis.Client, key RedisKey, val string) error {
	return rdb.Set(ctx, string(key), val, 0).Err()
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
			// special case empty string
			if valToSet == "_" {
				valToSet = ""
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
				return errors.New("list with key is not supported")
			}
			for _, key := range AllRouterKeys {
				val, err := rdb.Get(ctx, string(key)).Result()
				if err != nil {
					return fmt.Errorf("error getting %s: %w", key, err)
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
				Aliases:     []string{"s"},
				Name:        "set",
				Usage:       "set router params",
				Destination: &set,
			},
			&cli.BoolFlag{
				Aliases:     []string{"l"},
				Name:        "list",
				Usage:       "list router params",
				Destination: &read,
			},
		},
		ArgsUsage: "[key] [value]",
		Aliases:   []string{"p"},
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
			RedisInferenceEnabled:              "true",
			RedisInferenceBaseModel:            "meta/llama-3.1-8-instruct",
			RedisInferenceAdapter:              "pissa_init",
			RedisInferenceLoadFormat:           "",
			RedisInferenceBatchSize:            "32",
			RedisInferenceMaxModelLen:          "512",
			RedisInferenceGpuMemoryUtilization: "0.85",
			RedisInferenceMaxNewTokens:         "128",
			RedisInferenceNumReturnSequences:   "3",
			RedisInferenceNumBeams:             "3",

			RedisTrainingBaseModel:       "meta/llama-3.1-8-instruct",
			RedisTrainingAdapter:         "pissa_init",
			RedisTrainingDoUpdateAdapter: "true",
			RedisTrainingAutogroupTokens: "700",

			RedisExecutionRepoUrl: os.Getenv("HOSTED_GIT_CONNECTION_STRING") + "zaporter/byb-v1.git",
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
		Name:    "router",
		Usage:   "router",
		Aliases: []string{"r"},
		Commands: []*cli.Command{
			createRouterParamsCli(),
			createInitializeRouterParamsCli(),
		},
	}
}
