package main

import (
	"encoding/json"
)

type CompilationTask struct {
	BranchName BranchName `json:"branch_name"`
}
type CompilationResult struct {
	BranchName BranchName `json:"branch_name"`
	Message    string     `json:"message"`
	ExitCode   int        `json:"exit_code"`
}

func (t CompilationTask) ToJSON() string {
	json, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(json)
}

func CompilationTaskFromJSON(input string) CompilationTask {
	var task CompilationTask
	err := json.Unmarshal([]byte(input), &task)
	if err != nil {
		panic(err)
	}
	return task
}

func ExecuteAction(action XMLAction) (string, error) {
	return "", nil

}
