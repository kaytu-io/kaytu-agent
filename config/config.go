package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	WorkingDirectory string `json:"workingDirectory" yaml:"workingDirectory" koanf:"working_directory"`
}

var userHomeDir, _ = os.UserHomeDir()

var DefaultConfig = Config{
	WorkingDirectory: filepath.Join(userHomeDir, ".kaytu", "agent"),
}

// We'll use functions to get computed fields

func (c Config) GetOutputDirectory() string {
	return filepath.Join(c.WorkingDirectory, "output")
}

func (c Config) GetDBFilePath() string {
	return filepath.Join(c.WorkingDirectory, "db.json")
}
