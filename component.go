package pages

import (
	"github.com/PuerkitoBio/goquery"
	"os"
	"golang.org/x/net/html"
	"strings"
	"github.com/hoisie/mustache"
	"errors"
)

type Component struct {
	Name     string
	Template *goquery.Selection
	Script   *goquery.Selection

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

			//innerTemplate, _ := selection.Html()

			/*buf := new(bytes.Buffer)
			buf.WriteString("<" + c.Name + ">" + innerTemplate + "</" + c.Name + ">")
			temp, _ := goquery.NewDocumentFromReader(buf)
			buf.Reset()*/
			c.Template = selection.Children()

			return false
		}
		return true
	})
	if err != nil {
		return c, err
	}

	if c.Template != nil {
		// find <script>
		/*body.Children().EachWithBreak(func(i int, selection *goquery.Selection) bool {
			name := goquery.NodeName(selection)
			if name == "script" {
				c.Script = selection
				return false
			}
			return true
		})
		if err != nil {
			return c, err
		}*/
	} else {
		c.Template = doc.Selection
	}

	return c, nil
}

func (p *Pages) Render(c *Component, content string, data interface{}) string {
	doc := c.Template.Clone()
	doc.Find("*").Each(func(i int, selection *goquery.Selection) {
		name := goquery.NodeName(selection)
		if c, ok := p.components[name]; ok {
			selectionContent, _ := selection.Html()
			selection.SetHtml(p.Render(c, selectionContent, data))
		}
	})
	selHtml, _ := doc.Html()
	return mustache.RenderInLayout(content, selHtml, data)
}

func (p *Pages) Assemble(c *Component, content string, data interface{}) *goquery.Selection {
	doc := c.Template.Clone()
	doc.Find("*").Each(func(i int, selection *goquery.Selection) {
		name := goquery.NodeName(selection)
		if child, ok := p.components[name]; ok {
			selectionContent, _ := selection.Html()
			assembledChild := p.Assemble(child, selectionContent, data)
			//election.ReplaceWithSelection(assembledChild)
			assembledChildHtml, _ := goquery.OuterHtml(assembledChild)
			selection.ReplaceWithHtml("<" + child.Name + ">" + assembledChildHtml + "</" + child.Name + ">")
		}
	})
	selHtml, _ := doc.Html()
	doc.SetHtml(mustache.RenderInLayout(content, selHtml, data))
	return doc
}

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (string, error) {
	var outerHtml string
	doc := p.Assemble(layout, "", nil)

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
			outletSelection.SetHtml("<" + routeComponent.Name + ">" + p.Render(routeComponent, "", nil) + "</" + routeComponent.Name + ">")
		} else {
			return outerHtml, errors.New("trying to access undefined component " + route.Component)
		}
	}

	return goquery.OuterHtml(doc)
}

type Matcher struct {
	goquery.Matcher
}

func (m *Matcher) Match(node *html.Node) bool {
	return strings.Contains(node.Namespace, "-")
}

func (m *Matcher) MatchAll(node *html.Node) []*html.Node {
	if m.Match(node) {
		return []*html.Node{node}
	}
	return nil
}

func (m *Matcher) Filter(node *html.Node) []*html.Node {
	if m.Match(node) {
		return []*html.Node{node}
	}
	return nil
}

func (c *Component) JSTemplateLiteral() string {
	if c.isTemplateConverted {
		return c.templateLiteral
	}
	html, _ := c.Template.Html()
	c.templateLiteral = ConvertMustache(html)
	c.isTemplateConverted = true
	return c.templateLiteral
}
