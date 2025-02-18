package lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type LambdaAvailability struct {
	Time         time.Time `json:"time"`
	InstanceName string    `json:"instance_name"`
	Region       string    `json:"region"`
}

// outfile is a jsonl file
func scrapeAvailability(ctx context.Context, outfile string) error {
	logger := zerolog.Ctx(ctx)
	for {
		time.Sleep(10 * time.Second)
		instanceTypes, err := GetInstanceTypes()
		if err != nil {
			logger.Error().Err(err).Msg("getting instance types")
			continue
		}
		avails := []LambdaAvailability{}
		time := time.Now()
		for _, instanceType := range instanceTypes.Data {
			logger.Info().Str("instance_type", instanceType.InstanceType.Name).Msg("instance type")
			for _, region := range instanceType.RegionsWithCapacityAvailable {
				logger.Debug().Str("region", region.Name).Msg("region")
				avail := LambdaAvailability{
					Time:         time,
					InstanceName: instanceType.InstanceType.Name,
					Region:       region.Name,
				}
				avails = append(avails, avail)
			}
		}
		lines := []string{}
		for _, avail := range avails {
			bytes, err := json.Marshal(avail)
			if err != nil {
				logger.Error().Err(err).Msg("marshalling availability")
				continue
			}
			lines = append(lines, string(bytes)+"\n")
		}

		f, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			logger.Error().Err(err).Msg("opening file")
			continue
		}
		_, err = f.Write([]byte(strings.Join(lines, "")))
		if err1 := f.Close(); err1 != nil && err == nil {
			err = err1
		}
		if err != nil {
			logger.Error().Err(err).Msg("writing to file")
			continue
		}
		logger.Info().Msg("wrote to file")
	}
}

type regionInstanceType struct {
	Region       string
	InstanceType string
}

func printAvailabilityStats(ctx context.Context, outfile string) error {
	logger := zerolog.Ctx(ctx)
	avails := []LambdaAvailability{}
	bytes, err := os.ReadFile(outfile)
	if err != nil {
		logger.Error().Err(err).Msg("reading file")
		return err
	}
	lines := strings.Split(string(bytes), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		avail := LambdaAvailability{}
		err := json.Unmarshal([]byte(line), &avail)
		if err != nil {
			logger.Fatal().Err(err).Str("line", line).Msg("unmarshalling availability")
		}
		avails = append(avails, avail)
	}
	// Should already be sorted, but this is a sanity check
	// Optimize if this takes too long
	sort.Slice(avails, func(i, j int) bool {
		return avails[i].Time.Before(avails[j].Time)
	})
	allInstanceTypes := map[regionInstanceType]bool{}
	for _, avail := range avails {
		allInstanceTypes[regionInstanceType{Region: avail.Region, InstanceType: avail.InstanceName}] = true
	}
	instanceTypeKeys := []regionInstanceType{}
	for k := range allInstanceTypes {
		instanceTypeKeys = append(instanceTypeKeys, k)
	}
	sort.Slice(instanceTypeKeys, func(i, j int) bool {
		if instanceTypeKeys[i].InstanceType != instanceTypeKeys[j].InstanceType {
			return instanceTypeKeys[i].InstanceType < instanceTypeKeys[j].InstanceType
		}
		return instanceTypeKeys[i].Region < instanceTypeKeys[j].Region
	})
	for _, instanceType := range instanceTypeKeys {
		logger.Info().Str("instance_type", instanceType.InstanceType).Str("region", instanceType.Region).Msg("instance type")
	}

	availsForInstanceType := func(instanceType regionInstanceType) []LambdaAvailability {
		locAvails := []LambdaAvailability{}
		for _, avail := range avails {
			if avail.InstanceName == instanceType.InstanceType && avail.Region == instanceType.Region {
				locAvails = append(locAvails, avail)
			}
		}
		return locAvails
	}
	totalTimeForScrape := avails[len(avails)-1].Time.Sub(avails[0].Time)
	fmt.Printf("Total time for scrape: %s\n", totalTimeForScrape)

	for _, instanceType := range instanceTypeKeys {
		availRanges := []time.Duration{}
		availTimes := []time.Time{}
		var lastTime *time.Time
		var lastAvailStart *time.Time
		availsForMe := availsForInstanceType(instanceType)
		for i, avail := range availsForMe {
			if lastTime == nil {
				lastTime = &avail.Time
				lastAvailStart = &avail.Time
				continue
			}
			if i == len(availsForMe)-1 || avail.Time.Sub(*lastTime) > 15*time.Second {
				availRanges = append(availRanges, lastTime.Sub(*lastAvailStart))
				availTimes = append(availTimes, *lastAvailStart)
				lastAvailStart = &avail.Time
			}
			lastTime = &avail.Time
		}
		totalAvailTime := time.Duration(0)
		for _, availRange := range availRanges {
			totalAvailTime += availRange
		}
		avgAvailTime := totalAvailTime / time.Duration(len(availRanges))
		availPercentage := 100 * float64(totalAvailTime) / float64(totalTimeForScrape)

		unavailDurations := []time.Duration{}
		for i, availTime := range availTimes {
			if i == len(availTimes)-1 {
				continue
			}
			unavailDurations = append(unavailDurations, availTimes[i+1].Sub(availTime.Add(availRanges[i])))
		}
		minUnavailTime := time.Duration(math.MaxInt64)
		maxUnavailTime := time.Duration(0)
		totalUnavailTime := time.Duration(0)
		for _, unavailDuration := range unavailDurations {
			totalUnavailTime += unavailDuration
			if unavailDuration < minUnavailTime {
				minUnavailTime = unavailDuration
			}
			if unavailDuration > maxUnavailTime {
				maxUnavailTime = unavailDuration
			}
		}
		avgWaitTime := time.Duration(0)
		if len(unavailDurations) > 0 {
			avgWaitTime = totalUnavailTime / time.Duration(len(unavailDurations))
		}

		fmt.Print("\n\n")
		fmt.Printf("------ %s @ %s -------\n", instanceType.InstanceType, instanceType.Region)
		fmt.Printf("Total avail duration: %s\n", totalAvailTime)
		fmt.Printf("Avg avail duration: %s\n", avgAvailTime)
		fmt.Printf("Avail percentage: %f\n", availPercentage)
		fmt.Printf("Avg wait duration: %s\n", avgWaitTime)
		fmt.Printf("Total unavail duration: %s\n", totalUnavailTime)
		if len(unavailDurations) > 0 {
			fmt.Printf("Min unavail duration: %s\n", minUnavailTime)
			fmt.Printf("Max unavail duration: %s\n", maxUnavailTime)
		}
		fmt.Printf("Num avail ranges: %d\n", len(availRanges))
		fmt.Printf("Num avail times: %d\n", len(availTimes))
		fmt.Printf("Num avail messages: %d\n", len(availsForMe))
	}

	return nil
}
