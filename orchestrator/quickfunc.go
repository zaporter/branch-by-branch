package orchestrator

import (
	"context"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"github.com/zaporter/branch-by-branch/orchestrator/lambda"
)

func qfStart() *cli.Command {
	instanceRequests := map[string]*lambda.InstanceRequest{
		"inf": {
			Type:        regexp.MustCompile("^gpu_1x_h100.*$"),
			Count:       1,
			RegionMatch: regexp.MustCompile("us-.*"),
			SetupCmd:    "/home/ubuntu/branch-by-branch/scripts/lambda-start-inference.sh",
		},
		"trn": {
			Type:        regexp.MustCompile("^gpu_1x_h100.*$"),
			Count:       1,
			RegionMatch: regexp.MustCompile("us-.*"),
			SetupCmd:    "/home/ubuntu/branch-by-branch/scripts/lambda-start-training.sh",
		},
	}
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		logger.Info().Msg("qfStart")
		rdb, err := ConnectToRedis(ctx)
		if err != nil {
			return err
		}
		defer rdb.Close()
		// if err := rdb.Set(ctx, string(RedisTrainingAdapter), "pissa_init", 0).Err(); err != nil {
		// 	return err
		// }
		// if err := rdb.Set(ctx, string(RedisInferenceAdapter), "pissa_init", 0).Err(); err != nil {
		// 	return err
		// }
		// if err := rdb.Set(ctx, string(RedisTrainingBaseModel), "zap/llama-3.1-8-r64", 0).Err(); err != nil {
		// 	return err
		// }
		// if err := rdb.Set(ctx, string(RedisInferenceBaseModel), "zap/llama-3.1-8-r64", 0).Err(); err != nil {
		// 	return err
		// }

		return lambda.ReserveInstances(ctx, instanceRequests)
	}
	return &cli.Command{
		Name:   "start",
		Action: action,
	}
}

func qfTest() *cli.Command {
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		logger.Info().Msg("qfTest")
		return nil
	}
	return &cli.Command{
		Name:   "test",
		Action: action,
	}
}

func CreateQuickfuncCli() *cli.Command {
	return &cli.Command{
		Name:     "quickfunc",
		Aliases:  []string{"qf"},
		Commands: []*cli.Command{qfTest(), qfStart()},
	}
}
