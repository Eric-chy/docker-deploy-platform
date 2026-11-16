package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Addr                string
	DatabasePath        string
	JWTSecret           []byte
	SecretEncryptionKey []byte
	CallbackToken       string
	StaticDir           string
	AllowedOrigin       string
	PublicURL           string
}

func LoadConfig() (Config, error) {
	secret := os.Getenv("DEPLOY_JWT_SECRET")
	if len(secret) < 32 {
		return Config{}, errors.New("DEPLOY_JWT_SECRET must be at least 32 characters")
	}
	encryptionKey := os.Getenv("DEPLOY_SECRET_KEY")
	if len(encryptionKey) != 32 {
		return Config{}, errors.New("DEPLOY_SECRET_KEY must be exactly 32 characters")
	}
	callbackToken := os.Getenv("DEPLOY_CALLBACK_TOKEN")
	if len(callbackToken) < 32 {
		return Config{}, errors.New("DEPLOY_CALLBACK_TOKEN must be at least 32 characters")
	}

	dataDir := getenv("DEPLOY_DATA_DIR", "/data/control")
	staticDir := os.Getenv("DEPLOY_STATIC_DIR")
	if staticDir == "" {
		staticDir = "web/dist"
	}

	return Config{
		Addr:                getenv("DEPLOY_ADDR", "127.0.0.1:8080"),
		DatabasePath:        filepath.Join(dataDir, "control.sqlite"),
		JWTSecret:           []byte(secret),
		SecretEncryptionKey: []byte(encryptionKey),
		CallbackToken:       callbackToken,
		StaticDir:           staticDir,
		AllowedOrigin:       getenv("DEPLOY_ALLOWED_ORIGIN", ""),
		PublicURL:           strings.TrimRight(getenv("DEPLOY_PUBLIC_URL", "http://127.0.0.1:8080"), "/"),
	}, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
