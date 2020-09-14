package resource

import (
	"fmt"
	"os"

	"github.com/nagypeterjob-edu/application-values/pkg/templates"
)

type Pipeline struct {
	Namespace   string
	ServiceName string
	Spinnaker   SpinnakerConfig
}

func (p *Pipeline) Write(path string) error {
	handler, err := os.Create(fmt.Sprintf("%s/%s-%s.json", path, p.ServiceName, p.Namespace))
	if err != nil {
		fmt.Printf("%s, err: %s \n", CreatingTemplateErrMsg, err)
		os.Exit(1)
	}
	defer handler.Close()
	return writeTmpl(p, handler, templates.PipelineTmpl)
}
