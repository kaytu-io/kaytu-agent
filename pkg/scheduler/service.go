package scheduler

import (
	"context"
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu-agent/config"
	"github.com/kaytu-io/kaytu-agent/pkg/database"
	kaytuCmd "github.com/kaytu-io/kaytu-agent/pkg/kaytu/cmd"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

var Commands = []string{
	"kubernetes-pods",
	"kubernetes-deployments",
	"kubernetes-statefulsets",
	"kubernetes-daemonsets",
	"kubernetes-jobs",
	"kubernetes",
}

type Service struct {
	kaytuCmd *kaytuCmd.KaytuCmd
	logger   *zap.Logger
	cfg      *config.Config

	optimizationJobsRepo database.OptimizationJobsRepo
}

func New(kaytuCmd *kaytuCmd.KaytuCmd, logger *zap.Logger, cfg *config.Config, optimizationJobsRepo database.OptimizationJobsRepo) *Service {
	return &Service{
		kaytuCmd:             kaytuCmd,
		logger:               logger,
		cfg:                  cfg,
		optimizationJobsRepo: optimizationJobsRepo,
	}
}

func (s *Service) Start(ctx context.Context) {
	checkTicker := time.NewTicker(s.cfg.GetOptimizationCheckInterval())
	go s.runCheckCycle(ctx, checkTicker)

	scheduleTicker := time.NewTicker(s.cfg.GetOptimizationJobScheduleInterval())
	go s.runScheduleCycle(ctx, scheduleTicker)
}

func (s *Service) EnqueueOptimization(ctx context.Context, command string) error {
	job, err := s.optimizationJobsRepo.GetLatestOptimizationJobByCommand(ctx, command)
	if err != nil {
		return err
	}
	if job != nil && (job.Status == database.OptimizationJobStatusCreated || job.Status == database.OptimizationJobStatusInProgress) {
		return status.New(codes.InvalidArgument, "optimization job already exists").Err()
	}

	s.logger.Info("enqueuing optimization job", zap.String("command", command))
	return s.optimizationJobsRepo.CreateOptimizationJob(ctx, command)
}

func (s *Service) GetLatestJobsForCommands(ctx context.Context, commands []string) (map[string]*database.OptimizationJob, error) {
	jobs := make(map[string]*database.OptimizationJob)
	for _, command := range commands {
		job, err := s.optimizationJobsRepo.GetLatestOptimizationJobByCommand(ctx, command)
		if err != nil {
			return nil, err
		}
		if job != nil {
			jobs[command] = job
		}
	}
	return jobs, nil
}

func (s *Service) runScheduleCycle(ctx context.Context, scheduleTicker *time.Ticker) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("recovered from panic in schedule func", zap.Any("panic", r))
			go s.runScheduleCycle(ctx, scheduleTicker)
		}
	}()

	for _, command := range Commands {
		if err := s.EnqueueOptimization(ctx, command); err != nil {
			s.logger.Error("failed to enqueue optimization job", zap.Error(err), zap.String("command", command))
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-scheduleTicker.C:
			for _, command := range Commands {
				if err := s.EnqueueOptimization(ctx, command); err != nil {
					s.logger.Error("failed to enqueue optimization job", zap.Error(err), zap.String("command", command))
				}
			}
		}
	}
}

func (s *Service) runCheckCycle(ctx context.Context, checkTicker *time.Ticker) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("recovered from panic in check func", zap.Any("panic", r))
			go s.runCheckCycle(ctx, checkTicker)
		}
	}()

	if err := s.checkForOptimizationJobs(ctx); err != nil {
		s.logger.Error("failed to check for optimization jobs", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-checkTicker.C:
			if err := s.checkForOptimizationJobs(ctx); err != nil {
				s.logger.Error("failed to check for optimization jobs", zap.Error(err))
			}
		}
	}
}

func (s *Service) checkForOptimizationJobs(ctx context.Context) error {
	err := s.optimizationJobsRepo.TimeoutOutdatedOptimizationJobs(ctx, s.cfg.GetOptimizationJobQueueTimeout())
	if err != nil {
		s.logger.Error("failed to timeout outdated optimization jobs", zap.Error(err))
		return err
	}

	job, err := s.optimizationJobsRepo.GetCreatedOptimizationJobAndSetInProgress(ctx)
	if err != nil {
		s.logger.Error("failed to get created optimization job and set in progress", zap.Error(err))
		return err
	}

	if job != nil {
		s.runOptimizationJob(ctx, job)
		return s.checkForOptimizationJobs(ctx)
	}

	return nil
}

func (s *Service) runOptimizationJob(ctx context.Context, job *database.OptimizationJob) {
	s.logger.Info("running optimization job", zap.String("command", job.Command))
	jobStatus := database.OptimizationJobStatusSucceeded
	errorMessage := ""
	defer func() {
		if err := s.optimizationJobsRepo.SetOptimizationJobStatus(ctx, job.ID, jobStatus, errorMessage); err != nil {
			s.logger.Error("failed to update optimization job", zap.Error(err))
		}
	}()

	err := s.kaytuCmd.Initialize(ctx)
	if err != nil {
		s.logger.Error("failed to initialize kaytu", zap.Error(err))
		jobStatus = database.OptimizationJobStatusFailed
		errorMessage = fmt.Sprintf("failed to initialize kaytu: %s", err.Error())
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, s.cfg.GetOptimizationJobRunTimeout())
	defer cancel()
	err = s.kaytuCmd.Optimize(jobCtx, job.Command)
	if err != nil {
		s.logger.Error("failed to run kaytu optimization", zap.String("command", job.Command), zap.Error(err))
		jobStatus = database.OptimizationJobStatusFailed
		errorMessage = err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			jobStatus = database.OptimizationJobStatusTimeout
			errorMessage = fmt.Sprintf("optimization job ran out of time (%s) to execute", s.cfg.GetOptimizationJobRunTimeout().String())
		}
		return
	}

	s.logger.Info("optimization job finished", zap.String("command", job.Command))
}
