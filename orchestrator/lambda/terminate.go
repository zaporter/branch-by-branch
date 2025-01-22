package lambda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// TerminateRequest represents the JSON payload for terminating instances.
type TerminateRequest struct {
	InstanceIDs []string `json:"instance_ids"`
}

// TerminateResponse represents the response structure for the terminate instance operation.
type TerminateResponse struct {
	Data struct {
		TerminatedInstances []Instance `json:"terminated_instances"`
	} `json:"data"`
}

// TerminateInstances makes a POST request to the instance-operations/terminate API endpoint and returns the response.
func TerminateInstances(request TerminateRequest) (*TerminateResponse, error) {
	username := os.Getenv("LAMBDA_USERNAME")
	password := os.Getenv("LAMBDA_PASSWORD")

	// Convert the request struct to JSON
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling request failed: %v", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://cloud.lambdalabs.com/api/v1/instance-operations/terminate", bytes.NewBuffer(jsonRequest))
	if err != nil {
		return nil, fmt.Errorf("creating request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body failed: %v", err)
	}

	var terminateResponse TerminateResponse
	err = json.Unmarshal(body, &terminateResponse)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed: %v", err)
	}

	return &terminateResponse, nil
}
