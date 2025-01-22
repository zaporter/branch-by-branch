package lambda

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// InstanceType represents the details of an instance type.
type InstanceType struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	PriceCentsPerHour int    `json:"price_cents_per_hour"`
	Specs             Specs  `json:"specs"`
}

// Specs represents the specifications of an instance type.
type Specs struct {
	VCPUs      int `json:"vcpus"`
	MemoryGiB  int `json:"memory_gib"`
	StorageGiB int `json:"storage_gib"`
}

// Region represents a region where an instance type is available.
type Region struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// InstanceTypeDetails represents the details of an instance type and its availability.
type InstanceTypeDetails struct {
	InstanceType                 InstanceType `json:"instance_type"`
	RegionsWithCapacityAvailable []Region     `json:"regions_with_capacity_available"`
}

// InstanceTypesResponse represents the response structure for the instance types API.
type InstanceTypesResponse struct {
	Data map[string]InstanceTypeDetails `json:"data"`
}

// GetInstanceTypes makes a GET request to the instance-types API endpoint and returns the response.
func GetInstanceTypes() (*InstanceTypesResponse, error) {
	username := os.Getenv("LAMBDA_USERNAME")
	password := os.Getenv("LAMBDA_PASSWORD")

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, "https://cloud.lambdalabs.com/api/v1/instance-types", nil)
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

	var instanceTypesResponse InstanceTypesResponse
	err = json.Unmarshal(body, &instanceTypesResponse)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response failed: %v", err)
	}

	return &instanceTypesResponse, nil
}
