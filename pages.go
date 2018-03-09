package pages

import (
	"html/template"
	"path"
	"path/filepath"
	"github.com/gorilla/mux"
	"bytes"
	"fmt"
	"net/http"
	"errors"
	"io"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	TemplateFilePaths []string
	templates         *template.Template
	translations      NamedTranslations
	functions         template.FuncMap

	currentContext *Context
}

type Options struct {
	base         string
	IsRendering  bool
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
	p.base = path.Dir(p.JsonFilePath)

	// set compile path
	if !path.IsAbs(p.Dist) {
		p.Dist = path.Join(p.base, p.Dist)
	}

	// set template functions
	if p.Options.IsRendering {
		p.functions = renderingTemplateFunctions(p)
	} else {
		p.functions = templateFunctions(p)
	}

	// parse templates
	for _, templatesPath := range p.Templates {
		if !path.IsAbs(templatesPath) {
			templatesPath = path.Join(p.base, templatesPath)
		}
		fs, err := filepath.Glob(templatesPath)
		if err != nil {
			panic(err)
		}
		p.TemplateFilePaths = append(p.TemplateFilePaths, fs...)
	}
	if len(p.TemplateFilePaths) > 0 {
		p.templates, err = template.New("").Funcs(p.functions).ParseFiles(p.TemplateFilePaths...)
		if err != nil {
			return p, err
		}
	} else {
		return p, errors.New("templates undefined")
	}

	return p, nil
}

func (p *Pages) Execute(wr io.Writer, layout string, page string) error {
	p.currentContext = &Context{
		/*data: map[string]interface{}{},*/
		Page: page,
	}
	buf := new(bytes.Buffer)
	err := p.templates.ExecuteTemplate(buf, page, p.currentContext)
	if err != nil {
		return err
	}
	p.currentContext.html = template.HTML(buf.String())
	return p.templates.ExecuteTemplate(wr, layout, p.currentContext)
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
	p.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		ctx := &Context{
			/*data: map[string]interface{}{},*/
			Page: route.Page,
			Vars: vars,
		}
		p.currentContext = ctx
		buf := new(bytes.Buffer)
		err := p.templates.ExecuteTemplate(buf, route.Page, ctx)
		if err != nil {
			panic(err)
		}
		ctx.html = template.HTML(buf.String())

		buf2 := new(bytes.Buffer)
		err = p.templates.ExecuteTemplate(buf2, route.Layout, ctx)
		if err != nil {
			fmt.Fprint(w, err)
		}
		w.Write(buf2.Bytes())
	})
}
