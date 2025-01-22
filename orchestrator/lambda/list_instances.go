package lambda

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Instance represents a virtual machine (VM) in Lambda Cloud.
type Instance struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	IP              *string      `json:"ip"` // Pointer to handle nullable field
	Status          string       `json:"status"`
	SSHKeyNames     []string     `json:"ssh_key_names"`
	FileSystemNames []string     `json:"file_system_names"`
	Region          Region       `json:"region"`
	InstanceType    InstanceType `json:"instance_type"`
	Hostname        *string      `json:"hostname"` // Pointer to handle nullable field
}

// InstancesResponse represents the response structure for the instances API.
type InstancesResponse struct {
	Data []Instance `json:"data"`
}

// ListInstances makes a GET request to the instances API endpoint and returns the response.
func ListInstances() (*InstancesResponse, error) {
	username := os.Getenv("LAMBDA_USERNAME")
	password := os.Getenv("LAMBDA_PASSWORD")

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, "https://cloud.lambdalabs.com/api/v1/instances", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request failed: %v", err)
	}

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

	var instancesResponse InstancesResponse
	err = json.Unmarshal(body, &instancesResponse)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed: %v", err)
	}

	return &instancesResponse, nil
}
