package scheduler

import (
	"context"
	kaytuCmd "github.com/kaytu-io/kaytu-agent/pkg/kaytu/cmd"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type Service struct {
	kaytuCmd *kaytuCmd.KaytuCmd
	logger   *zap.Logger
}

func New(kaytuCmd *kaytuCmd.KaytuCmd, logger *zap.Logger) *Service {
	return &Service{
		kaytuCmd: kaytuCmd,
		logger:   logger,
	}
}

func (s *Service) Start(ctx context.Context) error {
	commands := []string{"kubernetes-pods", "kubernetes-deployments", "kubernetes-statefulsets", "kubernetes-daemonsets", "kubernetes-jobs"}
	for _, command := range commands {
		opt, err := s.kaytuCmd.LatestOptimization(ctx, command)
		if err != nil {
			return err
		}
		if opt == nil {
			s.logger.Info("no previous optimization for command found, triggering optimization cycle", zap.String("command", command))
			go s.Trigger()
			break
		}
	}

	c := cron.New()

	_, err := c.AddFunc("0 30 1 * * *", func() { go s.Trigger() })
	if err != nil {
		return err
	}

	c.Start()

	return nil
}

func (s *Service) Trigger() {
	s.logger.Info("optimization cycle triggered")
	ctx := context.Background()
	_ = s.kaytuCmd.Initialize(ctx)

	commands := []string{"kubernetes-pods", "kubernetes-deployments", "kubernetes-statefulsets", "kubernetes-daemonsets", "kubernetes-jobs"}
	for _, command := range commands {
		err := s.kaytuCmd.Optimize(ctx, command)
		if err != nil {
			s.logger.Error("failed to run kaytu optimization", zap.String("command", command), zap.Error(err))
		}
	}
}
