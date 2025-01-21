package main

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createPlaygroundCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("playground")
		return nil
	}
	return &cli.Command{
		Name:   "playground",
		Usage:  "playground",
		Action: action,
	}
}
