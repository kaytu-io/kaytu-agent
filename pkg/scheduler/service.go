package scheduler

import (
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

func (s *Service) Start() error {
	opt, err := s.kaytuCmd.LatestOptimization()
	if err != nil {
		return err
	}
	if opt == nil {
		go s.Trigger()
	}
	c := cron.New()
	c.AddFunc("0 30 1 * * *", func() { go s.Trigger() })
	c.Start()

	return nil
}

func (s *Service) Trigger() {
	commands := []string{"kubernetes-pods", "kubernetes-deployments", "kubernetes-statefulsets", "kubernetes-daemonsets", "kubernetes-jobs"}
	for _, command := range commands {
		err := s.kaytuCmd.Optimize(command)
		if err != nil {
			s.logger.Error("failed to run kaytu optimization", zap.String("command", command), zap.Error(err))
		}
	}
}
