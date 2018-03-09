package pages

import (
	"html/template"
)

func templateFunctions(p *Pages) template.FuncMap {
	return template.FuncMap{
		"routerOutlet": func() template.HTML {
			return p.currentContext.html
		},
		/*"fetch": func(kind string, id string) map[string]interface{} {
			return nil
		},*/
		/*"alternative": func(c *Context) template.HTML {
			var links = ""
			for hreflang, href := range c.route.Alternative {
				links += fmt.Sprintf(`<link rel="alternate" hreflang="%s" href="%s">`, hreflang, href)
			}
			return template.HTML(links)
		},*/
		/*"t": func(c *Context, x string) string {
			return p.translations[x][c.Locale]
		},*/
	}
}

func renderingTemplateFunctions(p *Pages) template.FuncMap {
	return template.FuncMap{
		"routerOutlet": func() template.HTML {
			return p.currentContext.html
		},
		/*"fetch": func(kind string, id string) map[string]interface{} {
			rs, err := http.Get(p.API + kind + "/" + id)
			if err != nil {
				panic(err)
			}
			defer rs.Body.Close()
			bodyBytes, err := ioutil.ReadAll(rs.Body)
			if err != nil {
				panic(err)
			}
			var data map[string]interface{}
			err = json.Unmarshal(bodyBytes, &data)
			if err != nil {
				panic(err)
			}
			return data
		},*/
		/*"alternative": func(c *Context) template.HTML {
			var links = ""
			for hreflang, href := range c.route.Alternative {
				links += fmt.Sprintf(`<link rel="alternate" hreflang="%s" href="%s">`, hreflang, href)
			}
			return template.HTML(links)
		},*/
		/*"t": func(c *Context, x string) string {
			return p.translations[x][c.Locale]
		},*/
	}
}
