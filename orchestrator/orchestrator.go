package main

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createOrchestratorStartCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("starting orchestrator")
		return nil
	}
	return &cli.Command{
		Name:   "start",
		Usage:  "start the orchestrator",
		Action: action,
	}
}

func createOrchestratorCli() *cli.Command {
	return &cli.Command{
		Name:    "orchestrator",
		Aliases: []string{"or"},
		Usage:   "orchestrator",
		Commands: []*cli.Command{
			createOrchestratorStartCli(),
		},
	}
}
