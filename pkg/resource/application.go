package resource

import (
	"fmt"
	"os"
)

type Application struct {
	ServiceName       string
	KubernetesAccount string
	Namespace         string
}

func (a *Application) Write(path string, tmpl string) error {
	resourcePath := fmt.Sprintf("%s/%s.json", path, a.getServiceName())
	_, err := os.Stat(resourcePath)

	// We don't want to write <application>.json for each namespace
	// Open file handler only when there is no <application>.json yet
	if os.IsNotExist(err) {
		handler, err := os.Create(resourcePath)
		if err != nil {
			fmt.Printf("%s, err: %s \n", CreatingTemplateErrMsg, err)
			os.Exit(1)
		}
		defer handler.Close()
		return writeTmpl(a, handler, path, tmpl)
	}
	return nil
}

func (a *Application) getServiceName() string {
	return a.ServiceName
}

func (a *Application) getNs() string {
	return a.Namespace
}