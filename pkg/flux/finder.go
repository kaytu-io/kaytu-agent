package flux

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourceV1 "github.com/fluxcd/source-controller/api/v1"
	sourceV1Beta1 "github.com/fluxcd/source-controller/api/v1beta1"
	sourceV1Beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	git2 "github.com/kaytu-io/kaytu-agent/pkg/git"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"io/fs"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kustypes "sigs.k8s.io/kustomize/api/types"
	yaml2 "sigs.k8s.io/yaml"
	"strings"
)

type GeneralTemplate struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`

	Location string `yaml:"-"`
	Content  string `yaml:"-"`
	Changed  bool   `yaml:"-"`
}

type Chart struct {
	Location string
	Release  helmv2.HelmRelease
}

type Service struct {
	helmRepositoryV1 []sourceV1Beta1.HelmRepository
	helmRepositoryV2 []sourceV1Beta2.HelmRepository
	gitRepository    []sourceV1.GitRepository
	chartLocations   []Chart
	helmReleases     []helmv2.HelmRelease
	templates        []GeneralTemplate
	gitService       *git2.Service
}

func (s *Service) Walk(root, clusterFolder string) error {
	if clusterFolder == "" {
		err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
			if info.Name() == "gotk-sync.yaml" {
				clusterFolder = filepath.Dir(path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if clusterFolder == "" {
		return errors.New("cluster not found")
	}

	return s.walkOnCluster(root, filepath.Join(root, clusterFolder, "gotk-sync.yaml"))
}

func (s *Service) walkOnCluster(root, path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to get file stats due to %v", err)
	}

	if fileInfo.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("failed to read directory %s due to %v", path, err)
		}

		for _, f := range files {
			if f.Name() == "kustomization.yaml" {
				err = s.walkOnCluster(root, filepath.Join(path, f.Name()))
				if err != nil {
					return err
				}
			}
		}
	} else if strings.HasSuffix(fileInfo.Name(), ".yaml") {
		dirPath := filepath.Dir(path)
		filePath := filepath.Join(dirPath, fileInfo.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file %s due to %v", path, err)
		}
		templates := s.extractHelmTemplates(string(content))

		for _, template := range templates {
			var templateObj GeneralTemplate
			err = yaml2.Unmarshal([]byte(template), &templateObj)
			if err != nil {
				fmt.Printf("failed to unmarshal to general template for file %s due to %v\n", fileInfo.Name(), err)
			}
			templateObj.Content = template
			templateObj.Location = filePath

			if templateObj.ApiVersion == "kustomize.config.k8s.io/v1beta1" && templateObj.Kind == "Kustomization" {
				err = s.processKustomization(root, dirPath, template)
				if err != nil {
					return err
				}
			} else if templateObj.ApiVersion == "kustomize.toolkit.fluxcd.io/v1" && templateObj.Kind == "Kustomization" {
				err = s.processFluxKustomization(root, template)
				if err != nil {
					return err
				}
			} else if templateObj.ApiVersion == "helm.toolkit.fluxcd.io/v2beta1" && templateObj.Kind == "HelmRelease" {
				err = s.extractHelmRelease(template)
				if err != nil {
					return err
				}
			} else if templateObj.ApiVersion == "source.toolkit.fluxcd.io/v1" && templateObj.Kind == "GitRepository" {
				err = s.extractGitRepository(template)
				if err != nil {
					return err
				}
			} else if templateObj.ApiVersion == "source.toolkit.fluxcd.io/v1beta1" && templateObj.Kind == "HelmRepository" {
				err = s.extractHelmRepositoryV1(template)
				if err != nil {
					return err
				}
			} else if templateObj.ApiVersion == "source.toolkit.fluxcd.io/v1beta2" && templateObj.Kind == "HelmRepository" {
				err = s.extractHelmRepositoryV2(template)
				if err != nil {
					return err
				}
			} else {
				s.templates = append(s.templates, templateObj)
			}
		}
	} else {
		return fmt.Errorf("unknown file: %s", fileInfo.Name())
	}
	return nil
}

func (s *Service) extractHelmRelease(template string) error {
	var release helmv2.HelmRelease
	err := yaml2.Unmarshal([]byte(template), &release)
	if err != nil {
		return fmt.Errorf("failed to parse helm release yaml due to %v", err)
	}
	s.helmReleases = append(s.helmReleases, release)
	return nil
}

func (s *Service) processKustomization(root, dirPath, template string) error {
	var kustomization kustypes.Kustomization
	err := yaml.Unmarshal([]byte(template), &kustomization)
	if err != nil {
		return fmt.Errorf("failed to parse kustomization yaml due to %v", err)
	}

	for _, resource := range kustomization.Resources {
		err = s.walkOnCluster(root, filepath.Join(dirPath, resource))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processFluxKustomization(root, template string) error {
	var kustomization kustomizev1.Kustomization
	err := yaml2.Unmarshal([]byte(template), &kustomization)
	if err != nil {
		return fmt.Errorf("failed to parse flux kustomization yaml due to %v", err)
	}

	if kustomization.Spec.Path != "" {
		return s.walkOnCluster(root, filepath.Join(root, kustomization.Spec.Path))
	}
	return nil
}

func (s *Service) extractGitRepository(template string) error {
	var item sourceV1.GitRepository
	err := yaml2.Unmarshal([]byte(template), &item)
	if err != nil {
		return fmt.Errorf("failed to parse GitRepository yaml due to %v", err)
	}
	s.gitRepository = append(s.gitRepository, item)
	return nil
}

func (s *Service) extractHelmRepositoryV1(template string) error {
	var item sourceV1Beta1.HelmRepository
	err := yaml2.Unmarshal([]byte(template), &item)
	if err != nil {
		return fmt.Errorf("failed to parse HelmRepositoryV1 yaml due to %v", err)
	}

	s.helmRepositoryV1 = append(s.helmRepositoryV1, item)
	return nil
}

func (s *Service) extractHelmRepositoryV2(template string) error {
	var item sourceV1Beta2.HelmRepository
	err := yaml2.Unmarshal([]byte(template), &item)
	if err != nil {
		return fmt.Errorf("failed to parse HelmRepositoryV2 yaml due to %v", err)
	}

	s.helmRepositoryV2 = append(s.helmRepositoryV2, item)
	return nil
}

func (s *Service) extractHelmTemplates(input string) []string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	var currentTemplate strings.Builder
	var templates []string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" { // If the line is a separator, process the current template
			templates = append(templates, currentTemplate.String())
			currentTemplate.Reset()
		} else {
			// Escape '---' if it's within a YAML string
			if strings.Contains(line, "\"") || strings.Contains(line, "'") {
				line = strings.ReplaceAll(line, "---", "\\---")
			}
			currentTemplate.WriteString(line + "\n")
		}
	}
	// Process the last template if there is no trailing separator
	if currentTemplate.Len() > 0 {
		templates = append(templates, currentTemplate.String())
	}

	return templates
}

func (s *Service) gitPath(name, namespace string) string {
	return fmt.Sprintf("/tmp/gitrepo/%s/%s", name, namespace)
}

func (s *Service) GetCharts() []Chart {
	return s.chartLocations
}

func (s *Service) GetTemplates() []GeneralTemplate {
	return s.templates
}

func (s *Service) extractCharts() error {
	for _, release := range s.helmReleases {
		switch release.Spec.Chart.Spec.SourceRef.Kind {
		case "HelmRepository":
			for _, r := range s.helmRepositoryV1 {
				if r.Name == release.Spec.Chart.Spec.SourceRef.Name &&
					r.Namespace == release.Spec.Chart.Spec.SourceRef.Namespace {
					//fmt.Println("+++", r)
				}
			}
			for _, r := range s.helmRepositoryV2 {
				if r.Name == release.Spec.Chart.Spec.SourceRef.Name &&
					r.Namespace == release.Spec.Chart.Spec.SourceRef.Namespace {
					//fmt.Println("+++", r)
				}
			}
		case "GitRepository":
			for _, r := range s.gitRepository {
				if r.Name == release.Spec.Chart.Spec.SourceRef.Name &&
					r.Namespace == release.Spec.Chart.Spec.SourceRef.Namespace {
					chartPath := filepath.Join(s.gitService.GitFolder(r.Spec.URL), release.Spec.Chart.Spec.Chart)
					s.chartLocations = append(s.chartLocations, Chart{
						Location: chartPath,
						Release:  release,
					})
				}
			}
		default:
			fmt.Println("unknown source ref", release.Spec.Chart.Spec.SourceRef.Kind)
		}
	}

	return nil
}

func (s *Service) cloneGitRepositories() error {
	for _, item := range s.gitRepository {
		scheme := runtime.NewScheme()
		if err := helmv2.AddToScheme(scheme); err != nil {
			return err
		}
		if err := corev1.AddToScheme(scheme); err != nil {
			return err
		}
		kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
		if err != nil {
			return err
		}

		var secret corev1.Secret
		err = kubeClient.Get(context.Background(), client.ObjectKey{
			Namespace: item.Namespace,
			Name:      item.Spec.SecretRef.Name,
		}, &secret)
		if err != nil {
			return err
		}

		var username, password string
		for k, v := range secret.Data {
			if k == "username" {
				username = string(v)
			} else if k == "password" {
				password = string(v)
			}
		}

		err = s.gitService.Clone(item.Spec.URL, item.Spec.Reference.Branch, username, password)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PrepareCharts() error {
	if err := s.cloneGitRepositories(); err != nil {
		return err
	}
	if err := s.extractCharts(); err != nil {
		return err
	}
	return nil
}

func (s *Service) LoadHelmRelease(chartPath string, extraValues *apiextensionsv1.JSON, releaseName, releaseNamespace string) error {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("error loading chart: %v", err)
	}

	chartValues := map[string]interface{}{}

	values := chartutil.Values{}
	if extraValues != nil {
		err = json.Unmarshal(extraValues.Raw, &values)
		if err != nil {
			return fmt.Errorf("error loading extra values: %v", err)
		}
	}
	combinedValues, err := chartutil.CoalesceValues(chart, values)
	if err != nil {
		return fmt.Errorf("error coalescing values: %v", err)
	}
	chartValues["Values"] = combinedValues

	releaseValues := map[string]string{}
	releaseValues["Namespace"] = releaseNamespace
	chartValues["Release"] = releaseValues

	// Render templates with combined values
	renderedTemplates, err := engine.Render(chart, chartValues)
	if err != nil {
		return fmt.Errorf("error rendering templates: %v", err)
	}

	for name, data := range renderedTemplates {
		for _, template := range s.extractHelmTemplates(data) {
			var templateObj GeneralTemplate
			_ = yaml2.Unmarshal([]byte(template), &templateObj)
			templateObj.Content = template
			templateObj.Location = filepath.Join(filepath.Dir(chartPath), name)

			s.templates = append(s.templates, templateObj)
		}
	}
	return nil
}

func (s *Service) ChangeTemplate(idx int, content GeneralTemplate) {
	content.Changed = true
	s.templates[idx] = content
}

func (s *Service) Save() error {
	fileContent := map[string]string{}
	fileChanged := map[string]bool{}

	for _, template := range s.templates {
		if currentContent, ok := fileContent[template.Location]; ok {
			fileContent[template.Location] = fmt.Sprintf("%s\n---\n%s", currentContent, template.Content)
		} else {
			fileContent[template.Location] = template.Content
		}
		if template.Changed {
			fileChanged[template.Location] = true
		}
	}

	for k, v := range fileContent {
		if !fileChanged[k] {
			continue
		}
		err := os.WriteFile(k, []byte(v+"\n"), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
