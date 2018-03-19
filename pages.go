package pages

import (
	"github.com/gorilla/mux"
	"net/http"
	"github.com/PuerkitoBio/goquery"
	"path/filepath"
	"path"
	"github.com/cbroglie/mustache"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	components map[string]*Component
	routeCount int
	index      *goquery.Document

	custom string
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
		Options:    opt,
		Router:     mux.NewRouter(),
		Manifest:   new(Manifest),
		components: map[string]*Component{},
	}

	// read manifest
	err := readAndUnmarshal(p.JsonFilePath, p.Manifest)
	if err != nil {
		return p, err
	}

	// set base path from calling script absolute path and settings.json dir
	p.base = filepath.Dir(p.JsonFilePath)

	// read partials
	for _, imp := range p.Imports {
		if len(imp.URL) > 0 {
			// single file definition
			if !filepath.IsAbs(imp.URL) {
				imp.URL = filepath.Join(p.base, imp.URL)
			}

			p.components[imp.Name], err = NewComponent(imp.Name, imp.URL)
			if err != nil {
				return p, err
			}
		} else {
			if !filepath.IsAbs(imp.Glob) {
				imp.Glob = filepath.Join(p.base, imp.Glob)
			}
			fs, err := filepath.Glob(imp.Glob)
			if err != nil {
				return p, err
			}

			// read templates and load into map
			for _, f := range fs {
				name := filepath.Base(f)
				name = name[0 : len(name)-len(filepath.Ext(name))]
				if len(imp.Name) > 0 {
					name = imp.Name + "-" + name
				}
				p.components[name], err = NewComponent(name, f)
				if err != nil {
					return p, err
				}
			}
		}
	}

	return p, nil
}

func (p *Pages) iter(h map[string][]*Route, route *Route, basePath string, parents []*Route) map[string][]*Route {
	p.routeCount += 1

	route.parents = parents
	route.id = p.routeCount

	newPath := path.Join(basePath, route.Path)

	h[newPath] = append(h[newPath], parents...)
	h[newPath] = append(h[newPath], route)

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			h = p.iter(h, childRoute, newPath, ps)
		}
	}
	return h
}

func (p *Pages) BuildRouter() (err error) {
	p.Router = mux.NewRouter()
	p.routeCount = -1

	// attaches routes to paths - this way we don't have two Handlers for the same path

	var handle = map[string][]*Route{}
	for _, route := range p.Routes {
		handle = p.iter(handle, route, "/", nil)
	}

	for routePath, routes := range handle {
		err = p.handleRoute(p.Router, routePath, routes)
		if err != nil {
			return err
		}
	}

	// build custom.js
	p.custom = "(function(){'use strict';"
	for _, c := range p.components {
		p.custom += c.JSTemplateLiteral()
	}
	p.custom += "})();"

	// handle custom.js
	p.Router.HandleFunc("/custom", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(p.custom))
	})

	return err
}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *mux.Router, path string, routes []*Route) (err error) {
	//mux.NewRouter().PathPrefix(opt.HandlerPathPrefix).Subrouter(),

	//html = regexp.MustCompile(`{{\s*(&gt;)`).ReplaceAllString(html, "{{>")

	/*temp, err := mustache.ParseStringPartials(html, &p.partials)
	if err != nil {
		return err
	}*/

	html, _ := p.RenderRoute(p.components["index"], routes)
	temp, err := mustache.ParseString(html)
	if err != nil {
		return err
	}

	r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		//vars := mux.Vars(r)
		temp.FRender(w, map[string]interface{}{
			"test": "Hello World!",
		})
	})

	return err
}
