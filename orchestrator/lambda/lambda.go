package lambda

import (
	"context"
	"fmt"
	"sort"

	"github.com/rs/zerolog"
	"github.com/urfave/cli/v3"
)

func createInstanceTypesCli() *cli.Command {
	allBool := false
	action := func(_ context.Context, _ *cli.Command) error {
		instanceTypes, err := GetInstanceTypes()
		if err != nil {
			fmt.Printf("Error fetching instance types: %v\n", err)
			return nil
		}
		// get all values from instanceTypes.Data
		values := make([]InstanceTypeDetails, 0, len(instanceTypes.Data))
		for _, value := range instanceTypes.Data {
			values = append(values, value)
		}
		sort.Slice(values, func(i, j int) bool {
			return values[i].InstanceType.Name < values[j].InstanceType.Name
		})
		// Print the instance types details.
		for _, details := range values {
			if !allBool && len(details.RegionsWithCapacityAvailable) == 0 {
				continue
			}
			fmt.Printf("Instance Type: %s\n", details.InstanceType.Name)
			fmt.Printf("Description: %s\n", details.InstanceType.Description)
			fmt.Printf("Price (cents/hour): %v\n", details.InstanceType.PriceCentsPerHour)
			fmt.Printf("Specs: VCPUs: %d, Memory (GiB): %d, Storage (GiB): %d\n",
				details.InstanceType.Specs.VCPUs, details.InstanceType.Specs.MemoryGiB, details.InstanceType.Specs.StorageGiB)
			fmt.Println("Regions with capacity available:")
			for _, region := range details.RegionsWithCapacityAvailable {
				fmt.Printf("- %s (%s)\n", region.Name, region.Description)
			}
			fmt.Println()
		}
		return nil
	}
	return &cli.Command{
		Name: "instance-types",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "all",
				Destination: &allBool,
			},
		},
		Action: action,
	}
}
func createListInstancesCli() *cli.Command {
	action := func(_ context.Context, _ *cli.Command) error {
		instances, err := ListInstances()
		if err != nil {
			fmt.Printf("Error fetching instances: %v\n", err)
			return nil
		}

		// Print the instances details.
		for _, instance := range instances.Data {
			fmt.Printf("Instance ID: %s\n", instance.ID)
			fmt.Printf("Name: %s\n", instance.Name)
			fmt.Printf("IP: %s\n", stringValue(instance.IP))
			fmt.Printf("Status: %s\n", instance.Status)
			fmt.Printf("SSH Key Names: %v\n", instance.SSHKeyNames)
			fmt.Printf("File System Names: %v\n", instance.FileSystemNames)
			fmt.Printf("Region: %s\n", instance.Region.Name)
			fmt.Printf("Instance Type: %s\n", instance.InstanceType.Name)
			fmt.Printf("Hostname: %s\n", stringValue(instance.Hostname))
			fmt.Println()
		}
		return nil
	}
	return &cli.Command{
		Name:   "list",
		Flags:  []cli.Flag{},
		Action: action,
	}
}

func createLaunchCli() *cli.Command {
	instanceType := ""
	instanceRegion := ""
	instanceQuantity := int64(1)
	startInference := false
	action := func(ctx context.Context, _ *cli.Command) error {
		logger := zerolog.Ctx(ctx)
		filesystemNames := []string{}
		if instanceRegion == "us-west-2" {
			filesystemNames = []string{"cache-w2"}
		} else if instanceRegion == "us-west-1" {
			filesystemNames = []string{"cache-w1"}
		} else if instanceRegion == "us-west-3" {
			filesystemNames = []string{"cache-w3"}
		} else if instanceRegion == "us-east-1" {
			filesystemNames = []string{"cache-e1"}
		} else if instanceRegion == "us-south-1" {
			filesystemNames = []string{"cache-s1"}
		}

		if len(filesystemNames) == 0 {
			logger.Warn().Msgf("WARN: no filesystem names provided for region %s", instanceRegion)
		}
		launchRequest := LaunchRequest{
			RegionName:       instanceRegion,
			InstanceTypeName: instanceType,
			SSHKeyNames:      []string{"lambda-ssh"},
			FileSystemNames:  filesystemNames,
			Quantity:         int(instanceQuantity),
		}

		launchResponse, err := LaunchInstances(launchRequest)
		if err != nil {
			return err
		}

		// Print the instance IDs of the launched instances
		fmt.Println("Launched instance IDs:")
		for _, id := range launchResponse.Data.InstanceIDs {
			fmt.Println(id)
		}
		if startInference {
			for _, id := range launchResponse.Data.InstanceIDs {
				err := startInferenceOnLambda(id, 30)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}
	return &cli.Command{
		Name: "launch",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "type",
				Destination: &instanceType,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "region",
				Destination: &instanceRegion,
				Required:    true,
			},
			&cli.IntFlag{
				Name:        "num",
				Value:       instanceQuantity,
				Destination: &instanceQuantity,
			},
			&cli.BoolFlag{
				Name:        "start-inference",
				Usage:       "start inference on the launched instances",
				Value:       startInference,
				Destination: &startInference,
			},
		},
		Action: action,
	}
}

func createTerminateCli() *cli.Command {
	instanceID := ""
	all := false
	action := func(_ context.Context, _ *cli.Command) error {
		instances := []string{}
		if all {
			resp, err := ListInstances()
			if err != nil {
				return err
			}
			for _, i := range resp.Data {
				instances = append(instances, i.ID)
			}
		} else {
			instances = append(instances, instanceID)
		}
		terminateRequest := TerminateRequest{
			InstanceIDs: instances,
		}

		terminateResponse, err := TerminateInstances(terminateRequest)
		if err != nil {
			fmt.Printf("Error terminating instances: %v\n", err)
			return nil
		}

		// Print the instance IDs of the terminated instances
		fmt.Println("Terminated instance IDs:")
		for _, instance := range terminateResponse.Data.TerminatedInstances {
			fmt.Printf("Instance ID: %s, Name: %s\n", instance.ID, instance.Name)
		}
		return nil
	}
	return &cli.Command{
		Name: "terminate",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "id",
				Destination: &instanceID,
			},
			&cli.BoolFlag{
				Name:        "all",
				Destination: &all,
			},
		},
		Action: action,
	}
}
func createStartInferenceCli() *cli.Command {
	instanceID := ""
	action := func(_ context.Context, _ *cli.Command) error {
		return startInferenceOnLambda(instanceID, 30)
	}
	return &cli.Command{
		Name: "start-inference",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "id",
				Destination: &instanceID,
				Required:    true,
			},
		},
		Action: action,
	}
}
func createStatusCli() *cli.Command {
	cmd := "nvidia-smi"
	action := func(_ context.Context, _ *cli.Command) error {
		instances, err := ListInstances()
		if err != nil {
			fmt.Printf("Error fetching instances: %v\n", err)
			return nil
		}

		// Print the instances details.
		for _, instance := range instances.Data {
			fmt.Printf("Instance ID: %s\n", instance.ID)
			fmt.Printf("Name: %s\n", instance.Name)
			fmt.Printf("IP: %s\n", stringValue(instance.IP))
			fmt.Printf("Status: %s\n", instance.Status)
			fmt.Printf("File System Names: %v\n", instance.FileSystemNames)
			fmt.Printf("Region: %s\n", instance.Region.Name)
			fmt.Printf("Instance Type: %s\n", instance.InstanceType.Name)
			if instance.IP != nil && instance.Status != "terminating" {
				statusResult, err := execOnInstance(*instance.IP, cmd)
				if err != nil {
					fmt.Printf("StatusErr: %v\n", err)
				} else {
					fmt.Printf("Status:\n%v\n", statusResult)
				}
			}
			fmt.Println()
		}
		return nil
	}
	return &cli.Command{
		Name: "status",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "cmd",
				Value:       cmd,
				Destination: &cmd,
			},
		},
		Action: action,
	}
}

func CreateLambdaCli() *cli.Command {
	return &cli.Command{
		Name: "lambda",
		Commands: []*cli.Command{
			createInstanceTypesCli(),
			createListInstancesCli(),
			createLaunchCli(),
			createTerminateCli(),
			createStatusCli(),
			createStartInferenceCli(),
		},
	}
}

// stringValue safely returns the string value of a pointer to a string, or "N/A" if nil.
func stringValue(s *string) string {
	if s != nil {
		return *s
	}
	return "N/A"
}
