package pages

import (
	"github.com/PuerkitoBio/goquery"
	"errors"
	"regexp"
	"fmt"
	"io/ioutil"
	"strings"
)

type Component struct {
	Name     string
	Document *goquery.Document
	Template string
	Script   string

	isPartialParsed     bool
	isRealComponent     bool
	isTemplateConverted bool
	templateLiteral     string

	regex              *regexp.Regexp
	foundSubComponents []*Component
}

var (
	regTemplate = regexp.MustCompile(`<template[^>]*>([^$]+?)<\/template>`)
	regScript   = regexp.MustCompile(`<script[^>]*>([^$]+?)<\/script>`)
	regContent = regexp.MustCompile(`\{\{\s*(?P<var>content+)\s*\}\}`)
	regRouter = regexp.MustCompile(`(<router-outlet[^>]*>)([^$]*)(<\/router-outlet>)`)
)

func NewComponent(name string, filePath string) (*Component, error) {
	var c = new(Component)
	c.Name = name
	c.regex = regexp.MustCompile(`(<` + name + `[^>]*>)([^$]+?)(<\/` + name + `>)`)

	fs, err := ioutil.ReadFile(filePath)
	if err != nil {
		return c, err
	}

	html := string(fs)

	// find <template>
	c.Template = regTemplate.ReplaceAllString(html, `$1`)

	// find <script>
	c.Script = regScript.ReplaceAllString(html, `$1`)

	return c, nil
}

// find sub components
func (c *Component) Parse(provider []*Component) {
	for _, p := range provider {
		if p.regex.FindStringIndex(c.Template) != nil {
			// this component has a sub component p
			c.foundSubComponents = append(c.foundSubComponents, p)

			if c == p {
				panic(errors.New("component can't contain itself"))
			}
		}
	}
}

func (p *Pages) Assemble(c *Component, content string) string {
	doc := c.Template

	for _, sc := range c.foundSubComponents {
		// najdem celoten child componento tag v layoutu in ga nadomestim z assemblano verzijo
		replaceAllGroupFunc(sc.regex, doc, func(groups []string) string {
			// tukaj dobim content elementa
			var tagStart = groups[1]
			var content = groups[2]
			var tagEnd = groups[3]

			// nadaljujem z assemblanjem te sc componente kjer vstavim ta content notr
			assembledChild := p.Assemble(sc, content)

			// returnam assemblan string v regex
			return tagStart + assembledChild + tagEnd
		})
	}

	// tukaj še moram samo vstaviti content v obstoječ doc
	return regContent.ReplaceAllString(doc, content)
}

func renderPage() {
	doc := c.Template

	for _, sc := range c.foundSubComponents {
		// najdem celoten child componento tag v layoutu in ga nadomestim z assemblano verzijo
		replaceAllGroupFunc(sc.regex, doc, func(groups []string) string {
			// tukaj dobim content elementa
			var tagStart = groups[1]
			var content = groups[2]
			var tagEnd = groups[3]

			// nadaljujem z assemblanjem te sc componente kjer vstavim ta content notr
			assembledChild := p.Assemble(sc, content)

			// returnam assemblan string v regex
			return tagStart + assembledChild + tagEnd
		})
	}

	// tukaj še moram samo vstaviti content v obstoječ doc
	return regContent.ReplaceAllString(doc, content)
}

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (string, error) {
	var outerHtml string
	doc := p.Assemble(layout, "")

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

		// find router-outlet

		outletSelection := doc.Find(outlet)
		if outletSelection.Length() == 0 {
			return outerHtml, errors.New("can't find router outlet " + outlet)
		}

		if routeComponent, ok := p.components[route.Component]; ok {
			assembled := p.Assemble(routeComponent, "")
			assembledHtml, _ := assembled.Html()

			outletSelection.SetHtml("<" + routeComponent.Name + ">" + assembledHtml + "</" + routeComponent.Name + ">")
		} else {
			return outerHtml, errors.New("trying to access undefined component " + route.Component)
		}
	}

	layout.Document.Find("body").ReplaceWithSelection(doc)

	return layout.Document.Html()
	//return goquery.OuterHtml(doc)
}

func (c *Component) JSTemplateLiteral() string {
	if c.isTemplateConverted {
		return c.templateLiteral
	}

	c.templateLiteral = "customComponents.setTemplate('" + c.Name + "',function($){var $$=$;return" + ConvertMustache(c.Template) + "});"
	c.isTemplateConverted = true
	return c.templateLiteral
}

func (c *Component) ComponentScript() string {
	return "customComponents.define('" + c.Name + "',(function(){var module={};" + fmt.Sprint(c.Script) + ";return module})());"
}
