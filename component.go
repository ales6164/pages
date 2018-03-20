package pages

import (
	"github.com/PuerkitoBio/goquery"
	"os"
	"errors"
	"regexp"
	"fmt"
)

type Component struct {
	Name     string
	Document *goquery.Document
	Template *goquery.Selection
	Script   string

	isRealComponent     bool
	isTemplateConverted bool
	templateLiteral     string
}

func NewComponent(name string, filePath string) (*Component, error) {
	var c = new(Component)
	c.Name = name

	f, err := os.Open(filePath)
	if err != nil {
		return c, err
	}
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		return c, err
	}
	f.Close()

	body := doc.Find("body")

	// find <template>
	body.Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {
		if goquery.NodeName(selection) == "template" {
			c.isRealComponent = true
			c.Template = selection
			return false
		}
		return true
	})
	if err != nil {
		return c, err
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
			return c, err
		}
	} else {
		c.Document = doc
		c.Template = body
	}

	return c, nil
}

var reContent = regexp.MustCompile(`\{\{\s*(?P<var>content+)\s*\}\}`)
var setComponentContent = func(content, layout string) string {
	return reContent.ReplaceAllString(layout, content)
}

func (p *Pages) Assemble(c *Component, content string) (doc *goquery.Selection) {
	doc = c.Template.Clone()
	doc.Find("*").Each(func(i int, selection *goquery.Selection) {
		name := goquery.NodeName(selection)
		if child, ok := p.components[name]; ok {
			selectionContent, _ := selection.Html()
			assembledChild := p.Assemble(child, selectionContent)
			assembledChildHtml, _ := assembledChild.Html()
			selection.SetHtml(assembledChildHtml)
		}
	})
	selHtml, _ := doc.Html()

	// set innerHTML
	doc.SetHtml(setComponentContent(content, selHtml))

	return doc
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
	h, _ := c.Template.Html()
	c.templateLiteral = ConvertMustache(c.Name, h)
	c.isTemplateConverted = true
	return c.templateLiteral
}

func (c *Component) ComponentScript() string {
	return "customComponents.define('" + c.Name + "',(function(){var module={};" + fmt.Sprint(c.Script) + ";return module})());"
}
