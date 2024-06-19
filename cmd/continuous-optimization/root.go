package continuous_optimization

import (
	"encoding/json"
	"fmt"
	"github.com/kaytu-io/kaytu-agent/pkg/flux"
	git2 "github.com/kaytu-io/kaytu-agent/pkg/git"
	"github.com/kaytu-io/kaytu-agent/pkg/kaytu/optimization"
	"github.com/kaytu-io/kaytu/view"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	resource2 "k8s.io/apimachinery/pkg/api/resource"
	"os"
	yaml2 "sigs.k8s.io/yaml"
	"strings"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "kaytu-agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		gitURL := cmd.Flag("git-url").Value.String()
		gitBranch := cmd.Flag("git-branch").Value.String()
		gitUsername := cmd.Flag("git-username").Value.String()
		gitPassword := cmd.Flag("git-password").Value.String()
		fluxClusterFolder := cmd.Flag("flux-cluster-folder").Value.String()

		gitService := git2.New()
		//gitService.CleanUp()
		if err := gitService.Clone(gitURL, gitBranch, gitUsername, gitPassword); err != nil {
			return err
		}

		finderService := flux.Service{}
		err := finderService.Walk(gitService.GitFolder(gitURL), fluxClusterFolder)
		if err != nil {
			return err
		}

		err = finderService.PrepareCharts()
		if err != nil {
			return err
		}

		for _, chart := range finderService.GetCharts() {
			err = finderService.LoadHelmRelease(chart.Location, chart.Release.Spec.Values, chart.Release.Spec.ReleaseName, chart.Release.Spec.TargetNamespace)
			if err != nil {
				return err
			}
		}

		err = optimization.InstallPlugins(ctx)
		if err != nil {
			return err
		}

		_, err = optimization.Run(ctx, "kubernetes-deployments", nil)
		if err != nil {
			return err
		}
		content, _ := os.ReadFile("out.json")
		var results []view.PluginResult
		json.Unmarshal(content, &results)

		for _, deployment := range results {
			name := deployment.Properties["name"]
			namespace := deployment.Properties["namespace"]
			var deploymentTemplate flux.GeneralTemplate
			deploymentTemplateIdx := -1

			for idx, template := range finderService.GetTemplates() {
				if template.Kind == "Deployment" && template.ApiVersion == "apps/v1" {
					if template.Metadata.Name == name && template.Metadata.Namespace == namespace {
						deploymentTemplate = template
						deploymentTemplateIdx = idx
					}
				}
			}

			if deploymentTemplateIdx == -1 {
				fmt.Println("deployment template not found", name, namespace)
				continue
			}
			var deploymentObj v1.Deployment
			err = yaml2.Unmarshal([]byte(deploymentTemplate.Content), &deploymentObj)
			if err != nil {
				return err
			}

			for idx, c := range deploymentObj.Spec.Template.Spec.Containers {
				for _, resource := range deployment.Resources {
					if resource.Overview["name"] == c.Name+" - Overall" {

						c.Resources.Requests = map[v12.ResourceName]resource2.Quantity{}
						c.Resources.Limits = map[v12.ResourceName]resource2.Quantity{}

						if resource.Details["cpu_request"].Recommended != "" {
							c.Resources.Requests[v12.ResourceCPU] = parseCPU(resource.Details["cpu_request"].Recommended)
						}
						if resource.Details["memory_request"].Recommended != "" {
							c.Resources.Requests[v12.ResourceMemory] = parseMemory(resource.Details["memory_request"].Recommended)
						}
						if resource.Details["cpu_limit"].Recommended != "" {
							c.Resources.Limits[v12.ResourceCPU] = parseCPU(resource.Details["cpu_limit"].Recommended)
						}
						if resource.Details["memory_limit"].Recommended != "" {
							c.Resources.Limits[v12.ResourceMemory] = parseMemory(resource.Details["memory_limit"].Recommended)
						}

						deploymentObj.Spec.Template.Spec.Containers[idx] = c
					}
				}
			}
			content, err := yaml2.Marshal(deploymentObj)
			if err != nil {
				return err
			}
			deploymentTemplate.Content = string(content)
			finderService.ChangeTemplate(deploymentTemplateIdx, deploymentTemplate)
		}

		err = finderService.Save()
		if err != nil {
			return err
		}
		return nil
	},
}

func parseCPU(c string) resource2.Quantity {
	c = strings.TrimSpace(c)
	c = strings.ToLower(c)
	c = strings.TrimSuffix(c, " core")

	q, err := resource2.ParseQuantity(c)
	if err != nil {
		panic(err)
	}

	return q
}

func parseMemory(c string) resource2.Quantity {
	c = strings.TrimSpace(c)
	c = strings.ReplaceAll(c, " ", "")
	c = strings.ReplaceAll(c, "KiB", "Ki")
	c = strings.ReplaceAll(c, "KB", "K")
	c = strings.ReplaceAll(c, "MiB", "Mi")
	c = strings.ReplaceAll(c, "MB", "M")
	c = strings.ReplaceAll(c, "GiB", "Gi")
	c = strings.ReplaceAll(c, "GB", "G")
	q, err := resource2.ParseQuantity(c)
	if err != nil {
		panic(err)
	}
	//q.Format = resource2.BinarySI

	return q
}

func init() {
	rootCmd.Flags().String("git-url", "", "Flux git url")
	rootCmd.Flags().String("git-branch", "main", "git branch")
	rootCmd.Flags().String("git-username", "", "git username")
	rootCmd.Flags().String("git-password", "", "git password")
	rootCmd.Flags().String("flux-cluster-folder", "./", "relative path of flux cluster folder (the folder which contains gotk-sync.yaml)")

	rootCmd.MarkFlagRequired("git-url")
	rootCmd.MarkFlagRequired("flux-cluster-folder")
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println("Failed due to", err)
		os.Exit(1)
	}
}
