package main

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func testDefer() {
	defer fmt.Println("world")
	fmt.Println("hello")
	for i := 0; i < 10; i++ {
		fmt.Println("hello", i)
		defer fmt.Println(i)
	}
}

func createPlaygroundCli() *cli.Command {
	action := func(c context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(c)
		logger.Info().Msg("playground")
		testDefer()
		return nil
	}
	return &cli.Command{
		Name:   "playground",
		Usage:  "playground",
		Action: action,
	}
}
