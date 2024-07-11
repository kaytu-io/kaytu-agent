package server

import (
	"github.com/kaytu-io/kaytu-agent/pkg/database"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func dbOptimizationJobToApiOptimizationJob(job *database.OptimizationJob) *golang.OptimizationJob {
	return &golang.OptimizationJob{
		Id:           uint64(job.ID),
		Command:      job.Command,
		Status:       string(job.Status),
		ErrorMessage: job.ErrorMessage,
		CreatedAt:    timestamppb.New(job.CreatedAt),
		UpdatedAt:    timestamppb.New(job.UpdatedAt),
	}
}
