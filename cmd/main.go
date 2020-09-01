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

	"github.com/mitchellh/mapstructure"
	"github.com/nagypeterjob-edu/application-values/pkg/hash"
	"github.com/nagypeterjob-edu/application-values/pkg/resource"
	"gopkg.in/yaml.v3"
)

var (
	// Points to the resource templates
	TemplatesDir = ""
	// Poinst to the location of namespace and service information
	ResourcesDir = ""
	// Destination directory
	GeneratedDir = ""
	// Points to the directory containing the current version of resources synced from S3
	PathCurrent = ""
)

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

func parseYAML(path string) (YAML, error) {
	data, err := ioutil.ReadFile(path)
	check(err, fmt.Sprintf("%s for %s", ReadingResourceErrMsg, path))

	ret := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(data), &ret)
	return ret, err
}

var (
	NoDirectoriesErrMsg     = fmt.Sprintf("You should create directories for namespaces under %s", ResourcesDir)
	ReadingResourceErrMsg   = "Error occured while reading resources"
	ExecutingTemplateErrMsg = "Error orccured while executing template"
)

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func compareHash(a string, b string) (bool, error) {
	aHash, err := hash.CalculateHash(a)
	if err != nil {
		return false, err
	}

	bHash, err := hash.CalculateHash(b)
	if err != nil {
		return false, err
	}
	return aHash == bHash, nil
}

func resourcesToUpdate() ([]string, bool, error) {
	var filtered []string

	if !pathExists(PathCurrent) {
		// there are no previous resources so upload them anyways, hence return true
		return filtered, true, nil
	}

	resources, err := retrieveResources(GeneratedDir, "resources/**")
	if err != nil {
		return filtered, false, err
	}

	if len(resources) < 1 {
		// there are no previous resources so upload anyways, hence return true
		return resources, true, nil
	}

	// Iterate over generated files & try to find their pair among the files synced from S3 bucket
	for _, resource := range resources {
		// Get filename
		_, file := filepath.Split(resource)

		pairPath := fmt.Sprintf("%s/%s", PathCurrent, file)
		// Check if "pair" exists
		if !pathExists(pairPath) {
			filtered = append(filtered, resource)
			continue
		}

		noChange, err := compareHash(resource, pairPath)
		if err != nil {
			return filtered, false, err
		}
		if !noChange {
			filtered = append(filtered, resource)
		}
	}

	return filtered, false, nil
}

func generateResources(directories []string) map[string]resource.Resource {
	resources := make(map[string]resource.Resource, len(directories))

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

			key := fmt.Sprintf("%s-%s.yaml", serviceName, namespace)
			err = ioutil.WriteFile(fmt.Sprintf("%s/resources/%s", GeneratedDir, key), data, 0644)
			check(err, fmt.Sprintf("Error occured while writing merged resource %s", filePath))

			res := resource.Resource{}
			err = mapstructure.Decode(merged, &res)
			check(err, "Error occured converting map to struct")

			resources[key] = res
		}
	}
	return resources
}

func main() {

	optionsSet := flag.NewFlagSet("optionsSet", flag.ExitOnError)
	optionsSet.StringVar(&TemplatesDir, "templates", "templates", "Template files location")
	optionsSet.StringVar(&ResourcesDir, "values", "", "Values files location")
	optionsSet.StringVar(&GeneratedDir, "destination", "", "Generated files will end up here")
	optionsSet.StringVar(&PathCurrent, "current", "tmp", "Points to the directory containing the current version of resources")
	err := optionsSet.Parse(os.Args[1:])
	check(err, "Error parsing flags")

	directories, err := retrieveResources(ResourcesDir, "**")
	check(err, NoDirectoriesErrMsg)

	if len(directories) < 1 {
		fmt.Println(NoDirectoriesErrMsg)
		os.Exit(1)
	}

	resourceMap := generateResources(directories)

	resources, firstDeployment, err := resourcesToUpdate()
	check(err, "Error happened while collecting resources to update")
	if len(resources) < 1 && !firstDeployment {
		fmt.Println("There is no need to update any resources")
		os.Exit(0)
	}

	for _, path := range resources {

		_, filename := filepath.Split(path)
		extension := filepath.Ext(filename)
		name := filename[0 : len(filename)-len(extension)]
		parts := strings.SplitN(name, "-", 3)

		serviceName := strings.Join(parts[0:2], "-")
		ns := parts[2]

		res := resourceMap[filename]

		// Applications

		app := resource.Application{
			ServiceName:       serviceName,
			KubernetesAccount: res.Spinnaker.Chart.Variables["kubernetesAccount"],
			Namespace:         ns,
		}

		err = app.Write(fmt.Sprintf("%s/applications", GeneratedDir), fmt.Sprintf("%s/application.json", TemplatesDir))
		check(err, ExecutingTemplateErrMsg)

		// Pipelines

		pipeline := resource.Pipeline{
			ServiceName: serviceName,
			Namespace:   ns,
			Regexp:      res.Spinnaker.TriggerRegexp,
			Spinnaker:   res.Spinnaker,
		}

		err = pipeline.Write(fmt.Sprintf("%s/pipelines", GeneratedDir), fmt.Sprintf("%s/pipeline.json", TemplatesDir))
		check(err, ExecutingTemplateErrMsg)
		fmt.Printf("Resource generation has finished for %s namespace.\n", ns)
	}
}
