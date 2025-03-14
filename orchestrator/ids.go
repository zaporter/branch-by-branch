package orchestrator

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofrs/uuid"
)

type EngineTaskID string

func NewEngineTaskID() EngineTaskID {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return EngineTaskID(fmt.Sprintf("engine-task-%s", uuid.String()))
}

func IsValidEngineTaskID(id EngineTaskID) bool {
	return strings.HasPrefix(string(id), "engine-task-")
}

type GoalID string

func NewGoalID() GoalID {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return GoalID(fmt.Sprintf("goal-%s", uuid.String()))
}

// (other than `main`, we just want unique names that don't have to be human-readable)
type BranchName string

func NewBranchName() BranchName {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return BranchName(fmt.Sprintf("branch-%s", uuid.String()))
}

type NodeID string

func NewNodeID() NodeID {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return NodeID(fmt.Sprintf("node-%s", uuid.String()))
}

type RepoGraphID string

func NewRepoGraphID() RepoGraphID {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return RepoGraphID(fmt.Sprintf("repo-graph-%s", uuid.String()))
}

type TrainingGroupID string

func NewTrainingGroupID(graphName RepoGraphID, locator NodeLocator) TrainingGroupID {
	return TrainingGroupID(fmt.Sprintf("training-group/%s/%s/%s/%s", graphName, locator.CommitGraphLocator.BranchTargetLocator.BranchName, locator.CommitGraphLocator.GoalID, locator.NodeID))
}

func ParseTrainingGroupID(id TrainingGroupID) (RepoGraphID, NodeLocator, error) {
	parts := strings.Split(string(id), "/")
	if len(parts) != 5 {
		return "", NodeLocator{}, fmt.Errorf("invalid training group id")
	}
	return RepoGraphID(parts[1]), NodeLocator{
		CommitGraphLocator: CommitGraphLocator{
			BranchTargetLocator: BranchTargetLocator{
				BranchName: BranchName(parts[2]),
			},
			GoalID: GoalID(parts[3]),
		},
		NodeID: NodeID(parts[4]),
	}, nil
}

type ModelTreeNodeID string

func NewModelTreeNodeID(modelName string, adapterName string) ModelTreeNodeID {
	return ModelTreeNodeID(fmt.Sprintf("model-tree-node-%s-%s", modelName, adapterName))
}

type GoldenSampleID string

func NewGoldenSampleID() GoldenSampleID {
	uuid, err := uuid.NewV4()
	if err != nil {
		log.Fatalln(err)
	}
	return GoldenSampleID(fmt.Sprintf("golden-sample-%s", uuid.String()))
}
