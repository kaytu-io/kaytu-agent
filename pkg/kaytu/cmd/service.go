package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	githubAPI "github.com/google/go-github/v62/github"
	"github.com/rogpeppe/go-internal/semver"
	"go.uber.org/zap"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type KaytuCmd struct {
	logger *zap.Logger
}

func New(logger *zap.Logger) *KaytuCmd {
	return &KaytuCmd{
		logger: logger,
	}
}

func (c *KaytuCmd) Optimize(command string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	ops, err := os.ReadFile(path.Join(home, ".kaytu", "optimizations.json"))
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}

	var opConfig OptimizationsConfig
	err = json.Unmarshal(ops, &opConfig)
	if err != nil {
		return err
	}

	cmd := exec.Command("kaytu", "optimize", command, "--output", "json", "--observabilityDays", "14")
	cmd.Stderr = os.Stderr

	outRC, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	dirtyPath := filepath.Join(home, ".kaytu", fmt.Sprintf("out-%s-dirty.json", command))
	cleanPath := filepath.Join(home, ".kaytu", fmt.Sprintf("out-%s.json", command))
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
	for idx, op := range opConfig.Optimizations {
		if op.Command == command {
			op.LastUpdate = time.Now()
			opConfig.Optimizations[idx] = op
			exists = true
			break
		}
	}
	if !exists {
		opConfig.Optimizations = append(opConfig.Optimizations, Optimization{
			Command:    command,
			LastUpdate: time.Now(),
		})
	}
	ops, err = json.Marshal(opConfig)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(home, ".kaytu", "optimizations.json"), ops, os.ModePerm)
	if err != nil {
		return err
	}

	c.logger.Info("optimization finished")
	return os.Rename(dirtyPath, cleanPath)
}

func (c *KaytuCmd) LatestOptimization(command string) (*Optimization, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	ops, err := os.ReadFile(path.Join(home, ".kaytu", "optimizations.json"))
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}
	}

	var opConfig OptimizationsConfig
	err = json.Unmarshal(ops, &opConfig)
	if err != nil {
		return nil, err
	}

	for _, op := range opConfig.Optimizations {
		if op.Command == command {
			return &op, nil
		}
	}

	return nil, nil
}

func (c *KaytuCmd) Install() error {
	shouldInstall := false

	cmd := exec.Command("kaytu", "version")
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
		defer f.Close()

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return err
		}

		c.logger.Info("installing latest kaytu version")
		cmd = exec.Command("sh", "./install.sh")
		err = cmd.Run()
		if err != nil {
			return err
		}

		return c.Install()
	}

	c.logger.Info("kaytu is installed", zap.String("version", version))
	return nil
}
