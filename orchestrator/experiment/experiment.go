package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
)

/*
Experiments are organized via the filesystem.
Each experiment is grouped into an "experiment group" directory.
That folder must contain two files:
- a README.md that explains the purpose of the experiment
- a experiment.json that contains the experiment configuration
- This experiment.json has the following required fields:
 - executor: the name of the executor to use for the experiment
Then, each experiment is a subdirectory of the experiment group.
Within each experiment folder, there are two known files:
- a README.md that explains the purpose of the experiment (optional)
- a override.json which contains a subset of the experiment configuration with new values for the experiment.
	Try not to change the executor. It might work but I'm not sure I like that API lockin.

*/

func CreateExperimentCli() *cli.Command {
	return &cli.Command{
		Name:    "experiment",
		Aliases: []string{"e"},
		Usage:   "experiment for branch-by-branch",
		Commands: []*cli.Command{
			createExperimentRunCli(),
			createExperimentStatsCli(),
		},
	}
}

var executors = map[string]ExperimentExecutor{
	"noop": &NoopExecutor{},
}

func createExperimentRunCli() *cli.Command {
	var (
		experimentsFolder string
		experimentGroup   string
		experiment        string
	)
	action := func(ctx context.Context, _ *cli.Command) error {
		config := &experimentConfig{
			ExperimentsFolder: experimentsFolder,
			GroupFolder:       experimentGroup,
			ExperimentName:    experiment,
			FullPath:          filepath.Join(experimentsFolder, experimentGroup, experiment),
		}
		experimentConfig, err := readExperimentConfig[BaseExperimentConfig](config)
		if err != nil {
			return err
		}
		executor, ok := executors[experimentConfig.Executor]
		if !ok {
			return fmt.Errorf("executor %s not found", experimentConfig.Executor)
		}
		return executor.Execute(ctx, config)
	}
	return &cli.Command{
		Name:    "run",
		Aliases: []string{"r"},
		Usage:   "run an experiment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "experiments",
				Usage:       "the parent folder for the experiment groups",
				Destination: &experimentsFolder,
				Value:       "experiments",
			},
			&cli.StringFlag{
				Name:        "group",
				Aliases:     []string{"g"},
				Usage:       "the experiment group to run",
				Destination: &experimentGroup,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "experiment",
				Aliases:     []string{"e"},
				Usage:       "the experiment to run. Set to 'all' to run all experiments in the group",
				Destination: &experiment,
				Required:    true,
			},
		},
		Action: action,
	}
}

func createExperimentStatsCli() *cli.Command {
	var (
		experimentsFolder string
		experimentGroup   string
		experiment        string
	)
	action := func(ctx context.Context, _ *cli.Command) error {
		config := &experimentConfig{
			ExperimentsFolder: experimentsFolder,
			GroupFolder:       experimentGroup,
			ExperimentName:    experiment,
			FullPath:          filepath.Join(experimentsFolder, experimentGroup, experiment),
		}
		experimentConfig, err := readExperimentConfig[BaseExperimentConfig](config)
		if err != nil {
			return err
		}
		executor, ok := executors[experimentConfig.Executor]
		if !ok {
			return fmt.Errorf("executor %s not found", experimentConfig.Executor)
		}
		stats, err := executor.GetStats(ctx, config)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(stats)
	}
	return &cli.Command{
		Name:    "stats",
		Aliases: []string{"s"},
		Usage:   "get stats for an experiment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "experiments",
				Usage:       "the parent folder for the experiment groups",
				Destination: &experimentsFolder,
				Value:       "experiments",
			},
			&cli.StringFlag{
				Name:        "group",
				Aliases:     []string{"g"},
				Usage:       "the experiment group to run",
				Destination: &experimentGroup,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "experiment",
				Aliases:     []string{"e"},
				Usage:       "the experiment to run. Set to 'all' to run all experiments in the group",
				Destination: &experiment,
				Required:    true,
			},
		},
		Action: action,
	}
}

type experimentConfig struct {
	ExperimentsFolder string
	GroupFolder       string
	ExperimentName    string
	FullPath          string
}

type ExperimentExecutor interface {
	Execute(ctx context.Context, config *experimentConfig) error
	GetStats(ctx context.Context, config *experimentConfig) (map[string]any, error)
}

type BaseExperimentConfig struct {
	Executor string `json:"executor"`
}

func readExperimentConfig[T any](config *experimentConfig) (T, error) {
	var emptyT T
	mainExperimentConfigPath := filepath.Join(config.ExperimentsFolder, config.GroupFolder, "experiment.json")
	experimentOverridePath := filepath.Join(config.ExperimentsFolder, config.GroupFolder, config.ExperimentName, "override.json")
	mainExperimentConfigBytes, err := os.ReadFile(mainExperimentConfigPath)
	if err != nil {
		return emptyT, err
	}
	experimentOverrideBytes, err := os.ReadFile(experimentOverridePath)
	if err != nil {
		return emptyT, err
	}
	var experimentConfig T
	if err := json.Unmarshal(mainExperimentConfigBytes, &experimentConfig); err != nil {
		return emptyT, err
	}
	if err := json.Unmarshal(experimentOverrideBytes, &experimentConfig); err != nil {
		return emptyT, err
	}
	return experimentConfig, nil
}
