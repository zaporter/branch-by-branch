package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"github.com/zaporter/branch-by-branch/orchestrator/lambda"
)

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()
	ctx := logger.WithContext(context.Background())

	cmd := &cli.Command{
		Name:  "o",
		Usage: "orchestrator for branch-by-branch",
		Commands: []*cli.Command{
			createPlaygroundCli(),
			createRouterCli(),
			lambda.CreateLambdaCli(),
			createOrchestratorCli(),
			createGraphCreateCli(),
			createGoalFileCli(),
		},
	}
	if err := cmd.Run(ctx, os.Args); err != nil {
		log.Fatalln(err)
	}
}
