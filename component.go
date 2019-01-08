package pages

import (
	"errors"
	"github.com/aymerick/raymond"
	"io/ioutil"
)

type Component struct {
	*Import
	Template         *raymond.Template
	Raw              string
	RawSelfContained []byte
}

func NewComponent(im *Import) (*Component, error) {
	var c = new(Component)
	c.Import = im

	fs, err := ioutil.ReadFile(c.TemplatePath)
	if err != nil {
		return c, err
	}
	c.Raw = string(fs)

	if len(c.ComponentPath) > 0 {
		fs, err = ioutil.ReadFile(c.ComponentPath)
		if err != nil {
			return c, err
		}
		c.RawSelfContained = fs
	}

	raymond.RegisterPartial(c.Name, "<"+c.Name+">"+c.Raw+"</"+c.Name+">")
	c.Template, err = raymond.Parse(c.Raw)
	if err != nil {
		return c, errors.New("error parsing file: " + c.TemplatePath + "; " + err.Error())
	}

	return c, nil
}
