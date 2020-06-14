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

type applicationResource struct {
	ServiceName       string
	KubernetesAccount string
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
		data, err := ioutil.ReadFile(globalPath)
		check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, globalPath))

		global := make(map[interface{}]interface{})
		err = yaml.Unmarshal([]byte(data), &global)
		check(err, fmt.Sprintf("Error occured while parsing resource %s", globalPath))

		for _, filePath := range files {

			re := regexp.MustCompile(`^(.*/)?(?:$|(.+?)(?:(\.[^.]*$)|$))`)

			match := re.FindStringSubmatch(filePath)
			serviceName := match[2]

			if serviceName == "global" {
				continue
			}

			data, err := ioutil.ReadFile(filePath)
			check(err, fmt.Sprintf("Error occured while reading resource %s", filePath))

			values := make(map[interface{}]interface{})
			err = yaml.Unmarshal([]byte(data), &values)
			check(err, fmt.Sprintf("Error occured while parsing resource %s", filePath))

			merged := merge(values, global)
			data, err = yaml.Marshal(&merged)
			check(err, fmt.Sprintf("Error occured while marshaling merged resource %s", filePath))

			err = ioutil.WriteFile(fmt.Sprintf("%s/%s-%s.yaml", ResourcesPath, serviceName, namespace), data, 0644)
			check(err, fmt.Sprintf("Error occured while writing merged resource %s", filePath))

			res := resource{}
			err = mapstructure.Decode(merged, &res)
			check(err, fmt.Sprintf("Error occured converting map to struct: %s", err.Error()))

			app := applicationResource{
				ServiceName:       serviceName,
				KubernetesAccount: res.Spinnaker.Chart.Variables["kubernetesAccount"],
			}

			data, err = ioutil.ReadFile(applicationTemplatePath)
			check(err, fmt.Sprintf("Error occured while reading %s", applicationTemplatePath))

			tmpl, err := template.New(serviceName).Parse(string(data))
			check(err, ParsingTemplateErrMsg)

			f, err := os.Create(fmt.Sprintf("%s/%s-%s.yaml", ApplicationsPath, serviceName, namespace))
			check(err, "Error occured while creating template file")

			err = tmpl.Execute(f, app)
			check(err, ExecutingTemplateErrMsg)
			f.Close()

			// Pipelines

			triggerRegexp := retriveRegexp(namespace)

			pipeline := pipelineResource{
				ServiceName: serviceName,
				Namespace:   namespace,
				Regexp:      triggerRegexp,
				Spinnaker:   res.Spinnaker,
			}

			data, err = ioutil.ReadFile(PipelineTemplatePath)
			check(err, fmt.Sprintf("Error occured while reading %s", PipelineTemplatePath))

			tmpl, err = template.New(serviceName).Parse(string(data))
			check(err, ParsingTemplateErrMsg)

			f, err = os.Create(fmt.Sprintf("%s/%s-%s.yaml", PipelinesPath, serviceName, namespace))
			check(err, "Error occured while creating template file")

			err = tmpl.Execute(f, pipeline)
			check(err, ExecutingTemplateErrMsg)
			f.Close()

		}
	}
}