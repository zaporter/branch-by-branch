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
