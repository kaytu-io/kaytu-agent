package optimization

import (
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu/controller"
	"github.com/kaytu-io/kaytu/pkg/plugin"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/server"
	"github.com/kaytu-io/kaytu/pkg/version"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/kaytu/view"
	"os"
	"time"
)

func InstallPlugins() error {
	manager := plugin.New()
	err := manager.StartServer()
	if err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)

	version.VERSION = "99.99.99" //TODO-fix this

	err = manager.Install("kubernetes", "", false, false)
	if err != nil {
		return err
	}

	manager.StopServer()
	return nil
}

func Run(command string, pref []*golang.PreferenceItem) ([]view.PluginResult, error) {
	cfg, err := server.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config due to %v", err)
	}

	manager := plugin.New()
	manager.SetNonInteractiveView(true)
	err = manager.StartServer()
	if err != nil {
		return nil, fmt.Errorf("failed to start server due to %v", err)
	}

	err = manager.StartPlugin(command)
	if err != nil {
		return nil, fmt.Errorf("failed to start plugin due to %v", err)
	}

	for i := 0; i < 10; i++ {
		runningPlg := manager.GetPlugin("kaytu-io/plugin-kubernetes")
		if runningPlg != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	runningPlg := manager.GetPlugin("kaytu-io/plugin-kubernetes")
	if runningPlg == nil {
		return nil, fmt.Errorf("plugin not found")
	}

	if runningPlg.Plugin.Config.DevicesChart != nil && runningPlg.Plugin.Config.OverviewChart != nil {
		manager.NonInteractiveView.SetOptimizations(nil, controller.NewOptimizations[golang.ChartOptimizationItem](),
			runningPlg.Plugin.Config.OverviewChart, runningPlg.Plugin.Config.DevicesChart)
	} else {
		manager.NonInteractiveView.SetOptimizations(controller.NewOptimizations[golang.OptimizationItem](),
			nil, nil, nil)
	}

	flagValues := map[string]string{}
	flagValues["output"] = "json"

	for _, rcmd := range runningPlg.Plugin.Config.Commands {
		if rcmd.Name == command {
			preferences.Update(rcmd.DefaultPreferences)
			break
		}
	}
	preferences.Update(pref)

	os.Remove("out.json")
	f, err := os.OpenFile("out.json", os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("failed to open out.json due to %v", err)
	}

	manager.NonInteractiveView.SetOutput(f)

	err = runningPlg.Stream.Send(&golang.ServerMessage{
		ServerMessage: &golang.ServerMessage_Start{
			Start: &golang.StartProcess{
				Command:          command,
				Flags:            flagValues,
				KaytuAccessToken: cfg.AccessToken,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send start message due to %v", err)
	}

	err = manager.NonInteractiveView.WaitAndShowResults("json")
	if err != nil {
		return nil, fmt.Errorf("failed to get plugin result due to %v", err)
	}

	f.Close()

	content, err := os.ReadFile("out.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin result json due to %v", err)
	}

	var resp []view.PluginResult
	err = json.Unmarshal(content, &resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugin result json due to %v", err)
	}

	return resp, nil
}
