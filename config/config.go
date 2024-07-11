package config

import (
	"os"
	"path/filepath"
	"time"
)

const ConfigDirectory = "/config"

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
	ApiKey            string           `json:"apiKey" yaml:"apiKey" koanf:"api_key"`
}

type Config struct {
	GrpcPort         uint16 `json:"grpcPort" yaml:"grpcPort" koanf:"grpc_port"`
	WorkingDirectory string `json:"workingDirectory" yaml:"workingDirectory" koanf:"working_directory"`

	OptimizationCheckIntervalSeconds       int64 `json:"optimizationCheckIntervalSeconds" yaml:"optimizationCheckIntervalSeconds" koanf:"optimization_check_interval_seconds"`
	OptimizationJobScheduleIntervalSeconds int64 `json:"optimizationJobScheduleIntervalSeconds" yaml:"optimizationJobScheduleIntervalSeconds" koanf:"optimization_job_schedule_interval_seconds"`
	OptimizationJobRunTimeoutSeconds       int64 `json:"optimizationJobRunTimeoutSeconds" yaml:"optimizationJobRunTimeoutSeconds" koanf:"optimization_job_run_timeout_seconds"`
	OptimizationJobQueueTimeoutSeconds     int64 `json:"optimizationJobQueueTimeoutSeconds" yaml:"optimizationJobQueueTimeoutSeconds" koanf:"optimization_job_queue_timeout_seconds"`

	KaytuConfig KaytuConfig `json:"kaytuConfig" yaml:"kaytuConfig" koanf:"kaytu_config"`
}

var userHomeDir, _ = os.UserHomeDir()

var DefaultConfig = Config{
	GrpcPort:         8001,
	WorkingDirectory: filepath.Join(userHomeDir, ".kaytu", "agent"),

	OptimizationCheckIntervalSeconds:       60,
	OptimizationJobScheduleIntervalSeconds: 86400,
	OptimizationJobRunTimeoutSeconds:       7200,
	OptimizationJobQueueTimeoutSeconds:     86400,

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
	return filepath.Join(c.WorkingDirectory, "agent-sqlite.db")
}

func (c Config) GetOptimizationCheckInterval() time.Duration {
	return time.Duration(c.OptimizationCheckIntervalSeconds) * time.Second
}

func (c Config) GetOptimizationJobScheduleInterval() time.Duration {
	return time.Duration(c.OptimizationJobScheduleIntervalSeconds) * time.Second
}

func (c Config) GetOptimizationJobRunTimeout() time.Duration {
	return time.Duration(c.OptimizationJobRunTimeoutSeconds) * time.Second
}

func (c Config) GetOptimizationJobQueueTimeout() time.Duration {
	return time.Duration(c.OptimizationJobQueueTimeoutSeconds) * time.Second
}
