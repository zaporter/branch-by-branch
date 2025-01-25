package main

import (
	"encoding/json"
)

type CompilationPreCommand struct {
	Name string `json:"name"`
	// Will be executed with /bin/bash -c
	// Ensure this is sanitized
	Script string `json:"script"`
}

type CompilationTask struct {
	BranchName        BranchName              `json:"branch_name"`
	NewBranchName     BranchName              `json:"new_branch_name"`
	PreCommands       []CompilationPreCommand `json:"pre_commands"`
	CompilationScript string                  `json:"compilation_script"`
}

type CompilationResult struct {
	ActionName string `json:"action_name"`
	// merge of stdout and stderr
	Out      string `json:"out"`
	ExitCode int    `json:"exit_code"`
}

type CompilationTaskResponse struct {
	// The processing script has the ability to keep the old branch if no files have been changed
	BranchName         BranchName          `json:"branch_name"`
	PreCommandsResults []CompilationResult `json:"pre_commands_results"`
	CompilationResult  CompilationResult   `json:"compilation_result"`
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

func CompilationTaskResponseFromJSON(input string) CompilationTaskResponse {
	var result CompilationTaskResponse
	err := json.Unmarshal([]byte(input), &result)
	if err != nil {
		panic(err)
	}
	return result
}
