package pages

import (
	"net/http"
	"path/filepath"
	"path"
	"github.com/aymerick/raymond"
	"regexp"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
	"bytes"
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"io/ioutil"
	"net/url"

	"github.com/pkg/errors"
)

type Pages struct {
	router     *httprouter.Router
	*Options
	*Manifest
	Components map[string]*Component
	routeCount int
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

func HTTPSMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		x := r.Header.Get("x-forwarded-proto")
		if x == "http" {
			http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
			return
		}
		next(w, r, ps)
	}
}

func New(opt *Options) (*Pages, error) {
	p := &Pages{
		Options:    opt,
		Manifest:   new(Manifest),
		Components: map[string]*Component{},
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
		if len(imp.Path) > 0 {
			// single file definition
			if !filepath.IsAbs(imp.Path) {
				imp.Path = filepath.Join(p.base, imp.Path)
			}
			newC, err := NewComponent(imp.Name, imp.Path, imp.IsLayout)
			if err != nil {
				return p, err
			}
			p.Components[imp.Name] = newC
			if err != nil {
				return p, err
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

func (p *Pages) BuildRouter() (*httprouter.Router, error) {
	p.router = httprouter.New()
	p.routeCount = -1

	// attaches routes to paths - this way we don't have two Handlers for the same path

	var handle = map[string][]*Route{}
	for _, route := range p.Routes {
		handle = p.iter(handle, route, "/", nil)
	}

	for routePath, routes := range handle {
		err := p.handleRoute(p.router, routePath, routes)
		if err != nil {
			return p.router, err
		}
	}

	// serve static files
	public := path.Join(p.base, "public")
	files, err2 := ioutil.ReadDir(public)
	if err2 == nil {
		for _, file := range files {
			if file.IsDir() {
				p.router.ServeFiles("/"+file.Name()+"/*filepath", http.Dir(path.Join(public, file.Name())))
			} else {
				p.router.Handler(http.MethodGet, "/"+file.Name(), http.FileServer(http.Dir(public)))
			}
		}
	}

	return p.router, nil
}

// one path can have multiple routes defined -> when having multiple routers on one page
func (p *Pages) handleRoute(r *httprouter.Router, path string, routes []*Route) (err error) {
	var layout = DefaultLayout

	if len(routes) > 0 {
		if r := routes[0]; r != nil && len(r.Layout) > 0 {
			layout = r.Layout
		}
	}

	context, templ, apiUri, err := p.RenderRoute(p.Components[layout], routes)
	if err != nil {
		return err
	}

	var hasApi = len(apiUri) > 0

	var handleFunc httprouter.Handle = func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		ctx := appengine.NewContext(req)

		if hasApi {

			resolvedApiUri := regex.ReplaceAllStringFunc(apiUri, func(s string) string {
				return ps.ByName(s[1:])
			})

			// add query parameters to the api request

			apiUrl, err := url.Parse(resolvedApiUri)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			apiUrlQuery := apiUrl.Query()
			reqQuery := req.URL.Query()
			for paramName, val := range reqQuery {
				for _, v := range val {
					apiUrlQuery.Add(paramName, v)
				}
			}
			apiUrl.RawQuery = apiUrlQuery.Encode()

			client := urlfetch.Client(ctx)
			resp, err := client.Get(apiUrl.String())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			var data map[string]interface{}
			err = json.Unmarshal(buf.Bytes(), &data)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			context["data"] = data
		}

		html, err := templ.Exec(context)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write([]byte(html))
	}

	if p.ForceSSL {
		r.GET(path, HTTPSMiddleware(handleFunc))
	} else {
		r.GET(path, handleFunc)
	}

	return err
}

var (
	regex = regexp.MustCompile(`\$(\w+)`)
)

func (p *Pages) RenderRoute(layout *Component, routes []*Route) (map[string]interface{}, *raymond.Template, string, error) {
	var ctx = map[string]interface{}{}
	var body = layout.Template.Clone()
	var apiUri string
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

		if len(route.Api) > 0 {
			apiUri = route.Api
		}

		if route.Page != nil {
			for k, v := range route.Page {
				ctx[k] = v
			}
		}

		if component, ok := p.Components[route.Component]; ok {
			body.RegisterPartial(outlet, component.Raw)
		} else {
			return ctx, body, apiUri, errors.New("component " + route.Component + " doesn't exist")
		}


	}

	return ctx, body, apiUri, nil
}
