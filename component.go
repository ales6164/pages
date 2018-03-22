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
	Name     string
	Document *goquery.Document
	Template *goquery.Selection
	template string
	Script string

	isRealComponent     bool
	isTemplateConverted bool
	templateLiteral     string

	encodedTemplate string // encoded mustache template
	encoded         map[string]Stache
}

type Layout struct {
	Document *goquery.Document

	isTemplateConverted bool
	templateLiteral     string

	encoded             map[string]Stache
	encodedTemplate string // encoded mustache template
}

var (
	regTemplate = regexp.MustCompile(`<template[^>]*>([^$]+?)<\/template>`)
	regScript   = regexp.MustCompile(`<script[^>]*>([^$]+?)<\/script>`)
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
	l.encoded, l.encodedTemplate = Encode(html)

	buf := new(bytes.Buffer)
	buf.WriteString(l.encodedTemplate)
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

	html := string(fs)

	// find <template>
	c.template = regTemplate.ReplaceAllString(html, `$1`)
	// find <script>
	c.Script = regScript.ReplaceAllString(html, `$1`)

	// encode all mustache tags as html comments for later use
	c.encoded, c.encodedTemplate = Encode(c.template)

	buf := new(bytes.Buffer)
	buf.WriteString(c.encodedTemplate)
	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		return c, err
	}

	c.Template = doc.Find("body")

	return c, nil
}

func (p *Pages) Assemble(s *goquery.Selection, content string) (doc *goquery.Selection) {
	doc = s.Clone()
	doc.Find("*").Each(func(i int, selection *goquery.Selection) {
		name := goquery.NodeName(selection)
		if child, ok := p.components[name]; ok {
			selectionContent, _ := selection.Html()
			assembledChild := p.Assemble(child.Template, selectionContent)
			assembledChildHtml, _ := assembledChild.Html()
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
	body := p.Assemble(layout.Document.Find("body"), "")

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

		if routeComponent, ok := p.components[route.Component]; ok {
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
	c.templateLiteral = "customComponents.setTemplate('" + c.Name + "',function($){var $$=$;return" + ConvertMustache(c.template) + "});"
	c.isTemplateConverted = true
	return c.templateLiteral
}

func (c *Component) ComponentScript() string {
	return "customComponents.define('" + c.Name + "',(function(){var module={};" + fmt.Sprint(c.Script) + ";return module})());"
}
