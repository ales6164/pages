package pages

import (
	"io/ioutil"
	"github.com/aymerick/raymond"
	"github.com/ales6164/apis/errors"
)

type Component struct {
	Name     string
	Template *raymond.Template
	Raw      string

	isLayout bool
}

func NewComponent(name string, filePath string, isLayout bool) (*Component, error) {
	var c = new(Component)
	c.Name = name
	c.isLayout = isLayout

	fs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return c, err
	}

	c.Raw = string(fs)
	raymond.RegisterPartial(c.Name, c.Raw)
	c.Template, err = raymond.Parse(c.Raw)
	if err != nil {
		return c, errors.New("error parsing file: " + filePath + "; " + err.Error())
	}

	return c, nil
}
