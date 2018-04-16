package pages

import (
	"io/ioutil"
	"github.com/aymerick/raymond"
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
	c.Template = raymond.MustParse(c.Raw)

	return c, err
}
