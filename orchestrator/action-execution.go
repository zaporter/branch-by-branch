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

type Result struct {
	ActionName string `json:"action_name"`
	// merge of stdout and stderr
	Out      string `json:"out"`
	ExitCode int    `json:"exit_code"`
}

type CompilationResult struct {
	// The processing script has the ability to keep the old branch if no files have been changed
	BranchName         BranchName `json:"branch_name"`
	PreCommandsResults []Result   `json:"pre_commands_results"`
	CompilationResult  Result     `json:"compilation_result"`
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

func CompilationResultFromJSON(input string) CompilationResult {
	var result CompilationResult
	err := json.Unmarshal([]byte(input), &result)
	if err != nil {
		panic(err)
	}
	return result
}
