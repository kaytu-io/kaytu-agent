package server

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu-agent/pkg/scheduler"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"path/filepath"

	"github.com/kaytu-io/kaytu-agent/config"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
)

type AgentServer struct {
	golang.AgentServer
	cfg       *config.Config
	scheduler *scheduler.Service
}

func NewAgentServer(cfg *config.Config, scheduler *scheduler.Service) *AgentServer {
	return &AgentServer{
		cfg:       cfg,
		scheduler: scheduler,
	}
}

func (s *AgentServer) Ping(context.Context, *golang.PingMessage) (*golang.PingMessage, error) {
	return &golang.PingMessage{}, nil
}

func (s *AgentServer) GetReport(ctx context.Context, request *golang.GetReportRequest) (*golang.GetReportResponse, error) {
	path := filepath.Join(s.cfg.GetOutputDirectory(), fmt.Sprintf("out-%s.json", request.Command))
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &golang.GetReportResponse{
		Report: content,
	}, nil
}

func (s *AgentServer) TriggerJob(ctx context.Context, request *golang.TriggerJobRequest) (*emptypb.Empty, error) {
	if len(request.Commands) == 0 {
		request.Commands = scheduler.Commands
	}

	for _, command := range request.Commands {
		if err := s.scheduler.EnqueueOptimization(ctx, command); err != nil {
			return nil, err
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *AgentServer) GetLatestJobs(ctx context.Context, request *golang.GetLatestJobsRequest) (*golang.GetLatestJobsResponse, error) {
	result := &golang.GetLatestJobsResponse{
		Jobs: make(map[string]*golang.OptimizationJob),
	}

	if len(request.Commands) == 0 {
		request.Commands = scheduler.Commands
	}

	latestJobs, err := s.scheduler.GetLatestJobsForCommands(ctx, request.Commands)
	if err != nil {
		return nil, err
	}

	for command, job := range latestJobs {
		result.Jobs[command] = dbOptimizationJobToApiOptimizationJob(job)
	}

	return result, nil
}
