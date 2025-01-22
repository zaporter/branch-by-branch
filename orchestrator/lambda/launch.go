package lambda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// LaunchRequest represents the JSON payload for launching instances.
type LaunchRequest struct {
	RegionName       string   `json:"region_name"`
	InstanceTypeName string   `json:"instance_type_name"`
	SSHKeyNames      []string `json:"ssh_key_names"`
	FileSystemNames  []string `json:"file_system_names,omitempty"` // omitempty to allow for optional field
	Quantity         int      `json:"quantity,omitempty"`          // omitempty to use default if not specified
	Name             string   `json:"name,omitempty"`              // omitempty to allow for optional field
}

// LaunchResponse represents the response structure for the launch instance operation.
type LaunchResponse struct {
	Data struct {
		InstanceIDs []string `json:"instance_ids"`
	} `json:"data"`
}

// LaunchInstances makes a POST request to the instance-operations/launch API endpoint and returns the response.
func LaunchInstances(request LaunchRequest) (*LaunchResponse, error) {
	username := os.Getenv("LAMBDA_USERNAME")
	password := os.Getenv("LAMBDA_PASSWORD")

	// Convert the request struct to JSON
	jsonRequest, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshalling request failed: %v", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://cloud.lambdalabs.com/api/v1/instance-operations/launch", bytes.NewBuffer(jsonRequest))
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

	var launchResponse LaunchResponse
	err = json.Unmarshal(body, &launchResponse)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed: %v", err)
	}

	return &launchResponse, nil
}
