package cmd

import (
	"fmt"
	kaytuCmd "github.com/kaytu-io/kaytu-agent/pkg/kaytu/cmd"
	"github.com/kaytu-io/kaytu-agent/pkg/proto/src/golang"
	"github.com/kaytu-io/kaytu-agent/pkg/scheduler"
	"github.com/kaytu-io/kaytu-agent/pkg/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "kaytu-agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		logger, err := zap.NewProduction()
		if err != nil {
			return err
		}

		logger.Info("listening on :8001")
		lis, err := net.Listen("tcp", ":8001")
		if err != nil {
			return err
		}

		logger.Info("checking kaytu installation")
		kc := kaytuCmd.New(logger)
		err = kc.Install()
		if err != nil {
			return err
		}

		logger.Info("starting scheduler")
		scheduler := scheduler.New(kc, logger)
		err = scheduler.Start()
		if err != nil {
			return err
		}

		grpcServer := grpc.NewServer()
		handler := server.AgentServer{}
		golang.RegisterAgentServer(grpcServer, &handler)
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
