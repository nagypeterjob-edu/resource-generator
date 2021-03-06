package resource

import (
	"fmt"
	"os"
	"text/template"
)

type ResourceType interface {
	Write(string) error
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
