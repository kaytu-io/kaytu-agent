package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	githubAPI "github.com/google/go-github/v62/github"
	"github.com/kaytu-io/kaytu-agent/config"
	"github.com/rogpeppe/go-internal/semver"
	"go.uber.org/zap"
)

type KaytuCmd struct {
	logger *zap.Logger
	cfg    *config.Config
}

func New(logger *zap.Logger, cfg *config.Config) *KaytuCmd {
	return &KaytuCmd{
		logger: logger,
		cfg:    cfg,
	}
}

func (c *KaytuCmd) Optimize(ctx context.Context, command string) error {
	c.logger.Info("running optimization", zap.String("command", command))

	if err := ctx.Err(); err != nil {
		c.logger.Error("context error", zap.Error(err))
		return err
	}

	kaytuWorkingDir := c.cfg.WorkingDirectory
	kaytuOutputDir := c.cfg.GetOutputDirectory()
	err := os.MkdirAll(kaytuWorkingDir, os.ModePerm)
	if err != nil {
		c.logger.Error("failed to create kaytu working directory", zap.Error(err))
		return err
	}
	err = os.MkdirAll(kaytuOutputDir, os.ModePerm)
	if err != nil {
		c.logger.Error("failed to create kaytu output directory", zap.Error(err))
		return err
	}

	dbConfig, err := OpenDBConfig(ctx, c.logger, c.cfg)
	if err != nil {
		c.logger.Error("failed to open db config", zap.Error(err))
		return err
	}

	args := []string{"optimize", command, "--output", "json", "--agent-disabled", "true"}
	if c.cfg.KaytuConfig.ObservabilityDays > 0 {
		args = append(args, "--observabilityDays", fmt.Sprintf("%d", c.cfg.KaytuConfig.ObservabilityDays))
	}
	if c.cfg.KaytuConfig.Prometheus.Address != "" {
		args = append(args, "--prom-address", c.cfg.KaytuConfig.Prometheus.Address)
	}
	if c.cfg.KaytuConfig.Prometheus.Username != "" {
		args = append(args, "--prom-username", c.cfg.KaytuConfig.Prometheus.Username)
	}
	if c.cfg.KaytuConfig.Prometheus.Password != "" {
		args = append(args, "--prom-password", c.cfg.KaytuConfig.Prometheus.Password)
	}
	if c.cfg.KaytuConfig.Prometheus.ClientId != "" {
		args = append(args, "--prom-client-id", c.cfg.KaytuConfig.Prometheus.ClientId)
	}
	if c.cfg.KaytuConfig.Prometheus.ClientSecret != "" {
		args = append(args, "--prom-client-secret", c.cfg.KaytuConfig.Prometheus.ClientSecret)
	}
	if c.cfg.KaytuConfig.Prometheus.TokenUrl != "" {
		args = append(args, "--prom-token-url", c.cfg.KaytuConfig.Prometheus.TokenUrl)
	}
	if c.cfg.KaytuConfig.Prometheus.Scopes != "" {
		args = append(args, "--prom-scopes", c.cfg.KaytuConfig.Prometheus.Scopes)
	}
	cmd := exec.CommandContext(ctx, "kaytu", args...)
	cmd.Stderr = os.Stderr

	outRC, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = os.MkdirAll(kaytuOutputDir, os.ModePerm)
	if err != nil {
		c.logger.Error("failed to create output directory", zap.Error(err))
		return err
	}
	dirtyPath := filepath.Join(c.cfg.GetOutputDirectory(), fmt.Sprintf("out-%s-dirty.json", command))
	cleanPath := filepath.Join(c.cfg.GetOutputDirectory(), fmt.Sprintf("out-%s.json", command))
	os.Remove(dirtyPath)
	f, err := os.OpenFile(dirtyPath, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	go func() {
		io.Copy(f, outRC)
	}()

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}

	exists := false
	for idx, op := range dbConfig.Optimizations {
		if op.Command == command {
			op.LastUpdate = time.Now()
			dbConfig.Optimizations[idx] = op
			exists = true
			break
		}
	}
	if !exists {
		dbConfig.Optimizations = append(dbConfig.Optimizations, Optimization{
			Command:    command,
			LastUpdate: time.Now(),
		})
	}

	if err := UpdateDBConfig(ctx, c.logger, c.cfg, dbConfig); err != nil {
		c.logger.Error("failed to update db config", zap.Error(err))
		return err
	}

	c.logger.Info("optimization finished", zap.String("command", command))
	return os.Rename(dirtyPath, cleanPath)
}

func (c *KaytuCmd) LatestOptimization(ctx context.Context, command string) (*Optimization, error) {
	if err := ctx.Err(); err != nil {
		c.logger.Error("context error", zap.Error(err))
		return nil, err
	}

	dbConfig, err := OpenDBConfig(ctx, c.logger, c.cfg)
	if err != nil {
		c.logger.Error("failed to open db config", zap.Error(err))
		return nil, err
	}

	for _, op := range dbConfig.Optimizations {
		if op.Command == command {
			return &op, nil
		}
	}

	return nil, nil
}

// Install checks if kaytu is installed and installs the latest version if it is outdated.
// TODO: remove this and pin the version in docker image
func (c *KaytuCmd) Install(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		c.logger.Error("context error", zap.Error(err))
		return err
	}

	shouldInstall := false

	cmd := exec.CommandContext(ctx, "kaytu", "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(err.Error(), "file not found") {
			return err
		}
		c.logger.Warn("kaytu is not installed.")
		shouldInstall = true
	}
	version := strings.TrimSpace(string(out))
	if !shouldInstall && version == "" {
		c.logger.Error("version is empty!")
	}

	api := githubAPI.NewClient(nil)
	release, _, err := api.Repositories.GetLatestRelease(context.Background(), "kaytu-io", "kaytu")
	if err != nil {
		return err
	}

	for _, asset := range release.Assets {
		pattern := fmt.Sprintf("kaytu_([a-z0-9\\.]+)_%s_%s", runtime.GOOS, runtime.GOARCH)
		r, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}

		if asset.Name != nil && r.MatchString(*asset.Name) {
			latestVersion := strings.Split(*asset.Name, "_")[1]
			if version != "" && semver.Compare("v"+latestVersion, "v"+version) > 0 {
				c.logger.Warn("kaytu is outdated", zap.String("current", version), zap.String("latest", latestVersion))
				shouldInstall = true
			}
		}
	}

	if shouldInstall {
		c.logger.Info("downloading installation script")
		resp, err := http.Get("https://raw.githubusercontent.com/kaytu-io/kaytu/main/scripts/install.sh")
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		f, err := os.OpenFile("install.sh", os.O_CREATE|os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}
		defer os.Remove("install.sh")
		defer f.Close()

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return err
		}

		c.logger.Info("installing latest kaytu version")
		cmd = exec.CommandContext(ctx, "bash", "./install.sh")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		if err != nil {
			return err
		}

		return c.Install(ctx)
	}

	cmd = exec.CommandContext(ctx, "kaytu", "plugin", "install", "kubernetes")
	out, err = cmd.CombinedOutput()
	c.logger.Info("installing kubernetes plugin", zap.String("output", string(out)))
	if err != nil {
		c.logger.Error("failed to install kubernetes plugin", zap.Error(err), zap.String("output", string(out)))
		return err
	}

	c.logger.Info("kaytu is installed", zap.String("version", version))
	return nil
}
