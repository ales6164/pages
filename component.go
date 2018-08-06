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
	Render   bool

	isLayout bool
}

func NewComponent(name, filePath string, isLayout, render bool) (*Component, error) {
	var c = new(Component)
	c.Name = name
	c.isLayout = isLayout
	c.Render = render

	fs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return c, err
	}

	c.Raw = string(fs)
	raymond.RegisterPartial(c.Name, "<"+c.Name+">"+c.Raw+"</"+c.Name+">")
	c.Template, err = raymond.Parse(c.Raw)
	if err != nil {
		return c, errors.New("error parsing file: " + filePath + "; " + err.Error())
	}

	return c, nil
}
