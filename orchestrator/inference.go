package main

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createInferenceParamsCli() *cli.Command {
	set := false
	read := false
	toSet := ""
	valToSet := ""
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		if set {
			logger.Info().Msgf("set inference params %s=%s", toSet, valToSet)
		} else if read {
			logger.Info().Msg("read inference params")
			if toSet != "" {
				return errors.New("read with key is not supported")
			}
		}
		return nil
	}
	return &cli.Command{
		Name:   "params",
		Usage:  "inference params",
		Action: action,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "set",
				Usage:       "set inference params",
				Destination: &set,
			},
			&cli.BoolFlag{
				Name:        "read",
				Usage:       "read inference params",
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

func createInferenceCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("inference")
		return nil
	}
	return &cli.Command{
		Name:   "inference",
		Usage:  "inference",
		Action: action,
		Commands: []*cli.Command{
			createInferenceParamsCli(),
		},
	}
}
