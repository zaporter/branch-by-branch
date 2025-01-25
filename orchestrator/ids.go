package main

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
