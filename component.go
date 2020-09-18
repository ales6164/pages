package main

import (
	"errors"
	"github.com/ales6164/raymond"
	"io/ioutil"
)

type Component struct {
	*Import
	Template *raymond.Template
	//Raw              string
}

func NewComponent(im *Import) (*Component, error) {
	var c = new(Component)
	c.Import = im

	fs, err := ioutil.ReadFile(c.TemplatePath)
	if err != nil {
		return c, err
	}
	raw := string(fs)
	fs = []byte("")

	if len(c.ComponentPath) > 0 {
		fs, err = ioutil.ReadFile(c.ComponentPath)
		if err != nil {
			return c, err
		}
		/*c.RawSelfContained = fs*/
	}

	if im.Render {
		if im.OmitTags {
			raymond.RegisterPartial(c.Name, raw)
		} else {
			raymond.RegisterPartial(c.Name, "<"+c.Name+">"+raw+"</"+c.Name+">")
		}
	} else {
		raymond.RegisterPartial(c.Name, "<"+c.Name+"></"+c.Name+">")
	}

	//raymond.RegisterPartial(c.Name, "<"+c.Name+">"+c.Raw+"</"+c.Name+">")
	c.Template, err = raymond.Parse(raw)
	raw = ""
	if err != nil {
		return c, errors.New("error parsing file: " + c.TemplatePath + "; " + err.Error())
	}

	return c, nil
}
