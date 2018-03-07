package pages

import (
	"html/template"
	"path"
	"path/filepath"
	"github.com/gorilla/mux"
	"bytes"
	"fmt"
	"os"
	"net/http"
	"errors"
	"io"
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
	err := readAndUnmarshal(p.JsonFilePath, p.Manifest)
	if err != nil {
		return p, err
	}

	// set base path from calling script absolute path and settings.json dir
	p.Base = path.Join(p.Base, path.Dir(p.JsonFilePath))

	// parse resources
	/*for _, res := range p.Resources {
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
	}*/

	// set compile path
	if !path.IsAbs(p.Dist) {
		p.Dist = path.Join(p.Base, p.Dist)
	}

	// set template functions
	p.functions = templateFunctions(p)

	// parse templates
	var files []string
	for _, templatesPath := range p.Templates {
		if !path.IsAbs(templatesPath) {
			templatesPath = path.Join(p.Base, templatesPath)
		}
		fs, err := filepath.Glob(templatesPath)
		if err != nil {
			panic(err)
		}
		files = append(files, fs...)
	}
	if len(files) > 0 {
		p.templates, err = template.New("").Funcs(p.functions).ParseFiles(files...)
		if err != nil {
			return p, err
		}
	} else {
		return p, errors.New("templates undefined")
	}

	// build router
	/*var layout string
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
			*//*route.locale = p.DefaultLocale
			if route.Alternative != nil {
				route.Alternative[p.DefaultLocale] = pattern
			}*//*

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
	}*/

	return p, nil
}

func (p *Pages) Execute(wr io.Writer, layout string, page string) error {
	ctx := &Context{
		data: map[string]interface{}{},
		Page: page,
	}
	buf := new(bytes.Buffer)
	err := p.templates.ExecuteTemplate(buf, page, ctx)
	if err != nil {
		return err
	}
	ctx.html = template.HTML(buf.String())
	return p.templates.ExecuteTemplate(wr, layout, ctx)
}

func (p *Pages) BuildRouter() {
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

			/*if route.Alternative != nil {
				route.Alternative[p.DefaultLocale] = pattern
			}*/

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
}

func (p *Pages) handle(pattern string, route *Route) {
	ctx := &Context{
		data:   map[string]interface{}{},
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
		err := p.templates.ExecuteTemplate(w, route.Layout, ctx)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	})
}
