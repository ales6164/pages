package pages

import (
	"html/template"
	"path"
	"io/ioutil"
	"encoding/json"
	"path/filepath"
	"github.com/gorilla/mux"
	"bytes"
	"fmt"
	"os"
	"net/http"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	templates    *template.Template
	translations NamedTranslations
	functions    template.FuncMap
}

type Options struct {
	Base         string
	JsonFilePath string
}

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:      opt,
		Router:       mux.NewRouter(),
		Manifest:     new(Manifest),
		translations: NamedTranslations{},
	}

	// read manifest
	err := readAndUnmarshal(path.Join(p.Base, p.JsonFilePath), p.Manifest)
	if err != nil {
		return p, err
	}

	// parse resources
	for _, res := range p.Resources {
		bs, err := ioutil.ReadFile(path.Join(p.Base, res.Src))
		if err != nil {
			panic(err)
		}

		switch res.Type {
		case "key-locale":
			var locale NamedTranslations
			err = json.Unmarshal(bs, &locale)
			if err != nil {
				panic(err)
			}
			for k, v := range locale {
				p.translations[k] = v
			}
		}
	}

	// set template functions
	p.functions = templateFunctions(p)

	// parse templates
	var files []string
	for _, templatesPath := range p.Templates {
		fs, err := filepath.Glob(path.Join(p.Base, templatesPath))
		if err != nil {
			panic(err)
		}
		files = append(files, fs...)
	}
	if len(files) > 0 {
		p.templates = template.Must(template.New("").Funcs(p.functions).ParseFiles(files...))
	}

	// build router
	var layout string
	for _, router := range p.Routers {
		layout = p.Layout
		if len(router.Layout) > 0 {
			layout = router.Layout
		}
		for pattern, route := range router.Handle {
			if len(route.Layout) == 0 {
				route.Layout = layout
			}
			route.pattern = pattern
			route.locale = p.DefaultLocale
			route.Alternative[p.DefaultLocale] = pattern

			p.handle(pattern, route)

			for locale, pattern := range route.Alternative {
				p.handle(pattern, &Route{
					locale:      locale,
					pattern:     pattern,
					Alternative: route.Alternative,
					Page:        route.Page,
					Layout:      route.Layout,
					layout:      route.layout,
				})
			}
		}
	}

	return p, nil
}

func (p *Pages) handle(pattern string, route *Route) {
	ctx := &Context{
		route:  route,
		data:   map[string]interface{}{},
		Locale: route.locale,
		Page:   route.Page,
	}
	buf := new(bytes.Buffer)
	err := p.templates.ExecuteTemplate(buf, route.Page, ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	ctx.html = template.HTML(buf.String())
	p.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx.r = r
		err := p.templates.ExecuteTemplate(w, route.Layout, ctx)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	})
}
