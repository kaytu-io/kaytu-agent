package database

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type OptimizationJobStatus string

const (
	OptimizationJobStatusCreated    OptimizationJobStatus = "CREATED"
	OptimizationJobStatusInProgress OptimizationJobStatus = "IN_PROGRESS"
	OptimizationJobStatusSucceeded  OptimizationJobStatus = "SUCCEEDED"
	OptimizationJobStatusFailed     OptimizationJobStatus = "FAILED"
	OptimizationJobStatusTimeout    OptimizationJobStatus = "TIMEOUT"
)

type OptimizationJob struct {
	gorm.Model
	Command      string                `json:"command" gorm:"index"`
	Status       OptimizationJobStatus `json:"status" gorm:"index"`
	ErrorMessage string                `json:"errorMessage"`
}

type OptimizationJobsRepo interface {
	CreateOptimizationJob(ctx context.Context, command string) error
	SetOptimizationJobStatus(ctx context.Context, id uint, status OptimizationJobStatus, errorMessage string) error
	GetOptimizationJob(ctx context.Context, id uint) (*OptimizationJob, error)
	GetCreatedOptimizationJobAndSetInProgress(ctx context.Context) (*OptimizationJob, error)
	GetLatestOptimizationJobByCommand(ctx context.Context, command string) (*OptimizationJob, error)
	TimeoutOutdatedOptimizationJobs(ctx context.Context, timeout time.Duration) error
}

type OptimizationJobsRepoImpl struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewOptimizationJobsRepo(db *AgentDatabase, logger *zap.Logger) *OptimizationJobsRepoImpl {
	return &OptimizationJobsRepoImpl{db: db.db, logger: logger}
}

func (r *OptimizationJobsRepoImpl) CreateOptimizationJob(ctx context.Context, command string) error {
	job := &OptimizationJob{
		Command: command,
		Status:  OptimizationJobStatusCreated,
	}
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *OptimizationJobsRepoImpl) SetOptimizationJobStatus(ctx context.Context, id uint, status OptimizationJobStatus, errorMessage string) error {
	return r.db.WithContext(ctx).Model(&OptimizationJob{}).Where("id = ?", id).Updates(map[string]any{
		"status":        status,
		"error_message": errorMessage,
	}).Error
}

func (r *OptimizationJobsRepoImpl) GetOptimizationJob(ctx context.Context, id uint) (*OptimizationJob, error) {
	job := &OptimizationJob{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(job).Error
	return job, err
}

func (r *OptimizationJobsRepoImpl) GetCreatedOptimizationJobAndSetInProgress(ctx context.Context) (*OptimizationJob, error) {
	job := &OptimizationJob{}
	// Lock for update in transaction random order
	tx := r.db.WithContext(ctx).Begin()
	defer tx.Rollback()
	err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ?", OptimizationJobStatusCreated).Order("RANDOM()").First(job).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		r.logger.Error("failed to get created optimization job", zap.Error(err))
		return nil, err
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	err = tx.Model(job).Update("status", OptimizationJobStatusInProgress).Where("id = ?", job.ID).Error
	if err != nil {
		r.logger.Error("failed to set optimization job status to in progress", zap.Error(err))
		return nil, err
	}
	err = tx.Commit().Error
	if err != nil {
		r.logger.Error("failed to commit transaction", zap.Error(err))
		return nil, err
	}
	return job, err
}

func (r *OptimizationJobsRepoImpl) GetLatestOptimizationJobByCommand(ctx context.Context, command string) (*OptimizationJob, error) {
	job := &OptimizationJob{}
	err := r.db.WithContext(ctx).Where("command = ?", command).Order("created_at desc").First(job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return job, err
}

func (r *OptimizationJobsRepoImpl) TimeoutOutdatedOptimizationJobs(ctx context.Context, timeout time.Duration) error {
	return r.db.WithContext(ctx).Model(&OptimizationJob{}).Where("status IN ? AND created_at < ?", []string{
		string(OptimizationJobStatusCreated),
		string(OptimizationJobStatusInProgress),
	}, time.Now().Add(-timeout)).Update("status", OptimizationJobStatusTimeout).Error
}
