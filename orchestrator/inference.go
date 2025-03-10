package orchestrator

import "encoding/json"

type InferenceTask struct {
	Prompt string `json:"prompt"`
}

type InferenceTaskResponse struct {
	ReturnSequences []string `json:"return_sequences"`
}

func (i InferenceTask) ToJSON() string {
	json, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	return string(json)
}

func InferenceTaskResponseFromJSON(val string) *InferenceTaskResponse {
	var i InferenceTaskResponse
	err := json.Unmarshal([]byte(val), &i)
	if err != nil {
		panic(err)
	}
	return &i
}
