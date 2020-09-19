package pages

import (
	"github.com/cbroglie/mustache"
	"github.com/gorilla/mux"
	"net/http"
	"path"
	"path/filepath"
)

type Pages struct {
	*mux.Router
	*Options
	*Manifest
	Components map[string]*Component
	Layouts    map[string]*Layout
	routeCount int

	custom string
}

type Options struct {
	base         string
	IsRendering  bool
	JsonFilePath string
	ForceSSL     bool
}

var (
	DefaultOutlet = "router-outlet"
	DefaultLayout = "index"
)

func HTTPSMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		x := r.Header.Get("x-forwarded-proto")
		if x == "http" {
			http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:    opt,
		Router:     mux.NewRouter(),
		Manifest:   new(Manifest),
		Components: map[string]*Component{},
		Layouts:    map[string]*Layout{},
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
			name := filepath.Base(imp.URL)
			name = name[0 : len(name)-len(filepath.Ext(name))]
			if len(imp.Prefix) > 0 {
				name = imp.Prefix + "-" + name
			}
			if imp.IsLayout {
				newL, err := NewLayout(imp.URL)
				if err != nil {
					return p, err
				}
				p.Layouts[name] = newL
				if err != nil {
					return p, err
				}
			} else {
				newC, err := NewComponent(name, imp.URL)
				if err != nil {
					return p, err
				}
				p.Components[name] = newC
				if err != nil {
					return p, err
				}
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
				if len(imp.Prefix) > 0 {
					name = imp.Prefix + "-" + name
				}
				if imp.IsLayout {
					newL, err := NewLayout(f)
					if err != nil {
						return p, err
					}
					p.Layouts[name] = newL
				} else {
					newC, err := NewComponent(name, f)
					if err != nil {
						return p, err
					}
					p.Components[name] = newC
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
	if route.Path == "/" {
		newPath += "/"
	}

	h[newPath] = append(h[newPath], parents...)

	// this IF is because we don't want to render a path that has children by it's own - should always be rendered only when rendering with child path
	if len(route.Children) == 0 {
		h[newPath] = append(h[newPath], route)
	}

	if len(route.Children) > 0 {
		ps := append(parents, route)
		for _, childRoute := range route.Children {
			h = p.iter(h, childRoute, newPath, ps)
		}
	}
	return h
}

func (p *Pages) BuildRouter(pathPrefix string) (err error) {
	p.Router = mux.NewRouter().PathPrefix(pathPrefix).Subrouter()
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
	// add templates and scripts
	p.custom = `(function(){'use strict';const arr=function(v){return v!=null?(v.constructor===Array?v:(v===false?Array(0):[v])):Array(0)};const rearr=function(v){return v=v?v.constructor===Array?v.reverse():Array(0):[v]};const html=function(a){for(var e=a.raw,f='',c=arguments.length,b=Array(1<c?c-1:0),d=1;d<c;d++)b[d-1]=arguments[d];return b.forEach(function(g,h){var j=e[h];Array.isArray(g)&&(g=g.join('')),f+=j,f+=g}),f+=e[e.length-1],f};const customComponents=new function(){this._templates={};this.setTemplate=function(name,templateFunc){this._templates[name]=templateFunc;};this.define=function(name,module){if(module&&module.hasOwnProperty('exports')){module.exports.prototype.template=this._templates[name];window.customElements.define(name,module.exports)}}};window['customComponents']=customComponents;`
	for _, c := range p.Components {
		p.custom += c.JSTemplateLiteral()
		p.custom += c.ComponentScript()
	}
	p.custom += "})();"
	// add scripts

	// handle custom.js
	p.Router.HandleFunc("/custom.js", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(p.custom))
	})

	if p.Options.ForceSSL {
		p.Router.Use(HTTPSMiddleware)
	}

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

	var layout = DefaultLayout

	if len(routes) > 0 {
		if r := routes[0]; r != nil && len(r.Layout) > 0 {
			layout = r.Layout
		}
	}

	html, err := p.RenderRoute(p.Layouts[layout], routes)
	if err != nil {
		return err
	}
	html = Decode(html)
	temp, err := mustache.ParseString(html)
	if err != nil {
		return err
	}

	r.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		//vars := mux.Vars(r)
		temp.FRender(w, nil)
	})

	return err
}
