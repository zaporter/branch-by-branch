package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
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
		Aliases: []string{"ex"},
		Usage:   "experiment for branch-by-branch",
		Commands: []*cli.Command{
			createExperimentRunCli(),
			createExperimentStatsCli(),
		},
	}
}

var executors = map[string]ExperimentExecutor{
	"noop":      &NoopExecutor{},
	"grpo_loop": &GrpoLoopExecutor{},
}

func createExperimentRunCli() *cli.Command {
	var (
		experimentsFolder string
		experimentGroup   string
		experiment        string
		noReserve         bool
		noSetParams       bool
	)
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		config := &experimentConfig{
			ExperimentsFolder: experimentsFolder,
			GroupFolder:       experimentGroup,
			ExperimentName:    experiment,
			FullPath:          filepath.Join(experimentsFolder, experimentGroup, experiment),
		}
		fullConfig, err := readExperimentConfig[map[string]any](config)
		if err != nil {
			return err
		}
		logger.Info().Msgf("running experiment %s with config %v", config.FullPath, fullConfig)
		result := &executionResult{
			StartTime: time.Now(),
			Config:    fullConfig,
		}
		if err := result.WriteTo(config); err != nil {
			return err
		}
		experimentConfig, err := readExperimentConfig[BaseExperimentConfig](config)
		if err != nil {
			return err
		}
		executor, ok := executors[experimentConfig.Executor]
		if !ok {
			return fmt.Errorf("executor %s not found", experimentConfig.Executor)
		}

		if !noSetParams && experimentConfig.RedisParams != nil {
			if err := setParams(ctx, experimentConfig.RedisParams); err != nil {
				return err
			}
		}

		if !noReserve && experimentConfig.InstanceRequests != nil {
			if err := reserveInstances(ctx, experimentConfig.InstanceRequests); err != nil {
				return err
			}
		}

		if !noSetParams && experimentConfig.RedisParams != nil {
			if err := setParams(ctx, experimentConfig.RedisParams); err != nil {
				return err
			}
		}

		err = executor.Execute(ctx, config)
		if err != nil {
			return err
		}
		result.EndTime = time.Now()
		logger.Info().Msgf("experiment %s finished in %s", config.FullPath, time.Since(result.StartTime))
		return result.WriteTo(config)
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
			&cli.BoolFlag{
				Name:        "no-reserve",
				Usage:       "don't reserve instances",
				Destination: &noReserve,
				Value:       false,
			},
			&cli.BoolFlag{
				Name:        "no-set-params",
				Usage:       "don't set params in redis",
				Destination: &noSetParams,
				Value:       false,
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
		logger := zerolog.Ctx(ctx)
		getStats := func(experimentName string) (map[string]any, error) {
			config := &experimentConfig{
				ExperimentsFolder: experimentsFolder,
				GroupFolder:       experimentGroup,
				ExperimentName:    experimentName,
				FullPath:          filepath.Join(experimentsFolder, experimentGroup, experimentName),
			}
			experimentConfig, err := readExperimentConfig[BaseExperimentConfig](config)
			if err != nil {
				return nil, err
			}
			executor, ok := executors[experimentConfig.Executor]
			if !ok {
				return nil, fmt.Errorf("executor %s not found", experimentConfig.Executor)
			}
			stats, err := executor.GetStats(ctx, config)
			if err != nil {
				return nil, err
			}
			return stats, nil
		}
		if experiment == "" {
			// Get stats for all experiments in the group
			experiments, err := os.ReadDir(filepath.Join(experimentsFolder, experimentGroup))
			if err != nil {
				return err
			}
			statsMap := make(map[string]any)
			for _, experiment := range experiments {
				if experiment.IsDir() && !strings.HasPrefix(experiment.Name(), ".") {
					// if the experiment has no result.json, it hasn't been run yet
					if _, err := os.Stat(filepath.Join(experimentsFolder, experimentGroup, experiment.Name(), "result.json")); os.IsNotExist(err) {
						logger.Warn().Msgf("experiment %s has no result.json, skipping", experiment.Name())
						continue
					}
					stats, err := getStats(experiment.Name())
					if err != nil {
						return err
					}
					statsMap[experiment.Name()] = stats
				}
			}
			logger.Info().Msgf("got stats for %d experiments. Writing to stats.json", len(statsMap))
			outputPath := filepath.Join(experimentsFolder, experimentGroup, "stats.json")
			bytes, err := json.Marshal(statsMap)
			if err != nil {
				return err
			}
			err = os.WriteFile(outputPath, bytes, 0644)
			if err != nil {
				return err
			}
			logger.Info().Msgf("wrote stats to %s", outputPath)
			logger.Info().Msgf("stats: %v", statsMap)
			return nil
		} else {
			stats, err := getStats(experiment)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(stats)
		}
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
				Usage:       "the experiment to run. Do not set this if you want to get stats for all experiments in the group",
				Destination: &experiment,
			},
		},
		Action: action,
	}
}

type executionResult struct {
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Config    map[string]any `json:"config"`
}

func (e *executionResult) WriteTo(config *experimentConfig) error {
	outputPath := filepath.Join(config.FullPath, "result.json")
	bytes, err := json.MarshalIndent(e, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, bytes, 0644)
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
	Executor         string                            `json:"executor"`
	RedisParams      map[string]any                    `json:"redis_params"`
	InstanceRequests map[string]instanceReservationReq `json:"instance_requests"`
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

	var baseConfig map[string]any
	if err := json.Unmarshal(mainExperimentConfigBytes, &baseConfig); err != nil {
		return emptyT, err
	}

	var overrideConfig map[string]any
	if err := json.Unmarshal(experimentOverrideBytes, &overrideConfig); err != nil {
		return emptyT, err
	}

	// Merge the override into the base config
	mergeMap(baseConfig, overrideConfig)

	mergedBytes, err := json.Marshal(baseConfig)
	if err != nil {
		return emptyT, err
	}

	var experimentConfig T
	if err := json.Unmarshal(mergedBytes, &experimentConfig); err != nil {
		return emptyT, err
	}

	return experimentConfig, nil
}

// mergeMap recursively merges override into base
func mergeMap(base, override map[string]any) {
	for key, overrideVal := range override {
		if baseVal, ok := base[key]; ok {
			// If both values are maps, merge them recursively
			if baseMap, isBaseMap := baseVal.(map[string]any); isBaseMap {
				if overrideMap, isOverrideMap := overrideVal.(map[string]any); isOverrideMap {
					mergeMap(baseMap, overrideMap)
					continue
				}
			}
		}
		// For all other cases, override the value
		base[key] = overrideVal
	}
}
