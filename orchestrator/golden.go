package orchestrator

import (
	"encoding/json"
	"os"
	"time"
)

const goldenSampleOutputfile = "golden_samples.jsonl"

type GoldenSampleType string

const (
	GoldenSampleTypeProofGeneration GoldenSampleType = "proof_generation"
)

type GoldenSample struct {
	ID          GoldenSampleID   `json:"id"`
	Type        GoldenSampleType `json:"type"`
	Timestamp   time.Time        `json:"timestamp"`
	RepoID      RepoGraphID      `json:"repo_id"`
	NodeLocator NodeLocator      `json:"node_locator"`
	Prompt      string           `json:"prompt"`
	Completion  string           `json:"completion"`
}

func (g *GoldenSample) Write() error {
	file, err := os.OpenFile(goldenSampleOutputfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	json, err := json.Marshal(g)
	if err != nil {
		return err
	}

	_, err = file.Write(append(json, '\n'))
	return err
}
