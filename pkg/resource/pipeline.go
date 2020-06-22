package resource

import (
	"fmt"
	"os"
)

type Pipeline struct {
	ServiceName string
	Namespace   string
	Regexp      string
	Spinnaker   Spinnaker
}

func (p *Pipeline) Write(path string, tmpl string) error {
	handler, err := os.Create(fmt.Sprintf("%s/%s-%s.json", path, p.getServiceName(), p.getNs()))
	if err != nil {
		fmt.Printf("%s, err: %s \n", CreatingTemplateErrMsg, err)
		os.Exit(1)
	}
	defer handler.Close()
	return writeTmpl(p, handler, path, tmpl)
}

func (a *Pipeline) getServiceName() string {
	return a.ServiceName
}

func (a *Pipeline) getNs() string {
	return a.Namespace
}
