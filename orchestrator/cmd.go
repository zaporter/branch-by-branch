package main

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
	"github.com/zaporter/branch-by-branch/orchestrator/lambda"
)

func qfStart() *cli.Command {
	type instReq struct {
		Type        *regexp.Regexp
		Count       int
		RegionMatch *regexp.Regexp
		SetupCmd    string
	}
	instanceRequests := map[string]*instReq{
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
	type qfInstance struct {
		instType   string
		instRegion string
		instName   string
	}
	wouldSatisfy := func(qfInst qfInstance, key string, req *instReq) bool {
		return req.Type.MatchString(qfInst.instType) &&
			req.RegionMatch.MatchString(qfInst.instRegion) &&
			(qfInst.instName == "unnamed" || strings.HasPrefix(qfInst.instName, key+"-"))
	}
	getSatisfiedReq := func(qfInst qfInstance, reqs map[string]*instReq) string {
		for key, req := range reqs {
			if req.Count == 0 {
				continue
			}
			if wouldSatisfy(qfInst, key, req) {
				return key
			}
		}
		return ""
	}
	unsatisfiedReqs := func(reqs map[string]*instReq) []string {
		unsatisfied := []string{}
		for key, req := range reqs {
			if req.Count > 0 {
				unsatisfied = append(unsatisfied, key)
			}
		}
		return unsatisfied
	}
	getNextInstanceName := func(prefix string, names []string) string {
		for i := 1; ; i++ {
			name := fmt.Sprintf("%s-%d", prefix, i)
			if !slices.Contains(names, name) {
				return name
			}
		}
	}

	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		logger.Info().Msg("qfStart")
		rdb, err := connectToRedis(ctx)
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

		instances, err := lambda.ListInstances()
		if err != nil {
			return err
		}
		instanceNames := []string{}
		for _, inst := range instances.Data {
			instanceNames = append(instanceNames, inst.Name)
		}
		wg := sync.WaitGroup{}
		pushAndExec := func(id string, cmd string) {
			defer wg.Done()
			err := lambda.PushAndExecOnLambdaInstance(id, 150, cmd)
			if err != nil {
				logger.Error().Err(err).Msgf("failed to push and exec on instance %s", id)
			}
			logger.Info().Msgf("Started %s on instance %s", cmd, id)
		}
		for _, inst := range instances.Data {
			if inst.Status == "terminating" {
				continue
			}
			qfInst := qfInstance{
				instType:   inst.InstanceType.Name,
				instRegion: inst.Region.Name,
				instName:   inst.Name,
			}
			satisfiedReq := getSatisfiedReq(qfInst, instanceRequests)
			if satisfiedReq != "" {
				instanceRequests[satisfiedReq].Count--
				wg.Add(1)
				go pushAndExec(inst.ID, instanceRequests[satisfiedReq].SetupCmd)
			}
		}
		for len(unsatisfiedReqs(instanceRequests)) > 0 {
			logger.Info().Msgf("Unsatisfied requests: %v", unsatisfiedReqs(instanceRequests))
			availableInstances, err := lambda.GetInstanceTypes()
			if err != nil {
				logger.Error().Err(err).Msg("failed to get instance types")
				continue
			}
			for _, instType := range availableInstances.Data {
				if instType.RegionsWithCapacityAvailable != nil {
				reserveFailedLoopContinue:
					for _, region := range instType.RegionsWithCapacityAvailable {
						qfInst := qfInstance{
							instType:   instType.InstanceType.Name,
							instRegion: region.Name,
							instName:   "unnamed",
						}
						satisfiedReq := getSatisfiedReq(qfInst, instanceRequests)
						// keep trying to reserve instance until all requests are satisfied
						for ; satisfiedReq != ""; satisfiedReq = getSatisfiedReq(qfInst, instanceRequests) {
							// try to reserve instance
							instName := getNextInstanceName(satisfiedReq, instanceNames)
							launchReq := lambda.LaunchRequest{
								RegionName:       qfInst.instRegion,
								InstanceTypeName: qfInst.instType,
								SSHKeyNames:      []string{"lambda-ssh"},
								Quantity:         1,
								Name:             instName,
							}
							logger.Info().Msgf("Launching instance %s in region %s, type %s", instName, region.Name, qfInst.instType)
							launchResp, err := lambda.LaunchInstances(launchReq)
							if err != nil {
								logger.Error().Err(err).Msgf("failed to launch instance %s in region %s", satisfiedReq, region.Name)
								continue reserveFailedLoopContinue
							}
							instanceNames = append(instanceNames, instName)
							instanceRequests[satisfiedReq].Count--
							logger.Info().Msgf("Launched instance ID: %v, name: %s, region: %s, type: %s",
								launchResp.Data.InstanceIDs, launchReq.Name, launchReq.RegionName, launchReq.InstanceTypeName)
							wg.Add(1)
							go pushAndExec(launchResp.Data.InstanceIDs[0], instanceRequests[satisfiedReq].SetupCmd)
						}
					}
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(1500 * time.Millisecond):
			}
		}
		logger.Info().Msg("All instances reserved. Waiting for all instances to start...")
		wg.Wait()
		logger.Info().Msg("All instances started")
		return nil
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
