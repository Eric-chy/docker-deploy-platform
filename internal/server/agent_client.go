package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type NodeDeployRequest struct {
	TaskID      string `json:"task_id"`
	Project     string `json:"project"`
	Version     string `json:"version"`
	ComposeYAML string `json:"compose_yaml"`
	BeforeYAML  string `json:"before_yaml"`
	Action      string `json:"action"`
}

type NodeDeployResponse struct {
	Status string `json:"status"`
	TaskID string `json:"task_id"`
}

type NodeHealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type NodeClient struct {
	client *http.Client
}

func NewNodeClient() *NodeClient {
	return &NodeClient{client: &http.Client{Timeout: 5 * time.Second}}
}

func (client *NodeClient) Deploy(ctx context.Context, endpoint, apiKey string, request NodeDeployRequest) (NodeDeployResponse, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return NodeDeployResponse{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/api/deploy", bytes.NewReader(body))
	if err != nil {
		return NodeDeployResponse{}, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	httpResponse, err := client.client.Do(httpRequest)
	if err != nil {
		return NodeDeployResponse{}, err
	}
	defer httpResponse.Body.Close()

	var response NodeDeployResponse
	decoder := json.NewDecoder(httpResponse.Body)
	if err := decoder.Decode(&response); err != nil {
		return NodeDeployResponse{}, fmt.Errorf("decode node response: %w", err)
	}
	if httpResponse.StatusCode == http.StatusAccepted || httpResponse.StatusCode == http.StatusOK {
		return response, nil
	}
	return response, fmt.Errorf("node returned %d", httpResponse.StatusCode)
}

func (client *NodeClient) Health(ctx context.Context, endpoint, apiKey string) (NodeHealthResponse, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health", nil)
	if err != nil {
		return NodeHealthResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)

	httpResponse, err := client.client.Do(httpRequest)
	if err != nil {
		return NodeHealthResponse{}, err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return NodeHealthResponse{}, fmt.Errorf("node returned %d", httpResponse.StatusCode)
	}

	var response NodeHealthResponse
	if err := json.NewDecoder(httpResponse.Body).Decode(&response); err != nil {
		return NodeHealthResponse{}, fmt.Errorf("decode node response: %w", err)
	}
	if response.Status != "ok" {
		return NodeHealthResponse{}, fmt.Errorf("invalid node status %q", response.Status)
	}
	return response, nil
}
