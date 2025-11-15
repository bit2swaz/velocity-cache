package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type AuthFile struct {
	Token string `json:"token"`
}

func tokenFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	path := filepath.Join(configDir, "velocity")
	return filepath.Join(path, "auth.json"), nil
}

func SaveToken(token string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "velocity")
	if err := os.MkdirAll(configPath, 0o700); err != nil {
		return err
	}

	filePath := filepath.Join(configPath, "auth.json")

	data, err := json.MarshalIndent(AuthFile{Token: token}, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0o600)
}

func LoadToken() (string, error) {
	filePath, err := tokenFilePath()
	if err != nil {
		return "", err
	}

	payload, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}

	var data AuthFile
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("decode token file: %w", err)
	}

	if data.Token == "" {
		return "", errors.New("token is empty")
	}

	return data.Token, nil
}
