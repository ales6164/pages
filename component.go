package pages

import (
	"github.com/PuerkitoBio/goquery"
	"os"
	"golang.org/x/net/html"
	"strings"
)

type Component struct {
	Name         string
	Document     *goquery.Document
	HTMLTemplate string
	Script       string

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
	defer f.Close()

	c.Document, err = goquery.NewDocumentFromReader(f)
	if err != nil {
		return c, err
	}

	// find <template>
	c.Document.ChildrenFiltered("template").EachWithBreak(func(i int, selection *goquery.Selection) bool {
		c.HTMLTemplate, err = selection.Html()
		return false
	})
	if err != nil {
		return c, err
	}

	// find <script>
	c.Document.ChildrenFiltered("script").EachWithBreak(func(i int, selection *goquery.Selection) bool {
		c.Script, err = selection.Html()
		return false
	})
	if err != nil {
		return c, err
	}

	return c, nil
}

func (p *Pages) Render(sel *goquery.Selection) string {
	sel.FindMatcher(Matcher{}).Each(func(i int, selection *goquery.Selection) {
		var name = selection.Get(0).Namespace
		if c, ok := p.components[name]; ok {
			selection.
		}
	})
	return c.templateLiteral
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

func (m *Matcher) Filter(node *html.Node)  []*html.Node {
	if m.Match(node) {
		return []*html.Node{node}
	}
	return nil
}

func (c *Component) JSTemplateLiteral() string {
	if c.isTemplateConverted {
		return c.templateLiteral
	}
	c.templateLiteral = ConvertMustache(c.HTMLTemplate)
	c.isTemplateConverted = true
	return c.templateLiteral
}
