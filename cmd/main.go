package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

var (
	// Templates location
	TemplatesDir = ""
	// Location of namespace and service information
	ResourcesDir = ""
	// Destination directory
	GeneratedDir = ""
)

type ResourceType interface {
	write(string, string) error
	getServiceName() string
	getNs() string
}

type ApplicationResource struct {
	ServiceName       string
	KubernetesAccount string
	Namespace         string
}

type PipelineResource struct {
	ServiceName string
	Namespace   string
	Regexp      string
	Spinnaker   Spinnaker
}

type Spinnaker struct {
	TemplateId    string `mapstructure:"templateId"`
	TriggerRegexp string `mapstructure:"triggerRegexp"`
	Chart         struct {
		Name      string            `mapstructure:"name"`
		Variables map[string]string `mapstructure:"variables"`
	} `mapstructure:"chart"`
}

type Resource struct {
	ReplicaCount         int                    `mapstructure:"replicaCount"`
	EnvironmentVariables map[string]interface{} `mapstructure:"environmentVariables"`

	Spinnaker `mapstructure:"spinnaker"`
}

type YAML = map[string]interface{}

func retrieveResources(root string, glob string) ([]string, error) {
	return filepath.Glob(fmt.Sprintf("%s/%s", root, glob))
}

func check(err error, msg string) {
	if err != nil {
		fmt.Printf("%s, err: %s \n", msg, err)
		os.Exit(1)
	}
}

func merge(values YAML, global YAML) YAML {
	// map[string]interface{} type
	yamlType := reflect.TypeOf(global)

	for key, rightVal := range global {
		if leftVal, present := values[key]; present {
			// if value is already present in <values>.yaml,
			// and  also not a map type then do nothing
			if reflect.TypeOf(leftVal) != yamlType {
				continue
			}
			// if value is already present in <values>.yaml,
			// and also a map type: continue next level
			values[key] = merge(leftVal.(YAML), rightVal.(YAML))
		} else {
			// if value is not yet present in <values>.yaml, add it
			values[key] = rightVal
		}
	}
	return values
}

func (a *ApplicationResource) write(path string, tmpl string) error {
	resourcePath := fmt.Sprintf("%s/%s.json", path, a.getServiceName())
	_, err := os.Stat(resourcePath)

	// We don't want to write <application>.json for each namespace
	// Open file handler only when there is no <application>.json yet
	if os.IsNotExist(err) {
		handler, err := os.Create(resourcePath)
		check(err, CreatingTemplateErrMsg)
		defer handler.Close()
		return writeTmpl(a, handler, path, tmpl)
	}
	return nil
}

func (a *ApplicationResource) getServiceName() string {
	return a.ServiceName
}

func (a *ApplicationResource) getNs() string {
	return a.Namespace
}

func (p *PipelineResource) write(path string, tmpl string) error {
	handler, err := os.Create(fmt.Sprintf("%s/%s-%s.json", path, p.getServiceName(), p.getNs()))
	check(err, CreatingTemplateErrMsg)
	defer handler.Close()
	return writeTmpl(p, handler, path, tmpl)
}

func (a *PipelineResource) getServiceName() string {
	return a.ServiceName
}

func (a *PipelineResource) getNs() string {
	return a.Namespace
}

func writeTmpl(res ResourceType, handler *os.File, path string, tmplPath string) error {
	data, err := ioutil.ReadFile(tmplPath)
	check(err, fmt.Sprintf("Error occured while reading %s", tmplPath))

	tmpl, err := template.New(res.getServiceName()).Parse(string(data))
	check(err, ParsingTemplateErrMsg)

	err = tmpl.Execute(handler, res)
	return err
}

func parseYAML(path string) (YAML, error) {
	data, err := ioutil.ReadFile(path)
	check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, path))

	ret := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &ret)
	return ret, err
}

var (
	NoDirectoriesErrMsg     = fmt.Sprintf("You should create namespace directories under %s", ResourcesDir)
	ReadingResourceErrMsg   = "Error occured while reading resources"
	ParsingTemplateErrMsg   = "Error orccured while parsing template"
	ExecutingTemplateErrMsg = "Error orccured while executing template"
	CreatingTemplateErrMsg  = "Error creating file for go template to fill"
)

func main() {

	optionsSet := flag.NewFlagSet("optionsSet", flag.ExitOnError)
	optionsSet.StringVar(&TemplatesDir, "templates", "templates", "Template files location")
	optionsSet.StringVar(&ResourcesDir, "values", "", "Values files location")
	optionsSet.StringVar(&GeneratedDir, "destination", "", "Generated files will end up here")
	err := optionsSet.Parse(os.Args[1:])
	check(err, "Error parsing flags")

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

			err = ioutil.WriteFile(fmt.Sprintf("%s/resources/%s-%s.yaml", GeneratedDir, serviceName, namespace), data, 0644)
			check(err, fmt.Sprintf("Error occured while writing merged resource %s", filePath))

			res := Resource{}
			err = mapstructure.Decode(merged, &res)
			check(err, "Error occured converting map to struct")

			// Applicationx

			app := ApplicationResource{
				ServiceName:       serviceName,
				KubernetesAccount: res.Spinnaker.Chart.Variables["kubernetesAccount"],
				Namespace:         namespace,
			}

			err = app.write(fmt.Sprintf("%s/applications", GeneratedDir), fmt.Sprintf("%s/application.json", TemplatesDir))
			check(err, ExecutingTemplateErrMsg)

			// Pipelines

			pipeline := PipelineResource{
				ServiceName: serviceName,
				Namespace:   namespace,
				Regexp:      res.TriggerRegexp,
				Spinnaker:   res.Spinnaker,
			}

			err = pipeline.write(fmt.Sprintf("%s/pipelines", GeneratedDir), fmt.Sprintf("%s/pipeline.json", TemplatesDir))
			check(err, ExecutingTemplateErrMsg)
			fmt.Printf("Resource generation has finished for %s namespace.\n", namespace)
		}
	}
}
