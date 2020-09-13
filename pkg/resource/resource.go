package resource

import (
	"fmt"
	"os"
	"text/template"
)

type ResourceType interface {
	Write(string) error
}

type Spinnaker struct {
	TemplateId    string `mapstructure:"templateId"`
	TriggerRegexp string `mapstructure:"triggerRegexp"`
	Chart         struct {
		Name      string            `mapstructure:"name"`
		Variables map[string]string `mapstructure:"variables"`
	} `mapstructure:"chart"`
}

type Values struct {
	ReplicaCount         int                    `mapstructure:"replicaCount"`
	EnvironmentVariables map[string]interface{} `mapstructure:"environmentVariables"`

	Spinnaker `mapstructure:"spinnaker"`
}

var (
	CreatingTemplateErrMsg = "Error creating file for go template to fill"
	ParsingTemplateErrMsg  = "Error orccured while parsing template"
)

func writeTmpl(res ResourceType, handler *os.File, rawTmpl string) error {
	tmpl, err := template.New("tmpl").Parse(rawTmpl)
	if err != nil {
		fmt.Printf("%s, err: %s \n", ParsingTemplateErrMsg, err)
		os.Exit(1)
	}

	err = tmpl.Execute(handler, res)
	return err
}
