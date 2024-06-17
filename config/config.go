package config

import (
	"os"
	"path/filepath"
)

type PrometheusConfig struct {
	Address string `json:"address" yaml:"address" koanf:"address"`

	Username string `json:"username" yaml:"username" koanf:"username"`
	Password string `json:"password" yaml:"password" koanf:"password"`

	ClientId     string `json:"clientId" yaml:"clientId" koanf:"client_id"`
	ClientSecret string `json:"clientSecret" yaml:"clientSecret" koanf:"client_secret"`
	TokenUrl     string `json:"tokenUrl" yaml:"tokenUrl" koanf:"token_url"`
	Scopes       string `json:"scopes" yaml:"scopes" koanf:"scopes"`
}

type KaytuConfig struct {
	ObservabilityDays int              `json:"observabilityDays" yaml:"observabilityDays" koanf:"observability_days"`
	Prometheus        PrometheusConfig `json:"prometheus" yaml:"prometheus" koanf:"prometheus"`
	AuthToken         string           `json:"authToken" yaml:"authToken" koanf:"auth_token"`
}

type Config struct {
	GrpcPort         uint16 `json:"grpcPort" yaml:"grpcPort" koanf:"grpc_port"`
	WorkingDirectory string `json:"workingDirectory" yaml:"workingDirectory" koanf:"working_directory"`

	KaytuConfig KaytuConfig `json:"kaytuConfig" yaml:"kaytuConfig" koanf:"kaytu_config"`
}

var userHomeDir, _ = os.UserHomeDir()

var DefaultConfig = Config{
	GrpcPort:         8001,
	WorkingDirectory: filepath.Join(userHomeDir, ".kaytu", "agent"),

	KaytuConfig: KaytuConfig{
		ObservabilityDays: 14,
		Prometheus:        PrometheusConfig{},
	},
}

// We'll use functions to get computed fields

func (c Config) GetOutputDirectory() string {
	return filepath.Join(c.WorkingDirectory, "output")
}

func (c Config) GetDBFilePath() string {
	return filepath.Join(c.WorkingDirectory, "db.json")
}
