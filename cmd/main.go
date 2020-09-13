package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/nagypeterjob-edu/application-values/pkg/resource"
	"gopkg.in/yaml.v3"
)

var (
	// Poinst to the location of namespace and service information
	ResourcesDir = ""
	// Destination directory
	GeneratedDir = ""
)

type yml = map[string]interface{}

type global = map[string]yml

type service struct {
	filename  string
	name      string
	namespace string
	yaml      yml
}

func walkResources(root string, ctx context.Context) (<-chan string, <-chan error) {
	resources := make(chan string)
	errc := make(chan error, 1)

	go func() {
		defer close(resources)
		defer close(errc)
		errc <- filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("filepath %s not found", path)
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			select {
			case resources <- path:
			case <-ctx.Done():
				return errors.New("walk canceled")
			}

			return nil
		})
	}()
	return resources, errc
}

func check(err error, msg string) {
	if err != nil {
		fmt.Printf("%s, err: %s \n", msg, err)
		os.Exit(1)
	}
}

func merge(values yml, global yml) yml {
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
			values[key] = merge(leftVal.(yml), rightVal.(yml))
		} else {
			// if value is not yet present in <values>.yaml, add it
			values[key] = rightVal
		}
	}
	return values
}

func parseYaml(path string) (yml, error) {
	data, err := ioutil.ReadFile(path)
	check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, path))

	ret := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &ret)
	return ret, err
}

var (
	ReadingResourceErrMsg   = "Error occured while reading resources"
	ExecutingTemplateErrMsg = "Error orccured while executing template"
)

func main() {

	optionsSet := flag.NewFlagSet("optionsSet", flag.ExitOnError)
	optionsSet.StringVar(&ResourcesDir, "values", "", "Values files location")
	optionsSet.StringVar(&GeneratedDir, "destination", "", "Generated files will end up here")
	err := optionsSet.Parse(os.Args[1:])
	check(err, "Error parsing flags")

	paths, errc := walkResources(ResourcesDir, context.Background())

	services := []*service{}
	globals := global{}
	for path := range paths {
		directory := filepath.Dir(path)
		filename := filepath.Base(path)
		ext := filepath.Ext(path)
		namespace := directory[len(ResourcesDir)+1:]
		name := strings.TrimSuffix(filename, ext)

		content, err := parseYaml(path)
		check(err, "Error parsing yaml")

		if name == "global" {
			globals[namespace] = content
			continue
		}

		services = append(services, &service{
			filename:  filename,
			name:      name,
			namespace: namespace,
			yaml:      content,
		})
	}

	if err = <-errc; err != nil {
		fmt.Printf("Error reading resources, err: %s \n", err)
		os.Exit(1)
	}

	for _, service := range services {

		merged := merge(service.yaml, globals[service.namespace])

		data, err := yaml.Marshal(merged)
		check(err, "Error writing resource")

		values := resource.Values{}
		err = mapstructure.Decode(merged, &values)
		check(err, "Error decoding merged resource")

		filename := fmt.Sprintf("%s-%s.yaml", service.name, service.namespace)
		err = ioutil.WriteFile(fmt.Sprintf("%s/resources/%s", GeneratedDir, filename), data, 0644)
		check(err, fmt.Sprintf("Error occured while writing merged resource %s", filename))

		// Applications

		app := resource.Application{
			ServiceName:       service.name,
			KubernetesAccount: values.Spinnaker.Chart.Variables["kubernetesAccount"],
			Namespace:         service.namespace,
		}

		err = app.Write(fmt.Sprintf("%s/applications", GeneratedDir))
		check(err, ExecutingTemplateErrMsg)

		// Pipelines

		pipeline := resource.Pipeline{
			ServiceName: service.name,
			Namespace:   service.namespace,
			Spinnaker:   values.Spinnaker,
		}

		err = pipeline.Write(fmt.Sprintf("%s/pipelines", GeneratedDir))
		check(err, ExecutingTemplateErrMsg)
		fmt.Printf("Values generation has finished for %s namespace.\n", service.namespace)
	}
}
