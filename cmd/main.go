package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	mapstructure "github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

const (
	ResourcesDir            = "resources"
	PipelinesPath           = "generated/pipelines"
	PipelineTemplatePath    = "templates/pipeline.json"
	ApplicationsPath        = "generated/applications"
	applicationTemplatePath = "templates/application.json"
	ResourcesPath           = "generated/resources"
)

type resourceType interface {
	Write(string, string) error
	getServiceName() string
	getNs() string
}

type applicationResource struct {
	ServiceName       string
	KubernetesAccount string
	Namespace         string
}

type pipelineResource struct {
	ServiceName string
	Namespace   string
	Regexp      string
	Spinnaker   Spinnaker
}

type Spinnaker struct {
	TemplateId string `mapstructure:"templateId"`
	Chart      struct {
		Name      string            `mapstructure:"name"`
		Variables map[string]string `mapstructure:"variables"`
	} `mapstructure:"chart"`
}

type resource struct {
	ReplicaCount         int                    `mapstructure:"replicaCount"`
	EnvironmentVariables map[string]interface{} `mapstructure:"environmentVariables"`

	Spinnaker `mapstructure:"spinnaker"`
}

func retrieveResources(root string, glob string) ([]string, error) {
	return filepath.Glob(fmt.Sprintf("%s/%s", root, glob))
}

func retriveRegexp(ns string) string {
	switch ns {
	case "production":
		return "v(\\\\d+\\\\.?)+.*"
	default:
		return "\\\\b[0-9a-f]{40}"
	}
}

func merge(values map[interface{}]interface{}, global map[interface{}]interface{}) map[interface{}]interface{} {
	for key, value := range values {
		global[key] = value
	}
	return global
}

func check(err error, msg string) {
	if err != nil {
		fmt.Printf("%s, err: %s \n", msg, err)
		os.Exit(1)
	}
}

func (a *applicationResource) Write(path string, tmpl string) error {
	return writeTmpl(a, path, tmpl, func() (*os.File, error) {
		resourcePath := fmt.Sprintf("%s/%s.json", path, a.getServiceName())
		_, err := os.Stat(resourcePath)

		// We don't want to write <application>.json for each namespace
		// Open file handler when there is no <application>.json yet
		if os.IsNotExist(err) {
			return os.Create(resourcePath)
		}
		return nil, nil
	})
}

func (a *applicationResource) getServiceName() string {
	return a.ServiceName
}

func (a *applicationResource) getNs() string {
	return a.Namespace
}

func (p *pipelineResource) Write(path string, tmpl string) error {
	return writeTmpl(p, path, tmpl, func() (*os.File, error) {
		return os.Create(fmt.Sprintf("%s/%s-%s.json", path, p.getServiceName(), p.getNs()))
	})
}

func (a *pipelineResource) getServiceName() string {
	return a.ServiceName
}

func (a *pipelineResource) getNs() string {
	return a.Namespace
}

func writeTmpl(res resourceType, path string, tmplPath string, toFile func() (*os.File, error)) error {
	f, err := toFile()
	check(err, "Error occured while creating template file")

	// <application>.json already exits, do not rewrite it
	if f == nil {
		return nil
	}

	data, err := ioutil.ReadFile(tmplPath)
	check(err, fmt.Sprintf("Error occured while reading %s", tmplPath))

	tmpl, err := template.New(res.getServiceName()).Parse(string(data))
	check(err, ParsingTemplateErrMsg)

	err = tmpl.Execute(f, res)
	f.Close()
	return err
}

func parseYAML(path string) (map[interface{}]interface{}, error) {
	data, err := ioutil.ReadFile(path)
	check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, path))

	ret := make(map[interface{}]interface{})
	err = yaml.Unmarshal([]byte(data), &ret)
	return ret, err
}

var (
	NoDirectoriesErrMsg     = fmt.Sprintf("You should create namespace directories under %s", ResourcesDir)
	ReadingResourceErrMsg   = "Error occured while reading resources"
	ParsingTemplateErrMsg   = "Error orccured while parsing template"
	ExecutingTemplateErrMsg = "Error orccured while executing template"
)

func main() {
	directories, err := retrieveResources(ResourcesDir, "**")
	check(err, NoDirectoriesErrMsg)

	if len(directories) < 1 {
		fmt.Println(NoDirectoriesErrMsg)
		os.Exit(1)
	}

	for _, path := range directories {
		var namespace string
		tmp := strings.SplitAfter(path, "/")

		if len(tmp) < 1 {
			fmt.Println(NoDirectoriesErrMsg)
			os.Exit(1)
		}
		namespace = tmp[1]

		files, err := retrieveResources(ResourcesDir, fmt.Sprintf("%s/**", namespace))
		check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, namespace))

		globalPath := fmt.Sprintf("%s/%s/global.yml", ResourcesDir, namespace)
		global, err := parseYAML(globalPath)
		check(err, fmt.Sprintf("Error occured while parsing resource %s", globalPath))

		for _, filePath := range files {

			re := regexp.MustCompile(`^(.*/)?(?:$|(.+?)(?:(\.[^.]*$)|$))`)

			match := re.FindStringSubmatch(filePath)
			serviceName := match[2]

			if serviceName == "global" {
				continue
			}

			// Merge global & service values

			values, err := parseYAML(filePath)
			check(err, fmt.Sprintf("Error occured while parsing resource %s", filePath))

			merged := merge(values, global)
			data, err := yaml.Marshal(&merged)
			check(err, fmt.Sprintf("Error occured while marshaling merged resource %s", filePath))

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s-%s.yaml", ResourcesPath, serviceName, namespace), data, 0644)
			check(err, fmt.Sprintf("Error occured while writing merged resource %s", filePath))

			res := resource{}
			err = mapstructure.Decode(merged, &res)
			check(err, fmt.Sprintf("Error occured converting map to struct"))

			// Applicationx

			app := applicationResource{
				ServiceName:       serviceName,
				KubernetesAccount: res.Spinnaker.Chart.Variables["kubernetesAccount"],
				Namespace:         namespace,
			}

			err = app.Write(ApplicationsPath, applicationTemplatePath)
			check(err, ExecutingTemplateErrMsg)

			// Pipelines

			triggerRegexp := retriveRegexp(namespace)

			pipeline := pipelineResource{
				ServiceName: serviceName,
				Namespace:   namespace,
				Regexp:      triggerRegexp,
				Spinnaker:   res.Spinnaker,
			}

			err = pipeline.Write(PipelinesPath, PipelineTemplatePath)
			check(err, ExecutingTemplateErrMsg)
			fmt.Printf("Resource generation has finished for %s namespace.\n", namespace)
		}
	}
}
