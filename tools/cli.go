package tools

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// ReadFromStringOrPath parses strOrPath and read payload,
// then return as sequence of bytes.
// If single hyphen is given, read remaining contents from
// stdin. If absolute path is given, read all contents from
// the file. Otherwise, just returns byte representation of the
// given string.
func ReadFromStringOrPath(strOrPath string) ([]byte, error) {
	if strOrPath == "" {
		return nil, fmt.Errorf("value should be string, absolute path or single hyphen as stdin")
	}
	if strOrPath == "-" {
		return ioutil.ReadAll(bufio.NewReader(os.Stdin))
	}

	if !filepath.IsAbs(strOrPath) {
		return ioutil.ReadAll(strings.NewReader(strOrPath))
	}

	file, e := os.Open(strOrPath)
	if e != nil {
		return nil, e
	}
	defer file.Close()

	return ioutil.ReadAll(bufio.NewReader(file))
}
