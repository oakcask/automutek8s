package tools

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"text/template"
)

// ApplyTextTemplate applies template to source Reader with
// Config as data. Then returns Reader which reads result text.
func ApplyTextTemplate(source io.Reader, config Config) (io.Reader, error) {
	sourceText, e := ioutil.ReadAll(source)
	if e != nil {
		return nil, e
	}

	tmpl, e := template.New("config").Parse(string(sourceText))
	if e != nil {
		return nil, e
	}

	var buffer bytes.Buffer

	if e = tmpl.Execute(&buffer, &config); e != nil {
		return nil, e
	}

	return bufio.NewReader(&buffer), nil
}
