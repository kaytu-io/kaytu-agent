package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaytu-io/kaytu-agent/config"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
)

type AgentServer struct {
	golang.AgentServer
	cfg *config.Config
}

func NewAgentServer(cfg *config.Config) *AgentServer {
	return &AgentServer{
		cfg: cfg,
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
