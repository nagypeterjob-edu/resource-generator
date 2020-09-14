package resource

import (
	"fmt"
	"os"

	"github.com/nagypeterjob-edu/application-values/pkg/templates"
)

type Application struct {
	ServiceName       string
	KubernetesAccount string
}

func (a *Application) Write(path string) error {
	resourcePath := fmt.Sprintf("%s/%s.json", path, a.ServiceName)
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
		return writeTmpl(a, handler, templates.ApplicationTmpl)
	}
	return nil
}
