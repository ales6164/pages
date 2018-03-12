package pages

import (
	"path"
	"path/filepath"
	"github.com/gorilla/mux"
	"bytes"
	"net/http"
	"io/ioutil"
	"github.com/cbroglie/mustache"
	"github.com/PuerkitoBio/goquery"
	"errors"
	"regexp"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	partials  mustache.StaticProvider
	templates map[string]*mustache.Template
	documents map[string]*goquery.Document
}

type Options struct {
	base         string
	IsRendering  bool
	JsonFilePath string
}

var (
	DefaultOutlet = "#outlet"
)

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:   opt,
		Router:    mux.NewRouter(),
		Manifest:  new(Manifest),
		templates: map[string]*mustache.Template{},
		documents: map[string]*goquery.Document{},
	}

	// read manifest
	err := readAndUnmarshal(p.JsonFilePath, p.Manifest)
	if err != nil {
		return p, err
	}

	// set base path from calling script absolute path and settings.json dir
	p.base = path.Dir(p.JsonFilePath)

	// parse templates
	var partials = map[string]string{}
	for _, templatesPath := range p.Imports {
		if !path.IsAbs(templatesPath) {
			templatesPath = path.Join(p.base, templatesPath)
		}
		fs, err := filepath.Glob(templatesPath)
		if err != nil {
			return p, err
		}

		// read templates and load into map

		for _, f := range fs {
			templateBytes, err := ioutil.ReadFile(f)
			if err != nil {
				return p, err
			}
			name := path.Base(f)
			name = name[0 : len(name)-len(path.Ext(name))]
			content := string(templateBytes)

			partials[name] = content
		}
	}

	p.partials = mustache.StaticProvider{Partials: partials}

	// parse partials and replace {{>partial}} with template
	// is this even needed here?
	for name, part := range partials {
		p.templates[name], err = mustache.ParseStringPartials(part, &p.partials)
		if err != nil {
			return p, err
		}
	}

	for name, temp := range p.partials.Partials {
		buf := new(bytes.Buffer)
		buf.WriteString(temp)
		doc, err := goquery.NewDocumentFromReader(buf)
		if err != nil {
			return p, err
		}
		p.documents[name] = doc
	}

	return p, nil
}


//todo: 1
func iter(h map[string][]*Route, route *Route, basePath string, parents []*Route) map[string][]*Route {
	route.parents = parents

	if entry, ok := h[basePath+route.Path]; ok {
		entry = append(entry, route)
	} else {
		h[basePath+route.Path] = []*Route{route}
	}

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			h = iter(h, childRoute, basePath+route.Path, ps)
		}
	}
	return h
}

func (p *Pages) BuildRouter() (err error) {
	p.Router = mux.NewRouter()

	// attaches routes to paths - this way we don't have two Handlers for the same path

	var handle map[string][]*Route
	for _, route := range p.Routes {
		handle = iter(map[string][]*Route{}, route, "",nil)
	}

	for path, routes := range handle {
		err = p.handleRoute(p.Router, path, routes)
		if err != nil {
			return err
		}
	}

	return err
}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *mux.Router, path string, routes []*Route) (err error) {
	//mux.NewRouter().PathPrefix(opt.HandlerPathPrefix).Subrouter(),

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			err = p.buildRoute(r, childRoute, basePath+route.Path, ps)
			if err != nil {
				return err
			}
		}
	}

	err = p.handle(r, route, basePath, parents)
	if err != nil {
		return err
	}

	return err
}

// also needs to pass parent routes that need to be rendered before any child
func (p *Pages) buildRoute(r *mux.Router, route *Route, basePath string, parents []*Route) (err error) {
	//mux.NewRouter().PathPrefix(opt.HandlerPathPrefix).Subrouter(),

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			err = p.buildRoute(r, childRoute, basePath+route.Path, ps)
			if err != nil {
				return err
			}
		}
	}

	err = p.handle(r, route, basePath, parents)
	if err != nil {
		return err
	}

	return err
}

func (p *Pages) handle(router *mux.Router, r *Route, basePath string, parents []*Route) error {
	// 1. Render parents first
	// 2. Render actual route

	var document = p.documents["index"].Clone()

	for _, parent := range append(parents, r) {
		// set outlet
		outlet := parent.Outlet
		if len(outlet) == 0 {
			outlet = DefaultOutlet
		}

		outletSelection := document.Find(outlet)
		if outletSelection.Length() == 0 {
			return errors.New("can't find router outlet " + outlet)
		}

		component := p.documents[parent.Component].Clone()
		if component.Children().Length() == 0 {
			return errors.New("component empty " + parent.Component)
		}

		componentHtml, err := component.Html()
		if err != nil {
			return err
		}

		outletSelection.SetHtml(componentHtml)
	}

	html, err := document.Html()
	if err != nil {
		return err
	}

	html = regexp.MustCompile(`{{\s*(&gt;)`).ReplaceAllString(html, "{{>")

	temp, err := mustache.ParseStringPartials(html, &p.partials)
	if err != nil {
		return err
	}

	router.HandleFunc(basePath+r.Path, func(w http.ResponseWriter, req *http.Request) {
		//vars := mux.Vars(r)

		temp.FRender(w, map[string]interface{}{
			"page": r.Component,
		})
		//w.Write([]byte(html))
	})

	/*router.PathPrefix(basePath+r.Path).HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		//vars := mux.Vars(r)

		temp.FRender(w, map[string]interface{}{
			"page": r.Component,
		})
		//w.Write([]byte(html))
	})*/

	return nil
}
