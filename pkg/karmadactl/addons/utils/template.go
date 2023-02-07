package utils

import (
	"bytes"
	"fmt"
	"text/template"
)

// ParseTemplate validates and parses passed as argument template
func ParseTemplate(strTmpl string, obj interface{}) ([]byte, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("template").Parse(strTmpl)
	if err != nil {
		return nil, fmt.Errorf("error when parsing template: %w", err)
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return nil, fmt.Errorf("error when executing template: %w", err)
	}
	return buf.Bytes(), nil
}
