package agent

import (
	"errors"
	"os"
	"strconv"
)

const Version = "0.1.0"

type Config struct {
	Addr          string
	DataDir       string
	APIKey        string
	ControlURL    string
	CallbackToken string
	DeployTimeout int
}

func LoadConfig() (Config, error) {
	apiKey := os.Getenv("AGENT_API_KEY")
	if len(apiKey) < 32 {
		return Config{}, errors.New("AGENT_API_KEY must be at least 32 characters")
	}
	controlURL := os.Getenv("AGENT_CONTROL_URL")
	if controlURL == "" {
		return Config{}, errors.New("AGENT_CONTROL_URL is required")
	}
	callbackToken := os.Getenv("AGENT_CALLBACK_TOKEN")
	if len(callbackToken) < 32 {
		return Config{}, errors.New("AGENT_CALLBACK_TOKEN must be at least 32 characters")
	}
	deployTimeout := 120
	if value := os.Getenv("AGENT_DEPLOY_TIMEOUT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return Config{}, errors.New("AGENT_DEPLOY_TIMEOUT must be a positive integer")
		}
		deployTimeout = parsed
	}

	return Config{
		Addr:          getenv("AGENT_ADDR", "127.0.0.1:8120"),
		DataDir:       getenv("AGENT_DATA_DIR", "/data/agent"),
		APIKey:        apiKey,
		ControlURL:    controlURL,
		CallbackToken: callbackToken,
		DeployTimeout: deployTimeout,
	}, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
