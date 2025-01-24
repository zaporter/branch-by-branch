package main

import (
	"encoding/json"
)

type CompilationTask struct {
	BranchName    BranchName `json:"branch_name"`
	NewBranchName BranchName `json:"new_branch_name"`
	PreCommands   []struct {
		Name string `json:"name"`
		// Will be executed with /bin/bash -c
		// Ensure this is sanitized
		Script string `json:"script"`
	} `json:"pre_commands"`
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
	PreCotmandsResults []Result   `json:"pre_commands_results"`
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
