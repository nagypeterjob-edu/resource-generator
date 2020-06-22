package resource

import (
	"fmt"
	"io/ioutil"
	"os"
	"text/template"
)

type ResourceType interface {
	Write(string, string) error
	getServiceName() string
	getNs() string
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

var (
	CreatingTemplateErrMsg = "Error creating file for go template to fill"
	ParsingTemplateErrMsg  = "Error orccured while parsing template"
)

func writeTmpl(res ResourceType, handler *os.File, path string, tmplPath string) error {
	data, err := ioutil.ReadFile(tmplPath)
	if err != nil {
		fmt.Printf("Error occured while reading %s", tmplPath)
		os.Exit(1)
	}

	tmpl, err := template.New(res.getServiceName()).Parse(string(data))
	if err != nil {
		fmt.Printf("%s, err: %s \n", ParsingTemplateErrMsg, err)
		os.Exit(1)
	}

	err = tmpl.Execute(handler, res)
	return err
}
