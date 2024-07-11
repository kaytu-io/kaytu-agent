package database

import (
	"context"
	"github.com/glebarez/sqlite"
	"github.com/kaytu-io/kaytu-agent/config"
	"go.uber.org/zap"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
	"moul.io/zapgorm2"
	"os"
	"time"
)

type AgentDatabase struct {
	db *gorm.DB
}

func NewAgentDatabase(ctx context.Context, logger *zap.Logger, cfg *config.Config) (*AgentDatabase, error) {
	if _, err := os.Stat(cfg.GetDBFilePath()); err != nil && os.IsNotExist(err) {
		_ = os.MkdirAll(cfg.WorkingDirectory, os.ModePerm)
	}

	gormLogger := zapgorm2.New(logger)
	gormLogger.IgnoreRecordNotFoundError = true
	gormLogger.SlowThreshold = time.Second

	db, err := gorm.Open(sqlite.Open(cfg.GetDBFilePath()), &gorm.Config{
		Logger: gormLogger.LogMode(glogger.Warn),
	})
	if err != nil {
		logger.Error("failed to open db", zap.Error(err))
		return nil, err
	}

	err = db.AutoMigrate(&OptimizationJob{})
	if err != nil {
		logger.Error("failed to auto migrate", zap.Error(err))
		return nil, err
	}

	return &AgentDatabase{
		db: db,
	}, nil
}

func (d *AgentDatabase) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}
