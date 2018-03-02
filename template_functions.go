package pages

import (
	"fmt"
	"html/template"
)

func templateFunctions(p *Pages) template.FuncMap {
	return template.FuncMap{
		"routerOutlet": func(c *Context) template.HTML {
			return c.html
		},
		"alternative": func(c *Context) template.HTML {
			var links = ""
			for hreflang, href := range c.route.Alternative {
				links += fmt.Sprintf(`<link rel="alternate" hreflang="%s" href="%s">`, hreflang, href)
			}
			return template.HTML(links)
		},
		"t": func(c *Context, x string) string {
			return p.translations[x][c.Locale]
		},
	}
}