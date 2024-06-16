package cmd

import (
	"context"
	"encoding/json"
	"github.com/kaytu-io/kaytu-agent/config"
	"go.uber.org/zap"
	"os"
	"sync"
	"time"
)

var dbLock = sync.Mutex{}

type Optimization struct {
	Command    string    `json:"command"`
	LastUpdate time.Time `json:"lastUpdate"`
}

type DBConfig struct {
	Optimizations []Optimization `json:"optimizations"`
}

func OpenDBConfig(ctx context.Context, logger *zap.Logger, cfg *config.Config) (*DBConfig, error) {
	if err := ctx.Err(); err != nil {
		logger.Error("context error", zap.Error(err))
		return nil, err
	}

	dbLock.Lock()
	defer dbLock.Unlock()

	if err := ctx.Err(); err != nil {
		logger.Error("context error", zap.Error(err))
		return nil, err
	}

	// TODO move db to sqlite sidecar/separate pod
	var dbContent []byte
	if _, err := os.Stat(cfg.GetDBFilePath()); err != nil && os.IsNotExist(err) {
		dbContent, err = json.Marshal(DBConfig{})
		if err != nil {
			logger.Error("failed to marshal db file", zap.Error(err))
			return nil, err
		}
		err = os.WriteFile(cfg.GetDBFilePath(), dbContent, os.ModePerm)
		if err != nil {
			logger.Error("failed to write db file", zap.Error(err))
			return nil, err
		}
	} else {
		dbContent, err = os.ReadFile(cfg.GetDBFilePath())
		if err != nil {
			logger.Error("failed to read db file", zap.Error(err))
			return nil, err
		}
	}
	var opConfig DBConfig
	err := json.Unmarshal(dbContent, &opConfig)
	if err != nil {
		logger.Error("failed to unmarshal db file", zap.Error(err))
		return nil, err
	}

	return &opConfig, nil
}

func UpdateDBConfig(ctx context.Context, logger *zap.Logger, cfg *config.Config, opConfig *DBConfig) error {
	if err := ctx.Err(); err != nil {
		logger.Error("context error", zap.Error(err))
		return err
	}

	dbLock.Lock()
	defer dbLock.Unlock()

	if err := ctx.Err(); err != nil {
		logger.Error("context error", zap.Error(err))
		return err
	}

	dbContent, err := json.Marshal(opConfig)
	if err != nil {
		logger.Error("failed to marshal db file", zap.Error(err))
		return err
	}
	err = os.WriteFile(cfg.GetDBFilePath(), dbContent, os.ModePerm)
	if err != nil {
		logger.Error("failed to write db file", zap.Error(err))
		return err
	}

	return nil
}
