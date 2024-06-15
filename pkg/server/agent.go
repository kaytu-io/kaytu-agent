package server

import (
	"context"
	"fmt"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
	"os"
	"path/filepath"
)

type AgentServer struct {
	golang.AgentServer
}

func (s *AgentServer) GetOverview(ctx context.Context, request *golang.GetReportRequest) (*golang.GetReportResponse, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".kaytu", fmt.Sprintf("out-%s.json", request.Command))
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &golang.GetReportResponse{
		Report: content,
	}, nil
}
