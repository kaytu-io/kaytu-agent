package cmd

import (
	"fmt"
	"github.com/kaytu-io/kaytu-agent/config"
	"github.com/kaytu-io/kaytu-agent/pkg/database"
	kaytuCmd "github.com/kaytu-io/kaytu-agent/pkg/kaytu/cmd"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
	"github.com/kaytu-io/kaytu-agent/pkg/scheduler"
	"github.com/kaytu-io/kaytu-agent/pkg/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"math"
	"net"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "kaytu-agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		logger, err := zap.NewProduction()
		if err != nil {
			return err
		}

		logger.Info("loading config")
		cfg := config.Provide(nil, config.DefaultConfig)

		db, err := database.NewAgentDatabase(ctx, logger, &cfg)
		defer db.Close()
		if err != nil {
			logger.Error("failed to open db", zap.Error(err))
			return err
		}
		optimizationJobsRepo := database.NewOptimizationJobsRepo(db, logger)

		logger.Info(fmt.Sprintf("listening on :%d", cfg.GrpcPort))
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GrpcPort))
		if err != nil {
			return err
		}

		logger.Info("checking kaytu installation")
		kc := kaytuCmd.New(logger, &cfg)

		logger.Info("starting scheduler")
		scheduler := scheduler.New(kc, logger, &cfg, optimizationJobsRepo)
		scheduler.Start(ctx)

		grpcServer := grpc.NewServer(
			grpc.MaxRecvMsgSize(128*1024*1024),
			grpc.MaxSendMsgSize(math.MaxInt),
		)
		handler := server.NewAgentServer(&cfg, scheduler)
		golang.RegisterAgentServer(grpcServer, handler)
		logger.Info("starting grpc server")
		return grpcServer.Serve(lis)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Failed due to", err)
		os.Exit(1)
	}
}
