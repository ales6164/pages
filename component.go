package pages

import (
	"github.com/PuerkitoBio/goquery"
	"errors"
	"regexp"
	"fmt"
	"io/ioutil"
	"bytes"
)

type Component struct {
	Name             string
	Template         *goquery.Selection
	EncodedTemplate  string // encoded mustache template
	Script           string

	isRealComponent     bool
	isTemplateConverted bool
	templateLiteral     string
}

type Layout struct {
	Document *goquery.Document

	isTemplateConverted bool
	templateLiteral     string

	EncodedTemplate string // encoded mustache template
}

var (
	//regTemplate = regexp.MustCompile(`<template[^>]*>([^$]+?)<\/template>`)
	regContent  = regexp.MustCompile(`<!--stache-content-->`)
)

func NewLayout(filePath string) (*Layout, error) {
	var l = new(Layout)

	fs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return l, err
	}

	html := string(fs)

	// encode all mustache tags as html comments for later use
	l.EncodedTemplate = Encode(html)

	buf := new(bytes.Buffer)
	buf.WriteString(l.EncodedTemplate)
	l.Document, err = goquery.NewDocumentFromReader(buf)

	return l, err

}

func NewComponent(name string, filePath string) (*Component, error) {
	var c = new(Component)
	c.Name = name

	fs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return c, err
	}

	err = c.Parse(string(fs))

	return c, err
}

func (c *Component) Parse(html string) error {
	html = Encode(html)

	buf := new(bytes.Buffer)
	buf.WriteString(html)

	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		return err
	}

	body := doc.Find("body")

	// find <template>
	body.Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {
		if goquery.NodeName(selection) == "template" {
			c.isRealComponent = true
			c.EncodedTemplate, _ = selection.Html()
			c.Template = selection
			return false
		}
		return true
	})
	if err != nil {
		return err
	}

	if c.Template != nil {
		// find <script>
		body.Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {
			if goquery.NodeName(selection) == "script" {
				c.Script = selection.Text()
				return false
			}
			return true
		})
		if err != nil {
			return err
		}
	} else {
		c.Template = body
	}

	return nil
}

func (p *Pages) Assemble(doc *goquery.Selection, content string) *goquery.Selection {
	doc.Find("*").Each(func(i int, selection *goquery.Selection) {
		name := goquery.NodeName(selection)
		if child, ok := p.Components[name]; ok {
			//selectionContent, _ := selection.Html() // element html content
			assembledChild := p.Assemble(child.Template.Clone(), "") // assemble element with above content
			assembledChildHtml, _ := assembledChild.Html()           // get new html content
			selection.SetHtml(assembledChildHtml)
		}
	})
	selHtml, _ := doc.Html()

	// set innerHTML
	doc.SetHtml(regContent.ReplaceAllString(selHtml, content))

	return doc
}

func (p *Pages) RenderRoute(layout *Layout, routes []*Route) (string, error) {
	var outerHtml string
	body := p.Assemble(layout.Document.Find("body").Clone(), "")

	var done = map[int]bool{}
	//var routesToHandle []*Route
	for _, route := range routes {
		// one path match with multiple routes
		// how to handle multiple routes?
		// compare if it's been handled already

		if _, ok := done[route.id]; ok {
			continue
		}
		done[route.id] = true

		// set outlet
		outlet := route.Outlet
		if len(outlet) == 0 {
			outlet = DefaultOutlet
		}

		outletSelection := body.Find(outlet)
		if outletSelection.Length() == 0 {
			return outerHtml, errors.New("can't find router outlet " + outlet)
		}

		if routeComponent, ok := p.Components[route.Component]; ok {
			assembled := p.Assemble(routeComponent.Template, "")
			assembledHtml, _ := assembled.Html()

			outletSelection.SetHtml("<" + routeComponent.Name + ">" + assembledHtml + "</" + routeComponent.Name + ">")
		} else {
			return outerHtml, errors.New("trying to access undefined component " + route.Component)
		}
	}

	layout.Document.Find("body").ReplaceWithSelection(body)

	return layout.Document.Html()
	//return goquery.OuterHtml(doc)
}

func (c *Component) JSTemplateLiteral() string {
	if c.isTemplateConverted {
		return c.templateLiteral
	}
	c.templateLiteral = "customComponents.setTemplate('" + c.Name + "',function($){var $$=$;return html\x60" + ConvertMustache(Decode(c.EncodedTemplate), false) + "\x60});"
	c.isTemplateConverted = true
	return c.templateLiteral
}

func (c *Component) ComponentScript() string {
	return "customComponents.define('" + c.Name + "',(function(){var module={};" + fmt.Sprint(c.Script) + ";return module})());"
}
