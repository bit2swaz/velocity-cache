package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type tokenFile struct {
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
	filePath, err := tokenFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	payload, err := json.Marshal(tokenFile{Token: token})
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}

	if err := os.WriteFile(filePath, payload, 0o600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}

	return nil
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

	var data tokenFile
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("decode token file: %w", err)
	}

	if data.Token == "" {
		return "", errors.New("token is empty")
	}

	return data.Token, nil
}
