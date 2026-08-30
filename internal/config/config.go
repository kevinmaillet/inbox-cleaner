package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	Directory       string
	CredentialsFile string
	TokenFile       string
}

func DefaultPaths() (Paths, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("find user config directory: %w", err)
	}
	dir = filepath.Join(dir, "inboxcleaner")
	return Paths{
		Directory:       dir,
		CredentialsFile: filepath.Join(dir, "credentials.json"),
		TokenFile:       filepath.Join(dir, "token.json"),
	}, nil
}

func EnsureDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	return nil
}
